package ik

import (
	"os"
	"testing"
)

func TestGetPublicFS(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	env.createTestFile(t, "dist/kiruna/static/public/test.txt", "public content")

	publicFS, err := env.config.GetPublicFS()
	if err != nil {
		t.Fatalf("GetPublicFS() error = %v", err)
	}

	content, err := publicFS.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(content) != "public content" {
		t.Errorf("ReadFile() content = %v, want %v", string(content), "public content")
	}
}

func TestGetPrivateFS(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	env.createTestFile(t, "dist/kiruna/static/private/test.txt", "private content")

	privateFS, err := env.config.GetPrivateFS()
	if err != nil {
		t.Fatalf("GetPrivateFS() error = %v", err)
	}

	content, err := privateFS.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(content) != "private content" {
		t.Errorf("ReadFile() content = %v, want %v", string(content), "private content")
	}
}

func TestGetUniversalFS(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	env.createTestFile(t, "dist/kiruna/static/public/test.txt", "universal content")

	universalFS, err := env.config.GetUniversalFS()
	if err != nil {
		t.Fatalf("GetUniversalFS() error = %v", err)
	}

	content, err := universalFS.ReadFile("static/public/test.txt")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(content) != "universal content" {
		t.Errorf("ReadFile() content = %v, want %v", string(content), "universal content")
	}

	// Test Sub method
	subFS, err := universalFS.Sub("static/public")
	if err != nil {
		t.Fatalf("Sub() error = %v", err)
	}

	content, err = subFS.ReadFile("test.txt")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(content) != "universal content" {
		t.Errorf("ReadFile() content = %v, want %v", string(content), "universal content")
	}

	// Test ReadDir method
	entries, err := universalFS.ReadDir("static")
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	expectedDirs := map[string]bool{"public": true, "private": true}
	for _, entry := range entries {
		if entry.IsDir() {
			if !expectedDirs[entry.Name()] {
				t.Errorf("Unexpected directory: %s", entry.Name())
			}
			delete(expectedDirs, entry.Name())
		}
	}

	if len(expectedDirs) > 0 {
		t.Errorf("Missing expected directories: %v", expectedDirs)
	}
}

func TestFSEdgeCases(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	t.Run("NonexistentFile", func(t *testing.T) {
		universalFS, _ := env.config.GetUniversalFS()
		_, err := universalFS.ReadFile("nonexistent.txt")
		if !os.IsNotExist(err) {
			t.Errorf("Expected os.IsNotExist(err) to be true for nonexistent file, got %v", err)
		}
	})

	t.Run("EmptyFile", func(t *testing.T) {
		env.createTestFile(t, "dist/kiruna/static/public/empty.txt", "")
		universalFS, _ := env.config.GetUniversalFS()
		content, err := universalFS.ReadFile("static/public/empty.txt")
		if err != nil {
			t.Errorf("Unexpected error reading empty file: %v", err)
		}
		if len(content) != 0 {
			t.Errorf("Expected empty content, got %d bytes", len(content))
		}
	})

	t.Run("ReadDirOnFile", func(t *testing.T) {
		env.createTestFile(t, "dist/kiruna/static/public/test.txt", "content")
		universalFS, _ := env.config.GetUniversalFS()
		_, err := universalFS.ReadDir("static/public/test.txt")
		if err == nil {
			t.Errorf("Expected error when calling ReadDir on a file")
		}
	})
}

func TestGetIsUsingEmbeddedFS(t *testing.T) {
	env := setupTestEnv(t)
	defer teardownTestEnv(t)

	// Test when DistFS is set (default in setupTestEnv)
	if !env.config.getIsUsingEmbeddedFS() {
		t.Errorf("getIsUsingEmbeddedFS() = false, want true when DistFS is set")
	}

	// Test when DistFS is nil
	env.config.DistFS = nil
	if env.config.getIsUsingEmbeddedFS() {
		t.Errorf("getIsUsingEmbeddedFS() = true, want false when DistFS is nil")
	}
}
