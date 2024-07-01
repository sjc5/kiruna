package ik

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

func (c *Config) mustSetupWatcher() {
	defer c.mustKillAppDev()
	cleanRootDir := c.getCleanRootDir()

	for _, p := range naiveIgnoreDirPatterns {
		ignoredDirPatterns = append(ignoredDirPatterns, filepath.Join(cleanRootDir, p))
	}
	for _, p := range c.DevConfig.IgnorePatterns.Dirs {
		ignoredDirPatterns = append(ignoredDirPatterns, filepath.Join(cleanRootDir, p))
	}
	for _, p := range c.DevConfig.IgnorePatterns.Files {
		ignoredFilePatterns = append(ignoredFilePatterns, filepath.Join(cleanRootDir, p))
	}

	// Loop through all WatchedFiles...
	for i, wfc := range c.DevConfig.WatchedFiles {
		// and make each WatchedFile's Pattern relative to cleanRootDir...
		c.DevConfig.WatchedFiles[i].Pattern = filepath.Join(cleanRootDir, wfc.Pattern)

		// then loop through such WatchedFile's OnChangeCallbacks...
		for j, oc := range wfc.OnChangeCallbacks {
			// and make each such OnChangeCallback's ExcludedPatterns also relative to cleanRootDir
			for k, p := range oc.ExcludedPatterns {
				c.DevConfig.WatchedFiles[i].OnChangeCallbacks[j].ExcludedPatterns[k] = filepath.Join(cleanRootDir, p)
			}
		}
	}

	defaultWatchedFiles = append(defaultWatchedFiles, WatchedFile{
		Pattern:    filepath.Join(cleanRootDir, "static/{public,private}/**/*"),
		RestartApp: true,
	})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		errMsg := fmt.Sprintf("error: failed to create watcher: %v", err)
		c.Logger.Error(errMsg)
		panic(errMsg)
	}
	defer watcher.Close()
	c.watcher = watcher

	err = c.addDirs(c.getCleanRootDir())
	if err != nil {
		errMsg := fmt.Sprintf("error: failed to add directories to watcher: %v", err)
		c.Logger.Error(errMsg)
		panic(errMsg)
	}
	done := make(chan bool)
	go func() {
		c.mustKillAppDev()
		err := c.compileBinary()
		if err != nil {
			c.Logger.Errorf("error: failed to build app: %v", err)
		}
		c.mustStartAppDev()
		c.mustHandleWatcherEmissions()
	}()
	<-done
}

func (c *Config) addDirs(path string) error {
	return filepath.Walk(path, func(walkedPath string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking path: %v", err)
		}
		if info.IsDir() {
			if c.getIsIgnored(walkedPath, &ignoredDirPatterns) {
				return filepath.SkipDir
			}
			err := c.watcher.Add(walkedPath)
			if err != nil {
				return fmt.Errorf("error adding directory to watcher: %v", err)
			}
		}
		return nil
	})
}
