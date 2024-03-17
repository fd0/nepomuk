package ingest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rjeczalik/notify"
	"github.com/sirupsen/logrus"
)

// Watcher calls OnNewFile when a new file is placed in Dir.
type Watcher struct {
	Dir string

	OnNewFile func(filename string)

	log logrus.FieldLogger
}

const defaultInotifyChanBuf = 20

// SetLogger updates the logger to use.
func (w *Watcher) SetLogger(logger logrus.FieldLogger) {
	w.log = logger.WithField("component", "watcher")
}

// Run starts the watcher, it terminates when ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	// process all pre-existing files
	entries, err := os.ReadDir(w.Dir)
	if err != nil {
		return fmt.Errorf("readdir %v: %w", w.Dir, err)
	}

	if len(entries) > 0 {
		w.log.Infof("process %d new files in %v", len(entries), w.Dir)
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		filename := filepath.Join(w.Dir, entry.Name())

		w.log.WithField("filename", filename).Infof("found new file")
		w.OnNewFile(filename)
	}

	ch := make(chan notify.EventInfo, defaultInotifyChanBuf)

	// watch for events fired after creating files
	err = watchDir(w.Dir, ch)
	if err != nil {
		return fmt.Errorf("inotify watch failed: %w", err)
	}

	w.log.Debugf("watch for new files in %v", w.Dir)

outer:
	for {
		select {
		case <-ctx.Done():
			break outer
		case ev, ok := <-ch:
			if !ok {
				return nil
			}

			w.log.Debugf("new file: %v, %#v", ev.Path(), ev)

			w.OnNewFile(ev.Path())
		}
	}

	return nil
}
