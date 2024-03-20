package runtime

import (
	"io/fs"
	"os"
	"path"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

var universalFS *UniversalFS

func GetUniversalDirFS(config *common.Config) (*UniversalFS, error) {
	universalFS = newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), "dist")))
	return universalFS, nil
}

func GetUniversalFS(config *common.Config) (*UniversalFS, error) {
	if universalFS != nil {
		return universalFS, nil
	}

	// DEV
	// There is an expectation that you run the dev server from the root of your project,
	// where your go.mod file is.
	if common.GetIsKirunaEnvDev() {
		util.Log.Infof("using disk file system (development)")
		universalFS = newUniversalFS(os.DirFS(path.Join(config.GetCleanRootDir(), "dist")))
		return universalFS, nil
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
		universalFS = newUniversalFS(FS)
		return universalFS, nil
	}

	// PROD
	// If we are not using the embedded file system, we should use the os file system,
	// and assume that the executable is a sibling to the kiruna-outputted "kiruna" directory
	util.Log.Infof("using disk file system (production)")
	universalFS = newUniversalFS(os.DirFS(util.GetExecDir()))
	return universalFS, nil
}
