package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// ExtractText returns the text in the pdf file.
func ExtractText(filename string) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	cmd := exec.Command("pdftotext", filename, "-")
	cmd.Stderr = os.Stderr
	cmd.Stdout = buf

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("pdftotext: %w", err)
	}

	return buf.Bytes(), nil
}
