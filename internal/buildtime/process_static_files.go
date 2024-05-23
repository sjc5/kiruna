package buildtime

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/runtime"
	"github.com/sjc5/kiruna/internal/util"
)

type staticFileProcessorOpts struct {
	DirName          string
	MapName          string
	ShouldBeGranular bool
	GetIsNoHashDir   func(string) bool
	WriteWithHash    bool
}

func processStaticFiles(config *common.Config, opts *staticFileProcessorOpts) error {
	cleanRootDir := config.GetCleanRootDir()
	srcDir := filepath.Join(cleanRootDir, "static", opts.DirName)
	distDir := filepath.Join(cleanRootDir, "dist", "kiruna", "static", opts.DirName)

	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil
	}

	newFileMap := make(map[string]string)
	oldFileMap := make(map[string]string)

	// Load old file map if granular updates are enabled
	if opts.ShouldBeGranular {
		var err error
		oldFileMap, err = runtime.LoadMapFromGob(config, opts.MapName, true)
		if err != nil {
			return fmt.Errorf("error reading old file map: %v", err)
		}
	}

	// Walk the directory and process files
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("error walking dir: %v", err)
		}
		if d.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("error getting relative path: %v", err)
		}

		// Normalize path
		relativePath = filepath.ToSlash(relativePath)
		isNoHashDir := opts.GetIsNoHashDir(relativePath)
		if isNoHashDir {
			relativePath = strings.TrimPrefix(relativePath, "__nohash/")
		}
		relativePathUnderscores := strings.ReplaceAll(relativePath, "/", "_")

		var contentBytes []byte
		hasSetContentBytes := false

		var fileIdentifier string
		if isNoHashDir {
			fileIdentifier = relativePath
		} else {
			contentBytes, err = os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("error reading file: %v", err)
			}
			hasSetContentBytes = true

			fileIdentifier = util.GetHashedFilename(contentBytes, relativePathUnderscores)
		}

		newFileMap[relativePath] = fileIdentifier

		// Skip unchanged files if granular updates are enabled
		if opts.ShouldBeGranular {
			if oldHash, exists := oldFileMap[relativePath]; exists && oldHash == fileIdentifier {
				return nil
			}
		}

		var distPath string
		if opts.WriteWithHash {
			distPath = filepath.Join(distDir, fileIdentifier)
		} else {
			distPath = filepath.Join(distDir, relativePath)
		}

		err = os.MkdirAll(filepath.Dir(distPath), 0755)
		if err != nil {
			return fmt.Errorf("error creating directory: %v", err)
		}

		if !hasSetContentBytes {
			contentBytes, err = os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("error reading file: %v", err)
			}
			hasSetContentBytes = true // not strictly needed, but trying to be nice to my future self
		}

		err = os.WriteFile(distPath, contentBytes, 0644)
		if err != nil {
			return fmt.Errorf("error writing file: %v", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking dir: %v", err)
	}

	// Cleanup old moot files if granular updates are enabled
	if opts.ShouldBeGranular {
		for relativePath, oldHash := range oldFileMap {
			newHash := newFileMap[relativePath]

			if oldHash != newHash {
				oldDistPath := filepath.Join(distDir, oldHash)
				err := os.Remove(oldDistPath)
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("error removing old static file from dist (%s/%s): %v", opts.DirName, oldHash, err)
				}
			}
		}
	}

	// Save the updated file map
	err = saveMapToGob(cleanRootDir, newFileMap, opts.MapName)
	if err != nil {
		return fmt.Errorf("error saving file map: %v", err)
	}

	return nil
}
