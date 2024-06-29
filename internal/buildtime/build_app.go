package buildtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/common"
)

// If you want to do a custom build command, just use
// Kiruna.BuildWithoutCompilingGo() instead of Kiruna.Build(),
// and then you can control your build yourself afterwards.
func CompileBinary(config *common.Config) error {
	cleanRootDir := config.GetCleanRootDir()
	buildDest := filepath.Join(cleanRootDir, "dist/bin/main")
	entryPoint := filepath.Join(cleanRootDir, config.EntryPoint)
	buildCmd := exec.Command("go", "build", "-o", buildDest, entryPoint)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	err := buildCmd.Run()
	if err != nil {
		return fmt.Errorf("error compiling binary: %v", err)
	}
	config.Logger.Infof("compilation complete: %s", buildDest)
	return nil
}
