package util

import (
	"os"
	"path/filepath"
)

func GetExecDir() string {
	execPath, err := os.Executable()
	if err != nil {
		Log.Errorf("error getting executable path: %v", err)
	}
	return filepath.Dir(execPath)
}
