package runtime

import (
	"io/fs"
	"os"
	"path"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

var cachedUniFS *UniversalFS
var fsType string

const fsTypeDev = "dev"

func GetUniversalDirFS(config *common.Config) (*UniversalFS, error) {
	cachedUniFS = newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), "dist")))
	return cachedUniFS, nil
}

func GetUniversalFS(config *common.Config) (*UniversalFS, error) {
	if cachedUniFS != nil {
		needsReset := common.KirunaEnv.GetIsDev() && fsType != fsTypeDev
		if !needsReset {
			return cachedUniFS, nil
		}
	}

	// DEV
	// There is an expectation that you run the dev server from the root of your project,
	// where your go.mod file is.
	if common.KirunaEnv.GetIsDev() {
		fsType = fsTypeDev
		util.Log.Infof("using disk file system (development)")
		cachedUniFS = newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), "dist")))
		return cachedUniFS, nil
	}

	// PROD
	// If we are using the embedded file system, we should use the dist file system
	if config.GetIsUsingEmbeddedFS() {
		util.Log.Infof("using embedded file system (production)")
		distFS := config.DistFS
		FS, err := fs.Sub(distFS, "dist")
		if err != nil {
			return nil, err
		}
		cachedUniFS = newUniversalFS(FS)
		return cachedUniFS, nil
	}

	// PROD
	// If we are not using the embedded file system, we should use the os file system,
	// and assume that the executable is a sibling to the kiruna-outputted "kiruna" directory
	util.Log.Infof("using disk file system (production)")
	cachedUniFS = newUniversalFS(os.DirFS(util.GetExecDir()))
	return cachedUniFS, nil
}
