package ik

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/sjc5/kit/pkg/executil"
)

type UniversalFS interface {
	ReadFile(name string) ([]byte, error)
	Open(name string) (fs.File, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	Sub(dir string) (UniversalFS, error)
}

type universalFS struct {
	FS fs.FS
}

func (u *universalFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(u.FS, name)
}

func (u *universalFS) Open(name string) (fs.File, error) {
	return u.FS.Open(name)
}

func (u *universalFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(u.FS, name)
}

func (u *universalFS) Sub(dir string) (UniversalFS, error) {
	subFS, err := fs.Sub(u.FS, dir)
	if err != nil {
		return nil, err
	}
	return &universalFS{FS: subFS}, nil
}

func (c *Config) getIsUsingEmbeddedFS() bool {
	return c.DistFS != nil
}

func (c *Config) getInitialUniversalDirFS() (UniversalFS, error) {
	cleanDirs := c.getCleanDirs()
	fs := &universalFS{FS: os.DirFS(path.Join(cleanDirs.Dist, distKirunaDir))}
	return fs, nil
}

func (c *Config) getFS(subDir string) (UniversalFS, error) {
	// __LOCATION_ASSUMPTION: Inside "dist/kiruna"
	path := filepath.Join(staticDir, subDir)

	FS, err := c.GetUniversalFS()
	if err != nil {
		errMsg := fmt.Sprintf("error getting %s FS: %v", subDir, err)
		c.Logger.Errorf(errMsg)
		return nil, errors.New(errMsg)
	}
	subFS, err := FS.Sub(path)
	if err != nil {
		errMsg := fmt.Sprintf("error getting %s FS: %v", subDir, err)
		c.Logger.Errorf(errMsg)
		return nil, errors.New(errMsg)
	}
	return subFS, nil
}

func (c *Config) GetPublicFS() (UniversalFS, error) {
	return c.cache.publicFS.Get()
}

func (c *Config) GetPrivateFS() (UniversalFS, error) {
	return c.cache.privateFS.Get()
}

// GetUniversalFS returns a filesystem interface that works across different environments (dev/prod)
// and supports both embedded and non-embedded filesystems.
func (c *Config) GetUniversalFS() (UniversalFS, error) {
	return c.cache.uniFS.Get()
}

// GetUniversalFS returns a filesystem interface that works across different environments (dev/prod)
// and supports both embedded and non-embedded filesystems.
func (c *Config) getInitialUniversalFS() (UniversalFS, error) {
	useVerboseLogs := getUseVerboseLogs()

	// DEV
	// There is an expectation that you run the dev server from the root of your project,
	// where your go.mod file is.
	if getIsDev() {
		if useVerboseLogs {
			c.Logger.Infof("using disk filesystem (dev)")
		}

		cleanDirs := c.getCleanDirs()
		fs := &universalFS{FS: os.DirFS(path.Join(cleanDirs.Dist, distKirunaDir))}
		return fs, nil
	}

	// If we are using the embedded file system, we should use the dist file system
	if c.getIsUsingEmbeddedFS() {
		if useVerboseLogs {
			c.Logger.Infof("using embedded filesystem (prod)")
		}

		// Assuming the embed directive looks like this:
		// //go:embed kiruna
		// That means that the kiruna folder itself (not just its contents) is embedded.
		// So we have to drop down into the kiruna folder here.
		FS, err := fs.Sub(c.DistFS, kirunaDir)
		if err != nil {
			return nil, err
		}

		return &universalFS{FS: FS}, nil
	}

	if useVerboseLogs {
		c.Logger.Infof("using disk filesystem (prod)")
	}

	// If we are not using the embedded file system, we should use the os file system,
	// and assume that the executable is a sibling to the kiruna-outputted "kiruna" directory
	execDir, err := executil.GetExecutableDir()
	if err != nil {
		return nil, err
	}

	return &universalFS{
		FS: os.DirFS(execDir),
	}, nil
}
