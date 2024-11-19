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
	b := time.Now()
	c.Logger.Infof("Go binary compilation took: %v", b.Sub(a))
	if err != nil {
		return fmt.Errorf("error compiling binary: %v", err)
	}
	c.Logger.Infof("compilation complete: %s", buildDest)
	return nil
}
