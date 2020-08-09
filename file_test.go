package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TempDir(t testing.TB) (dir string, cleanup func()) {
	dir, err := ioutil.TempDir("", "go-test-")
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

func TestFindLastFilename(t *testing.T) {
	var tests = []struct {
		filenames []string
		last      string
		err       bool
	}{
		{
			filenames: []string{},
			err:       true,
		},
		{
			filenames: []string{
				"foo", "bar", "baz",
			},
			err: true,
		},
		{
			filenames: []string{
				"foo", "bar", "20200809-112501.pdf", "baz",
			},
			last: "20200809-112501.pdf",
		},
		{
			filenames: []string{
				"20200809-112601.pdf",
				"20200809-112701.pdf",
				"20200809-112501.pdf",
			},
			last: "20200809-112701.pdf",
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			tempdir, cleanup := TempDir(t)
			t.Cleanup(cleanup)

			for _, filename := range test.filenames {
				err := ioutil.WriteFile(filepath.Join(tempdir, filename), []byte(filename), 0600)
				if err != nil {
					t.Fatalf("writing %v: %v", filename, err)
				}
			}

			last, err := FindLastFilename(tempdir)
			if err != nil && !test.err {
				t.Error(err)
			}

			if test.err && err == nil {
				t.Error("expected error not found")
			}

			if last != test.last {
				t.Errorf("unexpected filename returned, want %q, got %q", test.last, last)
			}
		})
	}
}
