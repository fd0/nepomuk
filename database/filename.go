package database

import (
	"errors"
	"regexp"
)

var filenameRegex = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}) ?(.*)? ([a-f0-9]{8})\.pdf$`)

var ErrInvalidFilename = errors.New("invalid file name")

// ParseFilename tries to extract the metadata of a file from the file name.
func ParseFilename(filename string) (date, title, id string, err error) {
	if !filenameRegex.MatchString(filename) {
		return "", "", "", ErrInvalidFilename
	}

	matches := filenameRegex.FindStringSubmatch(filename)
	if len(matches) != 4 {
		return "", "", "", ErrInvalidFilename
	}

	return matches[1], matches[2], matches[3], nil
}
