package ik

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/sjc5/kit/pkg/grace"
)

type PIDFile struct {
	cleanDistDir string
}

func newPIDFile(cleanDistDir string) *PIDFile {
	return &PIDFile{cleanDistDir: cleanDistDir}
}

func (p *PIDFile) getPIDFileRef() string {
	return filepath.Join(p.cleanDistDir, distKirunaDir, internalDir, pidFile)
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
	cleanDirs := c.getCleanDirs()
	pidFile := &PIDFile{cleanDistDir: cleanDirs.Dist}
	priorPID, err := pidFile.readPIDFile()
	if err != nil {
		c.Logger.Error(fmt.Sprintf("Error reading PID file: %v", err))
		return
	}
	if priorPID <= 0 {
		return
	}

	priorProcess, err := os.FindProcess(priorPID)
	if err != nil {
		c.Logger.Error(fmt.Sprintf("error finding process with PID %d: %v", priorPID, err))
		return
	}

	// Check if the process is running
	err = priorProcess.Signal(syscall.Signal(0))
	if err != nil {
		if err == os.ErrProcessDone {
			c.Logger.Info(fmt.Sprintf("process with PID %d is already terminated", priorPID))
		} else {
			c.Logger.Error(fmt.Sprintf("error checking process with PID %d: %v", priorPID, err))
		}
		return
	}

	// Process is running, attempt to terminate it
	if err := grace.TerminateProcess(priorProcess, 5*time.Second, c.Logger); err != nil {
		if !errors.Is(err, os.ErrProcessDone) {
			c.Logger.Error(fmt.Sprintf("Error terminating prior app with PID %d: %v", priorPID, err))
		}
		return
	}

	c.Logger.Info(fmt.Sprintf("Killed prior app with PID %d", priorPID))

	// Wait for the process to fully terminate
	_, err = priorProcess.Wait()
	if err != nil {
		c.Logger.Error(fmt.Sprintf("Error waiting for process %d to terminate: %v", priorPID, err))
		// now just move on, not the end of the world
	}

	// Remove the PID file
	err = pidFile.deletePIDFile()
	if err != nil {
		c.Logger.Error(fmt.Sprintf("Error removing PID file: %v", err))
	}
}

func (c *Config) writePIDFile(pid int) error {
	cleanDirs := c.getCleanDirs()
	pidFile := newPIDFile(cleanDirs.Dist)
	return pidFile.writePIDFile(pid)
}

func (c *Config) deletePIDFile() error {
	cleanDirs := c.getCleanDirs()
	pidFile := newPIDFile(cleanDirs.Dist)
	return pidFile.deletePIDFile()
}
