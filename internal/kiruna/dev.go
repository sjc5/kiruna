package ik

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/sync/errgroup"
)

const (
	healthCheckWarningA = `WARNING: No healthcheck endpoint found, setting to "/".`
	healthCheckWarningB = `To set this explicitly, use the "HealthcheckEndpoint" field in your dev config.`
	healthCheckWarning  = healthCheckWarningA + "\n" + healthCheckWarningB
)

func (c *Config) MustStartDev() {
	// Short circuit if no dev config
	if c.DevConfig == nil {
		errMsg := "error: no dev config found"
		c.Logger.Error(errMsg)
		panic(errMsg)
	}

	if len(c.DevConfig.HealthcheckEndpoint) == 0 {
		c.Logger.Warning(healthCheckWarning)
		c.DevConfig.HealthcheckEndpoint = "/"
	}

	setModeToDev()

	c.devInitOnce()

	c.killPriorPID()

	// take a breather for prior process to clean up
	// not sure why needed, but it allows same port to be used
	time.Sleep(10 * time.Millisecond)

	// Warm port right away, in case default is unavailable
	// Also, env needs to be set in this scope
	MustGetPort()

	// Set refresh server port
	if freePort, err := getFreePort(defaultFreePort); err == nil {
		setRefreshServerPort(freePort)
	} else {
		c.Logger.Errorf("error: failed to get free port for refresh server: %v", err)
		panic(err)
	}

	err := c.Build(false, false)
	if err != nil {
		errMsg := fmt.Sprintf("error: failed to build app: %v", err)
		c.Logger.Error(errMsg)
		panic(errMsg)
	}

	if c.DevConfig.ServerOnly {
		c.mustSetupWatcher()
		return
	}

	c.Logger.Infof("initializing sidecar refresh server on port %d", getRefreshServerPort())

	go c.manager.start()
	go c.mustSetupWatcher()

	mux := http.NewServeMux()

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		sseHandler(c.manager)(w, r)
	})

	mux.HandleFunc("/get-refresh-script-inner", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/javascript")
		w.Write([]byte(GetRefreshScriptInner(getRefreshServerPort())))
	})

	if err := http.ListenAndServe(":"+strconv.Itoa(getRefreshServerPort()), mux); err != nil {
		errMsg := fmt.Sprintf("error: failed to start refresh server: %v", err)
		c.Logger.Error(errMsg)
		panic(errMsg)
	}
}

func (c *Config) mustKillAppDev() {
	c.lastBuildCmd.mu.Lock()
	defer c.lastBuildCmd.mu.Unlock()

	if c.lastBuildCmd.v != nil {
		if err := c.lastBuildCmd.v.Process.Kill(); err != nil {
			errMsg := fmt.Sprintf(
				"error: failed to kill running app with pid %d: %v",
				c.lastBuildCmd.v.Process.Pid,
				err,
			)
			c.Logger.Error(errMsg)
			panic(errMsg)
		} else {
			c.Logger.Infof("killed app with pid %d", c.lastBuildCmd.v.Process.Pid)

			if err := c.deletePIDFile(); err != nil {
				c.Logger.Errorf("error: failed to delete PID file: %v", err)
				// now just move on, not the end of the world
			}

			c.lastBuildCmd.v = nil
		}
	}
}

func (c *Config) mustStartAppDev() {
	c.lastBuildCmd.mu.Lock()
	defer c.lastBuildCmd.mu.Unlock()

	buildDest := filepath.Join(c.getCleanRootDir(), binOutPath)

	c.lastBuildCmd.v = exec.Command(buildDest)
	c.lastBuildCmd.v.Stdout = os.Stdout
	c.lastBuildCmd.v.Stderr = os.Stderr

	if err := c.lastBuildCmd.v.Start(); err != nil {
		errMsg := fmt.Sprintf("error: failed to start app: %v", err)
		c.Logger.Error(errMsg)
		panic(errMsg)
	}

	c.Logger.Infof("app started with pid %d", c.lastBuildCmd.v.Process.Pid)

	if err := c.writePIDFile(c.lastBuildCmd.v.Process.Pid); err != nil {
		c.Logger.Errorf("error: failed to write PID file: %v", err)
		// now just move on, not the end of the world
	}
}

func (c *Config) mustHandleWatcherEmissions() {
	debouncer := newDebouncer(30*time.Millisecond, func(events []fsnotify.Event) {
		c.processBatchedEvents(events)
	})

	for {
		select {
		case evt := <-c.watcher.Events:
			debouncer.addEvent(evt)
		case err := <-c.watcher.Errors:
			c.Logger.Errorf("watcher error: %v", err)
		}
	}
}

