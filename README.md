# Simple PDF Archive

This program implements an archive for scanned PDF documents. It consists of several different parts:

 * An ingester process which runs an FTP server and collects files from an `incoming` directory. Files are de-duplexed and then processed with the program `ocrmypdf` which runs OCR and optimizes the PDF files
 * A sorter process which stores processed files from the ingester into a directory structure

The directory structure is as follows:

 * `incoming/` place new files here (also used by the FTP server)
 * `db.json` contains data about the individual files
 * `archive/` contains subdirs for each correspondent, which then contains the files

# FTP Server

For testing the FTP server, the script `upload.lftp` can be used to upload two
PDF files with odd and even pages. The even pages are in reverse order. Sample
files can be found in `testdata/`. The script is run like this:

    lftp -f upload.lftp

In order for this to work, the file names must start with the strings
`duplex-odd` and `duplex-even`, and the upload must happen in that order (first
odd then even).

The service can be run with systemd socket activation, sample unit files can be
found in the `doc/` subdir.
