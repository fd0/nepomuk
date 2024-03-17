package database

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rjeczalik/notify"
)

func watchDir(dirname string, ch chan<- notify.EventInfo) error {
	return notify.Watch(
		dirname,
		ch,
		notify.InMovedTo, notify.InDelete)
}

// Run starts a process which watches archiveDir for renames and deletions and
// provides a callback for such files.
func (w *Watcher) Run(ctx context.Context) error {
	abspath, err := filepath.Abs(w.ArchiveDir)
	if err != nil {
		return fmt.Errorf("unable to find absolute dir: %w", err)
	}

	absInternalPath := filepath.Clean(filepath.Join(abspath, ".nepomuk")) + "/"
	absIncomingPath := filepath.Clean(filepath.Join(abspath, "incoming")) + "/"

	ch := make(chan notify.EventInfo, defaultInotifyChanBuf)

	// recursively watch for events fired when files are moved or renamed
	err = watchDir(filepath.Join(w.ArchiveDir, "..."), ch)
	if err != nil {
		return fmt.Errorf("inotify watch failed: %w", err)
	}

	if w.OnStartWatching != nil {
		w.log.Debug("run hook OnStartWatching")
		w.OnStartWatching()
	}

	w.log.Debugf("watch files in %v", w.ArchiveDir)

outer:
	for {
		select {
		case <-ctx.Done():
			break outer
		case evinfo, ok := <-ch:
			if !ok {
				return nil
			}

			// ignore events in an internal path or incoming
			if strings.HasPrefix(evinfo.Path(), absInternalPath) || strings.HasPrefix(evinfo.Path(), absIncomingPath) {
				w.log.Debugf("ignore event for path %v", evinfo.Path())

				continue
			}

			w.log.Debugf("event for path %v", evinfo.Path())

			// ignore events in incoming/, will be processed by the extracter
			if filepath.Base(filepath.Dir(evinfo.Path())) == "incoming" {
				continue
			}

			// keep state until we have collected both events
			switch evinfo.Event() {
			case notify.InDelete:
				w.OnFileDeleted(evinfo.Path())

				continue
			case notify.InMovedFrom:
				// ignored
			case notify.InMovedTo:
				w.log.WithField("filename", evinfo.Path()).Info("rename detected")
				w.OnFileRenamed(evinfo.Path())
			default:
				return fmt.Errorf("invalid event type %T received: %#v", evinfo.Event(), evinfo.Event())
			}
		}
	}

	return nil
}
