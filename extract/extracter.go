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
	"golang.org/x/sys/unix"
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

	file := database.File{
		Title: strings.TrimRight(filepath.Base(filename), ".pdf"),
	}

	file.Correspondent, err = FindCorrespondent(s.Correspondents, text)

	if err != nil {
		log.Info(err)

		file.Correspondent = ""
	}

	file.Date, err = Date(filename, text)
	if err != nil {
		log.Infof("find date failed: %v, using today", err)

		// use today's date for now
		file.Date = time.Now().Format("02.01.2006")
	}

	log.WithField("data", file).Print("found data")

	// try to find a unique name, just in case the file at the location already exists
	for counter := 0; ; counter++ {
		rnd := ""
		if counter != 0 {
			rnd = fmt.Sprintf("- %d", counter)
		}

		newFilename, err := file.GenerateFilename(rnd)
		if err != nil {
			return fmt.Errorf("generate filename for %v failed: %w", filename, err)
		}

		// if correspondent could be found, create dir and move the file there
		// otherwise, move it to the "unknown" directory
		newLocation := filepath.Join(s.ArchiveDir, DirectoryUnknownCorrespondent, newFilename)
		if file.Correspondent != "" {
			newLocation = filepath.Join(s.ArchiveDir, file.Correspondent, newFilename)
		}

		err = os.MkdirAll(filepath.Dir(newLocation), newDirMode)
		if err != nil {
			return fmt.Errorf("unable to create dir for target file %v: %w", newLocation, err)
		}

		err = unix.Renameat2(unix.AT_FDCWD, filename, unix.AT_FDCWD, newLocation, unix.RENAME_NOREPLACE)
		if os.IsExist(err) {
			log.Warnf("destination file already exists, retrying with new filename")

			continue
		}

		if err != nil {
			return fmt.Errorf("move %v -> %v failed: %w", filename, newLocation, err)
		}

		file.Filename = newFilename
		s.Database.SetFile(id, file)

		err = os.Chmod(newLocation, destinationFileMode)
		if err != nil {
			return fmt.Errorf("chmod %v failed: %w", newLocation, err)
		}

		log.Printf("extracter: move to %v", newLocation)

		break
	}

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
