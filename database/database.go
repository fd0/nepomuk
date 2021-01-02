package database

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type Database struct {
	Annotations map[string]Annotation `yaml:"annotations"`
}

type Annotation struct {
	Correspondent string `yaml:"correspondent"`
	Date          string `yaml:"date"`
	Title         string `yaml:"title"`
}

// New returns a new empty database.
func New() *Database {
	return &Database{
		Annotations: make(map[string]Annotation),
	}
}

// Load loads a database from file. If the file does not exist, an empty Database is returned.
func Load(filename string) (*Database, error) {
	f, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}

	if err != nil {
		return nil, fmt.Errorf("open database failed: %w", err)
	}

	var db Database

	err = json.NewDecoder(f).Decode(&db)
	if err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("decode database %v failed: %w", filename, err)
	}

	err = f.Close()
	if err != nil {
		return nil, fmt.Errorf("close database %v failed: %w", filename, err)
	}

	return &db, nil
}

// Save saves the database to filename.
func (db *Database) Save(filename string) error {
	f, err := os.OpenFile(filename, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("save database %v failed: %w", filename, err)
	}

	err = json.NewEncoder(f).Encode(db)
	if err != nil {
		_ = f.Close()

		return fmt.Errorf("serialize database to JSON failed: %w", err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("close database failed: %w", err)
	}

	return nil
}

// GetAnnotation returns the annotation for a file ID.
func (db *Database) GetAnnotation(id string) (Annotation, bool) {
	a, ok := db.Annotations[id]

	return a, ok
}

// SetAnnotation saves the annotation for a file ID.
func (db *Database) SetAnnotation(id string, a Annotation) {
	db.Annotations[id] = a
}

// FileID returns the ID for filename.
func FileID(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("file ID open %v failed: %w", filename, err)
	}

	hash := sha256.New()

	_, err = io.Copy(hash, f)
	if err != nil {
		_ = f.Close()

		return "", fmt.Errorf("hashing %v failed: %w", filename, err)
	}

	err = f.Close()
	if err != nil {
		return "", fmt.Errorf("close file: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)[:4]), nil
}