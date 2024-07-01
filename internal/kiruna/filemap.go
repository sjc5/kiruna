package ik

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjc5/kit/pkg/fsutil"
)

const (
	PublicFileMapGobName  = "public_filemap.gob"
	PrivateFileMapGobName = "private_filemap.gob"
)

func (c *Config) loadMapFromGob(gobFileName string, useDirFS bool) (map[string]string, error) {
	var FS UniversalFS
	var err error
	if useDirFS {
		FS = c.getUniversalDirFS()
	} else {
		FS, err = c.GetUniversalFS()
	}
	if err != nil {
		return nil, fmt.Errorf("error getting FS: %v", err)
	}

	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	file, err := FS.Open(filepath.Join(internalDir, gobFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %v", gobFileName, err)
	}

	defer file.Close()

	var mapFromGob map[string]string
	err = fsutil.FromGobInto(file, &mapFromGob)
	if err != nil {
		return nil, fmt.Errorf("error decoding gob: %v", err)
	}
	return mapFromGob, nil
}

func (c *Config) saveMapToGob(mapToSave map[string]string, dest string) error {
	file, err := os.Create(filepath.Join(c.getCleanRootDir(), distKirunaDir, internalDir, dest))
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	return encoder.Encode(mapToSave)
}
