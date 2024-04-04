package dev

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sjc5/kiruna/internal/buildtime"
	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/runtime"
	"github.com/sjc5/kiruna/internal/util"
)

func setupWatcher(manager *ClientManager, config *common.Config) {
	defer killAppDev()
	setupExtKeys(config)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		util.Log.Panicf("error: failed to create watcher: %v", err)
	}
	defer watcher.Close()
	err = addDirs(config, watcher, config.GetCleanRootDir())
	if err != nil {
		util.Log.Panicf("error: failed to add directories to watcher: %v", err)
	}
	done := make(chan bool)
	go func() {
		killAppDev()
		mustBuild(config)
		startAppDev(config)
		handleWatcherEmissions(config, manager, watcher)
	}()
	<-done
}

func handleWatcherEmissions(config *common.Config, manager *ClientManager, watcher *fsnotify.Watcher) {
	for {
		select {
		case evt := <-watcher.Events:
			time.Sleep(10 * time.Millisecond) // let the file system settle
			if getIsModifyEvt(evt) {
				evtDetails := getEvtDetails(config, evt)
				if getIsGo(evt) {
					handleGoFileChange(config, manager, evt, evtDetails)
				} else {
					if !config.DevConfig.ServerOnly {
						if evtDetails.isCriticalCss {
							handleCSSFileChange(config, manager, evt, ChangeTypeCriticalCSS)
						} else if evtDetails.isNormalCss {
							handleCSSFileChange(config, manager, evt, ChangeTypeNormalCSS)
						} else if evtDetails.shouldReload {
							handleOtherFileChange(config, manager, evt, evtDetails)
						}
					}
				}
			} else if evt.Has(fsnotify.Create) {
				fileInfo, err := os.Stat(evt.Name)
				if err == nil && fileInfo.IsDir() {
					err := addDirs(config, watcher, evt.Name)
					if err != nil {
						util.Log.Errorf("error: failed to add directory to watcher: %v", err)
						return
					}
				}
			}
			// Only other option is CHMOD, which we don't care about
		case err := <-watcher.Errors:
			util.Log.Errorf("watcher error: %v", err)
		}
	}
}

type sortedOnChangeCallbacks struct {
	stratPre        []common.OnChange
	stratConcurrent []common.OnChange
	stratPost       []common.OnChange
	exists          bool
}

func sortOnChangeCallbacks(onChanges []common.OnChange) sortedOnChangeCallbacks {
	stratPre := []common.OnChange{}
	stratConcurrent := []common.OnChange{}
	stratPost := []common.OnChange{}
	exists := false
	if len(onChanges) == 0 {
		return sortedOnChangeCallbacks{}
	} else {
		exists = true
	}
	for _, o := range onChanges {
		if o.Strategy == common.OnChangeStrategyPre || o.Strategy == "" {
			stratPre = append(stratPre, o)
		}
		if o.Strategy == common.OnChangeStrategyConcurrent {
			stratConcurrent = append(stratConcurrent, o)
		}
		if o.Strategy == common.OnChangeStrategyPost {
			stratPost = append(stratPost, o)
		}
	}
	return sortedOnChangeCallbacks{
		stratPre:        stratPre,
		stratConcurrent: stratConcurrent,
		stratPost:       stratPost,
		exists:          exists,
	}
}

func getIsIgnored(evtName string, excludedFiles []string) bool {
	isIgnored := false
	if len(excludedFiles) > 0 {
		for _, ignoreFile := range excludedFiles {
			if strings.HasSuffix(evtName, ignoreFile) {
				isIgnored = true
				break
			}
		}
	}
	return isIgnored
}

func runConcurrentOnChangeCallbacks(sortedOnChanges *sortedOnChangeCallbacks, evtName string) {
	if len(sortedOnChanges.stratConcurrent) > 0 {
		wg := sync.WaitGroup{}
		wg.Add(len(sortedOnChanges.stratConcurrent))
		for _, o := range sortedOnChanges.stratConcurrent {
			if getIsIgnored(evtName, o.ExcludedFiles) {
				wg.Done()
				continue
			}
			go func(o common.OnChange) {
				defer wg.Done()
				err := o.Func(evtName)
				if err != nil {
					util.Log.Errorf("error running extension callback: %v", err)
				}
			}(o)
		}
		wg.Wait()
	}
}

