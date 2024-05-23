package buildtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sjc5/kiruna/internal/common"
)

func Build(config *common.Config, recompileBinary bool, shouldBeGranular bool) error {
	cleanRootDir := config.GetCleanRootDir()

	if !shouldBeGranular {
		// nuke the dist/kiruna directory
		err := os.RemoveAll(filepath.Join(cleanRootDir, "dist", "kiruna"))
		if err != nil {
			return fmt.Errorf("error removing dist/kiruna directory: %v", err)
		}

		// re-make required directories
		isServerOnly := config.DevConfig != nil && config.DevConfig.ServerOnly
		if !isServerOnly {
			err = SetupDistDir(config.RootDir)
			if err != nil {
				return fmt.Errorf("error making requisite directories: %v", err)
			}
		}
	}

	// Must be complete before BuildCSS in case the CSS references any public files
	err := handlePublicFiles(config, shouldBeGranular)
	if err != nil {
		return fmt.Errorf("error handling public files: %v", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 2) // Buffer to hold up to 2 errors
	wg.Add(2)                      // Two tasks to do concurrently

	// goroutine 1
	go func() {
		defer wg.Done()
		if err = copyPrivateFiles(config, shouldBeGranular); err != nil {
			errChan <- PrecompileError{task: "copyPrivateFiles", err: err}
		}
	}()

	// goroutine 2
	go func() {
		defer wg.Done()
		if err = BuildCSS(config); err != nil {
			errChan <- PrecompileError{task: "BuildCSS", err: err}
		}
	}()

	// Wait for all tasks to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for e := range errChan {
		if e != nil {
			return e
		}
	}

	if recompileBinary {
		err = CompileBinary(config)
		if err != nil {
			return fmt.Errorf("error compiling binary: %v", err)
		}
	}
	return nil
}

// Define a custom error type for more specific error handling
type PrecompileError struct {
	task string
	err  error
}

func (e PrecompileError) Error() string {
	return fmt.Sprintf("error during precompile task %s: %v", e.task, e.err)
}

func handlePublicFiles(config *common.Config, shouldBeGranular bool) error {
	return processStaticFiles(config, &staticFileProcessorOpts{
		DirName:          "public",
		MapName:          common.PublicFileMapGobName,
		ShouldBeGranular: shouldBeGranular,
		GetIsNoHashDir: func(path string) bool {
			return strings.HasPrefix(path, "__nohash/")
		},
		WriteWithHash: true,
	})
}

func copyPrivateFiles(config *common.Config, shouldBeGranular bool) error {
	return processStaticFiles(config, &staticFileProcessorOpts{
		DirName:          "private",
		MapName:          common.PrivateFileMapGobName,
		ShouldBeGranular: shouldBeGranular,
		GetIsNoHashDir: func(path string) bool {
			return false
		},
		WriteWithHash: false,
	})
}
