package process

import (
	"os"
	"path/filepath"
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

func TestFindLastFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filenames []string
		current   string
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
				"foo", "bar", "baz",
			},
			current: "zzz",
			err:     true,
		},
		{
			filenames: []string{
				"foo", "bar", "20200809-112501.pdf", "baz",
			},
			last: "20200809-112501.pdf",
		},
		{
			filenames: []string{
				"foo", "bar", "20200809-112501.pdf", "baz",
			},
			current: "20200809-112501.pdf",
			err:     true,
		},
		{
			filenames: []string{
				"20200809-112601.pdf",
				"20200809-112701.pdf",
				"20200809-112501.pdf",
			},
			last: "20200809-112701.pdf",
		},
		{
			filenames: []string{
				"20200809-112601.pdf",
				"20200809-112701.pdf",
				"20200809-112501.pdf",
			},
			current: "20200809-112701.pdf",
			last:    "20200809-112601.pdf",
		},
		{
			filenames: []string{
				"20200809-112601.pdf",
				"20200809-112701.pdf",
				"20200809-112501.pdf",
			},
			last: "20200809-112701.pdf",
		},
		{
			filenames: []string{
				"20200809-112601.pdf",
				"20200809-112701_duplex-odd.pdf",
				"20200809-112501.pdf",
			},
			last: "20200809-112701_duplex-odd.pdf",
		},
		{
			filenames: []string{
				"20200809-112601.pdf",
				"20200809-112401_duplex-even.pdf",
				"20200809-112501.pdf",
			},
			last: "20200809-112601.pdf",
		},
		{
			filenames: []string{
				"20200809-112601_duplex-even.pdf",
				"20200809-112401_duplex-even.pdf",
				"20200809-112501.pdf",
			},
			last: "20200809-112601_duplex-even.pdf",
		},
	}

	for _, test := range tests {
		// make a local copy of the range var
		test := test

		t.Run("", func(t *testing.T) {
			t.Parallel()

			tempdir := t.TempDir()

			for _, filename := range test.filenames {
				err := os.WriteFile(filepath.Join(tempdir, filename), []byte(filename), 0600)
				if err != nil {
					t.Fatalf("writing %v: %v", filename, err)
				}
			}

			last, err := FindLastFilename(tempdir, test.current)
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
