package ik

import (
	"path/filepath"
	"testing"
)

func TestGetCleanRootDir(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	expected := filepath.Clean(env.config.RootDir)
	actual := env.config.getCleanRootDir()

	if actual != expected {
		t.Errorf("getCleanRootDir() = %v, want %v", actual, expected)
	}
}