func simpleRunOnChangeCallbacks(onChanges []common.OnChange, evtName string) {
	for _, o := range onChanges {
		if getIsIgnored(evtName, o.ExcludedFiles) {
			continue
		}
		err := o.Func(evtName)
		if err != nil {
			util.Log.Errorf("error running extension callback: %v", err)
		}
	}
}

func handleGoFileChange(
	config *common.Config,
	manager *ClientManager,
	evt fsnotify.Event,
	evtDetails EvtDetails,
) {
	killAppDev()

	if !config.DevConfig.ServerOnly {
		manager.broadcast <- RefreshFilePayload{
			ChangeType: ChangeTypeRebuilding,
		}
	}

	wfc := (config.DevConfig.WatchedFiles)[evtDetails.complexExtension]
	sortedOnChanges := sortOnChangeCallbacks(wfc.OnChangeCallbacks)

	if sortedOnChanges.exists {
		simpleRunOnChangeCallbacks(sortedOnChanges.stratPre, evt.Name)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			mustBuild(config)
		}()

		runConcurrentOnChangeCallbacks(&sortedOnChanges, evt.Name)
		wg.Wait()
	} else {
		mustBuild(config)
	}

	simpleRunOnChangeCallbacks(sortedOnChanges.stratPost, evt.Name)

	startAppDev(config)

	if !config.DevConfig.ServerOnly {
		util.Log.Infof("hard reloading browser")
		reloadBroadcast(
			config,
			manager,
			RefreshFilePayload{
				ChangeType: ChangeTypeOther,
			},
		)
	}
}

func mustProcessCSS(config *common.Config, cssType ChangeType) {
	err := buildtime.ProcessCSS(config, string(cssType))
	if err != nil {
		util.Log.Panicf("error processing %s CSS: %v", cssType, err)
	}
}

func handleCSSFileChange(
	config *common.Config,
	manager *ClientManager,
	evt fsnotify.Event,
	cssType ChangeType,
) {
	sortedOnChanges := sortOnChangeCallbacks(config.DevConfig.CSSConfig.OnChangeCallbacks)
	if sortedOnChanges.exists {
		simpleRunOnChangeCallbacks(sortedOnChanges.stratPre, evt.Name)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			mustProcessCSS(config, cssType)
		}()

		runConcurrentOnChangeCallbacks(&sortedOnChanges, evt.Name)
		wg.Wait()
	} else {
		mustProcessCSS(config, cssType)
	}
	util.Log.Infof("modified: %s", evt.Name)
	util.Log.Infof("hot reloading browser")
	reloadBroadcast(
		config,
		manager,
		RefreshFilePayload{
			ChangeType: cssType,

			// These must be called AFTER ProcessCSS
			CriticalCSS:  runtime.GetCriticalCSS(config),
			NormalCSSURL: runtime.GetStyleSheetURL(config),
		},
	)
}

func handleOtherFileChange(config *common.Config, manager *ClientManager, evt fsnotify.Event, evtDetails EvtDetails) {
	wfc := (config.DevConfig.WatchedFiles)[evtDetails.complexExtension]

	if wfc.RecompileBinary || wfc.RestartApp {
		util.Log.Infof("killing running app")
		killAppDev()
	}

	if !wfc.SkipRebuildingNotification {
		manager.broadcast <- RefreshFilePayload{
			ChangeType: ChangeTypeRebuilding,
		}
		util.Log.Infof("modified: %s, rebuilding static assets", evt.Name)
	}
	sortedOnChanges := sortOnChangeCallbacks(wfc.OnChangeCallbacks)

	if sortedOnChanges.exists {
		simpleRunOnChangeCallbacks(sortedOnChanges.stratPre, evt.Name)

		// You would do this if you want to trigger a process that itself
		// saves to a file (evt.g., to styles/critical/whatever.css) that
		// would in turn trigger the rebuild in another run
		if wfc.RunOnChangeOnly {
			util.Log.Infof("onchange complete, work is done")
			return
		}

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()

			// This is different than inside of handleGoFileChange, because here we
			// assume we need to re-run other build steps too, not just recompile Go.
			// Also, we don't necessarily recompile Go here (we only necessarily) run
			// the other build steps. We only recompile Go if wfc.RecompileBinary is true.
			buildtime.MustSetupNewBuild(config)
			buildtime.MustRunPrecompileTasks(config)
			if wfc.RecompileBinary {
				buildtime.MustRecompileBinary(config)
			}
		}()

		runConcurrentOnChangeCallbacks(&sortedOnChanges, evt.Name)
		wg.Wait()
	} else {
		buildtime.MustSetupNewBuild(config)
		buildtime.MustRunPrecompileTasks(config)
		if wfc.RecompileBinary {
			buildtime.MustRecompileBinary(config)
		}
	}

	if wfc.RecompileBinary {
		util.Log.Infof("doing a full recompile")
	} else {
		util.Log.Infof("skipping full recompile")
	}

	if wfc.RecompileBinary || wfc.RestartApp {
		util.Log.Infof("restarting app")
		startAppDev(config)
	}
	util.Log.Infof("hard reloading browser")
	reloadBroadcast(
		config,
		manager,
		RefreshFilePayload{
			ChangeType: ChangeTypeOther,
		},
	)
}

