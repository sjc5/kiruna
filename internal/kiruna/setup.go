package ik

import (
	"fmt"
	"os"
	"path/filepath"
)

func SetupDistDir(rootDir string) error {
	cleanRootDir := filepath.Clean(rootDir)

	// make a dist/kiruna/internal directory
	path := filepath.Join(cleanRootDir, distKirunaDir, internalDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making internal directory: %v", err)
	}

	// add a x file so that go:embed doesn't complain
	path = filepath.Join(cleanRootDir, distKirunaDir, "x")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		return fmt.Errorf("error making x file: %v", err)
	}

	// need an empty dist/kiruna/public directory
	path = filepath.Join(cleanRootDir, distKirunaDir, staticDir, publicDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making public directory: %v", err)
	}

	// need an empty dist/kiruna/private directory
	path = filepath.Join(cleanRootDir, distKirunaDir, staticDir, privateDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making private directory: %v", err)
	}

	return nil
}
