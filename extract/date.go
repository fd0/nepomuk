package extract

import (
	"errors"
	"regexp"
)

var dateRegexp = regexp.MustCompile(`\d{2}\.\d{2}\.\d{4}`)
var filnenameDateRegexp = regexp.MustCompile(`(\d{8})\-(\d{6})`)

func ExtractDate(filename string, data []byte) (string, error) {
	date := dateRegexp.Find(data)
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
