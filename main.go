package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/fd0/nepomuk/database"
	"github.com/fd0/nepomuk/extract"
	"github.com/fd0/nepomuk/ingest"
	"github.com/fd0/nepomuk/notify"
	"github.com/fd0/nepomuk/process"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/net/webdav"
	"golang.org/x/sync/errgroup"
)

// CheckTargetDir ensures that dir exists and is a directory.
func CheckTargetDir(dir string) error {
	fi, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		log.Printf("create target dir %v", dir)

		err = os.MkdirAll(dir, 0770)
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

var log *logrus.Logger

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

const defaultChannelBufferSize = 500

type Options struct {
	Config       string
	BaseDir      string
	ListenFTP    string
	ListenWebDAV string
	LogLevel     string
	Verbose      bool
}

func main() {
	var opts Options

	defaultConfigPath, ok := os.LookupEnv("NEPOMUK_CONFIG")
	if !ok {
		defaultConfigPath = ".nepomuk/config.yml"
	}

	fs := pflag.NewFlagSet("nepomuk", pflag.ContinueOnError)
	fs.StringVar(&opts.Config, "config", defaultConfigPath, "load config from `config.yml`, path may be relative to base directory")
	fs.StringVar(&opts.BaseDir, "base-dir", "archive", "archive base `directory`")
	fs.StringVar(&opts.ListenFTP, "listen-ftp", ":2121", "run FTP server on `addr:port`")
	fs.StringVar(&opts.ListenWebDAV, "listen-webdav", ":8080", "run WebDAV-Server on `addr:port`")
	fs.StringVar(&opts.LogLevel, "log-level", "debug", "set log level")
	fs.BoolVar(&opts.Verbose, "verbose", false, "print verbose messages")

	err := fs.Parse(os.Args)
	if errors.Is(err, pflag.ErrHelp) {
		os.Exit(0)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err = run(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(opts Options) error {
	// configure logging
	log = logrus.New()
	log.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
		DisableQuote:     true,
	})

	level, err := logrus.ParseLevel(opts.LogLevel)
	if err != nil {
		return err
	}

	log.SetLevel(level)

	configPath := opts.Config
	if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(opts.BaseDir, configPath)
	}

	log.Debugf("load config from %v", configPath)

	cfg, err := LoadConfig(configPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("config at %v not found", configPath)

		cfg = Config{}
		err = nil
	}

	if err != nil {
		return err
	}

	if opts.Verbose {
		log.Printf("loaded config from %v", opts.Config)
	}

	err = CheckTargetDir(opts.BaseDir)
	if err != nil {
		return err
	}

	incomingDir := filepath.Join(opts.BaseDir, "incoming")
	uploadedDir := filepath.Join(opts.BaseDir, ".nepomuk/uploaded")
	processedDir := filepath.Join(opts.BaseDir, ".nepomuk/processed")

	for _, dir := range []string{incomingDir, uploadedDir, processedDir, opts.BaseDir} {
		err = CheckTargetDir(dir)
		if err != nil {
			return err
		}
	}

	db := database.New(opts.BaseDir)

	err = db.Load(filepath.Join(opts.BaseDir, ".nepomuk/db.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}

	db.SetLogger(log)

	db.OnChange = func(id string, oldAnnotation, newAnnotation database.File) {
		log.Infof("database: data for file %v changed, saving database", id)

		err := db.Save(filepath.Join(opts.BaseDir, ".nepomuk/db.json"))
		if err != nil {
			log.Printf("database: save error: %v", err)
		}
	}

	err = db.Scan()
	if err != nil {
		log.Warnf("db scan returned error: %v", err)
	}

	wg, ctx, cancel := setupRootContext()

	newFiles := make(chan string, defaultChannelBufferSize)

	// receive files via FTP
	ingest.SetLogger(log)

	wg.Go(func() error {
		srv := &ingest.FTPServer{
			TargetDir: uploadedDir,
			Verbose:   opts.Verbose,
			Bind:      opts.ListenFTP,
			OnFileUpload: func(filename string) {
				log.Printf("ftp: new file uploaded: %v", filepath.Base(filename))
				newFiles <- filename
			},
		}

		return srv.Run(ctx)
	})

	wg.Go(func() error {
		log := log.WithField("component", "webdav-server")

		log.Debugf("start on %v", opts.ListenWebDAV)

		var logRequest func(*http.Request, error)
		if log.Level >= logrus.DebugLevel {
			logRequest = func(req *http.Request, err error) {
				log.Printf("%v %v -> %v", req.Method, req.URL.Path, err)
			}
		}

		handler := &webdav.Handler{
			FileSystem: &ingest.UploadOnlyFS{
				Log: log,
				Create: func(name string) (io.WriteCloser, error) {
					filename := time.Now().Format(ingest.UploadFilenameTimeFormat) + ".pdf"

					f, err := os.OpenFile(filepath.Join(incomingDir, filename), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
					if err != nil {
						log.Debugf("create new file failed: %v", err)

						return nil, fmt.Errorf("create file: %w", err)
					}

					log.Infof("upload file %v as %v", name, filename)

					return f, nil
				},
			},
			LockSystem: webdav.NewMemLS(),
			Logger:     logRequest,
		}

		server := http.Server{
			Addr:    opts.ListenWebDAV,
			Handler: handler,
		}

		// ensure cancelling the context stops the server
		wg.Go(func() error {
			<-ctx.Done()
			log.Debugf("shutdown webdav server")

			// pass a cancelled context to Shutdown so it terminates directly
			ctx, cancel := context.WithCancel(ctx)
			cancel()

			err := server.Shutdown(ctx)
			if err != nil {
				return fmt.Errorf("shutdown webdav server: %w", err)
			}

			return nil
		})

		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		if err != nil {
			return fmt.Errorf("listen webdav: %w", err)
		}

		return nil
	})

	// watch for new files in incoming/
	wg.Go(func() error {
		watcher := &ingest.Watcher{
			Dir: incomingDir,
			OnNewFile: func(filename string) {
				newFiles <- filename
			},
		}
		watcher.SetLogger(log)

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

		processor.SetLogger(log)

		return processor.Run(ctx, newFiles)
	})

	// extract data and sort processed files
	wg.Go(func() error {
		extracter := extract.Extracter{
			Database:       db,
			ArchiveDir:     opts.BaseDir,
			ProcessedDir:   processedDir,
			Correspondents: cfg.Correspondents,
			OnNewFile: func(file database.File) {
				notify.Notify(log, file)
			},
		}

		extracter.SetLogger(log)

		return extracter.Run(ctx, processedFiles)
	})

	// watch archive directory and make sure files are in sync between the database and the filenames
	wg.Go(func() error {
		watcher := database.Watcher{
			ArchiveDir: opts.BaseDir,
			OnFileMoved: func(oldName, newName string) {
				err := db.OnRename(newName)
				if err != nil {
					log.WithField("filename", newName).Warnf("rename failed: %v", err)
				}
			},
			OnFileDeleted: func(oldName string) {
				err := db.OnDelete(oldName)
				if err != nil {
					log.WithField("filename", oldName).Warnf("delete in database failed: %v", err)
				}
			},
		}

		watcher.SetLogger(log)

		return watcher.Run(ctx)
	})

	// wait for all processes to complete
	err = wg.Wait()

	log.Printf("save database before shutdown")

	dberr := db.Save(filepath.Join(opts.BaseDir, ".nepomuk/db.json"))
	if dberr != nil {
		fmt.Fprintf(os.Stderr, "error saving database: %v", dberr)

		if err == nil {
			err = dberr
		}
	}

	cancel()

	return err
}
