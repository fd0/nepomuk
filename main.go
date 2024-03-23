package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
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

const defaultChannelBufferSize = 500

type Options struct {
	BaseDir      string
	ListenWebDAV string
	LogLevel     string
	Verbose      bool
}

func main() {
	var opts Options

	fs := pflag.NewFlagSet("nepomuk", pflag.ContinueOnError)
	fs.StringVar(&opts.BaseDir, "base-dir", "archive", "archive base `directory`")
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

// we need to use the dot to specify millisecond precision, it will be replaced later
const uploadFilenameTimeFormat = "20060102-150405.000000"

func runWebDAVServer(ctx context.Context, wg *errgroup.Group, logger *logrus.Logger, addr, incomingDir string) {
	log := logger.WithField("component", "webdav-server")

	log.Debugf("start on %v", addr)

	var logRequest func(*http.Request, error)
	if logger.Level >= logrus.DebugLevel {
		logRequest = func(req *http.Request, err error) {
			log.Printf("%v %v -> %v", req.Method, req.URL.Path, err)
		}
	}

	// we use a MemFS to temporarily store files
	filesystem := webdav.NewMemFS()
	locksystem := webdav.NewMemLS()

	handler := &webdav.Handler{
		FileSystem: filesystem,
		LockSystem: locksystem,
		Logger:     logRequest,
	}

	server := http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// watch the file system for new files and move them into incomingDir
	wg.Go(func() error {
		ticker := time.NewTicker(20 * time.Millisecond)

		walkFS := newWebDAVFS(filesystem)

	outer:
		for {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				break outer
			}

			err := fs.WalkDir(walkFS, "", func(filename string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				// ignore dirs
				if d.IsDir() {
					return nil
				}

				// ignore files with . or _ as first characters
				if filename != "" && (filename[0] == '.' || filename[0] == '_') {
					log.Tracef("ignore file with special filename %v", filename)

					return nil
				}

				fi, err := d.Info()
				if err != nil {
					log.Warnf("get FileInfo for %v: %v", filename, err)

					return nil
				}

				// ignore very small files
				if fi.Size() < 1000 {
					log.Tracef("ignore small file %v (%d bytes)", filename, fi.Size())

					return nil
				}

				// ignore very new files
				if time.Since(fi.ModTime()) < 200*time.Millisecond {
					log.Tracef("ignore %v, too new", filename)

					return nil
				}

				// try to get lock, ignore locked files
				token, err := locksystem.Create(time.Now(), webdav.LockDetails{
					Root:      filename,
					Duration:  -1,
					OwnerXML:  "nepomuk",
					ZeroDepth: true,
				})

				if err != nil {
					log.Debugf("did not get lock for %v, skipping", filename)

					return nil
				}

				log.Tracef("got lock, token %v", token)
				defer func() {
					err := locksystem.Unlock(time.Now(), token)
					if err != nil {
						log.Debugf("unlock return error: %v", err)
					}
				}()

				buf, err := fs.ReadFile(walkFS, filename)
				if err != nil {
					return fmt.Errorf("readfile: %w", err)
				}

				log.Debugf("found new file %v, %d bytes\n", filename, len(buf))

				err = walkFS.RemoveAll(filename)
				if err != nil {
					return fmt.Errorf("remove %v: %w", filename, err)
				}

				name := time.Now().Format(uploadFilenameTimeFormat)
				// replace the dot used for specifying millisecond precision
				name = strings.ReplaceAll(name, ".", "_")
				name += path.Ext(filename)

				err = os.WriteFile(filepath.Join(incomingDir, name), buf, 0600)
				if err != nil {
					return fmt.Errorf("write to incoming dir: %w", err)
				}

				return nil
			})

			if err != nil {
				return fmt.Errorf("walk() FS: %w", err)
			}
		}

		return nil
	})

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

	wg.Go(func() error {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		if err != nil {
			return fmt.Errorf("listen webdav: %w", err)
		}

		return nil
	})
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

	err = CheckTargetDir(opts.BaseDir)
	if err != nil {
		return err
	}

	incomingDir := filepath.Join(opts.BaseDir, "incoming")
	processedDir := filepath.Join(opts.BaseDir, ".nepomuk/processed")

	for _, dir := range []string{incomingDir, processedDir, opts.BaseDir} {
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

	db.OnChange = func(id string, _, _ database.File) {
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

	// create new root context, cancel on SIGINT
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// couple this context with an errgroup
	wg, ctx := errgroup.WithContext(ctx)

	newFiles := make(chan string, defaultChannelBufferSize)

	runWebDAVServer(ctx, wg, log, opts.ListenWebDAV, incomingDir)

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

	// process files received via incoming/
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
			Correspondents: []extract.Correspondent{},
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
			OnFileRenamed: func(newName string) {
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
