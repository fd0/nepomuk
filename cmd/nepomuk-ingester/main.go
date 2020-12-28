package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/pflag"
	"goftp.io/server/core"
)

var opts = struct {
	BaseDir string
	Listen  string
	Verbose bool
}{}

// CheckTargetDir ensures that dir exists and is a directory.
func CheckTargetDir(dir string) error {
	fi, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		log.Printf("creating target dir %v", dir)

		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("creating target dir %v: %w", dir, err)
		}

		fi, err = os.Lstat(dir)
	}

	if err != nil {
		return fmt.Errorf("accessing target dir %v: %w", dir, err)
	}

	if !fi.IsDir() {
		return fmt.Errorf("target dir %v is not a directory", dir)
	}

	return nil
}

func main() {
	log.SetFlags(0)

	fs := pflag.NewFlagSet("nepomuk-ingester", pflag.ContinueOnError)
	fs.StringVar(&opts.BaseDir, "base-dir", "archive", "nepomuk base `directory`")
	fs.StringVar(&opts.Listen, "listen", ":2121", "listen on `addr`")
	fs.BoolVar(&opts.Verbose, "verbose", false, "print verbose messages")

	err := fs.Parse(os.Args)
	if err == pflag.ErrHelp {
		os.Exit(0)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err = CheckTargetDir(opts.BaseDir)
	if err != nil {
		log.Fatal(err)
	}

	incomingDir := filepath.Join(opts.BaseDir, "incoming")
	uploadDir := filepath.Join(opts.BaseDir, "upload")

	err = CheckTargetDir(incomingDir)
	if err != nil {
		log.Fatal(err)
	}

	err = CheckTargetDir(uploadDir)
	if err != nil {
		log.Fatal(err)
	}

	serverOpts := &core.ServerOpts{
		WelcomeMessage: "Nepomuk Archive System",
		Auth:           AllowAll{},
		Factory: Factory{
			targetdir: uploadDir,
			OnFileUpload: func(filename string) {
				log.Printf("uploaded new file as %v", filename)
			},
		},
	}

	if !opts.Verbose {
		serverOpts.Logger = &core.DiscardLogger{}
	}

	srv := core.NewServer(serverOpts)

	var listener net.Listener

	log.Printf("listen on %v\n", opts.Listen)

	listener, err = net.Listen("tcp", opts.Listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
	}

	err = srv.Serve(listener)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Serve: %v\n", err)
	}
}
