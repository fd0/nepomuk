package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rjeczalik/notify"
)

func watchDir(dirname string, ch chan<- notify.EventInfo) error {
	return notify.Watch(
		dirname,
		ch,
		notify.All,
		// notify.FSEventsRemoved,
		// notify.FSEventsCreated,
		// notify.FSEventsRenamed,
	)
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

			ev, ok := evinfo.Sys().(*notify.FSEvent)
			if !ok {
				w.log.Warnf("received event is not *unix.FSEvent but %T: %v", evinfo, evinfo)
			}

			// ignore events in an internal path or incoming
			if strings.HasPrefix(evinfo.Path(), absInternalPath) || strings.HasPrefix(evinfo.Path(), absIncomingPath) {
				w.log.Debugf("ignore event for path %v", evinfo.Path())

				continue
			}

			// ignore events in incoming/, will be processed by the extracter
			if filepath.Base(filepath.Dir(evinfo.Path())) == "incoming" {
				continue
			}

			// ignore events on non-files
			if ev.Flags&notify.FSEventsIsFile == 0 {
				continue
			}

			switch evinfo.Event() {
			case notify.FSEventsCreated:
				w.log.Debugf("create detected for %v", evinfo.Path())
				w.OnFileRenamed(evinfo.Path())

			case notify.FSEventsRemoved:
				w.log.Debugf("remove detected for %v", evinfo.Path())
				w.OnFileDeleted(evinfo.Path())

			case notify.FSEventsRenamed:
				// try to stat the path, if that works, we got the new filename
				_, err := os.Lstat(evinfo.Path())
				if err != nil && errors.Is(err, os.ErrNotExist) {
					// ignore this event, this was for the old filename
					continue
				}

				w.log.Debugf("rename detected, new name: %v", evinfo.Path())
				w.OnFileRenamed(evinfo.Path())

			default:
				// return fmt.Errorf("invalid event type %T received: %#v", evinfo.Event(), evinfo.Event())
				w.log.Warnf("unknown event %#v received: %#v", evinfo.Event(), evinfo.Sys())
			}
		}
	}

	return nil
}
