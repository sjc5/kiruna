package buildtime

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

// If you want to do a custom build command, just use
// Kiruna.BuildWithoutCompilingGo() instead of Kiruna.Build(),
// and then you can control your build yourself afterwards.
func BuildApp(config *common.Config) error {
	cleanRootDir := config.GetCleanRootDir()
	if config.BinOutputFilename == "" {
		config.BinOutputFilename = "main"
	}
	buildDest := filepath.Join(cleanRootDir, "dist", "bin", config.BinOutputFilename)
	entryPoint := filepath.Join(cleanRootDir, config.EntryPoint)
	buildCmd := exec.Command("go", "build", "-o", buildDest, entryPoint)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	err := buildCmd.Run()
	if err != nil {
		util.Log.Errorf("error building app: %v", err)
		return err
	}
	util.Log.Infof("compilation complete: %s", buildDest)
	return nil
}
