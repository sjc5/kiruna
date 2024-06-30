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

const fsTypeDev = "dev"

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
	return newUniversalFS(subFS), nil
}

func newUniversalFS(fs fs.FS) UniversalFS {
	return &universalFS{FS: fs}
}

func (c *Config) GetIsUsingEmbeddedFS() bool {
	return c.DistFS != nil
}

func (c *Config) GetUniversalDirFS() UniversalFS {
	if hit, isCached := cache.uniDirFS.Load(c); isCached {
		return hit
	}
	fs := newUniversalFS(os.DirFS(path.Join(c.getCleanRootDir(), distKirunaDir)))
	actualFS, _ := cache.uniDirFS.LoadOrStore(c, fs)
	return actualFS
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
	return c.getFS(publicDir)
}

func (c *Config) GetPrivateFS() (UniversalFS, error) {
	return c.getFS(privateDir)
}

// GetUniversalFS returns a filesystem interface that works across different environments (dev/prod)
// and supports both embedded and non-embedded filesystems.
func (c *Config) GetUniversalFS() (UniversalFS, error) {
	if hit, isCached := cache.uniFS.Load(c); isCached {
		cachedFSType, _ := cache.fsType.Load(c)
		skipCache := KirunaEnv.GetIsDev() && cachedFSType != fsTypeDev
		if !skipCache {
			return hit, nil
		}
	}

	// DEV
	// There is an expectation that you run the dev server from the root of your project,
	// where your go.mod file is.
	if KirunaEnv.GetIsDev() {
		// ensures "needsReset" is always true in dev
		cache.fsType.Store(c, fsTypeDev)

		c.Logger.Infof("using disk file system (development)")
		fs := newUniversalFS(os.DirFS(path.Join(c.getCleanRootDir(), distKirunaDir)))
		actualFS, _ := cache.uniFS.LoadOrStore(c, fs) // cache the fs
		return actualFS, nil
	}

	// PROD
	// If we are using the embedded file system, we should use the dist file system
	if c.GetIsUsingEmbeddedFS() {
		c.Logger.Infof("using embedded file system (production)")

		// Assuming the embed directive looks like this:
		// //go:embed kiruna
		// That means that the kiruna folder itself (not just its contents) is embedded.
		// So we have to drop down into the kiruna folder here.
		FS, err := fs.Sub(c.DistFS, "kiruna")
		if err != nil {
			return nil, err
		}
		fs := newUniversalFS(FS)
		actualFS, _ := cache.uniFS.LoadOrStore(c, fs) // cache the fs
		return actualFS, nil
	}

	// PROD
	// If we are not using the embedded file system, we should use the os file system,
	// and assume that the executable is a sibling to the kiruna-outputted "kiruna" directory
	c.Logger.Infof("using disk file system (production)")
	execDir, err := executil.GetExecutableDir()
	if err != nil {
		return nil, err
	}
	fs := newUniversalFS(os.DirFS(execDir))
	actualFS, _ := cache.uniFS.LoadOrStore(c, fs) // cache the fs
	return actualFS, nil
}
