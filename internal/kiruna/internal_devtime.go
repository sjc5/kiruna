package ik

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/sjc5/kit/pkg/safecache"
	"golang.org/x/sync/semaphore"
)

type withMu[T any] struct {
	v  T
	mu sync.Mutex
}

type dev struct {
	initOnce               sync.Once
	watcher                *fsnotify.Watcher
	manager                *clientManager
	fileSemaphore          *semaphore.Weighted
	ignoredDirPatterns     *[]string
	ignoredFilePatterns    *[]string
	naiveIgnoreDirPatterns *[]string
	defaultWatchedFile     *WatchedFile
	defaultWatchedFiles    *[]WatchedFile
	lastBuildCmd           withMu[*exec.Cmd]
	matchResults           *safecache.CacheMap[potentialMatch, string, bool]
	pidFile                *PIDFile
}

func (c *Config) devInitOnce() {
	c.dev.initOnce.Do(func() {
		cleanRootDir := c.getCleanRootDir()

		// watcher
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			errMsg := fmt.Sprintf("error: failed to create watcher: %v", err)
			c.Logger.Error(errMsg)
			panic(errMsg)
		}
		c.watcher = watcher

		// manager
		c.manager = newClientManager()

		// fileSemaphore
		c.fileSemaphore = semaphore.NewWeighted(100)

		// ignored setup
		c.ignoredDirPatterns = &[]string{}
		c.ignoredFilePatterns = &[]string{}
		c.naiveIgnoreDirPatterns = &[]string{
			"**/.git", "**/node_modules", "dist/bin", distKirunaDir,
		}
		for _, p := range *c.naiveIgnoreDirPatterns {
			*c.ignoredDirPatterns = append(*c.ignoredDirPatterns, filepath.Join(cleanRootDir, p))
		}
		for _, p := range c.DevConfig.IgnorePatterns.Dirs {
			*c.ignoredDirPatterns = append(*c.ignoredDirPatterns, filepath.Join(cleanRootDir, p))
		}
		for _, p := range c.DevConfig.IgnorePatterns.Files {
			*c.ignoredFilePatterns = append(*c.ignoredFilePatterns, filepath.Join(cleanRootDir, p))
		}

		// default watched files
		c.defaultWatchedFile = &WatchedFile{}
		c.defaultWatchedFiles = &[]WatchedFile{{
			Pattern: filepath.Join(
				cleanRootDir, fmt.Sprintf("%s/{%s,%s}/**/*", staticDir, publicDir, privateDir),
			),
			RestartApp: true,
		}}

		// matches
		c.matchResults = safecache.NewMap(c.getInitialMatchResults, c.matchResultsKeyMaker, nil)

		// single file stores
		c.pidFile = newPIDFile(cleanRootDir)
	})
}
