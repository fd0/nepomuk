package database

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// DB is the serialized data structure of a database.
type DB struct {
	Annotations map[string]File `yaml:"annotations"`
}

type Database struct {
	DB

	// OnChange is called when the annotation for a file is changed.
	OnChange func(id string, oldAnnotation, newAnnotation File) `yaml:"-"`
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
		DB: DB{
			Annotations: make(map[string]File),
		},
	}
}

// Load loads a database from file. If the file does not exist, an empty Database is returned.
func (db *Database) Load(filename string) error {
	f, err := os.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		db.DB = DB{
			Annotations: make(map[string]File),
		}

		return nil
	}

	if err != nil {
		return fmt.Errorf("open database failed: %w", err)
	}

	db.DB = DB{}

	err = json.NewDecoder(f).Decode(&db.DB)
	if err != nil {
		_ = f.Close()

		return fmt.Errorf("decode database %v failed: %w", filename, err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("close database %v failed: %w", filename, err)
	}

	return nil
}

// Save saves the database to filename.
func (db *Database) Save(filename string) error {
	f, err := os.OpenFile(filename, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("save database %v failed: %w", filename, err)
	}

	err = json.NewEncoder(f).Encode(db.DB)
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
	old := db.Annotations[id]
	db.Annotations[id] = a

	if db.OnChange != nil {
		db.OnChange(id, old, a)
	}
}

// Filename returns the filename based on the metadata.
func (db *Database) Filename(id string) (string, error) {
	file, ok := db.DB.Annotations[id]
	if !ok {
		return "", errors.New("id not found")
	}

	date, err := time.Parse("02.01.2006", file.Date)
	if err != nil {
		return "", fmt.Errorf("parse date %q failed: %w", file.Date, err)
	}

	filename := date.Format("2006-01-02")
	if file.Title != "" {
		filename += " " + file.Title
	}

	filename += " " + id
	filename += ".pdf"

	return filename, nil
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
