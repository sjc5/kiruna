package buildtime

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

func handlePublicFiles(cleanRootDir string) error {
	publicDir := filepath.Join(cleanRootDir, "static", "public")
	if _, err := os.Stat(publicDir); os.IsNotExist(err) {
		return nil
	}
	publicFileMap := make(map[string]string)

	err := filepath.WalkDir(publicDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relativePath, err := filepath.Rel(publicDir, path)
		if err != nil {
			return err
		}
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// normalize
		relativePath = filepath.ToSlash(relativePath)
		isNoHashDir := strings.HasPrefix(relativePath, "__nohash/")

		if isNoHashDir {
			relativePath = strings.TrimPrefix(relativePath, "__nohash/")
		}

		relativePathUnderscores := strings.ReplaceAll(relativePath, "/", "_")
		hashed := util.GetHashedFilename(contentBytes, relativePathUnderscores)

		if !isNoHashDir {
			publicFileMap[relativePath] = hashed

			// Now actually copy the file to the dist directory
			distPath := filepath.Join(cleanRootDir, "dist", "kiruna", "static", "public", hashed)
			err = os.WriteFile(distPath, contentBytes, 0644)
			if err != nil {
				return err
			}
		} else {
			// Actually copy the files over
			distPath := filepath.Join(cleanRootDir, "dist", "kiruna", "static", "public", relativePath)
			// Make sure the directory exists
			err = os.MkdirAll(filepath.Dir(distPath), 0755)
			if err != nil {
				return err
			}
			err = os.WriteFile(distPath, contentBytes, 0644)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = saveMapToGob(cleanRootDir, publicFileMap, common.PublicFileMapGobName)
	if err != nil {
		return err
	}

	return nil
}
