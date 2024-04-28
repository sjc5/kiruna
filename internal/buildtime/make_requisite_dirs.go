package buildtime

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjc5/kiruna/internal/common"
)

func MakeRequisiteDirs(config *common.Config) error {
	cleanRootDir := config.GetCleanRootDir()
	if config.DevConfig != nil && config.DevConfig.ServerOnly {
		return nil
	}
	// make a dist/kiruna/internal directory
	path := filepath.Join(cleanRootDir, "dist", "kiruna", "internal")
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making internal directory: %v", err)
	}
	// add a x file so that go:embed doesn't complain
	path = filepath.Join(cleanRootDir, "dist", "kiruna", "x")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		return fmt.Errorf("error making x file: %v", err)
	}
	// need an empty dist/kiruna/public directory
	path = filepath.Join(cleanRootDir, "dist", "kiruna", "static", "public")
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making public directory: %v", err)
	}
	// need an empty dist/kiruna/private directory
	path = filepath.Join(cleanRootDir, "dist", "kiruna", "static", "private")
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("error making private directory: %v", err)
	}
	return nil
}
