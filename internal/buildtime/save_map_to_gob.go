package buildtime

import (
	"encoding/gob"
	"os"
	"path/filepath"
)

func saveMapToGob(cleanRootDir string, mapToSave map[string]string, dest string) error {
	file, err := os.Create(filepath.Join(cleanRootDir, "dist", "kiruna", "internal", dest))
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(mapToSave)
	return err
}
