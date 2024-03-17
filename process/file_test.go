package process

import (
	"os"
	"testing"
)

func TempDir(t testing.TB) (dir string, cleanup func()) {
	dir, err := os.MkdirTemp("", "go-test-")
	if err != nil {
		t.Fatalf("create tempdir: %v", err)
	}

	cleanup = func() {
		err = os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("remove tempdir: %v", err)
		}
	}

	return dir, cleanup
}
