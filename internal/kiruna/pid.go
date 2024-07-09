package ik

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type PIDFile struct {
	cleanRootDir string
}

func newPIDFile(cleanRootDir string) *PIDFile {
	return &PIDFile{cleanRootDir: cleanRootDir}
}

func (p *PIDFile) getPIDFileRef() string {
	return filepath.Join(p.cleanRootDir, distKirunaDir, internalDir, pidFile)
}

func (p *PIDFile) writePIDFile(pid int) error {
	return os.WriteFile(p.getPIDFileRef(), []byte(strconv.Itoa(pid)), 0644)
}

func (p *PIDFile) readPIDFile() (int, error) {
	data, err := os.ReadFile(p.getPIDFileRef())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("error reading PID file: %v", err)
	}
	return strconv.Atoi(string(data))
}

func (p *PIDFile) deletePIDFile() error {
	return os.Remove(p.getPIDFileRef())
}

func (c *Config) killPriorPID() {
	pidFile := &PIDFile{cleanRootDir: c.getCleanRootDir()}

	priorPID, err := pidFile.readPIDFile()
	if err == nil && priorPID > 0 {
		priorProcess, _ := os.FindProcess(priorPID)
		if priorProcess != nil {
			if err := priorProcess.Kill(); err != nil {
				if !errors.Is(err, os.ErrProcessDone) {
					errMsg := fmt.Sprintf("error: failed to kill prior app with pid %d: %v", priorPID, err)
					c.Logger.Error(errMsg)
				}
			} else {
				c.Logger.Infof("killed prior app with pid %d", priorPID)
			}
		}
	}
}

func (c *Config) writePIDFile(pid int) error {
	pidFile := newPIDFile(c.getCleanRootDir())
	return pidFile.writePIDFile(pid)
}

func (c *Config) deletePIDFile() error {
	pidFile := newPIDFile(c.getCleanRootDir())
	return pidFile.deletePIDFile()
}
