package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
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
		return nil, err
	}

	names, err := f.Readdirnames(-1)
	if err != nil {
		_ = f.Close()

		return nil, err
	}

	err = f.Close()
	if err != nil {
		return nil, err
	}

	return names, nil
}

func FindLastFilename(dir, currentFilename string) (string, error) {
	names, err := readdirnames(dir)
	if err != nil {
		return "", err
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

	log.Printf("upload new file named %v as %v", path, name)

	f, err := os.Create(filepath.Join(targetdir, name))
	if err != nil {
		return "", 0, err
	}

	n, err = io.Copy(f, rd)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())

		return "", n, err
	}

	err = f.Close()
	if err != nil {
		return "", n, err
	}

	return name, n, nil
}

func JoinPages(dir, odd, even string) (filename string, err error) {
	var tempdir string

	tempdir, err = ioutil.TempDir("", "join-duplex-")
	if err != nil {
		return "", err
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

	sort.Strings(oddFiles)

	evenFiles, err := readdirnames(evenDir)
	if err != nil {
		return "", fmt.Errorf("read dir %v: %w", oddDir, err)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(evenFiles)))

	var files []string
	for i := 0; i < len(oddFiles); i++ {
		files = append(files, filepath.Join(oddDir, oddFiles[i]))

		if i < len(evenFiles) {
			files = append(files, filepath.Join(evenDir, evenFiles[i]))
		}
	}

	targetfile := strings.Split(even, "_")[0] + ".pdf"

	err = pdfcpu.MergeCreateFile(files, filepath.Join(dir, targetfile), nil)
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

func TryJoinPages(targetdir, filename string) {
	lastfile, err := FindLastFilename(targetdir, filename)
	if err != nil {
		log.Printf("error finding last file in %v: %v", targetdir, err)

		return
	}

	log.Printf("trying to join pages, filename %v, last %v", filename, lastfile)

	if !strings.HasSuffix(lastfile, "_duplex-odd.pdf") {
		log.Printf("odd pages not found")

		return
	}

	combined, err := JoinPages(targetdir, lastfile, filename)
	if err != nil {
		log.Printf("joining pages for %v and %v failed: %v", lastfile, filename, err)

		return
	}

	log.Printf("successfully joined %v and %v into %v", lastfile, filename, combined)
}
