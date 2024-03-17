package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// Files is used to sort a list of files in naming order (so foo1.pdf is
// followed by foo2.pdf, not foo10.pdf).
type Files []string

func (f Files) Len() int {
	return len(f)
}

func (f Files) Less(i, j int) bool {
	if len(f[i]) < len(f[j]) {
		return true
	}

	if len(f[i]) > len(f[j]) {
		return false
	}

	return f[i] < f[j]
}

func (f Files) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

// PostProcess runs OCR and optimizations on filename. On success, the file is
// written to targetDir.
func PostProcess(ctx context.Context, log logrus.FieldLogger, targetDir, filename string) (string, error) {
	fi, err := os.Lstat(filename)
	if err != nil {
		return "", fmt.Errorf("stat: %w", err)
	}

	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("file %v is not regular file, mode: %v", filename, fi.Mode())
	}

	// ignore files of size zero
	if fi.Size() == 0 {
		log.Infof("ignore empty file %v", filename)

		return "", nil
	}

	dest := filepath.Join(targetDir, filepath.Base(filename))

	stderr := bytes.NewBuffer(nil)

	cmd := exec.CommandContext(ctx,
		"ocrmypdf",
		// "--quiet",
		"--deskew", "--clean", "--clean-final",
		"--language", "deu", // use Germany by default
		"--skip-text", // skip OCR for pages which already have text
		// "--remove-backgound", // try to make files smaller by removing the background
		filename, dest)
	cmd.Stderr = stderr

	err = cmd.Run()

	var exiterr *exec.ExitError
	if errors.As(err, &exiterr) {
		if exiterr.ExitCode() == 10 {
			// docs say: A valid PDF was created, PDF/A conversion failed. The file will be available.
			// we just ignore this error
			err = nil
		}
	}

	if err != nil {
		return "", fmt.Errorf("run ocrmypdf: %w, stderr: %v", err, stderr.String())
	}

	return dest, nil
}
