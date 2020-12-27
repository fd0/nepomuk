package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func CheckTargetDir(dir string) error {
	fi, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		log.Printf("creating target dir %v", dir)

		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("creating target dir %v: %w", dir, err)
		}

		fi, err = os.Lstat(dir)
	}

	if err != nil {
		return fmt.Errorf("accessing target dir %v: %w", dir, err)
	}

	if !fi.IsDir() {
		return fmt.Errorf("target dir %v is not a directory", dir)
	}

	return nil
}

func processFile(filename string) error {
	data, err := ExtractText(filename)
	if err != nil {
		return fmt.Errorf("error processing %v: %w", filename, err)
	}

	correspondent, err := FindCorrespondent(data)
	if err != nil {
		log.Printf("error processing %v: %v", filename, err)
	}

	date, err := ExtractDate(data)
	if err != nil {
		log.Printf("error extracting date: %v", err)
	}

	log.Printf("%v: correspondent %q, date %v", filename, correspondent, date)

	return nil
}

func processFiles(incoming, target string) error {
	err := CheckTargetDir(incoming)
	if err != nil {
		return err
	}

	entries, err := ioutil.ReadDir(incoming)
	if err != nil {
		return fmt.Errorf("readdir: %w", err)
	}

	for _, entry := range entries {
		err := processFile(filepath.Join(incoming, entry.Name()))
		if err != nil {
			log.Printf("%v: error %v", entry.Name(), err)
		}
	}

	return nil
}
