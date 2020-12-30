package ingest

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/rjeczalik/notify"
)

// Watcher calls OnNewFile when a new file is placed in Dir.
type Watcher struct {
	Dir     string
	Verbose bool

	OnNewFile func(filename string)
}

// Run starts the watcher, it terminates when ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	// process all pre-existing files
	entries, err := ioutil.ReadDir(w.Dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		w.OnNewFile(filepath.Join(w.Dir, entry.Name()))
	}

	ch := make(chan notify.EventInfo, 20)

	// watch for events fired after creating files
	err = notify.Watch(w.Dir, ch, notify.InCloseWrite, notify.InMovedTo)
	if err != nil {
		return fmt.Errorf("inotify watch failed: %w", err)
	}

	if w.Verbose {
		log.Printf("Watch for new files in %v", w.Dir)
	}

outer:
	for {
		select {
		case <-ctx.Done():
			break outer
		case ev, ok := <-ch:
			if !ok {
				return nil
			}

			if w.Verbose {
				log.Printf("received event %+v", ev)
			}

			w.OnNewFile(ev.Path())
		}
	}

	return nil
}