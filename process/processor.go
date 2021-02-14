package process

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// Processor handles OCR and optimizations for incoming files.
type Processor struct {
	ProcessedDir string
	TempDir      string

	log logrus.FieldLogger

	OnFileProcessed func(string)
}

// SetLogger updates the logger to use.
func (p *Processor) SetLogger(logger logrus.FieldLogger) {
	p.log = logger.WithField("component", "processor")
}

// processFile runs post processing for a single file. On success, the source
// file is removed and the filename of the processed file (within ProcessedDir)
// is returned.
func (p *Processor) processFile(ctx context.Context, filename string) (string, error) {
	log := p.log.WithField("filename", filename)

	// ignore first files of duplex documents (with the odd pages)
	if strings.HasSuffix(filename, "_duplex-odd.pdf") {
		log.Info("ignore file with odd pages")

		return "", nil
	}

	// try to join duplex pages when the second file (with the even pages) is uploaded
	if strings.HasSuffix(filename, "_duplex-even.pdf") {
		sourcefile, err := TryJoinPages(log, filename)
		if err != nil {
			return "", fmt.Errorf("de-duplexing failed: %w", err)
		}

		log.Infof("joined file is at %v", sourcefile)

		// process the joined file name
		return p.processFile(ctx, sourcefile)
	}

	log.Infof("running post-process")

	processed, err := PostProcess(ctx, p.ProcessedDir, filename)
	if err != nil {
		return "", fmt.Errorf("post-processing: %w", err)
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
				p.log.WithField("filename", filename).Warnf("process failed: %v", err)

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
