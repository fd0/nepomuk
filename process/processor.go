package process

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Processor handles OCR and optimizations for incoming files.
type Processor struct {
	ProcessedDir string
	TempDir      string

	OnFileProcessed func(string)
}

// processFile runs post processing for a single file. On success, the source
// file is removed and the filename of the processed file (within ProcessedDir)
// is returned.
func (p *Processor) processFile(ctx context.Context, filename string) (string, error) {
	// ignore first files of duplex documents (with the odd pages)
	if strings.HasSuffix(filename, "_duplex-odd.pdf") {
		log.Printf("%v: ignoring file with odd pages for now", filename)
		return "", nil
	}

	// try to join duplex pages when the second file (with the even pages) is uploaded
	if strings.HasSuffix(filename, "_duplex-even.pdf") {
		sourcefile, err := TryJoinPages(filename)
		if err != nil {
			return "", fmt.Errorf("de-duplexing failed: %w", err)
		}

		log.Printf("joined file is at %v", sourcefile)

		// process the joined file name
		return p.processFile(ctx, sourcefile)
	}

	log.Printf("running post-process for %v", filepath.Base(filename))

	processed, err := PostProcess(ctx, p.ProcessedDir, filename)
	if err != nil {
		return "", fmt.Errorf("post-processing: %v", err)
	}

	err = os.Remove(filename)
	if err != nil {
		return "", fmt.Errorf("remove source %v failed: %w", filename, err)
	}

	return processed, nil
}

func (p *Processor) Run(ctx context.Context, newFiles <-chan string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case filename := <-newFiles:
			processedFile, err := p.processFile(ctx, filename)
			if err != nil {
				log.Printf("process %v failed: %v", filepath.Base(filename), err)
				continue
			}

			// skip files that cannot be processed right now
			if processedFile == "" {
				continue
			}

			p.OnFileProcessed(processedFile)
		}
	}
}
