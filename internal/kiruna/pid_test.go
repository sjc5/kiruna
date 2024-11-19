package ik

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestPIDFile(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	pidFile := newPIDFile(env.config.getCleanDirs().Dist)

	t.Run("WritePIDFile", func(t *testing.T) {
		testPID := 12345
		err := pidFile.writePIDFile(testPID)
		if err != nil {
			t.Fatalf("writePIDFile() error = %v", err)
		}

		content, err := os.ReadFile(pidFile.getPIDFileRef())
		if err != nil {
			t.Fatalf("Failed to read PID file: %v", err)
		}

		if string(content) != strconv.Itoa(testPID) {
			t.Errorf("PID file content = %v, want %v", string(content), testPID)
		}
	})

	t.Run("ReadPIDFile", func(t *testing.T) {
		testPID := 67890
		err := pidFile.writePIDFile(testPID)
		if err != nil {
			t.Fatalf("writePIDFile() error = %v", err)
		}

		readPID, err := pidFile.readPIDFile()
		if err != nil {
			t.Fatalf("readPIDFile() error = %v", err)
		}

		if readPID != testPID {
			t.Errorf("readPIDFile() = %v, want %v", readPID, testPID)
		}
	})

	t.Run("DeletePIDFile", func(t *testing.T) {
		err := pidFile.writePIDFile(12345)
		if err != nil {
			t.Fatalf("writePIDFile() error = %v", err)
		}

		err = pidFile.deletePIDFile()
		if err != nil {
			t.Fatalf("deletePIDFile() error = %v", err)
		}

		_, err = os.Stat(pidFile.getPIDFileRef())
		if !os.IsNotExist(err) {
			t.Errorf("PID file still exists after deletion")
		}
	})

	t.Run("ReadNonExistentPIDFile", func(t *testing.T) {
		// Ensure PID file doesn't exist
		_ = pidFile.deletePIDFile()

		pid, err := pidFile.readPIDFile()
		if err != nil {
			t.Fatalf("readPIDFile() error = %v", err)
		}

		if pid != 0 {
			t.Errorf("readPIDFile() = %v, want 0 for non-existent file", pid)
		}
	})
}

func TestConfigPIDMethods(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	t.Run("WritePIDFile", func(t *testing.T) {
		testPID := 13579
		err := env.config.writePIDFile(testPID)
		if err != nil {
			t.Fatalf("writePIDFile() error = %v", err)
		}

		pidFile := newPIDFile(env.config.getCleanDirs().Dist)
		content, err := os.ReadFile(pidFile.getPIDFileRef())
		if err != nil {
			t.Fatalf("Failed to read PID file: %v", err)
		}

		if string(content) != strconv.Itoa(testPID) {
			t.Errorf("PID file content = %v, want %v", string(content), testPID)
		}
	})

	t.Run("DeletePIDFile", func(t *testing.T) {
		err := env.config.writePIDFile(24680)
		if err != nil {
			t.Fatalf("writePIDFile() error = %v", err)
		}

		err = env.config.deletePIDFile()
		if err != nil {
			t.Fatalf("deletePIDFile() error = %v", err)
		}

		pidFile := newPIDFile(env.config.getCleanDirs().Dist)
		_, err = os.Stat(pidFile.getPIDFileRef())
		if !os.IsNotExist(err) {
			t.Errorf("PID file still exists after deletion")
		}
	})
}

func TestKillPriorPID(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	// Start a simple process that we can kill
	cmd := exec.Command("sleep", "30")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	pid := cmd.Process.Pid

	// Write the PID to the file
	pidFile := newPIDFile(env.config.getCleanDirs().Dist)
	err = pidFile.writePIDFile(pid)
	if err != nil {
		t.Fatalf("Failed to write PID file: %v", err)
	}

	// Run killPriorPID
	env.config.killPriorPID()

	// Give it a moment to kill the process
	time.Sleep(100 * time.Millisecond)

	// Check if the process was killed
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatalf("Failed to find process: %v", err)
	}

	err = process.Signal(syscall.Signal(0))
	if err == nil {
		t.Errorf("Process %d is still running after killPriorPID", pid)
		// Clean up if the test failed
		_ = process.Kill()
	}

	// The PID file should have been deleted
	_, err = os.Stat(pidFile.getPIDFileRef())
	if !os.IsNotExist(err) {
		t.Errorf("PID file still exists after killPriorPID")
	}
}

func TestPIDFileLocation(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	pidFile_ := newPIDFile(env.config.getCleanDirs().Dist)

	expectedLocation := filepath.Join(env.config.getCleanDirs().Dist, distKirunaDir, internalDir, pidFile)
	actualLocation := pidFile_.getPIDFileRef()

	if actualLocation != expectedLocation {
		t.Errorf("PID file location = %v, want %v", actualLocation, expectedLocation)
	}
}
