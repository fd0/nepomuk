package database

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func write(t testing.TB, filename, data string) {
	err := ioutil.WriteFile(filename, []byte(data), 0600)
	if err != nil {
		t.Fatalf("write %v failed: %v", filename, err)
	}
}

func rename(t testing.TB, oldname, newname string) {
	err := os.Rename(oldname, newname)
	if err != nil {
		t.Fatalf("rename %v -> %v failed: %v", oldname, newname, err)
	}
}

func TestFileRename(t *testing.T) {
	t.Parallel()

	createFiles := []string{
		"foo.pdf",
		"bar.pdf",
	}

	type RenameOp struct {
		Old string
		New string
	}

	renameSequence := []RenameOp{
		{"bar.pdf", "bar2.pdf"},
		{"foo.pdf", "bar.pdf"},
		{"bar.pdf", "foo.pdf"},
	}

	tempdir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg, ctx := errgroup.WithContext(ctx)

	found := make(chan RenameOp)
	ready := make(chan struct{})
	w := Watcher{
		Log:        logrus.StandardLogger(),
		ArchiveDir: tempdir,
		OnFileMoved: func(oldname, newname string) {
			found <- RenameOp{
				Old: oldname,
				New: newname,
			}
		},
		OnStartWatching: func() {
			close(ready)
		},
	}

	// first create the files
	for _, filename := range createFiles {
		write(t, filepath.Join(tempdir, filename), filename)
	}

	// run the watcher
	wg.Go(func() error {
		return w.Run(ctx)
	})

	// wait until the watcher is ready
	<-ready

	// run the sequence, rename file, then wait for the rename operation with a
	// timeout of one second
	for _, op := range renameSequence {
		oldFilename := filepath.Join(tempdir, op.Old)
		newFilename := filepath.Join(tempdir, op.New)
		rename(t, oldFilename, newFilename)

		select {
		case ev := <-found:
			t.Logf("got event %v -> %v", op.Old, op.New)

			if ev.Old != oldFilename {
				t.Errorf("wrong old filename, want %v, got %v", oldFilename, ev.Old)
			}

			if ev.New != newFilename {
				t.Errorf("wrong new filename, want %v, got %v", newFilename, ev.New)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for rename event %v -> %v", oldFilename, newFilename)
		}
	}

	// stop the background watcher and wait for it to complete
	cancel()

	if err := wg.Wait(); err != nil {
		t.Fatal(err)
	}
}
