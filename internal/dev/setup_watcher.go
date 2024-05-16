package dev

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
	"github.com/sjc5/kiruna/internal/buildtime"
	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/runtime"
	"github.com/sjc5/kiruna/internal/util"
)

var ignoredDirPatterns = []string{".git", "node_modules", "dist/bin", "dist/kiruna"}
var ignoredFilePatterns = []string{}

func mustSetupWatcher(manager *ClientManager, config *common.Config) {
	defer mustKillAppDev()
	ignoredDirPatterns = append(ignoredDirPatterns, config.DevConfig.IgnorePatterns.Dirs...)
	ignoredFilePatterns = append(ignoredFilePatterns, config.DevConfig.IgnorePatterns.Files...)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		errMsg := fmt.Sprintf("error: failed to create watcher: %v", err)
		util.Log.Error(errMsg)
		panic(errMsg)
	}
	defer watcher.Close()
	err = addDirs(watcher, config.GetCleanRootDir())
	if err != nil {
		errMsg := fmt.Sprintf("error: failed to add directories to watcher: %v", err)
		util.Log.Error(errMsg)
		panic(errMsg)
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

			if evt.Has(fsnotify.Create) || evt.Has(fsnotify.Rename) {
				fileInfo, err := os.Stat(evt.Name)
				if err == nil && fileInfo.IsDir() {
					err := addDirs(watcher, evt.Name)
					if err != nil {
						util.Log.Errorf("error: failed to add directory to watcher: %v", err)
						return
					}
				}
			}

			evtDetails := getEvtDetails(config, evt)
			if evtDetails.isIgnored {
				continue
			}
			if getIsGo(evt) {
				if evtDetails.matchingWatchedFile != nil {
					if evtDetails.matchingWatchedFile.TreatAsNonGo {
						mustHandleOtherFileChange(config, manager, evt, evtDetails)
						continue
					}
				}
				mustHandleGoFileChange(config, manager, evt, evtDetails)
			} else {
				if !config.DevConfig.ServerOnly {
					if evtDetails.isCriticalCss {
						mustHandleCSSFileChange(config, manager, evt, ChangeTypeCriticalCSS)
					} else if evtDetails.isNormalCss {
						mustHandleCSSFileChange(config, manager, evt, ChangeTypeNormalCSS)
					} else if evtDetails.matchingWatchedFile != nil {
						mustHandleOtherFileChange(config, manager, evt, evtDetails)
					}
				}
			}

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

func getIsIgnored(path string, ignoredPatterns *[]string) bool {
	for _, pattern := range *ignoredPatterns {
		if getIsMatch(pattern, path) {
			return true
		}
	}
	return false
}

func runConcurrentOnChangeCallbacks(sortedOnChanges *sortedOnChangeCallbacks, evtName string) {
	if len(sortedOnChanges.stratConcurrent) > 0 {
		wg := sync.WaitGroup{}
		wg.Add(len(sortedOnChanges.stratConcurrent))
		for _, o := range sortedOnChanges.stratConcurrent {
			if getIsIgnored(evtName, &o.ExcludedPatterns) {
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
		if getIsIgnored(evtName, &o.ExcludedPatterns) {
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
	if getIsNonEmptyCHMODOnly(evt) {
		return
	}

	if !config.DevConfig.ServerOnly {
		manager.broadcast <- RefreshFilePayload{
			ChangeType: ChangeTypeRebuilding,
		}
	}

	wfc := evtDetails.matchingWatchedFile
	if wfc == nil {
		wfc = &common.WatchedFile{}
	}

	sortedOnChanges := sortOnChangeCallbacks(wfc.OnChangeCallbacks)

	var buildErr error

	if sortedOnChanges.exists {
		simpleRunOnChangeCallbacks(sortedOnChanges.stratPre, evt.Name)

		// You would do this if you want to trigger a process that itself
		// saves to a file (evt.g., to styles/critical/whatever.css) that
		// would in turn trigger the rebuild in another run
		if wfc.RunOnChangeOnly {
			util.Log.Infof("ran applicable onChange callbacks")
			return
		}

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
	if getIsNonEmptyCHMODOnly(evt) {
		return
	}

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
	if getIsNonEmptyCHMODOnly(evt) {
		return
	}

	wfc := evtDetails.matchingWatchedFile
	if wfc == nil {
		return
	}

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
			util.Log.Infof("ran applicable onChange callbacks")
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
	errMsg := fmt.Sprintf("error: app never became ready: %v", rfp.ChangeType)
	util.Log.Error(errMsg)
	panic(errMsg)
}

func addDirs(watcher *fsnotify.Watcher, path string) error {
	return filepath.Walk(path, func(walkedPath string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking path: %v", err)
		}
		if info.IsDir() {
			if getIsIgnored(walkedPath, &ignoredDirPatterns) {
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

type EvtDetails struct {
	isIgnored           bool
	isCriticalCss       bool
	isNormalCss         bool
	matchingWatchedFile *common.WatchedFile
}

var cachedMatchResults = map[string]bool{}

func getIsMatch(pattern string, path string) bool {
	combined := pattern + path

	if hit, isCached := cachedMatchResults[combined]; isCached {
		return hit
	}

	normalizedPath := filepath.ToSlash(path)

	matches, err := doublestar.Match(pattern, normalizedPath)
	if err != nil {
		util.Log.Errorf("error: failed to match file: %v", err)
		return false
	}

	cachedMatchResults[combined] = matches // cache the result
	return matches
}

func getEvtDetails(config *common.Config, evt fsnotify.Event) EvtDetails {
	isCssSimple := getIsCss(evt)
	isCriticalCss := isCssSimple && getIsCssEvtType(config, evt, ChangeTypeCriticalCSS)
	isNormalCss := isCssSimple && getIsCssEvtType(config, evt, ChangeTypeNormalCSS)
	isKirunaCss := isCriticalCss || isNormalCss
	isIgnored := getIsIgnored(evt.Name, &ignoredFilePatterns)

	var matchingWatchedFile *common.WatchedFile

	if !isKirunaCss {
		for _, wfc := range config.DevConfig.WatchedFiles {
			isMatch := getIsMatch(wfc.Pattern, evt.Name)
			if isMatch {
				matchingWatchedFile = &wfc
				break
			}
		}
	}

	return EvtDetails{
		isIgnored:           isIgnored,
		isCriticalCss:       isCriticalCss,
		isNormalCss:         isNormalCss,
		matchingWatchedFile: matchingWatchedFile,
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

func getIsSolelyCHMOD(evt fsnotify.Event) bool {
	return !evt.Has(fsnotify.Write) && !evt.Has(fsnotify.Create) && !evt.Has(fsnotify.Remove) && !evt.Has(fsnotify.Rename)
}

func getIsEmptyFile(evt fsnotify.Event) bool {
	file, err := os.Open(evt.Name)
	if err != nil {
		util.Log.Errorf("error: failed to open file: %v", err)
		return false
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		util.Log.Errorf("error: failed to get file stats: %v", err)
		return false
	}
	return stat.Size() == 0
}

func getIsNonEmptyCHMODOnly(evt fsnotify.Event) bool {
	return getIsSolelyCHMOD(evt) && !getIsEmptyFile(evt)
}
