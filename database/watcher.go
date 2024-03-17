package database

import (
	"github.com/sirupsen/logrus"
)

const defaultInotifyChanBuf = 200

// Watcher keeps track of file renames.
type Watcher struct {
	ArchiveDir string

	log logrus.FieldLogger

	// OnStartWatching is called when the watcher has subscribes to the directory change events.
	OnStartWatching func()

	// OnFileRenamed is called when a file has been renamed. Only the new name is provided.
	OnFileRenamed func(newFilename string)

	// OnFileDeleted is called for removed files.
	OnFileDeleted func(oldFilename string)
}

// SetLogger updates the logger to use.
func (w *Watcher) SetLogger(logger logrus.FieldLogger) {
	w.log = logger.WithField("component", "database-watcher")
}
