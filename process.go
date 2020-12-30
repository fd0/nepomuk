package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fd0/nepomuk/process"
)

func processFile(ctx context.Context, dataDir string, filename string) error {
	log.Printf("process new file %v", filename)

	if !strings.HasSuffix(filename, "_duplex-odd.pdf") {
		go func() {

			var (
				sourcefile string
				err        error
			)

			if strings.HasSuffix(filename, "_duplex-even.pdf") {
				sourcefile, err = process.TryJoinPages(dataDir, filename)
				if err != nil {
					log.Printf("de-duplex pages: %v", err)
				}
			} else {
				sourcefile = filepath.Join(dataDir, filename)
			}

			log.Printf("running post-process for %v in the background", sourcefile)

			processed, err := process.PostProcess(sourcefile)
			if err != nil {
				log.Printf("post-processing %v failed: %v", sourcefile, err)
			} else {
				log.Printf("successfully ran post-process on %v", sourcefile)

				err = os.Rename(processed, sourcefile)
				if err != nil {
					log.Printf("renaming %v failed: %v", sourcefile, err)
				}
			}
		}()
	}

	return nil
}

func RunProcess(ctx context.Context, newFiles <-chan string, dataDir string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case filename := <-newFiles:
			err := processFile(ctx, dataDir, filename)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
