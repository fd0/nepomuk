package extract

import (
	"errors"
	"regexp"
)

var dateRegexp = regexp.MustCompile(`\d{2}\.\d{2}\.\d{4}`)
var filnenameDateRegexp = regexp.MustCompile(`^(\d{8})\-(\d{6})`)

// Date returns the first date found in the text, if that fails it tries to
// extract the date from filename.
func Date(filename string, text []byte) (string, error) {
	date := dateRegexp.Find(text)
	if date != nil {
		return string(date), nil
	}

	// try to extract date from filename
	matches := filnenameDateRegexp.FindStringSubmatch(filename)
	if matches != nil {
		return matches[1], nil
	}

	return "", errors.New("no date found")
}
