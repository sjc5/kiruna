package runtime

import (
	"encoding/gob"
	"fmt"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/common"
)

func loadMapFromGob(config *common.Config, gobFileName string, useDirFS bool) (common.Map, error) {
	var FS *UniversalFS
	var err error
	if useDirFS {
		FS, err = GetUniversalDirFS(config)
	} else {
		FS, err = GetUniversalFS(config)
	}
	if err != nil {
		return nil, fmt.Errorf("error getting FS: %v", err)
	}
	file, err := FS.Open(filepath.Join("kiruna", "internal", gobFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()
	decoder := gob.NewDecoder(file)
	var mapFromGob common.Map
	err = decoder.Decode(&mapFromGob)
	if err != nil {
		return nil, fmt.Errorf("error decoding gob: %v", err)
	}
	return mapFromGob, nil
}
