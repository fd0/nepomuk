package extract

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fd0/nepomuk/database"
	"github.com/sirupsen/logrus"
)

const DirectoryUnknownCorrespondent = "unknown"

// Extracter extracts data from files and moves them into the right directory.
type Extracter struct {
	ArchiveDir   string
	ProcessedDir string
	Database     *database.Database

	log logrus.FieldLogger

	Correspondents []Correspondent
}

const (
	newDirMode          = 0755
	destinationFileMode = 0444
)

// SetLogger updates the logger to use.
func (s *Extracter) SetLogger(logger logrus.FieldLogger) {
	s.log = logger.WithField("component", "extracter")
}

func (s *Extracter) processFile(filename string) error {
	id, err := database.FileID(filename)
	if err != nil {
		return fmt.Errorf("ID for %v failed: %w", filename, err)
	}

	log := s.log.WithField("filename", filename).WithField("id", id)

	text, err := Text(filename)
	if err != nil {
		return fmt.Errorf("extract text from %v failed: %w", filename, err)
	}

	a := database.File{
		Title: strings.TrimRight(filepath.Base(filename), ".pdf"),
	}

	a.Correspondent, err = FindCorrespondent(s.Correspondents, text)

	if err != nil {
		log.Info(err)

		a.Correspondent = ""
	}

	a.Date, err = Date(filename, text)
	if err != nil {
		log.Infof("find date failed: %v, using today", err)

		// use today's date for now
		a.Date = time.Now().Format("02.01.2006")
	}

	log.WithField("data", a).Print("found data")

	s.Database.SetFile(id, a)

	newFilename, err := s.Database.Filename(id)
	if err != nil {
		return fmt.Errorf("generate filename for %v failed: %w", filename, err)
	}

	// if correspondent could be found, create dir and move the file there
	// otherwise, move it to the "unknown" directory
	newLocation := filepath.Join(s.ArchiveDir, DirectoryUnknownCorrespondent, newFilename)
	if a.Correspondent != "" {
		newLocation = filepath.Join(s.ArchiveDir, a.Correspondent, newFilename)
	}

	err = os.MkdirAll(filepath.Dir(newLocation), newDirMode)
	if err != nil {
		return fmt.Errorf("unable to create dir for target file %v: %w", newLocation, err)
	}

	err = os.Rename(filename, newLocation)
	if err != nil {
		return fmt.Errorf("move %v -> %v failed: %w", filename, newLocation, err)
	}

	err = os.Chmod(newLocation, destinationFileMode)
	if err != nil {
		return fmt.Errorf("chmod %v failed: %w", newLocation, err)
	}

	log.Printf("extracter: move to %v", newLocation)

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
			s.log.WithField("filename", filename).Warnf("error: %v", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case filename := <-inFiles:
			err := s.processFile(filename)
			if err != nil {
				s.log.WithField("filename", filename).Warnf("error: %v", err)
			}
		}
	}
}
