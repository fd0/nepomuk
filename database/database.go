package database

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// DB is the serialized data structure of a database.
type DB struct {
	Annotations map[string]File `yaml:"annotations"`
}

type Database struct {
	DB
	Dir string

	log logrus.FieldLogger

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
func New(dir string) *Database {
	return &Database{
		DB: DB{
			Annotations: make(map[string]File),
		},
		Dir: dir,
		log: logrus.StandardLogger(),
	}
}

// SetLogger sets the logger the database will use.
func (db *Database) SetLogger(logger logrus.FieldLogger) {
	db.log = logger.WithField("component", "database")
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

	if db.OnChange != nil && old != a {
		db.OnChange(id, old, a)
	}
}

// Delete removes an entry from the database.
func (db *Database) Delete(id string) {
	old, ok := db.Annotations[id]
	if !ok {
		return
	}

	delete(db.Annotations, id)

	if db.OnChange != nil {
		db.OnChange(id, old, File{})
	}
}

// Scan traverses the database directory and synchronizes it with the internal
// database.
func (db *Database) Scan() error {
	db.log.Infof("synchronize database and files in %v", db.Dir)

	// first, insert or update all files found in the dir
	dirs, err := ioutil.ReadDir(db.Dir)
	if err != nil {
		return fmt.Errorf("readdir %v failed: %w", db.Dir, err)
	}

	for _, fi := range dirs {
		if strings.HasPrefix(fi.Name(), ".") {
			// ignore hidden entries
			continue
		}

		if !fi.IsDir() {
			continue
		}

		subdir := filepath.Join(db.Dir, fi.Name())
		err := db.scanSubdir(subdir)
		if err != nil {
			db.log.Warnf("scan %v failed: %v", subdir, err)
		}
	}

	// next, make sure all files in the db exist
	for id, file := range db.DB.Annotations {
		filename := filepath.Join(db.Dir, file.Correspondent, file.Filename)

		_, err := os.Stat(filename)
		if os.IsNotExist(err) {
			db.log.WithField("filename", filename).Info("delete removed file")
			db.Delete(id)
		}
	}

	db.log.Info("successfully synchronized database")

	return nil
}

func (db *Database) scanSubdir(subdir string) error {
	files, err := ioutil.ReadDir(subdir)
	if err != nil {
		return fmt.Errorf("readdri %v failed: %w", subdir, err)
	}

	for _, fi := range files {
		if !fi.Mode().IsRegular() {
			continue
		}

		if !strings.HasSuffix(fi.Name(), ".pdf") {
			db.log.Debugf("ignore non PDF file %v in %v", fi.Name(), subdir)
			continue
		}

		filename := filepath.Join(subdir, fi.Name())
		err := db.OnRename(filename)
		if err != nil {
			db.log.Warnf("scan file %v failed: %v", filename, err)
		}
	}

	return nil
}

// GenerateFilename returns the filename based on the metadata. The string rnd is
// appended to the title (before the extension) if it is not empty.
func (f File) GenerateFilename(rnd string) (string, error) {
	date, err := time.Parse("02.01.2006", f.Date)
	if err != nil {
		return "", fmt.Errorf("parse date %q failed: %w", f.Date, err)
	}

	filename := date.Format("2006-01-02")
	if f.Title != "" {
		filename += " " + f.Title
	}

	if rnd != "" {
		filename += " " + rnd
	}

	filename += ".pdf"

	return filename, nil
}

func (f File) String() string {
	return fmt.Sprintf("<File %q from %q, date %v, title %q>", f.Filename, f.Correspondent, f.Date, f.Title)
}

// OnDelete updates the database when a file is deleted by the user.
func (db *Database) OnDelete(oldName string) error {
	// try to find the filename
	filename := filepath.Base(oldName)
	correspondent := filepath.Base(filepath.Dir(oldName))

	log := db.log.WithField("filename", filename).WithField("correspondent", correspondent)

	for id, file := range db.DB.Annotations {
		if file.Correspondent != correspondent {
			continue
		}

		if file.Filename != filename {
			continue
		}

		log.Infof("delete file %v from database", id)
		db.Delete(id)
		return nil
	}

	return fmt.Errorf("unable to find file %v in database", oldName)
}

// OnRename updates the database when a file is renamed by the user.
func (db *Database) OnRename(newName string) error {
	// check if the new name is a file or dir
	fi, err := os.Lstat(newName)
	if err != nil {
		return fmt.Errorf("lstat error: %w", err)
	}

	if fi.IsDir() {
		// read the dir and trigger rename for all files
		entries, err := ioutil.ReadDir(newName)
		if err != nil {
			return fmt.Errorf("readdir failed: %w", err)
		}

		var firstError error
		for _, entry := range entries {
			filename := filepath.Join(newName, entry.Name())
			err = db.OnRename(filename)
			if err != nil {
				db.log.WithField("filename", filename).Warnf("rename failed: %v", err)
				if firstError == nil {
					firstError = err
				}
			}
		}

		return firstError
	}

	// ignore stuff and files that are not PDF files
	if !fi.Mode().IsRegular() || !strings.HasSuffix(newName, ".pdf") {
		return nil
	}

	// hash the file to get the ID
	id, err := FileID(newName)
	if err != nil {
		return fmt.Errorf("hash new filename failed: %w", err)
	}

	log := db.log.WithField("id", id)

	// extract new metadata from new name
	date, title, err := ParseFilename(filepath.Base(newName))
	if err != nil {
		return fmt.Errorf("parse new filename failed: %w", err)
	}

	// extract new correspondent
	correspondent := filepath.Base(filepath.Dir(newName))

	file, _ := db.GetFile(id)
	fileBefore := file

	file.Date = date
	file.Title = title
	file.Filename = filepath.Base(newName)
	file.Correspondent = correspondent

	if fileBefore != file {
		log.WithField("file", fileBefore).Debug("before")
		log.WithField("file", file).Debug("after")
	}

	db.SetFile(id, file)

	return nil
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
