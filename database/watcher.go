package database

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/rjeczalik/notify"
	"golang.org/x/sys/unix"
)

const defaultInotifyChanBuf = 20

// RunWatcher starts a process which watches archiveDir for changes and keeps
// it in sync with the db.
func RunWatcher(ctx context.Context, db *Database, archiveDir string) error {
	ch := make(chan notify.EventInfo, defaultInotifyChanBuf)

	// watch for events fired when files are moved or renamed
	err := notify.Watch(filepath.Join(archiveDir, "..."), ch, notify.InMovedFrom, notify.InMovedTo)
	if err != nil {
		return fmt.Errorf("inotify watch failed: %w", err)
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

			delete(newFilename, ev.Cookie)
			delete(oldFilename, ev.Cookie)

			log.Printf("rename %v -> %v", oldName, newName)
		}
	}

	return nil
}
