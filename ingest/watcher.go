package ingest

import (
	"context"
	"fmt"
	"io/ioutil"
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
	entries, err := ioutil.ReadDir(w.Dir)
	if err != nil {
		return fmt.Errorf("readdir %v: %w", w.Dir, err)
	}

	if len(entries) > 0 {
		w.log.Infof("process %d new files in %v", w.Dir)
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		w.OnNewFile(filepath.Join(w.Dir, entry.Name()))
	}

	ch := make(chan notify.EventInfo, defaultInotifyChanBuf)

	// watch for events fired after creating files
	err = notify.Watch(w.Dir, ch, notify.InCloseWrite, notify.InMovedTo)
	if err != nil {
		return fmt.Errorf("inotify watch failed: %w", err)
	}

	w.log.Infof("watching for new files in %v", w.Dir)

outer:
	for {
		select {
		case <-ctx.Done():
			break outer
		case ev, ok := <-ch:
			if !ok {
				return nil
			}

			w.OnNewFile(ev.Path())
		}
	}

	return nil
}
