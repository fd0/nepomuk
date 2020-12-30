package ingest

import (
	"context"
	"fmt"
	"log"

	"github.com/rjeczalik/notify"
)

func RunWatcher(ctx context.Context, incomingDir string, verbose bool, onNewFile func(filename string)) error {
	ch := make(chan notify.EventInfo, 20)

	// watch for events fired after creating files
	err := notify.Watch(incomingDir, ch, notify.InCloseWrite, notify.InMovedTo)
	if err != nil {
		return fmt.Errorf("inotify watch failed: %w", err)
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

			if verbose {
				log.Printf("received event %+v", ev)
			}

			onNewFile(ev.Path())
		}
	}

	return nil
}
