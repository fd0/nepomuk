package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	"golang.org/x/net/webdav"
)

// webDAVFS adapts a webdav.FileSystem to fs.FS.
type webDAVFS struct {
	fs webdav.FileSystem
}

// statically ensure that webDAVFS implements fs.FS.
var _ fs.StatFS = &webDAVFS{}

// newWebDAVFS adapts fs to a fs.FS.
func newWebDAVFS(fs webdav.FileSystem) *webDAVFS {
	return &webDAVFS{fs}
}

// Open opens the named file.
// When Open returns an error, it should be of type *PathError
// with the Op field set to "open", the Path field set to name,
// and the Err field describing the problem.
//
// Open should reject attempts to open names that do not satisfy
// ValidPath(name), returning a *PathError with Err set to
// ErrInvalid or ErrNotExist.
func (fs *webDAVFS) Open(name string) (fs.File, error) {
	f, err := fs.fs.OpenFile(context.Background(), name, os.O_RDONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}

	return &webDAVFile{f}, nil
}

// Stat returns a FileInfo describing the file.
// If there is an error, it should be of type *PathError.
func (fs *webDAVFS) Stat(name string) (fs.FileInfo, error) {
	return fs.fs.Stat(context.Background(), name)
}

// RemoveAll removes path and any children it contains. It removes everything
// it can but returns the first error it encounters. If the path does not
// exist, RemoveAll returns nil (no error). If there is an error, it will be of
// type *PathError.
func (fs *webDAVFS) RemoveAll(name string) error {
	return fs.fs.RemoveAll(context.Background(), name)
}

// webDAVFile adapts webdav.File to fs.File (different name for ReadDir() vs. Readdir()).
type webDAVFile struct {
	webdav.File
}

var _ fs.ReadDirFile = &webDAVFile{}

// ReadDir reads the contents of the directory and returns
// a slice of up to n DirEntry values in directory order.
// Subsequent calls on the same file will yield further DirEntry values.
//
// If n > 0, ReadDir returns at most n DirEntry structures.
// In this case, if ReadDir returns an empty slice, it will return
// a non-nil error explaining why.
// At the end of a directory, the error is io.EOF.
// (ReadDir must return io.EOF itself, not an error wrapping io.EOF.)
//
// If n <= 0, ReadDir returns all the DirEntry values from the directory
// in a single slice. In this case, if ReadDir succeeds (reads all the way
// to the end of the directory), it returns the slice and a nil error.
// If it encounters an error before the end of the directory,
// ReadDir returns the DirEntry list read until that point and a non-nil error.
func (f *webDAVFile) ReadDir(n int) ([]fs.DirEntry, error) {
	entries, err := f.File.Readdir(n)
	if err != nil {
		return nil, fmt.Errorf("ReadDir: %w", err)
	}

	list := make([]fs.DirEntry, 0, len(entries))
	for _, entry := range entries {
		list = append(list, fs.FileInfoToDirEntry(entry))
	}

	return list, nil
}
