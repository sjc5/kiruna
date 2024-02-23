package runtime

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

func getFS(config *common.Config, subDir string) (*UniversalFS, error) {
	path := filepath.Join("kiruna", "static", subDir)
	FS, err := GetUniversalFS(config)
	if err != nil {
		errMsg := fmt.Sprintf("error getting %s FS: %v", subDir, err)
		util.Log.Errorf(errMsg)
		return nil, errors.New(errMsg)
	}
	subFS, err := FS.Sub(path)
	if err != nil {
		errMsg := fmt.Sprintf("error getting %s FS: %v", subDir, err)
		util.Log.Errorf(errMsg)
		return nil, errors.New(errMsg)
	}
	return subFS, nil
}

func GetPublicFS(config *common.Config) (*UniversalFS, error) {
	return getFS(config, "public")
}

func GetPrivateFS(config *common.Config) (*UniversalFS, error) {
	return getFS(config, "private")
}
