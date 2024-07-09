package ik

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (c *Config) compileBinary() error {
	cleanRootDir := c.getCleanRootDir()
	buildDest := filepath.Join(cleanRootDir, binOutPath)
	entryPoint := filepath.Join(cleanRootDir, c.EntryPoint)
	buildCmd := exec.Command("go", "build", "-o", buildDest, entryPoint)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	a := time.Now()
	err := buildCmd.Run()
	b := time.Now()
	c.Logger.Infof("Go binary compilation took: %v", b.Sub(a))
	if err != nil {
		return fmt.Errorf("error compiling binary: %v", err)
	}
	c.Logger.Infof("compilation complete: %s", buildDest)
	return nil
}
