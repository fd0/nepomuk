package main

import (
	"context"

	"github.com/fsnotify/fsnotify"
)

func RunWatcher(ctx context.Context, incomingDir string, verbose bool) (err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil
	}

	defer func() {
		werr := watcher.Close()
		if err == nil {
			err = werr
		}
	}()

	err = watcher.Add(incomingDir)
	if err != nil {
		return err
	}

outer:
	for {
		select {
		case <-ctx.Done():
			break outer
		}
	}

	return nil
}
