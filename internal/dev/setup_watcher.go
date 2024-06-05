package dev

import (
	"errors"
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

var naiveIgnoreDirPatterns = [4]string{"**/.git", "**/node_modules", "dist/bin", "dist/kiruna"}
var ignoredDirPatterns = []string{}
var ignoredFilePatterns = []string{}
var defaultWatchedFiles = []common.WatchedFile{}

func mustSetupWatcher(manager *ClientManager, config *common.Config) {
	defer mustKillAppDev()
	cleanRootDir := config.GetCleanRootDir()

	for _, p := range naiveIgnoreDirPatterns {
		ignoredDirPatterns = append(ignoredDirPatterns, filepath.Join(cleanRootDir, p))
	}
	for _, p := range config.DevConfig.IgnorePatterns.Dirs {
		ignoredDirPatterns = append(ignoredDirPatterns, filepath.Join(cleanRootDir, p))
	}
	for _, p := range config.DevConfig.IgnorePatterns.Files {
		ignoredFilePatterns = append(ignoredFilePatterns, filepath.Join(cleanRootDir, p))
	}

	// Loop through all WatchedFiles...
	for i, wfc := range config.DevConfig.WatchedFiles {
		// and make each WatchedFile's Pattern relative to cleanRootDir...
		config.DevConfig.WatchedFiles[i].Pattern = filepath.Join(cleanRootDir, wfc.Pattern)

		// then loop through such WatchedFile's OnChangeCallbacks...
		for j, oc := range wfc.OnChangeCallbacks {
			// and make each such OnChangeCallback's ExcludedPatterns also relative to cleanRootDir
			for k, p := range oc.ExcludedPatterns {
				config.DevConfig.WatchedFiles[i].OnChangeCallbacks[j].ExcludedPatterns[k] = filepath.Join(cleanRootDir, p)
			}
		}
	}

	defaultWatchedFiles = append(defaultWatchedFiles, common.WatchedFile{
		Pattern: filepath.Join(cleanRootDir, "static/{public,private}/**/*"),
	})

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

			fileInfo, _ := os.Stat(evt.Name) // no need to check error, because we want to process either way
			if fileInfo != nil && fileInfo.IsDir() {
				if evt.Has(fsnotify.Create) || evt.Has(fsnotify.Rename) {
					if err := addDirs(watcher, evt.Name); err != nil {
						util.Log.Errorf("error: failed to add directory to watcher: %v", err)
						continue
					}
				}
				continue
			}

			evtDetails := getEvtDetails(config, evt)
			if evtDetails.isIgnored {
				continue
			}

			err := mustHandleFileChange(config, manager, evt, evtDetails)
			if err != nil {
				util.Log.Errorf("error: failed to handle file change: %v", err)
			}

		case err := <-watcher.Errors:
			util.Log.Errorf("watcher error: %v", err)
		}
	}
}

type sortedOnChangeCallbacks struct {
	stratPre              []common.OnChange
	stratConcurrent       []common.OnChange
	stratPost             []common.OnChange
	stratConcurrentNoWait []common.OnChange
	exists                bool
}

