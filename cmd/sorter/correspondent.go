package main

import (
	"bytes"
	"errors"
)

type Correspondent struct {
	Name     string
	Contains []byte
}

var Correspondents = []Correspondent{
	{Name: "ASEAG", Contains: []byte("aseag")},
	{Name: "Hauselfen", Contains: []byte("hauselfen")},
	{Name: "Deutsche Rentenversicherung", Contains: []byte("deutsche rentenversicherung")},
	{Name: "Finanzamt", Contains: []byte("finanzamt")},
	{Name: "Stadt Aachen", Contains: []byte("stadt aachen")},
}

func (c *Correspondent) Matches(data []byte) bool {
	return bytes.Contains(bytes.ToLower(data), bytes.ToLower(c.Contains))
}

func FindCorrespondent(data []byte) (string, error) {
	for _, c := range Correspondents {
		if c.Matches(data) {
			return c.Name, nil
		}
	}

	return "", errors.New("correspondent not found")
}
