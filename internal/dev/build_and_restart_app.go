package dev

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/common"
)

var lastBuildCmd *exec.Cmd

func mustKillAppDev(config *common.Config) {
	if lastBuildCmd != nil {
		if err := lastBuildCmd.Process.Kill(); err != nil {
			errMsg := fmt.Sprintf(
				"error: failed to kill running app with pid %d: %v",
				lastBuildCmd.Process.Pid,
				err,
			)
			config.Logger.Error(errMsg)
			panic(errMsg)
		}
	}
}

func mustStartAppDev(config *common.Config) {
	buildDest := filepath.Join(config.GetCleanRootDir(), "dist/bin/main")
	lastBuildCmd = exec.Command(buildDest)
	lastBuildCmd.Stdout = os.Stdout
	lastBuildCmd.Stderr = os.Stderr
	if err := lastBuildCmd.Start(); err != nil {
		errMsg := fmt.Sprintf("error: failed to start app: %v", err)
		config.Logger.Error(errMsg)
		panic(errMsg)
	}
	config.Logger.Infof("app started with pid %d", lastBuildCmd.Process.Pid)
}
