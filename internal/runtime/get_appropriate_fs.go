package runtime

import (
	"io/fs"
	"os"
	"path"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
	"github.com/sjc5/kit/pkg/executil"
)

const fsTypeDev = "dev"

var uniDirFSCacheMap = make(map[*common.Config]*UniversalFS)

func GetUniversalDirFS(config *common.Config) *UniversalFS {
	if hit, isCached := uniDirFSCacheMap[config]; isCached {
		return hit
	}
	fs := newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), "dist/kiruna")))
	uniDirFSCacheMap[config] = fs
	return fs
}

var uniFSCacheMap = make(map[*common.Config]*UniversalFS)
var fsTypeCacheMap = make(map[*common.Config]string)

func GetUniversalFS(config *common.Config) (*UniversalFS, error) {
	if hit, isCached := uniFSCacheMap[config]; isCached {
		skipCache := common.KirunaEnv.GetIsDev() && fsTypeCacheMap[config] != fsTypeDev
		if !skipCache {
			return hit, nil
		}
	}

	// DEV
	// There is an expectation that you run the dev server from the root of your project,
	// where your go.mod file is.
	if common.KirunaEnv.GetIsDev() {
		// ensures "needsReset" is always true in dev
		fsTypeCacheMap[config] = fsTypeDev

		util.Log.Infof("using disk file system (development)")
		fs := newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), "dist/kiruna")))
		uniFSCacheMap[config] = fs // cache the fs
		return fs, nil
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
		uniFSCacheMap[config] = fs // cache the fs
		return fs, nil
	}

	// PROD
	// If we are not using the embedded file system, we should use the os file system,
	// and assume that the executable is a sibling to the kiruna-outputted "kiruna" directory
	util.Log.Infof("using disk file system (production)")
	execDir, err := executil.GetExecutableDir()
	if err != nil {
		return nil, err
	}
	fs := newUniversalFS(os.DirFS(execDir))
	uniFSCacheMap[config] = fs // cache the fs
	return fs, nil
}
