package buildtime

import (
	"io/fs"
	"os"
	"path/filepath"
)

func copyPrivateFiles(cleanRootDir string) error {
	privateDir := filepath.Join(cleanRootDir, "static", "private")
	if _, err := os.Stat(privateDir); os.IsNotExist(err) {
		return nil
	}
	err := filepath.WalkDir(privateDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(privateDir, path)
		if err != nil {
			return err
		}
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		distPath := filepath.Join(cleanRootDir, "dist", "kiruna", "static", "private", relativePath)
		err = os.MkdirAll(filepath.Dir(distPath), 0755)
		if err != nil {
			return err
		}
		err = os.WriteFile(distPath, contentBytes, 0644)
		return err
	})
	return err
}
