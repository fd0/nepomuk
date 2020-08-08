package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/coreos/go-systemd/activation"
	"github.com/spf13/pflag"
	"goftp.io/server/core"
)

var opts = struct {
	TargetDir string
	Listen    string
}{}

type Driver struct{}

type fileinfo struct {
	os.FileInfo
}

func (fileinfo) Owner() string {
	return "root"
}

func (fileinfo) Group() string {
	return "root"
}

// params  - a file path
// returns - a time indicating when the requested path was last modified
//         - an error if the file doesn't exist or the user lacks
//           permissions
func (Driver) Stat(filename string) (core.FileInfo, error) {
	fi, err := os.Lstat(opts.TargetDir)
	if err != nil {
		return nil, err
	}

	return fileinfo{fi}, nil
}

// params  - path, function on file or subdir found
// returns - error
//           path
func (Driver) ListDir(string, func(core.FileInfo) error) error {
	return errors.New("not implemented")
}

// params  - path
// returns - nil if the directory was deleted or any error encountered
func (Driver) DeleteDir(string) error {
	return errors.New("not implemented")
}

// params  - path
// returns - nil if the file was deleted or any error encountered
func (Driver) DeleteFile(string) error {
	return errors.New("not implemented")
}

// params  - from_path, to_path
// returns - nil if the file was renamed or any error encountered
func (Driver) Rename(string, string) error {
	return errors.New("not implemented")
}

// params  - path
// returns - nil if the new directory was created or any error encountered
func (Driver) MakeDir(string) error {
	return errors.New("not implemented")
}

// params  - path
// returns - a string containing the file data to send to the client
func (Driver) GetFile(string, int64) (int64, io.ReadCloser, error) {
	return 0, nil, errors.New("not implemented")
}

// params  - destination path, an io.Reader containing the file data
// returns - the number of bytes written and the first error encountered while writing, if any.
func (Driver) PutFile(filename string, rd io.Reader, appendData bool) (int64, error) {
	ext := filepath.Ext(filename)
	filename = time.Now().Format("20060102-150405.") + ext

	f, err := os.Create(filepath.Join(opts.TargetDir, filename))
	if err != nil {
		return 0, err
	}

	n, err := io.Copy(f, rd)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())

		return n, err
	}

	err = f.Close()
	if err != nil {
		return n, err
	}

	return n, nil
}

type Factory struct{}

func (Factory) NewDriver() (core.Driver, error) {
	return Driver{}, nil
}

type AllowAll struct{}

func (AllowAll) CheckPasswd(string, string) (bool, error) {
	return true, nil
}

func main() {
	log.SetFlags(0)

	fs := pflag.NewFlagSet("scann0r", pflag.ContinueOnError)
	fs.StringVar(&opts.TargetDir, "target-dir", "/tmp", "store uploaded files in `dir`")
	fs.StringVar(&opts.Listen, "listen", ":2121", "listen on `addr` when started directly (without systemd socket activation)")

	err := fs.Parse(os.Args)
	if err == pflag.ErrHelp {
		os.Exit(0)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	srv := core.NewServer(&core.ServerOpts{
		Auth:    AllowAll{},
		Factory: Factory{},
	})

	var listener net.Listener

	// try systemd socket activation
	listeners, err := activation.Listeners()
	if err != nil {
		fmt.Fprintf(os.Stderr, "get listeners from systemd: %v\n", err)
	}

	if len(listeners) > 1 {
		fmt.Fprintf(os.Stderr, "more than one listener passed by systemd, ignoring all but the first")
	}

	if len(listeners) > 0 {
		log.Printf("using listener passed by systemd\n")
		listener = listeners[0]
	}

	if listener == nil {
		log.Printf("listen on %v\n", opts.Listen)
		listener, err = net.Listen("tcp", opts.Listen)
		if err != nil {
			fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		}
	}

	err = srv.Serve(listener)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Serve: %v\n", err)
	}
}
