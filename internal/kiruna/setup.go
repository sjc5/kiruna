package ik

import (
	"fmt"
	"os"
	"path/filepath"
)

func SetupDistDir(distDir string) error {
	cleanDistDir := filepath.Clean(distDir)

	// make a dist/kiruna/internal directory
	path := filepath.Join(cleanDistDir, distKirunaDir, internalDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making internal directory: %v", err)
	}

	// add an empty file so that go:embed doesn't complain
	path = filepath.Join(cleanDistDir, distKirunaDir, goEmbedFixerFile)
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		return fmt.Errorf("error making x file: %v", err)
	}

	// need an empty dist/kiruna/static/public/kiruna_internal__ directory
	path = filepath.Join(cleanDistDir, distKirunaDir, staticDir, publicDir, publicInternalDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making public directory: %v", err)
	}

	// need an empty dist/kiruna/static/private directory
	path = filepath.Join(cleanDistDir, distKirunaDir, staticDir, privateDir)
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making private directory: %v", err)
	}

	return nil
}
