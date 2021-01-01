package extract

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"time"
)

var (
	dateRegexp          = regexp.MustCompile(`[0123][0-9]\.[01][0-9]\.(19|20)\d{2}`)
	filnenameDateRegexp = regexp.MustCompile(`^((19|20)\d{6})\-`)
)

// reformatDate parses the date in s according to format and returns the standard format DD.MM.YYYY.
func reformatDate(s, format string) (string, error) {
	d, err := time.Parse(format, s)
	if err != nil {
		return "", fmt.Errorf("parse %q as %v failed: %w", s, format, err)
	}

	return d.Format("02.01.2006"), nil
}

// Date returns the first date found in the text, if that fails it tries to
// extract the date from filename.
func Date(filename string, text []byte) (string, error) {
	date := dateRegexp.Find(text)
	if date != nil {
		s, err := reformatDate(string(date), "02.01.2006")
		if err != nil {
			return "", err
		}

		return s, nil
	}

	// try to extract date from filename
	matches := filnenameDateRegexp.FindStringSubmatch(filepath.Base(filename))
	if matches != nil {
		s, err := reformatDate(matches[1], "20060102")
		if err != nil {
			return "", err
		}

		return s, nil
	}

	return "", errors.New("no date found")
}
