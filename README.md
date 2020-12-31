# Nepomuk PDF Archive

This program implements an archive for scanned PDF documents. The directory structure is as follows:

 * `incoming/` place new files here manually
 * `uploaded/` contains files uploaded via the FTP server
 * `processed/` holds files optimized and OCRed before sorting
 * `db.json` contains data about the individual files
 * `archive/` contains subdirs for each correspondent, which then contains the files
 * `config.yml` defines the matchers for the correspondents, among other things

File names within `archive/Foo` (for correspondent called `Foo`) are constructed of the following fields, joined by dashes:

 * Date (`YYYYMMDD`)
 * Title
 * ID (first four byte of the SHA256 hash of the file's content, in lower-case hex characters, e.g. `3c18aae3`)

Example: `20201132-Title of the Document-3c18aae3.pdf`:

The ID is used to look up the file in the `db.json` file, which contains additional metadata.

# FTP Server

For testing the FTP server, the script `upload.lftp` can be used to upload two
PDF files with odd and even pages. The even pages are in reverse order. Sample
files can be found in `testdata/`. The script is run like this:

    lftp -f upload.lftp

In order for this to work, the file names must start with the strings
`duplex-odd` and `duplex-even`, and the upload must happen in that order (first
odd then even).

# TODO

 * Reload `config.yml` automatically
