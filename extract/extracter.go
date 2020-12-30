package extract

import (
	"context"
	"log"
)

// Extracter extracts data from files and moves them into the right directory.
type Extracter struct {
	ArchiveDir string

	Correspondents []Correspondent
}

func (s *Extracter) processFile(filename string) error {
	log.Printf("extract data from %v", filename)
	return nil
}

func (s *Extracter) Run(ctx context.Context, inFiles <-chan string) error {
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
