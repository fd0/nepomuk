package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/fd0/nepomuk/ingest"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
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

// setupRootContext creates a root context that is cancelled when SIGINT is
// received, tied to a new errgroup.Group. The returned cancel() function
// cancels the outermost context.
func setupRootContext() (wg *errgroup.Group, ctx context.Context, cancel func()) {
	// create new root context, cancel on SIGINT
	ctx, cancel = context.WithCancel(context.Background())

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()

	// couple this context with an errgroup
	wg, ctx = errgroup.WithContext(ctx)

	return wg, ctx, cancel
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
	uploadedDir := filepath.Join(opts.BaseDir, "uploaded")
	dataDir := filepath.Join(opts.BaseDir, "data")

	err = CheckTargetDir(incomingDir)
	if err != nil {
		log.Fatal(err)
	}

	err = CheckTargetDir(uploadedDir)
	if err != nil {
		log.Fatal(err)
	}

	wg, ctx, cancel := setupRootContext()
	defer cancel()

	newFiles := make(chan string, 20)
	handler := func(filename string) {
		newFiles <- filename
	}

	// start all processes
	wg.Go(func() error {
		srv := &ingest.FTPServer{
			TargetDir:    uploadedDir,
			Verbose:      opts.Verbose,
			Bind:         opts.Listen,
			OnFileUpload: handler,
		}
		return srv.Run(ctx)
	})

	wg.Go(func() error {
		log.Printf("Watch for new files in %v", incomingDir)
		return ingest.RunWatcher(ctx, incomingDir, opts.Verbose, handler)
	})

	wg.Go(func() error {
		return RunProcess(ctx, newFiles, dataDir)
	})

	err = wg.Wait()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
