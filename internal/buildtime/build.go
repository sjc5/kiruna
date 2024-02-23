package buildtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/sjc5/kiruna/internal/common"
)

func setupNewBuild(config *common.Config) error {
	cleanRootDir := config.GetCleanRootDir()
	// nuke the dist/kiruna directory
	err := os.RemoveAll(filepath.Join(cleanRootDir, "dist", "kiruna"))
	if err != nil {
		return err
	}
	// re-make required directories
	err = MakeRequisiteDirs(config)
	if err != nil {
		return err
	}
	return nil
}

func runPrecompileTasks(config *common.Config) error {
	cleanRootDir := config.GetCleanRootDir()

	// Must be complete before BuildCSS in case the CSS references any public files
	err := handlePublicFiles(cleanRootDir)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 2) // Buffer to hold up to 2 errors
	wg.Add(2)                      // Two tasks to do concurrently

	// goroutine 1
	go func() {
		defer wg.Done()
		if err = copyPrivateFiles(cleanRootDir); err != nil {
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

func Build(config *common.Config, recompileBinary bool) error {
	err := setupNewBuild(config)
	if err != nil {
		return err
	}
	err = runPrecompileTasks(config)
	if err != nil {
		return err
	}
	if recompileBinary {
		err = BuildApp(config)
		if err != nil {
			return err
		}
	}
	return nil
}
