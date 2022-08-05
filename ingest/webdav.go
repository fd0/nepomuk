package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/webdav"
)

// fakeDir simulates an empty directory.
type fakeDir struct {
	name string
}

func (fi *fakeDir) Name() string {
	return fi.name
}

func (fi *fakeDir) Size() int64 {
	return 0
}

func (fi *fakeDir) Mode() fs.FileMode {
	return os.ModeDir | 0o755
}

func (fi *fakeDir) ModTime() time.Time {
	return time.Now()
}

func (fi *fakeDir) IsDir() bool {
	return true
}

func (fi *fakeDir) Sys() any {
	return nil
}

func (fi *fakeDir) Close() error {
	return nil
}

func (fi *fakeDir) Read([]byte) (int, error) {
	return 0, syscall.EIO
}

func (fi *fakeDir) Readdir(int) ([]fs.FileInfo, error) {
	return []fs.FileInfo{}, nil
}

func (fi *fakeDir) Seek(offset int64, whence int) (int64, error) {
	return 0, syscall.EIO
}

func (fi *fakeDir) Stat() (fs.FileInfo, error) {
	return fi, nil
}

func (fi *fakeDir) Write(p []byte) (int, error) {
	return 0, syscall.EIO
}

// ensure that *Dir implements fs.FileInfo and fs.Dir.
var _ fs.FileInfo = &fakeDir{}
var _ webdav.File = &fakeDir{}

// fakeFile simulates a new file. Write are passed through to the embedded WriteCloser.
type fakeFile struct {
	name string
	size int64

	bytesWritten int
	maxSize      int

	io.WriteCloser
}

func (fi *fakeFile) Name() string {
	return fi.name
}

func (fi *fakeFile) Size() int64 {
	return fi.size
}

func (fi *fakeFile) Mode() fs.FileMode {
	return 0o644
}

func (fi *fakeFile) ModTime() time.Time {
	return time.Now()
}

func (fi *fakeFile) IsDir() bool {
	return false
}

func (fi *fakeFile) Sys() any {
	return nil
}

func (fi *fakeFile) Read(p []byte) (int, error) {
	return 0, errors.New("not implemented")
}

func (fi *fakeFile) Readdir(int) ([]fs.FileInfo, error) {
	return nil, errors.New("not a directory")
}

func (fi *fakeFile) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("not implemented")
}

func (fi *fakeFile) Stat() (fs.FileInfo, error) {
	return fi, nil
}

// Write writes data to the underlying writer until enough bytes have been
// written.
func (fi *fakeFile) Write(p []byte) (int, error) {
	if fi.bytesWritten >= fi.maxSize {
		return 0, errors.New("file is full")
	}

	if fi.bytesWritten+len(p) > fi.maxSize {
		diff := fi.maxSize - fi.bytesWritten
		p = p[:diff]
	}

	n, err := fi.WriteCloser.Write(p)
	fi.bytesWritten += n

	return n, err
}

func (fi *fakeFile) Close() error {
	return fi.WriteCloser.Close()
}

// ensure that *File implements fs.FileInfo and fs.File.
var _ fs.FileInfo = &fakeFile{}
var _ webdav.File = &fakeFile{}

// UploadOnlyFS is a filesytem implementation that only allows uploads. The
// uploaded file name is discarded and a new file is opened using the callback.
// After MaxFileSize, the upload is cancelled.
type UploadOnlyFS struct {
	Log logrus.FieldLogger

	MaxFileSize int

	Create func(name string) (io.WriteCloser, error)
}

// DefaultMaxFileSize is used when MaxFileSize is zero.
const DefaultMaxFileSize = 50 * 1024 * 1024

// ensure that UploadOnlyFS implements webdav.FileSystem.
var _ webdav.FileSystem = &UploadOnlyFS{}

// Mkdir creates a new directory.
func (fs *UploadOnlyFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	fs.Log.Debugf("mkdir %v -> not implemented", name)

	return errors.New("not implemented")
}

// OpenFile opens a file.
//
// nolint:ireturn
func (fs *UploadOnlyFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	fs.Log.Debugf("OpenFile %v %v (0x%x) %v", name, flag, flag, perm)

	if name == "/" {
		if flag != os.O_RDONLY {
			fs.Log.Warnf("rejecting OpenFile %v with flag 0x%x", name, flag)

			return nil, syscall.EPERM
		}

		return &fakeDir{name: "/"}, nil
	}

	if flag == os.O_TRUNC|os.O_CREATE|os.O_RDWR {
		maxSize := fs.MaxFileSize
		if maxSize == 0 {
			maxSize = DefaultMaxFileSize
		}

		fs.Log.Infof("upload file %v", name)

		f, err := fs.Create(name)
		if err != nil {
			return nil, fmt.Errorf("create new file: %w", err)
		}

		return &fakeFile{name: name, size: 0, maxSize: maxSize, WriteCloser: f}, nil
	}

	return nil, syscall.EPERM
}

// RemoveAll recursively deletes name.
func (fs *UploadOnlyFS) RemoveAll(ctx context.Context, name string) error {
	fs.Log.Debugf("removeall %v -> not implemented", name)

	return errors.New("not implemented")
}

// Rename renames a file.
func (fs *UploadOnlyFS) Rename(ctx context.Context, oldName string, newName string) error {
	fs.Log.Debugf("rename %v, %v -> not implemented", oldName, newName)

	return errors.New("not implemented")
}

// Stat returns metadata about an item.
func (fs *UploadOnlyFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if name == "/" {
		return &fakeDir{name: "."}, nil
	}

	return nil, errors.New("not found")
}
