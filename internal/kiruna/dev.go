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
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
)

// Debouncer to handle event batching
type Debouncer struct {
	mu       sync.Mutex
	timer    *time.Timer
	events   []fsnotify.Event
	duration time.Duration
	callback func(events []fsnotify.Event)
}

func NewDebouncer(duration time.Duration, callback func(events []fsnotify.Event)) *Debouncer {
	return &Debouncer{
		duration: duration,
		callback: callback,
	}
}

func (d *Debouncer) AddEvent(event fsnotify.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.events = append(d.events, event)

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(d.duration, func() {
		d.mu.Lock()
		defer d.mu.Unlock()

		if len(d.events) > 0 {
			d.callback(d.events)
			d.events = nil
		}
	})
}

var (
	naiveIgnoreDirPatterns = [4]string{"**/.git", "**/node_modules", "dist/bin", distKirunaDir}
	ignoredDirPatterns     = []string{}
	ignoredFilePatterns    = []string{}
	defaultWatchedFile     = WatchedFile{}
	defaultWatchedFiles    = []WatchedFile{}
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

	KirunaEnv.setModeToDev()

	c.killPriorPID()

	// take a breather for prior process to clean up
	// not sure why needed, but it allows same port to be used
	time.Sleep(10 * time.Millisecond)

	// Warm port right away, in case default is unavailable
	// Also, env needs to be set in this scope
	MustGetPort()

	// Set refresh server port
	if freePort, err := getFreePort(defaultFreePort); err == nil {
		KirunaEnv.setRefreshServerPort(freePort)
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
		c.mustSetupWatcher(nil)
		return
	}

	c.Logger.Infof("initializing sidecar refresh server on port %d", KirunaEnv.getRefreshServerPort())

	manager := NewClientManager()
	go manager.start()
	go c.mustSetupWatcher(manager)

	mux := http.NewServeMux()

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		sseHandler(manager)(w, r)
	})

	mux.HandleFunc("/get-refresh-script-inner", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/javascript")
		w.Write([]byte(GetRefreshScriptInner(KirunaEnv.getRefreshServerPort())))
	})

	if err := http.ListenAndServe(":"+strconv.Itoa(KirunaEnv.getRefreshServerPort()), mux); err != nil {
		errMsg := fmt.Sprintf("error: failed to start refresh server: %v", err)
		c.Logger.Error(errMsg)
		panic(errMsg)
	}
}

func (c *Config) mustSetupWatcher(manager *ClientManager) {
	defer c.mustKillAppDev()
	cleanRootDir := c.getCleanRootDir()

	for _, p := range naiveIgnoreDirPatterns {
		ignoredDirPatterns = append(ignoredDirPatterns, filepath.Join(cleanRootDir, p))
	}
	for _, p := range c.DevConfig.IgnorePatterns.Dirs {
		ignoredDirPatterns = append(ignoredDirPatterns, filepath.Join(cleanRootDir, p))
	}
	for _, p := range c.DevConfig.IgnorePatterns.Files {
		ignoredFilePatterns = append(ignoredFilePatterns, filepath.Join(cleanRootDir, p))
	}

	// Loop through all WatchedFiles...
	for i, wfc := range c.DevConfig.WatchedFiles {
		// and make each WatchedFile's Pattern relative to cleanRootDir...
		c.DevConfig.WatchedFiles[i].Pattern = filepath.Join(cleanRootDir, wfc.Pattern)

		// then loop through such WatchedFile's OnChangeCallbacks...
		for j, oc := range wfc.OnChangeCallbacks {
			// and make each such OnChangeCallback's ExcludedPatterns also relative to cleanRootDir
			for k, p := range oc.ExcludedPatterns {
				c.DevConfig.WatchedFiles[i].OnChangeCallbacks[j].ExcludedPatterns[k] = filepath.Join(cleanRootDir, p)
			}
		}
	}

	defaultWatchedFiles = append(defaultWatchedFiles, WatchedFile{
		Pattern:    filepath.Join(cleanRootDir, "static/{public,private}/**/*"),
		RestartApp: true,
	})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		errMsg := fmt.Sprintf("error: failed to create watcher: %v", err)
		c.Logger.Error(errMsg)
		panic(errMsg)
	}
	defer watcher.Close()
	err = c.addDirs(watcher, c.getCleanRootDir())
	if err != nil {
		errMsg := fmt.Sprintf("error: failed to add directories to watcher: %v", err)
		c.Logger.Error(errMsg)
		panic(errMsg)
	}
	done := make(chan bool)
	go func() {
		c.mustKillAppDev()
		err := c.compileBinary()
		if err != nil {
			c.Logger.Errorf("error: failed to build app: %v", err)
		}
		c.mustStartAppDev()
		c.mustHandleWatcherEmissions(manager, watcher)
	}()
	<-done
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan RefreshFilePayload),
	}
}

