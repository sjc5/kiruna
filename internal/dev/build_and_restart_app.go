package dev

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/buildtime"
	"github.com/sjc5/kiruna/internal/common"
	"github.com/sjc5/kiruna/internal/util"
)

var lastBuildCmd *exec.Cmd

func mustBuild(config *common.Config) {
	err := buildtime.BuildApp(config)
	if err != nil {
		util.Log.Panicf("error: failed to build app: %v", err)
	}
}

func killAppDev() {
	if lastBuildCmd != nil {
		if err := lastBuildCmd.Process.Kill(); err != nil {
			util.Log.Panicf("error: failed to kill running app: %v", err)
		}
	}
}

func startAppDev(config *common.Config) {
	if config.BinOutputFilename == "" {
		config.BinOutputFilename = "main"
	}
	buildDest := filepath.Join(config.GetCleanRootDir(), "dist", "bin", config.BinOutputFilename)
	lastBuildCmd = exec.Command(buildDest)
	lastBuildCmd.Stdout = os.Stdout
	lastBuildCmd.Stderr = os.Stderr
	if err := lastBuildCmd.Start(); err != nil {
		util.Log.Panicf("error: failed to start app: %v", err)
		return
	}
	util.Log.Infof("app started with pid %d", lastBuildCmd.Process.Pid)
}
