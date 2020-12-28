package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/spf13/pflag"
)

var opts = struct {
	Incoming  string
	TargetDir string
}{}

func main() {
	log.SetFlags(0)

	fs := pflag.NewFlagSet("sorter", pflag.ContinueOnError)
	fs.StringVar(&opts.TargetDir, "target-dir", "data", "store uploaded files in `dir`")
	fs.StringVar(&opts.Incoming, "incoming", "incoming", "read new files from `dir`")

	err := fs.Parse(os.Args)
	if errors.Is(err, pflag.ErrHelp) {
		os.Exit(0)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err = processFiles(opts.Incoming, opts.TargetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
