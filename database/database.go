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
	Annotations map[string]File `yaml:"annotations"`
}

type File struct {
	Filename      string `yaml:"filename"`
	Correspondent string `yaml:"correspondent"`
	Date          string `yaml:"date"`
	Title         string `yaml:"title"`
}

// New returns a new empty database.
func New() *Database {
	return &Database{
		Annotations: make(map[string]File),
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

// GetFile returns the metadata for a file ID.
func (db *Database) GetFile(id string) (File, bool) {
	a, ok := db.Annotations[id]

	return a, ok
}

// SetFile updates the metadata for a file ID.
func (db *Database) SetFile(id string, a File) {
	db.Annotations[id] = a
}

// Filename returns the filename based on the metadata.
func (db *Database) Filename(id string) string {
	return ""
}

// ParseFilename tries to extract the metadata of a file from the file name.
func ParseFilename(filename string) (File, error) {
	return File{}, nil
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
