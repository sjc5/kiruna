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

func mustSetupWatcher(manager *ClientManager, config *common.Config) {
	defer mustKillAppDev()
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
		mustKillAppDev()
		err := buildtime.CompileBinary(config)
		if err != nil {
			util.Log.Errorf("error: failed to build app: %v", err)
		}
		mustStartAppDev(config)
		mustHandleWatcherEmissions(config, manager, watcher)
	}()
	<-done
}

func mustHandleWatcherEmissions(config *common.Config, manager *ClientManager, watcher *fsnotify.Watcher) {
	for {
		select {
		case evt := <-watcher.Events:
			time.Sleep(10 * time.Millisecond) // let the file system settle
			if getIsModifyEvt(evt) {
				evtDetails := getEvtDetails(config, evt)
				if getIsGo(evt) {
					mustHandleGoFileChange(config, manager, evt, evtDetails)
				} else {
					if !config.DevConfig.ServerOnly {
						if evtDetails.isCriticalCss {
							mustHandleCSSFileChange(config, manager, evt, ChangeTypeCriticalCSS)
						} else if evtDetails.isNormalCss {
							mustHandleCSSFileChange(config, manager, evt, ChangeTypeNormalCSS)
						} else if evtDetails.shouldReload {
							mustHandleOtherFileChange(config, manager, evt, evtDetails)
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

func mustHandleGoFileChange(
	config *common.Config,
	manager *ClientManager,
	evt fsnotify.Event,
	evtDetails EvtDetails,
) {
	if !config.DevConfig.ServerOnly {
		manager.broadcast <- RefreshFilePayload{
			ChangeType: ChangeTypeRebuilding,
		}
	}

	wfc := (config.DevConfig.WatchedFiles)[evtDetails.complexExtension]
	sortedOnChanges := sortOnChangeCallbacks(wfc.OnChangeCallbacks)

	var buildErr error

	if sortedOnChanges.exists {
		simpleRunOnChangeCallbacks(sortedOnChanges.stratPre, evt.Name)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			buildErr = buildtime.CompileBinary(config)
		}()

		runConcurrentOnChangeCallbacks(&sortedOnChanges, evt.Name)
		wg.Wait()
	} else {
		buildErr = buildtime.CompileBinary(config)
	}

	if buildErr != nil {
		util.Log.Errorf("error: failed to build app: %v", buildErr)
		return
	}

	simpleRunOnChangeCallbacks(sortedOnChanges.stratPost, evt.Name)

	mustKillAndRestart(config)

	if !config.DevConfig.ServerOnly {
		util.Log.Infof("hard reloading browser")
		mustReloadBroadcast(
			config,
			manager,
			RefreshFilePayload{
				ChangeType: ChangeTypeOther,
			},
		)
	}
}

func mustKillAndRestart(config *common.Config) {
	util.Log.Infof("killing and restarting app")
	mustKillAppDev()
	mustStartAppDev(config)
}

func mustHandleCSSFileChange(
	config *common.Config,
	manager *ClientManager,
	evt fsnotify.Event,
	cssType ChangeType,
) {
	var cssBuildErr error
	sortedOnChanges := sortOnChangeCallbacks(config.DevConfig.CSSConfig.OnChangeCallbacks)
	if sortedOnChanges.exists {
		simpleRunOnChangeCallbacks(sortedOnChanges.stratPre, evt.Name)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			cssBuildErr = buildtime.ProcessCSS(config, string(cssType))
		}()

		runConcurrentOnChangeCallbacks(&sortedOnChanges, evt.Name)
		wg.Wait()
	} else {
		cssBuildErr = buildtime.ProcessCSS(config, string(cssType))
	}

	if cssBuildErr != nil {
		util.Log.Errorf("error: failed to process %s CSS: %v", cssType, cssBuildErr)
		return
	}

	util.Log.Infof("modified: %s", evt.Name)
	util.Log.Infof("hot reloading browser")
	mustReloadBroadcast(
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

func mustHandleOtherFileChange(config *common.Config, manager *ClientManager, evt fsnotify.Event, evtDetails EvtDetails) {
	wfc := (config.DevConfig.WatchedFiles)[evtDetails.complexExtension]

	if !wfc.SkipRebuildingNotification {
		manager.broadcast <- RefreshFilePayload{
			ChangeType: ChangeTypeRebuilding,
		}
		util.Log.Infof("modified: %s, rebuilding static assets", evt.Name)
	}
	sortedOnChanges := sortOnChangeCallbacks(wfc.OnChangeCallbacks)

	if sortedOnChanges.exists {
		var buildErr error
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
			buildErr = buildtime.SetupNewBuild(config)
			if buildErr != nil {
				util.Log.Errorf("error: failed to setup new build: %v", buildErr)
				return
			}
			buildErr = buildtime.RunPrecompileTasks(config)
			if buildErr != nil {
				util.Log.Errorf("error: failed to run precompile tasks: %v", buildErr)
				return
			}
			if wfc.RecompileBinary {
				buildErr = buildtime.CompileBinary(config)
				if buildErr != nil {
					util.Log.Errorf("error: failed to recompile binary: %v", buildErr)
					return
				}
			}
		}()

		runConcurrentOnChangeCallbacks(&sortedOnChanges, evt.Name)
		wg.Wait()
	} else {
		var buildErr error
		buildErr = buildtime.SetupNewBuild(config)
		if buildErr != nil {
			util.Log.Errorf("error: failed to setup new build: %v", buildErr)
			return
		}
		buildErr = buildtime.RunPrecompileTasks(config)
		if buildErr != nil {
			util.Log.Errorf("error: failed to run precompile tasks: %v", buildErr)
			return
		}
		if wfc.RecompileBinary {
			buildErr = buildtime.CompileBinary(config)
			if buildErr != nil {
				util.Log.Errorf("error: failed to recompile binary: %v", buildErr)
				return
			}
		}
	}

	if wfc.RecompileBinary {
		util.Log.Infof("doing a full recompile")
	} else {
		util.Log.Infof("skipping full recompile")
	}

	if wfc.RecompileBinary || wfc.RestartApp {
		mustKillAndRestart(config)
	}

	util.Log.Infof("hard reloading browser")
	mustReloadBroadcast(
		config,
		manager,
		RefreshFilePayload{
			ChangeType: ChangeTypeOther,
		},
	)
}

func mustReloadBroadcast(config *common.Config, manager *ClientManager, rfp RefreshFilePayload) {
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
