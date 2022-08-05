package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"goftp.io/server/core"
)

var log logrus.FieldLogger = logrus.StandardLogger()

// SetLogger updates the logger to use.
func SetLogger(logger logrus.FieldLogger) {
	log = logger.WithField("component", "ftp-server")
}

type fileinfo struct {
	os.FileInfo
}

func (fileinfo) Owner() string {
	return "root"
}

func (fileinfo) Group() string {
	return "root"
}

// driver implements an FTP file system.
type driver struct {
	targetdir    string
	OnFileUpload func(filename string)
}

func (driver) Stat(filename string) (core.FileInfo, error) {
	fi, err := os.Lstat(".")
	if err != nil {
		return nil, fmt.Errorf("lstat %v: %w", filename, err)
	}

	return fileinfo{fi}, nil
}

func (driver) ListDir(string, func(core.FileInfo) error) error {
	return errors.New("not implemented")
}

func (driver) DeleteDir(string) error {
	return errors.New("not implemented")
}

func (driver) DeleteFile(string) error {
	return errors.New("not implemented")
}

func (driver) Rename(string, string) error {
	return errors.New("not implemented")
}

func (driver) MakeDir(string) error {
	return errors.New("not implemented")
}

func (driver) GetFile(string, int64) (int64, io.ReadCloser, error) {
	return 0, nil, errors.New("not implemented")
}

const UploadFilenameTimeFormat = "20060102-150405"

func (d driver) PutFile(path string, rd io.Reader, appendData bool) (int64, error) {
	ext := filepath.Ext(path)

	if ext != ".pdf" {
		log.Printf("PutFile: rejecting invalid extension %q: %q", path, ext)

		return -1, errors.New("invalid extension")
	}

	basename := filepath.Base(path)
	suffix := ""

	switch {
	case strings.HasPrefix(basename, "duplex-odd"):
		suffix = "_duplex-odd"
	case strings.HasPrefix(basename, "duplex-even"):
		suffix = "_duplex-even"
	}

	name := time.Now().Format(UploadFilenameTimeFormat) + suffix + ext
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

	d.OnFileUpload(filename)

	return n, nil
}

// factory implements a factory for creating an FTP file system using driver.
type factory struct {
	targetdir    string
	OnFileUpload func(filename string)
}

func (f factory) NewDriver() (core.Driver, error) {
	return driver{targetdir: f.targetdir, OnFileUpload: f.OnFileUpload}, nil // nolint:gosimple
}

type allowAll struct{}

func (allowAll) CheckPasswd(string, string) (bool, error) {
	return true, nil
}

// FTPServer implements an FTP server which only supports uploading files. The
// files will be placed in TargetDir and the callback OnFileUpload is run after
// an upload completed.
type FTPServer struct {
	TargetDir string
	Verbose   bool
	Bind      string

	OnFileUpload func(filename string)
}

// Run starts the server. When ctx is canceled, the listener is stopped.
func (srv *FTPServer) Run(ctx context.Context) error {
	log.Debugf("start server on %v", srv.Bind)

	// process all pre-existing files
	entries, err := ioutil.ReadDir(srv.TargetDir)
	if err != nil {
		return fmt.Errorf("readdir %v: %w", srv.TargetDir, err)
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		srv.OnFileUpload(filepath.Join(srv.TargetDir, entry.Name()))
	}

	serverOpts := &core.ServerOpts{
		WelcomeMessage: "Nepomuk Archive System",
		Auth:           allowAll{},
		Factory: factory{
			targetdir:    srv.TargetDir,
			OnFileUpload: srv.OnFileUpload,
		},
	}

	if !srv.Verbose {
		serverOpts.Logger = &core.DiscardLogger{}
	}

	ftpServer := core.NewServer(serverOpts)

	listener, err := net.Listen("tcp", srv.Bind)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	ch := make(chan error, 1)

	go func() {
		ch <- ftpServer.Serve(listener)
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
		if err != nil {
			return fmt.Errorf("listen: %w", listener.Close())
		}

		return nil
	}
}
