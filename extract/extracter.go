package extract

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fd0/nepomuk/database"
)

const DirectoryUnknownCorrespondent = "unknown"

// Extracter extracts data from files and moves them into the right directory.
type Extracter struct {
	ArchiveDir   string
	ProcessedDir string
	Database     *database.Database

	Correspondents []Correspondent
}

func generateFilename(id string, a database.Annotation) (string, error) {
	date, err := time.Parse("02.01.2006", a.Date)
	if err != nil {
		return "", fmt.Errorf("parse date %q failed: %w", a.Date, err)
	}

	return fmt.Sprintf("%s-%s-%s.pdf", date.Format("20060102"), id, a.Title), nil
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

	a := database.Annotation{
		Title: strings.TrimRight(filepath.Base(filename), ".pdf"),
	}

	a.Correspondent, err = FindCorrespondent(s.Correspondents, text)

	if err != nil {
		log.Printf("unable to find correspondent for %v: %v", filename, err)

		a.Correspondent = ""
	}

	a.Date, err = Date(filename, text)
	if err != nil {
		return fmt.Errorf("unable to find date for %v: %w", filename, err)
	}

	log.Printf("data for %v (%v): %+v", filepath.Base(filename), id, a)

	s.Database.SetAnnotation(id, a)

	newFilename, err := generateFilename(id, a)
	if err != nil {
		return fmt.Errorf("generate filename for %v failed: %w", filename, err)
	}

	// if correspondent could be found, create dir and move the file there
	// otherwise, move it to the "unknown" directory
	newLocation := filepath.Join(s.ArchiveDir, DirectoryUnknownCorrespondent, newFilename)
	if a.Correspondent != "" {
		newLocation = filepath.Join(s.ArchiveDir, a.Correspondent, newFilename)
	}

	err = os.MkdirAll(filepath.Dir(newLocation), 0755)
	if err != nil {
		return fmt.Errorf("unable to create dir for target file %v: %w", newLocation, err)
	}

	err = os.Rename(filename, newLocation)
	if err != nil {
		return fmt.Errorf("move %v -> %v failed: %w", filename, newLocation, err)
	}

	log.Printf("%v moved to %v", filepath.Base(filename), newLocation)

	return nil
}

func (s *Extracter) Run(ctx context.Context, inFiles <-chan string) error {
	// process all pre-existing files
	entries, err := ioutil.ReadDir(s.ProcessedDir)
	if err != nil {
		return fmt.Errorf("readdir %v: %w", s.ProcessedDir, err)
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
