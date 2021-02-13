package database

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

var filenameRegex = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}) ?(.*?)? ?([a-f0-9]{8})?\.pdf$`)

var ErrInvalidFilename = errors.New("invalid file name")

// reformatDate parses the date in s according to format and returns the standard format DD.MM.YYYY.
func reformatDate(s, format string) (string, error) {
	d, err := time.Parse(format, s)
	if err != nil {
		return "", fmt.Errorf("parse %q as %v failed: %w", s, format, err)
	}

	return d.Format("02.01.2006"), nil
}

// ParseFilename tries to extract the metadata of a file from the file name.
func ParseFilename(filename string) (date, title, id string, err error) {
	if !filenameRegex.MatchString(filename) {
		return "", "", "", ErrInvalidFilename
	}

	matches := filenameRegex.FindStringSubmatch(filename)
	if len(matches) != 4 {
		return "", "", "", ErrInvalidFilename
	}

	date, err = reformatDate(matches[1], "2006-01-02")
	if err != nil {
		return "", "", "", err
	}

	return date, matches[2], matches[3], nil
}
