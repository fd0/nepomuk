package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"
)

const filenameFormat = "20060102-150405"

var filenameRegex = regexp.MustCompile(`^\d{8}-\d{6}\.pdf$`)

var ErrNoLastFileFound = errors.New("no last file found")

// FindLastFilename scans
func FindLastFilename(dir string) (string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return "", err
	}

	names, err := f.Readdirnames(-1)
	if err != nil {
		return "", err
	}

	matches := make([]string, 0, len(names))

	for _, entry := range names {
		if !filenameRegex.MatchString(entry) {
			fmt.Printf("filename %v does not match regex\n", entry)
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

func SaveFile(targetdir, filename string, rd io.Reader) (int64, error) {
	log.Printf("upload new file named %v", filename)

	ext := filepath.Ext(filename)
	filename = time.Now().Format(filenameFormat) + ext

	f, err := os.Create(filepath.Join(targetdir, filename))
	if err != nil {
		return 0, err
	}

	n, err := io.Copy(f, rd)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())

		return n, err
	}

	err = f.Close()
	if err != nil {
		return n, err
	}

	return n, nil
}
