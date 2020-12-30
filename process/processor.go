package process

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

// Processor handles OCR and optimizations for incoming files.
type Processor struct {
	DataDir string
	TempDir string
}

func (p *Processor) processFile(ctx context.Context, filename string) error {
	log.Printf("process new file %v", filename)

	// ignore first files of duplex documents (with the odd pages)
	if strings.HasSuffix(filename, "_duplex-odd.pdf") {
		return nil
	}

	// try to join duplex pages when the second file (with the even pages) is uploaded
	if strings.HasSuffix(filename, "_duplex-even.pdf") {
		sourcefile, err := TryJoinPages(filename)
		if err != nil {
			return fmt.Errorf("de-duplexing failed: %w", err)
		}

		log.Printf("joined file is at %v", sourcefile)

		// process the joined file name
		return p.processFile(ctx, sourcefile)
	}

	log.Printf("running post-process for %v", filename)

	processed, err := PostProcess(ctx, p.DataDir, filename)
	if err != nil {
		log.Printf("post-processing %v failed: %v", filename, err)
	} else {
		log.Printf("successfully ran post-process on %v", filename)

		err = os.Rename(processed, filename)
		if err != nil {
			log.Printf("renaming %v failed: %v", filename, err)
		}
	}

	return nil
}

func (p *Processor) Run(ctx context.Context, newFiles <-chan string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case filename := <-newFiles:
			err := p.processFile(ctx, filename)
			if err != nil {
				log.Printf("processing %v failed: %v", filename, err)
				continue
			}
		}
	}
}