type ChangeType string

const (
	ChangeTypeNormalCSS   ChangeType = "normal"
	ChangeTypeCriticalCSS ChangeType = "critical"
	ChangeTypeOther       ChangeType = "other"
	ChangeTypeRebuilding  ChangeType = "rebuilding"
)

type Base64 = string

type RefreshFilePayload struct {
	ChangeType   ChangeType `json:"changeType"`
	CriticalCSS  Base64     `json:"criticalCss"`
	NormalCSSURL string     `json:"normalCssUrl"`
	At           time.Time  `json:"at"`
}

// Client represents a single SSE connection
type Client struct {
	id     string
	notify chan<- RefreshFilePayload
}

// ClientManager manages all SSE clients
type ClientManager struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan RefreshFilePayload
}

// Start the manager to handle clients and broadcasting
func (manager *ClientManager) start() {
	for {
		select {
		case client := <-manager.register:
			manager.clients[client] = true
		case client := <-manager.unregister:
			if _, ok := manager.clients[client]; ok {
				delete(manager.clients, client)
				close(client.notify)
			}
		case msg := <-manager.broadcast:
			for client := range manager.clients {
				client.notify <- msg
			}
		}
	}
}

var (
	lastBuildCmd  *exec.Cmd
	buildCmdMutex sync.Mutex
)

func (c *Config) mustKillAppDev() {
	buildCmdMutex.Lock()
	defer buildCmdMutex.Unlock()

	if lastBuildCmd != nil {
		if err := lastBuildCmd.Process.Kill(); err != nil {
			errMsg := fmt.Sprintf(
				"error: failed to kill running app with pid %d: %v",
				lastBuildCmd.Process.Pid,
				err,
			)
			c.Logger.Error(errMsg)
			panic(errMsg)
		} else {
			c.Logger.Infof("killed app with pid %d", lastBuildCmd.Process.Pid)

			if err := c.deletePIDFile(); err != nil {
				c.Logger.Errorf("error: failed to delete PID file: %v", err)
			}

			lastBuildCmd = nil
		}
	}
}

func (c *Config) addDirs(watcher *fsnotify.Watcher, path string) error {
	return filepath.Walk(path, func(walkedPath string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking path: %v", err)
		}
		if info.IsDir() {
			if c.getIsIgnored(walkedPath, &ignoredDirPatterns) {
				return filepath.SkipDir
			}
			err := watcher.Add(walkedPath)
			if err != nil {
				return fmt.Errorf("error adding directory to watcher: %v", err)
			}
		}
		return nil
	})
}

func (c *Config) mustStartAppDev() {
	buildCmdMutex.Lock()
	defer buildCmdMutex.Unlock()

	buildDest := filepath.Join(c.getCleanRootDir(), "dist/bin/main")
	lastBuildCmd = exec.Command(buildDest)
	lastBuildCmd.Stdout = os.Stdout
	lastBuildCmd.Stderr = os.Stderr
	if err := lastBuildCmd.Start(); err != nil {
		errMsg := fmt.Sprintf("error: failed to start app: %v", err)
		c.Logger.Error(errMsg)
		panic(errMsg)
	}
	c.Logger.Infof("app started with pid %d", lastBuildCmd.Process.Pid)
	if err := c.writePIDFile(lastBuildCmd.Process.Pid); err != nil {
		c.Logger.Errorf("error: failed to write PID file: %v", err)
	}
}

func (c *Config) mustHandleWatcherEmissions(manager *ClientManager, watcher *fsnotify.Watcher) {
	debouncer := NewDebouncer(30*time.Millisecond, func(events []fsnotify.Event) {
		c.processBatchedEvents(manager, watcher, events)
	})

	for {
		select {
		case evt := <-watcher.Events:
			debouncer.AddEvent(evt)
		case err := <-watcher.Errors:
			c.Logger.Errorf("watcher error: %v", err)
		}
	}
}

