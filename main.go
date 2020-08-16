package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/coreos/go-systemd/activation"
	"github.com/spf13/pflag"
	"goftp.io/server/core"
)

var opts = struct {
	TargetDir            string
	PaperlessIncomingDir string
	Listen               string
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

	fs := pflag.NewFlagSet("scann0r", pflag.ContinueOnError)
	fs.StringVar(&opts.TargetDir, "target-dir", "data", "store uploaded files in `dir`")
	fs.StringVar(&opts.PaperlessIncomingDir, "paperless-incoming-dir", "",
		"store a copy of the PDF in `dir` for processing by paperless")
	fs.StringVar(&opts.Listen, "listen", ":2121",
		"listen on `addr` when started directly (without systemd socket activation)")

	err := fs.Parse(os.Args)
	if err == pflag.ErrHelp {
		os.Exit(0)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err = CheckTargetDir(opts.TargetDir)
	if err != nil {
		log.Fatal(err)
	}

	if opts.PaperlessIncomingDir != "" {
		err = CheckTargetDir(opts.PaperlessIncomingDir)
		if err != nil {
			log.Fatal(err)
		}
	}

	srv := core.NewServer(&core.ServerOpts{
		Auth: AllowAll{},
		Factory: Factory{
			targetdir: opts.TargetDir,
			copydir:   opts.PaperlessIncomingDir,
		},
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
