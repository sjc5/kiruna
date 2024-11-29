package ik

import (
	"fmt"
	"os"
	"path/filepath"
)

func (c *Config) mustSetupWatcher() {
	defer c.mustKillAppDev()
	cleanWatchRoot := c.getCleanWatchRoot()

	// Loop through all WatchedFiles...
	for i, wfc := range c.DevConfig.WatchedFiles {
		// and make each WatchedFile's Pattern relative to cleanWatchRoot...
		c.DevConfig.WatchedFiles[i].Pattern = filepath.Join(cleanWatchRoot, wfc.Pattern)

		// then loop through such WatchedFile's OnChangeCallbacks...
		for j, oc := range wfc.OnChangeCallbacks {
			// and make each such OnChangeCallback's ExcludedPatterns also relative to cleanWatchRoot
			for k, p := range oc.ExcludedPatterns {
				c.DevConfig.WatchedFiles[i].OnChangeCallbacks[j].ExcludedPatterns[k] = filepath.Join(cleanWatchRoot, p)
			}
		}
	}

	defer c.watcher.Close()

	err := c.addDirs(cleanWatchRoot)
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
			c.Logger.Error(fmt.Sprintf("error: failed to build app: %v", err))
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
			if c.getIsIgnored(walkedPath, c.ignoredDirPatterns) {
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