func (c *Config) processBatchedEvents(manager *ClientManager, watcher *fsnotify.Watcher, events []fsnotify.Event) {
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
				if err := c.addDirs(watcher, evt.Name); err != nil {
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
			wfc = &defaultWatchedFile
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
		manager.broadcast <- RefreshFilePayload{
			ChangeType: ChangeTypeRebuilding,
		}
	}

	for _, evtDetails := range relevantFileChanges {
		err := c.mustHandleFileChange(manager, evtDetails, hasMultipleEvents)
		if err != nil {
			c.Logger.Errorf("error: failed to handle file change: %v", err)
		}
	}

	if hasMultipleEvents {
		if isGoOrNeedsHardReloadEvenIfNonGo {
			c.mustKillAndRestart()
			return
		}
		c.mustReloadBroadcast(
			manager,
			RefreshFilePayload{
				ChangeType: ChangeTypeOther,
			},
		)
	}
}

func (c *Config) getIsIgnored(path string, ignoredPatterns *[]string) bool {
	for _, pattern := range *ignoredPatterns {
		if c.getIsMatch(pattern, path) {
			return true
		}
	}
	return false
}

func (c *Config) getEvtDetails(evt fsnotify.Event) *EvtDetails {
	isCssSimple := filepath.Ext(evt.Name) == ".css"
	isCriticalCSS := isCssSimple && c.getIsCssEvtType(evt, ChangeTypeCriticalCSS)
	isNormalCSS := isCssSimple && c.getIsCssEvtType(evt, ChangeTypeNormalCSS)
	isKirunaCSS := isCriticalCSS || isNormalCSS

	var matchingWatchedFile *WatchedFile

	for _, wfc := range c.DevConfig.WatchedFiles {
		isMatch := c.getIsMatch(wfc.Pattern, evt.Name)
		if isMatch {
			matchingWatchedFile = &wfc
			break
		}
	}

	if matchingWatchedFile == nil {
		for _, wfc := range defaultWatchedFiles {
			isMatch := c.getIsMatch(wfc.Pattern, evt.Name)
			if isMatch {
				matchingWatchedFile = &wfc
				break
			}
		}
	}

	isGo := filepath.Ext(evt.Name) == ".go"
	if isGo && matchingWatchedFile != nil && matchingWatchedFile.TreatAsNonGo {
		isGo = false
	}

	isOther := !isGo && !isKirunaCSS

	isIgnored := c.getIsIgnored(evt.Name, &ignoredFilePatterns)
	if isOther && matchingWatchedFile == nil {
		isIgnored = true
	}

	return &EvtDetails{
		evt:                 &evt,
		isOther:             isOther,
		isKirunaCSS:         isKirunaCSS,
		isGo:                isGo,
		isIgnored:           isIgnored,
		isCriticalCSS:       isCriticalCSS,
		isNormalCSS:         isNormalCSS,
		wfc:                 matchingWatchedFile,
		isNonEmptyCHMODOnly: c.getIsNonEmptyCHMODOnly(evt),
	}
}

