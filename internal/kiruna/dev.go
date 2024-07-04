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
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
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
			}

			c.lastBuildCmd.v = nil
		}
	}
}

func (c *Config) mustStartAppDev() {
	c.lastBuildCmd.mu.Lock()
	defer c.lastBuildCmd.mu.Unlock()

	buildDest := filepath.Join(c.getCleanRootDir(), "dist/bin/main")

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
			isGoOrNeedsHardReloadEvenIfNonGo = wfc.RecompileBinary || wfc.RestartApp
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
		c.manager.broadcast <- refreshFilePayload{
			ChangeType: changeTypeRebuilding,
		}
	}

	wg := sync.WaitGroup{}
	if hasMultipleEvents && isGoOrNeedsHardReloadEvenIfNonGo {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Logger.Infof("killing app")
			c.mustKillAppDev()
		}()
	}

	for _, evtDetails := range relevantFileChanges {
		err := c.mustHandleFileChange(evtDetails, hasMultipleEvents)
		if err != nil {
			c.Logger.Errorf("error: failed to handle file change: %v", err)
		}
	}

	if hasMultipleEvents && isGoOrNeedsHardReloadEvenIfNonGo {
		wg.Wait()
		c.Logger.Infof("restarting app")
		c.mustStartAppDev()
		return
	}

	if hasMultipleEvents {
		c.mustReloadBroadcast(refreshFilePayload{ChangeType: changeTypeOther})
	}
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

	needsHardReloadEvenIfNonGo := wfc.RecompileBinary || wfc.RestartApp

	if evtDetails.isGo || wfc.RecompileBinary {
		c.Logger.Infof("recompiling binary")
	}

	needsKillAndRestart := (evtDetails.isGo || needsHardReloadEvenIfNonGo) && !isPartOfBatch

	wg := sync.WaitGroup{}
	if needsKillAndRestart {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Logger.Infof("killing app")
			c.mustKillAppDev()
		}()
	}

	sortedOnChanges := sortOnChangeCallbacks(wfc.OnChangeCallbacks)

	var buildErr error

	if sortedOnChanges.exists {
		go func() {
			c.runConcurrentOnChangeCallbacks(&sortedOnChanges.stratConcurrentNoWait, evtDetails.evt.Name, false)
		}()

		c.simpleRunOnChangeCallbacks(&sortedOnChanges.stratPre, evtDetails.evt.Name)

		if wfc.RunOnChangeOnly {
			c.Logger.Infof("ran applicable onChange callbacks")
			return nil
		}

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			buildErr = c.callback(wfc, evtDetails)
		}()

		c.runConcurrentOnChangeCallbacks(&sortedOnChanges.stratConcurrent, evtDetails.evt.Name, true)
		wg.Wait()
	} else {
		buildErr = c.callback(wfc, evtDetails)
	}

	if buildErr != nil {
		c.Logger.Errorf("error: failed to build: %v", buildErr)
		return buildErr
	}

	c.simpleRunOnChangeCallbacks(&sortedOnChanges.stratPost, evtDetails.evt.Name)

	if needsKillAndRestart {
		wg.Wait()
		c.Logger.Infof("restarting app")
		c.mustStartAppDev()
	}

	if c.DevConfig.ServerOnly || isPartOfBatch {
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
		if wfc.RecompileBinary || wfc.RestartApp {
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
