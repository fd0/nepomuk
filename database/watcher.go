package database

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rjeczalik/notify"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const defaultInotifyChanBuf = 200

// Watcher keeps track of file renames.
type Watcher struct {
	ArchiveDir string

	log logrus.FieldLogger

	// OnStartWatching is called when the watcher has subscribes to the directory change events
	OnStartWatching func()

	// OnFileMoved is called for each renamed file
	OnFileMoved func(oldFilename, newFilename string)

	// OnFileDeleted is called for removed files.
	OnFileDeleted func(oldFilename string)
}

// SetLogger updates the logger to use.
func (w *Watcher) SetLogger(logger logrus.FieldLogger) {
	w.log = logger.WithField("component", "database-watcher")
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
	err = notify.Watch(
		filepath.Join(w.ArchiveDir, "..."),
		ch,
		notify.InMovedFrom, notify.InMovedTo, notify.InDelete)
	if err != nil {
		return fmt.Errorf("inotify watch failed: %w", err)
	}

	if w.OnStartWatching != nil {
		w.log.Debug("run hook OnStartWatching")
		w.OnStartWatching()
	}

	w.log.Debugf("watch files in %v", w.ArchiveDir)

	// For each renamed file we get two events: InMovedTo (new filename) and
	// InMovedFrom (old filename), which are correlated by the same "Cookie"
	// value. We'll keep maps of cookie for both event types.
	oldFilename := make(map[uint32]string)
	newFilename := make(map[uint32]string)

outer:
	for {
		select {
		case <-ctx.Done():
			break outer
		case evinfo, ok := <-ch:
			if !ok {
				return nil
			}

			ev, ok := evinfo.Sys().(*unix.InotifyEvent)
			if !ok {
				w.log.Warnf("received event is not *unix.InotifyEvent but %T: %v", evinfo, evinfo)
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
				oldFilename[ev.Cookie] = evinfo.Path()
			case notify.InMovedTo:
				newFilename[ev.Cookie] = evinfo.Path()
			default:
				return fmt.Errorf("invalid event type %T received: %#v", evinfo.Event(), evinfo.Event())
			}

			oldName, ok := oldFilename[ev.Cookie]
			if !ok {
				continue
			}

			newName, ok := newFilename[ev.Cookie]
			if !ok {
				continue
			}

			w.log.WithField("oldName", oldName).WithField("filename", newName).Info("rename detected")
			w.OnFileMoved(oldName, newName)

			// remove state
			delete(newFilename, ev.Cookie)
			delete(oldFilename, ev.Cookie)
		}
	}

	return nil
}