func (c *Config) mustHandleFileChange(
	manager *ClientManager,
	evtDetails *EvtDetails,
	isPartOfBatch bool,
) error {
	wfc := evtDetails.wfc
	if wfc == nil {
		wfc = &defaultWatchedFile
	}

	if !c.DevConfig.ServerOnly && !wfc.SkipRebuildingNotification && !evtDetails.isKirunaCSS && !isPartOfBatch {
		manager.broadcast <- RefreshFilePayload{
			ChangeType: ChangeTypeRebuilding,
		}
	}

	if evtDetails.isGo || wfc.RecompileBinary {
		c.Logger.Infof("recompiling binary")
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

	needsHardReloadEvenIfNonGo := wfc.RecompileBinary || wfc.RestartApp

	if (evtDetails.isGo || needsHardReloadEvenIfNonGo) && !isPartOfBatch {
		c.mustKillAndRestart()
	}

	if c.DevConfig.ServerOnly || isPartOfBatch {
		return nil
	}

	if !evtDetails.isKirunaCSS || needsHardReloadEvenIfNonGo {
		c.Logger.Infof("hard reloading browser")
		c.mustReloadBroadcast(
			manager,
			RefreshFilePayload{
				ChangeType: ChangeTypeOther,
			},
		)
		return nil
	}
	// At this point, we know it's a CSS file

	cssType := ChangeTypeNormalCSS
	if evtDetails.isCriticalCSS {
		cssType = ChangeTypeCriticalCSS
	}

	c.Logger.Infof("hot reloading browser")
	c.mustReloadBroadcast(
		manager,
		RefreshFilePayload{
			ChangeType: cssType,

			// These must be called AFTER ProcessCSS
			CriticalCSS:  base64.StdEncoding.EncodeToString([]byte(c.GetCriticalCSS())),
			NormalCSSURL: c.GetStyleSheetURL(),
		},
	)

	return nil
}

func (c *Config) getIsMatch(pattern string, path string) bool {
	combined := pattern + path

	if hit, isCached := cache.matchResults.Load(combined); isCached {
		return hit
	}

	normalizedPath := filepath.ToSlash(path)

	matches, err := doublestar.Match(pattern, normalizedPath)
	if err != nil {
		c.Logger.Errorf("error: failed to match file: %v", err)
		return false
	}

	actualValue, _ := cache.matchResults.LoadOrStore(combined, matches)
	return actualValue
}

type EvtDetails struct {
	evt                 *fsnotify.Event
	isIgnored           bool
	isGo                bool
	isOther             bool
	isCriticalCSS       bool
	isNormalCSS         bool
	isKirunaCSS         bool
	wfc                 *WatchedFile
	isNonEmptyCHMODOnly bool
}

func (c *Config) getIsCssEvtType(evt fsnotify.Event, cssType ChangeType) bool {
	return strings.HasPrefix(evt.Name, filepath.Join(c.getCleanRootDir(), "styles/"+string(cssType)))
}

func (c *Config) getIsNonEmptyCHMODOnly(evt fsnotify.Event) bool {
	isSolelyCHMOD := !evt.Has(fsnotify.Write) && !evt.Has(fsnotify.Create) && !evt.Has(fsnotify.Remove) && !evt.Has(fsnotify.Rename)
	return isSolelyCHMOD && !c.getIsEmptyFile(evt)
}

func (c *Config) callback(wfc *WatchedFile, evtDetails *EvtDetails) error {
	if evtDetails.isGo {
		return c.compileBinary()
	}

	if evtDetails.isKirunaCSS {
		if wfc.RecompileBinary || wfc.RestartApp {
			return c.runOtherFileBuild(wfc)
		}
		cssType := ChangeTypeNormalCSS
		if evtDetails.isCriticalCSS {
			cssType = ChangeTypeCriticalCSS
		}
		return c.ProcessCSS(string(cssType))
	}

	return c.runOtherFileBuild(wfc)
}

func (c *Config) mustKillAndRestart() {
	c.Logger.Infof("killing and restarting app")
	c.mustKillAppDev()
	c.mustStartAppDev()
}

func (c *Config) mustReloadBroadcast(manager *ClientManager, rfp RefreshFilePayload) {
	if c.waitForAppReadiness() {
		manager.broadcast <- rfp
		return
	}
	errMsg := fmt.Sprintf("error: app never became ready: %v", rfp.ChangeType)
	c.Logger.Error(errMsg)
	panic(errMsg)
}

func (c *Config) getIsEmptyFile(evt fsnotify.Event) bool {
	file, err := os.Open(evt.Name)
	if err != nil {
		c.Logger.Errorf("error: failed to open file: %v", err)
		return false
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		c.Logger.Errorf("error: failed to get file stats: %v", err)
		return false
	}
	return stat.Size() == 0
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

const (
	maxReadinessAttempts = 100
	baseReadinessDelay   = 20 * time.Millisecond
)

func (c *Config) waitForAppReadiness() bool {
	for attempts := 0; attempts < maxReadinessAttempts; attempts++ {
		url := fmt.Sprintf(
			"http://localhost:%d%s",
			MustGetPort(),
			c.DevConfig.HealthcheckEndpoint,
		)

		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			return true
		}

		additionalDelay := time.Duration(attempts) * baseReadinessDelay
		time.Sleep(baseReadinessDelay + additionalDelay)
	}
	return false
}
