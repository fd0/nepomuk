package database

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/rjeczalik/notify"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const defaultInotifyChanBuf = 20

// Watcher keeps track of file renames.
type Watcher struct {
	ArchiveDir string

	Log logrus.FieldLogger

	// OnStartWatching is called when the watcher has subscribes to the directory change events
	OnStartWatching func()

	// OnFileMoved is called for each renamed file
	OnFileMoved func(oldFilename, newFilename string)
}

// Run starts a process which watches archiveDir for renames and provides a
// callback for such files.
func (w *Watcher) Run(ctx context.Context) error {
	ch := make(chan notify.EventInfo, defaultInotifyChanBuf)

	// recursively watch for events fired when files are moved or renamed
	err := notify.Watch(filepath.Join(w.ArchiveDir, "..."), ch, notify.InMovedFrom, notify.InMovedTo)
	if err != nil {
		return fmt.Errorf("inotify watch failed: %w", err)
	}

	if w.OnStartWatching != nil {
		w.Log.Debug("run hook OnStartWatching")
		w.OnStartWatching()
	}

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

			ev := evinfo.Sys().(*unix.InotifyEvent)

			// keep state until we have collected both events
			switch evinfo.Event() {
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

			w.OnFileMoved(oldName, newName)

			// remove state
			delete(newFilename, ev.Cookie)
			delete(oldFilename, ev.Cookie)
		}
	}

	return nil
}
