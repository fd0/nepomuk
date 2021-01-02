package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/fd0/nepomuk/database"
	"github.com/fd0/nepomuk/extract"
	"github.com/fd0/nepomuk/ingest"
	"github.com/fd0/nepomuk/process"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"
)

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

const defaultChannelBufferSize = 20

type Options struct {
	Config  string
	BaseDir string
	Listen  string
	Verbose bool
}

func parseOptions() (opts Options) {
	fs := pflag.NewFlagSet("nepomuk-ingester", pflag.ContinueOnError)
	fs.StringVar(&opts.Config, "config", "config.yml", "load config from `config.yml`, path may be relative to base directory")
	fs.StringVar(&opts.BaseDir, "base-dir", "archive", "archive base `directory`")
	fs.StringVar(&opts.Listen, "listen", ":2121", "listen on `addr`")
	fs.BoolVar(&opts.Verbose, "verbose", false, "print verbose messages")

	err := fs.Parse(os.Args)
	if errors.Is(err, pflag.ErrHelp) {
		os.Exit(0)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	return opts
}

func main() {
	// configure logging without any extra fields
	log.SetFlags(0)

	// parse flags and fill global struct
	opts := parseOptions()

	configPath := opts.Config
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(opts.BaseDir, configPath)
	}

	cfg, err := LoadConfig(configPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("no config.yml found at %v", configPath)

		cfg = Config{}
		err = nil
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if opts.Verbose {
		log.Printf("loaded config from %v", opts.Config)
	}

	db, err := database.Load(filepath.Join(opts.BaseDir, "db.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}

	err = CheckTargetDir(opts.BaseDir)
	if err != nil {
		log.Fatal(err)
	}

	incomingDir := filepath.Join(opts.BaseDir, "incoming")
	uploadedDir := filepath.Join(opts.BaseDir, "uploaded")
	processedDir := filepath.Join(opts.BaseDir, "processed")
	dataDir := filepath.Join(opts.BaseDir, "data")

	for _, dir := range []string{incomingDir, uploadedDir, processedDir, dataDir} {
		err = CheckTargetDir(dir)
		if err != nil {
			log.Fatal(err)
		}
	}

	wg, ctx, cancel := setupRootContext()

	newFiles := make(chan string, defaultChannelBufferSize)

	// receive files via FTP
	wg.Go(func() error {
		log.Printf("start FTP server on %v\n", opts.Listen)

		srv := &ingest.FTPServer{
			TargetDir: uploadedDir,
			Verbose:   opts.Verbose,
			Bind:      opts.Listen,
			OnFileUpload: func(filename string) {
				log.Printf("new file uploaded: %v", filepath.Base(filename))
				newFiles <- filename
			},
		}

		return srv.Run(ctx)
	})

	// watch for new files in incoming/
	wg.Go(func() error {
		log.Printf("watch for new files in %v", incomingDir)
		watcher := &ingest.Watcher{
			Dir: incomingDir,
			OnNewFile: func(filename string) {
				log.Printf("new file found: %v", filepath.Base(filename))
				newFiles <- filename
			},
		}

		return watcher.Run(ctx)
	})

	processedFiles := make(chan string, defaultChannelBufferSize)

	// process files received via FTP or incoming/
	wg.Go(func() error {
		processor := &process.Processor{
			ProcessedDir: processedDir,
			OnFileProcessed: func(filename string) {
				processedFiles <- filename
			},
		}

		return processor.Run(ctx, newFiles)
	})

	// extract data and sort processed files
	wg.Go(func() error {
		extracter := extract.Extracter{
			Database:       db,
			ArchiveDir:     dataDir,
			ProcessedDir:   processedDir,
			Correspondents: cfg.Correspondents,
		}

		return extracter.Run(ctx, processedFiles)
	})

	// watch archive directory and make sure files are in sync between the database and the filenames
	wg.Go(func() error {
		log.Printf("watch for moved or renamed files in %v", dataDir)

		return database.RunWatcher(ctx, db, dataDir)
	})

	// wait for all processes to complete
	err = wg.Wait()

	exitCode := 0

	log.Printf("save database")

	dberr := db.Save(filepath.Join(opts.BaseDir, "db.json"))
	if dberr != nil {
		fmt.Fprintf(os.Stderr, "error saving database: %v", dberr)

		exitCode = 1
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)

		exitCode = 1
	}

	cancel()
	os.Exit(exitCode)
}
