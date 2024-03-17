package ingest

import "github.com/rjeczalik/notify"

func watchDir(dirname string, ch chan<- notify.EventInfo) error {
	return notify.Watch(dirname, ch, notify.InCloseWrite, notify.InMovedTo)
}
