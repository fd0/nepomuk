package extract

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/fd0/nepomuk/database"
)

// Extracter extracts data from files and moves them into the right directory.
type Extracter struct {
	ArchiveDir   string
	ProcessedDir string
	Database     *database.Database

	Correspondents []Correspondent
}

func (s *Extracter) processFile(filename string) error {
	id, err := database.FileID(filename)
	if err != nil {
		return fmt.Errorf("ID for %v failed: %w", filename, err)
	}

	text, err := Text(filename)
	if err != nil {
		return fmt.Errorf("extract text from %v failed: %w", filename, err)
	}

	a := database.Annotation{}
	a.Correspondent, err = FindCorrespondent(s.Correspondents, text)
	if err != nil {
		log.Printf("unable to find correspondent for %v: %v", filename, err)
		a.Correspondent = ""
	}

	a.Date, err = Date(filename, text)
	if err != nil {
		return fmt.Errorf("unable to find date for %v: %v", filename, err)
	}

	log.Printf("data for %v (%v): %+v", filename, id, a)

	s.Database.SetAnnotation(id, a)

	// filename := fmt.Sprintf("%v")

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
