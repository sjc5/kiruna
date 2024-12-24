package ik

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (c *Config) compileBinary() error {
	cleanDirs := c.getCleanDirs()
	buildDest := filepath.Join(cleanDirs.Dist, binOutPath)
	buildCmd := exec.Command("go", "build", "-o", buildDest, c.MainAppEntry)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	a := time.Now()
	err := buildCmd.Run()
	if err != nil {
		return fmt.Errorf("error compiling binary: %v", err)
	}
	c.Logger.Info("Compiled Go binary", "duration", time.Since(a), "buildDest", buildDest)
	return nil
}
