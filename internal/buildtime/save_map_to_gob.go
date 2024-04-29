package buildtime

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

func saveMapToGob(cleanRootDir string, mapToSave map[string]string, dest string) error {
	file, err := os.Create(filepath.Join(cleanRootDir, "dist", "kiruna", "internal", dest))
	if err != nil {
		return fmt.Errorf("error creating file: %v", err)
	}
	defer file.Close()
	encoder := gob.NewEncoder(file)
	return encoder.Encode(mapToSave)
}
