package runtime

import "io/fs"

type UniversalFS struct {
	FS fs.FS
}

func newUniversalFS(fs fs.FS) *UniversalFS {
	return &UniversalFS{FS: fs}
}

func (u *UniversalFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(u.FS, name)
}

func (u *UniversalFS) Open(name string) (fs.File, error) {
	return u.FS.Open(name)
}

func (u *UniversalFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(u.FS, name)
}

func (u *UniversalFS) Sub(dir string) (*UniversalFS, error) {
	subFS, err := fs.Sub(u.FS, dir)
	if err != nil {
		return nil, err
	}
	FS := newUniversalFS(subFS)
	return FS, nil
}
