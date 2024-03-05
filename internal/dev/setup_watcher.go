package dev

import (
	"os"
	"path/filepath"
	"strings"
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
		killBuildAndRestartAppDev(config)
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

func conditionallyRunOnChangeCallback(
	wfc common.WatchedFile,
	evt fsnotify.Event,
) {
	if wfc.OnChange != nil {
		util.Log.Infof("running extension callback")
		err := wfc.OnChange(evt.Name)
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
	wfc := (config.DevConfig.WatchedFiles)[evtDetails.complexExtension]
	conditionallyRunOnChangeCallback(wfc, evt)
	if config.DevConfig.ServerOnly {
		util.Log.Infof("modified: %s, recompiling", evt.Name)
		killBuildAndRestartAppDev(config)
		return
	}
	manager.broadcast <- RefreshFilePayload{
		ChangeType: ChangeTypeRebuilding,
	}
	util.Log.Infof("modified: %s, needs a full recompile", evt.Name)
	killBuildAndRestartAppDev(config)
	if config.DevConfig.ServerOnly {
		return
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

func handleCSSFileChange(
	config *common.Config,
	manager *ClientManager,
	evt fsnotify.Event,
	cssType ChangeType,
) {
	err := buildtime.ProcessCSS(config, string(cssType))
	if err != nil {
		util.Log.Panicf("error processing %s CSS: %v", cssType, err)
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
	if !wfc.SkipRebuildingNotification {
		manager.broadcast <- RefreshFilePayload{
			ChangeType: ChangeTypeRebuilding,
		}
		util.Log.Infof("modified: %s, rebuilding static assets", evt.Name)
	}
	conditionallyRunOnChangeCallback(wfc, evt)
	if wfc.RunOnChangeOnly {
		// You would do this if you want to trigger a process that itself
		// saves to a file (evt.g., to styles/critical/whatever.css) that
		// would in turn trigger the rebuild in another run
		util.Log.Infof("onchange complete, work is done")
		return
	}
	if wfc.RecompileBinary {
		util.Log.Infof("doing a full recompile")
	} else {
		util.Log.Infof("skipping full recompile")
	}
	if wfc.RecompileBinary || wfc.RestartApp {
		util.Log.Infof("killing running app")
		killAppDev()
	}
	// This is different than inside of handleGoFileChange, because here we
	// assume we need to re-run other build steps too, not just recompile Go.
	// Also, we don't necessarily recompile Go here (we only necessarily) run
	// the other build steps. We only recompile Go if wfc.RecompileBinary is true.
	err := buildtime.Build(config, wfc.RecompileBinary)
	if err != nil {
		util.Log.Panicf("error building app: %v", err)
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

var standardIgnoreDirList = []string{}

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
					util.Log.Infof("ignoring directory: %s", walkedPath)
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
	isCss := getIsCss(evt)
	shouldReload := false
	var complexExtension string
	if !getIsCss(evt) {
		for _, ext := range extKeys {
			if strings.HasSuffix(evt.Name, ext) {
				shouldReload = true
				complexExtension = ext
				break
			}
		}
	}
	return EvtDetails{
		isCriticalCss:    isCss && getIsCssEvtType(config, evt, ChangeTypeCriticalCSS),
		isNormalCss:      isCss && getIsCssEvtType(config, evt, ChangeTypeNormalCSS),
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
