package buildtime

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

func copyPrivateFiles(cleanRootDir string) error {
	privateDir := filepath.Join(cleanRootDir, "static", "private")
	if _, err := os.Stat(privateDir); os.IsNotExist(err) {
		return nil
	}
	return filepath.WalkDir(privateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking private dir: %v", err)
		}
		if d.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(privateDir, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %v", err)
		}
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading file: %v", err)
		}
		distPath := filepath.Join(cleanRootDir, "dist", "kiruna", "static", "private", relativePath)
		err = os.MkdirAll(filepath.Dir(distPath), 0755)
		if err != nil {
			return fmt.Errorf("error creating directory: %v", err)
		}
		return os.WriteFile(distPath, contentBytes, 0644)
	})
}
