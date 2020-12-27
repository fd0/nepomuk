package main

import (
	"errors"
	"regexp"
)

var dateRegexp = regexp.MustCompile(`\d{2}\.\d{2}\.\d{4}`)

func ExtractDate(data []byte) (string, error) {
	date := dateRegexp.Find(data)
	if date == nil {
		return "", errors.New("no date found")
	}

	return string(date), nil
}