func sortOnChangeCallbacks(onChanges []common.OnChange) sortedOnChangeCallbacks {
	stratPre := []common.OnChange{}
	stratConcurrent := []common.OnChange{}
	stratPost := []common.OnChange{}
	stratConcurrentNoWait := []common.OnChange{}
	exists := false
	if len(onChanges) == 0 {
		return sortedOnChangeCallbacks{}
	} else {
		exists = true
	}
	for _, o := range onChanges {
		switch o.Strategy {
		case common.OnChangeStrategyPre, "":
			stratPre = append(stratPre, o)
		case common.OnChangeStrategyConcurrent:
			stratConcurrent = append(stratConcurrent, o)
		case common.OnChangeStrategyPost:
			stratPost = append(stratPost, o)
		case common.OnChangeStrategyConcurrentNoWait:
			stratConcurrentNoWait = append(stratConcurrentNoWait, o)
		}
	}
	return sortedOnChangeCallbacks{
		stratPre:              stratPre,
		stratConcurrent:       stratConcurrent,
		stratPost:             stratPost,
		stratConcurrentNoWait: stratConcurrentNoWait,
		exists:                exists,
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

func runConcurrentOnChangeCallbacks(onChanges *[]common.OnChange, evtName string, shouldWait bool) {
	if len(*onChanges) > 0 {
		wg := sync.WaitGroup{}
		wg.Add(len(*onChanges))
		for _, o := range *onChanges {
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
		if shouldWait {
			wg.Wait()
		}
	}
}

func simpleRunOnChangeCallbacks(onChanges *[]common.OnChange, evtName string) {
	for _, o := range *onChanges {
		if getIsIgnored(evtName, &o.ExcludedPatterns) {
			continue
		}
		err := o.Func(evtName)
		if err != nil {
			util.Log.Errorf("error running extension callback: %v", err)
		}
	}
}

func mustKillAndRestart(config *common.Config) {
	util.Log.Infof("killing and restarting app")
	mustKillAppDev()
	mustStartAppDev(config)
}

// This is different than inside of handleGoFileChange, because here we
// assume we need to re-run other build steps too, not just recompile Go.
// Also, we don't necessarily recompile Go here (we only necessarily) run
// the other build steps. We only recompile Go if wfc.RecompileBinary is true.
func runOtherFileBuild(config *common.Config, wfc *common.WatchedFile) error {
	err := buildtime.Build(config, wfc.RecompileBinary, true)
	if err != nil {
		msg := fmt.Sprintf("error: failed to build app: %v", err)
		util.Log.Error(msg)
		return errors.New(msg)
	}
	return nil
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
	isGo                bool
	isOther             bool
	isCriticalCSS       bool
	isNormalCSS         bool
	isKirunaCSS         bool
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
	isCssSimple := filepath.Ext(evt.Name) == ".css"
	isCriticalCSS := isCssSimple && getIsCssEvtType(config, evt, ChangeTypeCriticalCSS)
	isNormalCSS := isCssSimple && getIsCssEvtType(config, evt, ChangeTypeNormalCSS)
	isKirunaCSS := isCriticalCSS || isNormalCSS

	var matchingWatchedFile *common.WatchedFile

	for _, wfc := range config.DevConfig.WatchedFiles {
		isMatch := getIsMatch(wfc.Pattern, evt.Name)
		if isMatch {
			matchingWatchedFile = &wfc
			break
		}
	}

	if matchingWatchedFile == nil {
		for _, wfc := range defaultWatchedFiles {
			isMatch := getIsMatch(wfc.Pattern, evt.Name)
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

	isIgnored := getIsIgnored(evt.Name, &ignoredFilePatterns)
	if isOther && matchingWatchedFile == nil {
		isIgnored = true
	}

	return EvtDetails{
		isOther:             isOther,
		isKirunaCSS:         isKirunaCSS,
		isGo:                isGo,
		isIgnored:           isIgnored,
		isCriticalCSS:       isCriticalCSS,
		isNormalCSS:         isNormalCSS,
		matchingWatchedFile: matchingWatchedFile,
	}
}

func getIsCssEvtType(config *common.Config, evt fsnotify.Event, cssType ChangeType) bool {
	return strings.HasPrefix(evt.Name, filepath.Join(config.GetCleanRootDir(), "styles/"+string(cssType)))
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
	isSolelyCHMOD := !evt.Has(fsnotify.Write) && !evt.Has(fsnotify.Create) && !evt.Has(fsnotify.Remove) && !evt.Has(fsnotify.Rename)
	return isSolelyCHMOD && !getIsEmptyFile(evt)
}

func callback(config *common.Config, wfc *common.WatchedFile, evtDetails EvtDetails) error {
	if evtDetails.isGo {
		return buildtime.CompileBinary(config)
	}

	if evtDetails.isKirunaCSS {
		if wfc.RecompileBinary || wfc.RestartApp {
			return runOtherFileBuild(config, wfc)
		}
		cssType := ChangeTypeNormalCSS
		if evtDetails.isCriticalCSS {
			cssType = ChangeTypeCriticalCSS
		}
		return buildtime.ProcessCSS(config, string(cssType))
	}

	return runOtherFileBuild(config, wfc)
}

func mustHandleFileChange(
	config *common.Config,
	manager *ClientManager,
	evt fsnotify.Event,
	evtDetails EvtDetails,
) error {
	if getIsNonEmptyCHMODOnly(evt) {
		return nil
	}

	wfc := evtDetails.matchingWatchedFile
	if wfc == nil {
		wfc = &common.WatchedFile{}
	}

	if !config.DevConfig.ServerOnly && !wfc.SkipRebuildingNotification && !evtDetails.isKirunaCSS {
		manager.broadcast <- RefreshFilePayload{
			ChangeType: ChangeTypeRebuilding,
		}
	}

	util.Log.Infof("modified: %s", evt.Name)

	if evtDetails.isGo || wfc.RecompileBinary {
		util.Log.Infof("recompiling binary")
	}

	sortedOnChanges := sortOnChangeCallbacks(wfc.OnChangeCallbacks)

	var buildErr error

	if sortedOnChanges.exists {
		go func() {
			runConcurrentOnChangeCallbacks(&sortedOnChanges.stratConcurrentNoWait, evt.Name, false)
		}()

		simpleRunOnChangeCallbacks(&sortedOnChanges.stratPre, evt.Name)

		if wfc.RunOnChangeOnly {
			util.Log.Infof("ran applicable onChange callbacks")
			return nil
		}

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			buildErr = callback(config, wfc, evtDetails)
		}()

		runConcurrentOnChangeCallbacks(&sortedOnChanges.stratConcurrent, evt.Name, true)
		wg.Wait()
	} else {
		buildErr = callback(config, wfc, evtDetails)
	}

	if buildErr != nil {
		util.Log.Errorf("error: failed to build: %v", buildErr)
		return buildErr
	}

	simpleRunOnChangeCallbacks(&sortedOnChanges.stratPost, evt.Name)

	needsHardReloadEvenIfNonGo := wfc.RecompileBinary || wfc.RestartApp

	if evtDetails.isGo || needsHardReloadEvenIfNonGo {
		mustKillAndRestart(config)
	}

	if config.DevConfig.ServerOnly {
		return nil
	}

	if !evtDetails.isKirunaCSS || needsHardReloadEvenIfNonGo {
		util.Log.Infof("hard reloading browser")
		mustReloadBroadcast(
			config,
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

	return nil
}
