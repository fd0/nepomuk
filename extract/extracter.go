package extract

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
)

// Extracter extracts data from files and moves them into the right directory.
type Extracter struct {
	ArchiveDir   string
	ProcessedDir string

	Correspondents []Correspondent
}

func (s *Extracter) processFile(filename string) error {
	text, err := Text(filename)
	if err != nil {
		return fmt.Errorf("extract text from %v failed: %w", filename, err)
	}

	correspondent, err := FindCorrespondent(s.Correspondents, text)
	if err != nil {
		log.Printf("unable to find correspondent for %v: %v", filename, err)
		correspondent = ""
	}

	date, err := Date(filename, text)
	if err != nil {
		log.Printf("unable to find date for %v: %v", filename, err)
		date = ""
	}

	log.Printf("data for %v: %v %v", filename, date, correspondent)

	return nil
}

func (s *Extracter) Run(ctx context.Context, inFiles <-chan string) error {
	// process all pre-existing files
	entries, err := ioutil.ReadDir(s.ProcessedDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		filename := filepath.Join(s.ProcessedDir, entry.Name())
		err := s.processFile(filename)
		if err != nil {
			log.Printf("error extracting data from file %v: %v", filename, err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case filename := <-inFiles:
			err := s.processFile(filename)
			if err != nil {
				log.Printf("error extracting data from file %v: %v", filename, err)
			}
		}
	}
}
