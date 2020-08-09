package main

import (
	"errors"
	"io"
	"os"

	"goftp.io/server/core"
)

type fileinfo struct {
	os.FileInfo
}

func (fileinfo) Owner() string {
	return "root"
}

func (fileinfo) Group() string {
	return "root"
}

// Driver implements an FTP file system.
type Driver struct {
	targetdir string
}

func (Driver) Stat(filename string) (core.FileInfo, error) {
	fi, err := os.Lstat(".")
	if err != nil {
		return nil, err
	}

	return fileinfo{fi}, nil
}

func (Driver) ListDir(string, func(core.FileInfo) error) error {
	return errors.New("not implemented")
}

func (Driver) DeleteDir(string) error {
	return errors.New("not implemented")
}

func (Driver) DeleteFile(string) error {
	return errors.New("not implemented")
}

func (Driver) Rename(string, string) error {
	return errors.New("not implemented")
}

func (Driver) MakeDir(string) error {
	return errors.New("not implemented")
}

func (Driver) GetFile(string, int64) (int64, io.ReadCloser, error) {
	return 0, nil, errors.New("not implemented")
}

func (d Driver) PutFile(filename string, rd io.Reader, appendData bool) (int64, error) {
	return SaveFile(d.targetdir, filename, rd)
}

// Factory implements a factory for creating an FTP file system using Driver.
type Factory struct {
	targetdir string
}

func (f Factory) NewDriver() (core.Driver, error) {
	return Driver{targetdir: f.targetdir}, nil // nolint:gosimple
}

type AllowAll struct{}

func (AllowAll) CheckPasswd(string, string) (bool, error) {
	return true, nil
}
