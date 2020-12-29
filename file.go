package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/api"
)

const filenameFormat = "20060102-150405"

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

func SaveFile(targetdir, path string, rd io.Reader) (filename string, n int64, err error) {
	ext := filepath.Ext(path)
	basename := filepath.Base(path)
	suffix := ""

	switch {
	case strings.HasPrefix(basename, "duplex-odd"):
		suffix = "_duplex-odd"
	case strings.HasPrefix(basename, "duplex-even"):
		suffix = "_duplex-even"
	}

	name := time.Now().Format(filenameFormat) + suffix + ext

	f, err := os.Create(filepath.Join(targetdir, name))
	if err != nil {
		return "", 0, fmt.Errorf("create: %w", err)
	}

	n, err = io.Copy(f, rd)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())

		return "", n, fmt.Errorf("copy: %w", err)
	}

	err = f.Close()
	if err != nil {
		return "", n, fmt.Errorf("close: %w", err)
	}

	return name, n, nil
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

	tempdir, err = ioutil.TempDir("", "join-duplex-")
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

	err = pdfcpu.MergeCreateFile(files, targetfile, nil)
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

func TryJoinPages(targetdir, filename string) (string, error) {
	lastfile, err := FindLastFilename(targetdir, filename)
	if err != nil {
		return "", fmt.Errorf("find last file in %v: %w", targetdir, err)
	}

	log.Printf("trying to join pages, filename %v, last %v", filename, lastfile)

	if !strings.HasSuffix(lastfile, "_duplex-odd.pdf") {
		return "", fmt.Errorf("odd pages for %v not found", filename)
	}

	combined, err := JoinPages(targetdir, lastfile, filename)
	if err != nil {
		return "", fmt.Errorf("joining pages for %v and %v failed: %W", lastfile, filename, err)
	}

	return combined, nil
}

func PostProcess(filename string) (string, error) {
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	sidecar := base + ".txt"
	dest := base + "_processed" + ext

	cmd := exec.Command(
		"ocrmypdf", "--quiet",
		"--deskew", "--clean", "--clean-final",
		"-l", "deu",
		"--sidecar", sidecar, filename, dest)
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("run ocrmypdf: %w", err)
	}

	return dest, nil
}

func copyFile(sourcefile, destfile string) error {
	src, err := os.Open(sourcefile)
	if err != nil {
		return fmt.Errorf("open %v: %w", sourcefile, err)
	}

	dst, err := os.OpenFile(destfile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		_ = src.Close()
		return fmt.Errorf("create %v: %w", destfile, err)
	}

	_, err = io.Copy(dst, src)
	if err != nil {
		_ = src.Close()
		_ = dst.Close()
		_ = os.Remove(destfile)
		return fmt.Errorf("copy %v -> %v: %w", sourcefile, destfile, err)
	}

	err = src.Close()
	if err != nil {
		_ = dst.Close()
		_ = os.Remove(destfile)
		return fmt.Errorf("close %v: %w", sourcefile, err)
	}

	err = dst.Close()
	if err != nil {
		_ = os.Remove(destfile)
		return fmt.Errorf("close %v: %w", destfile, err)
	}

	return nil
}