func reloadBroadcast(config *common.Config, manager *ClientManager, rfp RefreshFilePayload) {
	if waitForAppReadiness(config) {
		manager.broadcast <- rfp
		return
	}
	util.Log.Panicf("error: app never became ready: %v", rfp.ChangeType)
}

var extKeys []string = nil

func setupExtKeys(config *common.Config) {
	if extKeys == nil {
		extKeys = []string{}
		if config.DevConfig.WatchedFiles != nil {
			for k := range config.DevConfig.WatchedFiles {
				extKeys = append(extKeys, k)
			}
		}
	}
}

var standardIgnoreDirList = []string{".git", "node_modules", "dist/bin", "dist/kiruna"}

func getStandardIgnoreDirList(config *common.Config) []string {
	if len(standardIgnoreDirList) > 0 {
		return standardIgnoreDirList
	}
	distRelativeToRootDir := filepath.Join(config.GetCleanRootDir(), "dist")
	execDir := util.GetExecDir()
	nodeModules := filepath.Join(execDir, "node_modules")
	gitDir := filepath.Join(execDir, ".git")
	return append(standardIgnoreDirList, distRelativeToRootDir, nodeModules, gitDir)
}

func isDirOrChildOfDir(dir string, parent string) bool {
	return strings.HasPrefix(filepath.Clean(dir), filepath.Clean(parent))
}

func addDirs(config *common.Config, watcher *fsnotify.Watcher, path string) error {
	return filepath.Walk(path, func(walkedPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			for _, ignoreDir := range append(getStandardIgnoreDirList(config), config.DevConfig.IgnoreDirs...) {
				ignoreDirRelative := filepath.Join(config.GetCleanRootDir(), ignoreDir)
				if isDirOrChildOfDir(walkedPath, ignoreDirRelative) {
					return filepath.SkipDir
				}
			}
			err := watcher.Add(walkedPath)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func getIsModifyEvt(evt fsnotify.Event) bool {
	return evt.Has(fsnotify.Write) || evt.Has(fsnotify.Remove) || evt.Has(fsnotify.Rename)
}

type EvtDetails struct {
	isCriticalCss    bool
	isNormalCss      bool
	shouldReload     bool
	complexExtension string
}

func getEvtDetails(config *common.Config, evt fsnotify.Event) EvtDetails {
	isCssSimple := getIsCss(evt)
	isCriticalCss := isCssSimple && getIsCssEvtType(config, evt, ChangeTypeCriticalCSS)
	isNormalCss := isCssSimple && getIsCssEvtType(config, evt, ChangeTypeNormalCSS)
	isCss := isCriticalCss || isNormalCss
	shouldReload := false
	var complexExtension string
	if !isCss {
		for _, ext := range extKeys {
			if strings.HasSuffix(evt.Name, ext) {
				shouldReload = true
				complexExtension = ext
				break
			}
		}
	}
	return EvtDetails{
		isCriticalCss:    isCriticalCss,
		isNormalCss:      isNormalCss,
		shouldReload:     shouldReload,
		complexExtension: complexExtension,
	}
}

func getIsCss(evt fsnotify.Event) bool {
	return filepath.Ext(evt.Name) == ".css"
}

func getIsGo(evt fsnotify.Event) bool {
	return filepath.Ext(evt.Name) == ".go"
}

func getIsCssEvtType(config *common.Config, evt fsnotify.Event, cssType ChangeType) bool {
	return strings.HasPrefix(evt.Name, filepath.Join(config.GetCleanRootDir(), "styles/"+string(cssType)))
}