func (c *Config) processBatchedEvents(events []fsnotify.Event) {
	fileChanges := make(map[string]fsnotify.Event)
	for _, evt := range events {
		fileChanges[evt.Name] = evt
	}

	relevantFileChanges := make(map[string]*EvtDetails)

	wfcsAlreadyHandled := make(map[string]bool)
	isGoOrNeedsHardReloadEvenIfNonGo := false

	for _, evt := range fileChanges {
		fileInfo, _ := os.Stat(evt.Name) // no need to check error, because we want to process either way
		if fileInfo != nil && fileInfo.IsDir() {
			if evt.Has(fsnotify.Create) || evt.Has(fsnotify.Rename) {
				if err := c.addDirs(evt.Name); err != nil {
					c.Logger.Errorf("error: failed to add directory to watcher: %v", err)
					continue
				}
			}
			continue
		}

		evtDetails := c.getEvtDetails(evt)
		if evtDetails.isIgnored {
			continue
		}

		wfc := evtDetails.wfc
		if wfc == nil {
			wfc = c.defaultWatchedFile
		}

		if _, alreadyHandled := wfcsAlreadyHandled[wfc.Pattern]; alreadyHandled {
			continue
		}

		wfcsAlreadyHandled[wfc.Pattern] = true

		if !isGoOrNeedsHardReloadEvenIfNonGo {
			isGoOrNeedsHardReloadEvenIfNonGo = evtDetails.isGo
		}
		if !isGoOrNeedsHardReloadEvenIfNonGo {
			isGoOrNeedsHardReloadEvenIfNonGo = getNeedsHardReloadEvenIfNonGo(wfc)
		}

		relevantFileChanges[evt.Name] = evtDetails
	}

	if len(relevantFileChanges) == 0 {
		return
	}

	hasMultipleEvents := len(relevantFileChanges) > 1

	if !hasMultipleEvents {
		var evtName string
		for evtName = range relevantFileChanges {
			break
		}
		if relevantFileChanges[evtName].isNonEmptyCHMODOnly {
			return
		}
	}

	if hasMultipleEvents {
		allEvtsAreNonEmptyCHMODOnly := true

		for _, evtDetails := range relevantFileChanges {
			if evtDetails.isNonEmptyCHMODOnly {
				continue
			} else {
				allEvtsAreNonEmptyCHMODOnly = false
				break
			}
		}

		if allEvtsAreNonEmptyCHMODOnly {
			return
		}

		c.manager.broadcast <- refreshFilePayload{
			ChangeType: changeTypeRebuilding,
		}
	}

	eg := errgroup.Group{}
	if hasMultipleEvents && isGoOrNeedsHardReloadEvenIfNonGo {
		eg.Go(func() error {
			c.Logger.Infof("killing app")
			c.mustKillAppDev()
			return nil
		})
	}

	for _, evtDetails := range relevantFileChanges {
		c.Logger.Info(evtDetails.evt.String())

		err := c.mustHandleFileChange(evtDetails, hasMultipleEvents)
		if err != nil {
			c.Logger.Errorf("error: failed to handle file change: %v", err)
			return
		}
	}

	if hasMultipleEvents && isGoOrNeedsHardReloadEvenIfNonGo {
		if err := eg.Wait(); err != nil {
			c.Logger.Errorf("error: failed to kill app: %v", err)
			return
		}
		c.Logger.Infof("restarting app")
		c.mustStartAppDev()
		return
	}

	if hasMultipleEvents {
		c.mustReloadBroadcast(refreshFilePayload{ChangeType: changeTypeOther})
	}
}

func getNeedsHardReloadEvenIfNonGo(wfc *WatchedFile) bool {
	return wfc.RecompileBinary || wfc.RestartApp
}

