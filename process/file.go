package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/sirupsen/logrus"
)

var filenameRegex = regexp.MustCompile(`^\d{8}-\d{6}(_duplex-odd|_duplex-even)?\.pdf$`)

var ErrNoLastFileFound = errors.New("no last file found")

func readdirnames(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("open %v: %w", dir, err)
	}

	names, err := f.Readdirnames(-1)
	if err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("readdir %v: %w", dir, err)
	}

	err = f.Close()
	if err != nil {
		return nil, fmt.Errorf("close %v: %w", dir, err)
	}

	return names, nil
}

func FindLastFilename(dir, currentFilename string) (string, error) {
	names, err := readdirnames(dir)
	if err != nil {
		return "", fmt.Errorf("readdirnames: %w", err)
	}

	matches := make([]string, 0, len(names))

	for _, entry := range names {
		if entry == currentFilename {
			continue
		}

		if !filenameRegex.MatchString(entry) {
			continue
		}

		matches = append(matches, entry)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i] >= matches[j]
	})

	if len(matches) > 0 {
		return matches[0], nil
	}

	return "", ErrNoLastFileFound
}

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

func JoinPages(dir, odd, even string) (filename string, err error) {
	var tempdir string

	tempdir, err = os.MkdirTemp("", "join-duplex-")
	if err != nil {
		return "", fmt.Errorf("tempdir: %w", err)
	}

	defer func() {
		e := os.RemoveAll(tempdir)

		if err == nil {
			err = e
		}
	}()

	oddDir := filepath.Join(tempdir, "odd")
	evenDir := filepath.Join(tempdir, "even")

	for _, d := range []string{oddDir, evenDir} {
		err := os.MkdirAll(d, 0700)
		if err != nil {
			return "", fmt.Errorf("create tempdir %v: %w", d, err)
		}
	}

	err = pdfcpu.SplitFile(filepath.Join(dir, odd), oddDir, 1, nil)
	if err != nil {
		return "", fmt.Errorf("split file: %w", err)
	}

	oddFiles, err := readdirnames(oddDir)
	if err != nil {
		return "", fmt.Errorf("read dir %v: %w", oddDir, err)
	}

	err = pdfcpu.SplitFile(filepath.Join(dir, even), evenDir, 1, nil)
	if err != nil {
		return "", fmt.Errorf("split file: %w", err)
	}

	sort.Sort(Files(oddFiles))

	evenFiles, err := readdirnames(evenDir)
	if err != nil {
		return "", fmt.Errorf("read dir %v: %w", oddDir, err)
	}

	sort.Sort(sort.Reverse(Files(evenFiles)))

	var files []string
	for i := 0; i < len(oddFiles); i++ {
		files = append(files, filepath.Join(oddDir, oddFiles[i]))

		if i < len(evenFiles) {
			files = append(files, filepath.Join(evenDir, evenFiles[i]))
		}
	}

	targetfile := filepath.Join(dir, strings.Split(even, "_")[0]+".pdf")

	err = pdfcpu.MergeCreateFile(files, targetfile, false, nil)
	if err != nil {
		return "", fmt.Errorf("merge file: %w", err)
	}

	err = os.Remove(filepath.Join(dir, odd))
	if err != nil {
		return "", fmt.Errorf("remove %v: %w", odd, err)
	}

	err = os.Remove(filepath.Join(dir, even))
	if err != nil {
		return "", fmt.Errorf("remove %v: %w", even, err)
	}

	return targetfile, nil
}

// TryJoinPages finds the matching file with odd pages and creates a joined
// document. The other file is searched for in the same directory. On success,
// the sources files are removed.
func TryJoinPages(log logrus.FieldLogger, filename string) (string, error) {
	dir := filepath.Dir(filename)
	file := filepath.Base(filename)

	lastfile, err := FindLastFilename(dir, file)
	if err != nil {
		return "", fmt.Errorf("find last file in %v for %v failed: %w", dir, file, err)
	}

	log.Printf("trying to join pages with %v and %v", lastfile, file)

	if !strings.HasSuffix(lastfile, "_duplex-odd.pdf") {
		return "", fmt.Errorf("odd pages for %v not found", file)
	}

	combined, err := JoinPages(dir, lastfile, file)
	if err != nil {
		return "", fmt.Errorf("joining pages for %v and %v failed: %w", lastfile, file, err)
	}

	return combined, nil
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
