package runtime

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
	"github.com/sjc5/kit/pkg/typed"
)

const fsTypeDev = "dev"

var uniDirFSCacheMap = typed.SyncMap[*common.Config, *UniversalFS]{}

func GetUniversalDirFS(config *common.Config) *UniversalFS {
	if hit, isCached := uniDirFSCacheMap.Load(config); isCached {
		return hit
	}
	fs := newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), "dist/kiruna")))
	actualFS, _ := uniDirFSCacheMap.LoadOrStore(config, fs)
	return actualFS
}

var uniFSCacheMap = typed.SyncMap[*common.Config, *UniversalFS]{}
var fsTypeCacheMap = typed.SyncMap[*common.Config, string]{}

func GetUniversalFS(config *common.Config) (*UniversalFS, error) {
	if hit, isCached := uniFSCacheMap.Load(config); isCached {
		cachedFSType, _ := fsTypeCacheMap.Load(config)
		skipCache := common.KirunaEnv.GetIsDev() && cachedFSType != fsTypeDev
		if !skipCache {
			return hit, nil
		}
	}

	// DEV
	// There is an expectation that you run the dev server from the root of your project,
	// where your go.mod file is.
	if common.KirunaEnv.GetIsDev() {
		// ensures "needsReset" is always true in dev
		fsTypeCacheMap.Store(config, fsTypeDev)

		util.Log.Infof("using disk file system (development)")
		fs := newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), "dist/kiruna")))
		actualFS, _ := uniFSCacheMap.LoadOrStore(config, fs) // cache the fs
		return actualFS, nil
	}

	// PROD
	// If we are using the embedded file system, we should use the dist file system
	if config.GetIsUsingEmbeddedFS() {
		util.Log.Infof("using embedded file system (production)")

		// Assuming the embed directive looks like this:
		// //go:embed kiruna
		// That means that the kiruna folder itself (not just its contents) is embedded.
		// So we have to drop down into the kiruna folder here.
		FS, err := fs.Sub(config.DistFS, "kiruna")
		if err != nil {
			return nil, err
		}
		fs := newUniversalFS(FS)
		actualFS, _ := uniFSCacheMap.LoadOrStore(config, fs) // cache the fs
		return actualFS, nil
	}

	// PROD
	// If we are not using the embedded file system, we should use the os file system,
	// and assume that the executable is a sibling to the kiruna-outputted "kiruna" directory
	util.Log.Infof("using disk file system (production)")
	execDir, err := getExecutableDir()
	if err != nil {
		return nil, err
	}
	fs := newUniversalFS(os.DirFS(execDir))
	actualFS, _ := uniFSCacheMap.LoadOrStore(config, fs) // cache the fs
	return actualFS, nil
}

func getExecutableDir() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("error getting executable path: %v", err)
	}
	return filepath.Dir(execPath), nil
}
