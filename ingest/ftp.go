package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	targetdir    string
	OnFileUpload func(filename string)
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

const filenameFormat = "20060102-150405"

func (d Driver) PutFile(path string, rd io.Reader, appendData bool) (int64, error) {
	ext := filepath.Ext(path)
	basename := filepath.Base(path)
	suffix := ""

	switch {
	case strings.HasPrefix(basename, "duplex-odd"):
		suffix = "_duplex-odd"
	case strings.HasPrefix(basename, "duplex-even"):
		suffix = "_duplex-even"
	}

	name := time.Now().Format(filenameFormat) + suffix + ext
	filename := filepath.Join(d.targetdir, name)

	f, err := os.Create(filename)
	if err != nil {
		log.Printf("PutFile: create: %v", err)
		return 0, fmt.Errorf("create: %w", err)
	}

	n, err := io.Copy(f, rd)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())

		log.Printf("PutFile: copy: %v", err)
		return n, fmt.Errorf("copy: %w", err)
	}

	err = f.Close()
	if err != nil {
		log.Printf("PutFile: close: %v", err)
		return n, fmt.Errorf("close: %w", err)
	}

	d.OnFileUpload(filepath.Join(d.targetdir, filename))

	return n, err
}

// Factory implements a factory for creating an FTP file system using Driver.
type Factory struct {
	targetdir    string
	OnFileUpload func(filename string)
}

func (f Factory) NewDriver() (core.Driver, error) {
	return Driver{targetdir: f.targetdir, OnFileUpload: f.OnFileUpload}, nil // nolint:gosimple
}

type AllowAll struct{}

func (AllowAll) CheckPasswd(string, string) (bool, error) {
	return true, nil
}

// RunFTPServer runs an ftp server on the given address, uploaded files will be
// placed into targetDir. After the upload is finished, onFileUpload is run
// with the new filename.
func RunFTPServer(ctx context.Context, targetDir string, verbose bool, bindaddr string, onFileUpload func(filename string)) error {
	serverOpts := &core.ServerOpts{
		WelcomeMessage: "Nepomuk Archive System",
		Auth:           AllowAll{},
		Factory: Factory{
			targetdir:    targetDir,
			OnFileUpload: onFileUpload,
		},
	}

	if !verbose {
		serverOpts.Logger = &core.DiscardLogger{}
	}

	srv := core.NewServer(serverOpts)

	var listener net.Listener

	listener, err := net.Listen("tcp", bindaddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	ch := make(chan error, 1)

	go func() {
		ch <- srv.Serve(listener)
	}()

	select {
	case err := <-ch:
		lerr := listener.Close()
		if err == nil {
			err = lerr
		}
		return err
	case <-ctx.Done():
		err := listener.Close()
		return err
	}
}