func (c *Config) mustHandleFileChange(
	evtDetails *EvtDetails,
	isPartOfBatch bool,
) error {
	wfc := evtDetails.wfc
	if wfc == nil {
		wfc = c.defaultWatchedFile
	}

	if !c.DevConfig.ServerOnly && !wfc.SkipRebuildingNotification && !evtDetails.isKirunaCSS && !isPartOfBatch {
		c.manager.broadcast <- refreshFilePayload{
			ChangeType: changeTypeRebuilding,
		}
	}

	needsHardReloadEvenIfNonGo := getNeedsHardReloadEvenIfNonGo(wfc)

	if evtDetails.isGo || wfc.RecompileBinary {
		c.Logger.Infof("recompiling binary")
	}

	needsKillAndRestart := (evtDetails.isGo || needsHardReloadEvenIfNonGo) && !isPartOfBatch

	killAndRestartEG := errgroup.Group{}
	if needsKillAndRestart {
		killAndRestartEG.Go(func() error {
			c.Logger.Infof("killing app")
			c.mustKillAppDev()
			return nil
		})
	}

	sortedOnChanges := sortOnChangeCallbacks(wfc.OnChangeCallbacks)

	if sortedOnChanges.exists {
		// Kiruna has no control over error handling for "no-wait" callbacks.
		// They might not even be finished until after Kiruna has already
		// restarted the app (in fact, that's the point).
		go func() {
			_ = c.runConcurrentOnChangeCallbacks(&sortedOnChanges.stratConcurrentNoWait, evtDetails.evt.Name, false)
		}()

		if err := c.simpleRunOnChangeCallbacks(&sortedOnChanges.stratPre, evtDetails.evt.Name); err != nil {
			c.Logger.Errorf("error: failed to build: %v", err)
			return err
		}

		if wfc.RunOnChangeOnly {
			c.Logger.Infof("ran applicable onChange callbacks")
			return nil
		}

		eg := errgroup.Group{}
		eg.Go(func() error {
			return c.callback(wfc, evtDetails)
		})

		if err := c.runConcurrentOnChangeCallbacks(&sortedOnChanges.stratConcurrent, evtDetails.evt.Name, true); err != nil {
			c.Logger.Errorf("error: failed to build: %v", err)
			return err
		}

		if err := eg.Wait(); err != nil {
			c.Logger.Errorf("error: failed to build: %v", err)
			return err
		}
	} else {
		if err := c.callback(wfc, evtDetails); err != nil {
			c.Logger.Errorf("error: failed to build: %v", err)
			return err
		}
	}

	if err := c.simpleRunOnChangeCallbacks(&sortedOnChanges.stratPost, evtDetails.evt.Name); err != nil {
		c.Logger.Errorf("error: failed to build: %v", err)
		return err
	}

	if needsKillAndRestart {
		if err := killAndRestartEG.Wait(); err != nil {
			c.Logger.Errorf("error: failed to kill app: %v", err)
			return err
		}
		c.Logger.Infof("restarting app")
		c.mustStartAppDev()
	}

	if c.DevConfig.ServerOnly || isPartOfBatch {
		return nil
	}

	if wfc.RunClientDefinedRevalidateFunc {
		c.Logger.Infof("revalidating browser")
		c.mustReloadBroadcast(refreshFilePayload{ChangeType: changeTypeRevalidate})
		return nil
	}

	if !evtDetails.isKirunaCSS || needsHardReloadEvenIfNonGo {
		c.Logger.Infof("hard reloading browser")
		c.mustReloadBroadcast(refreshFilePayload{ChangeType: changeTypeOther})
		return nil
	}
	// At this point, we know it's a CSS file

	cssType := changeTypeNormalCSS
	if evtDetails.isCriticalCSS {
		cssType = changeTypeCriticalCSS
	}

	c.Logger.Infof("hot reloading browser")
	c.mustReloadBroadcast(refreshFilePayload{
		ChangeType: cssType,

		// These must be called AFTER ProcessCSS
		CriticalCSS:  base64.StdEncoding.EncodeToString([]byte(c.GetCriticalCSS())),
		NormalCSSURL: c.GetStyleSheetURL(),
	})

	return nil
}

func (c *Config) callback(wfc *WatchedFile, evtDetails *EvtDetails) error {
	if evtDetails.isGo {
		return c.compileBinary()
	}

	if evtDetails.isKirunaCSS {
		if getNeedsHardReloadEvenIfNonGo(wfc) {
			return c.runOtherFileBuild(wfc)
		}
		cssType := changeTypeNormalCSS
		if evtDetails.isCriticalCSS {
			cssType = changeTypeCriticalCSS
		}
		return c.processCSS(string(cssType))
	}

	return c.runOtherFileBuild(wfc)
}

// This is different than inside of handleGoFileChange, because here we
// assume we need to re-run other build steps too, not just recompile Go.
// Also, we don't necessarily recompile Go here (we only necessarily) run
// the other build steps. We only recompile Go if wfc.RecompileBinary is true.
func (c *Config) runOtherFileBuild(wfc *WatchedFile) error {
	err := c.Build(wfc.RecompileBinary, true)
	if err != nil {
		msg := fmt.Sprintf("error: failed to build app: %v", err)
		c.Logger.Error(msg)
		return errors.New(msg)
	}
	return nil
}
