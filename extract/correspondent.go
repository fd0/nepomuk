package extract

import (
	"bytes"
	"errors"
)

type Correspondent struct {
	Name     string
	Contains string
}

func (c *Correspondent) Matches(data []byte) bool {
	return bytes.Contains(bytes.ToLower(data), bytes.ToLower([]byte(c.Contains)))
}

func FindCorrespondent(correspondents []Correspondent, data []byte) (string, error) {
	for _, c := range correspondents {
		if c.Matches(data) {
			return c.Name, nil
		}
	}

	return "", errors.New("correspondent not found")
}
