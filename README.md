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

For testing the FTP server, the scripts named `upload-*.lftp` can be used. The
scripts as well as some sample PDF files can be found in the `testdata/`
directory, run `lftp -f testdata/upload-duplex.lftp`.

If two files with filenames starting with `duplex-odd` followed by
`duplex-even` are uploaded, the archive will join them. This can be used to
easily scan duplex documents with a simplex only scanner (e.g. with document
feeder) by first scanning the odd pages, turning the whole paper stack around
and scanning the even pages backwards. This means the even pages are in reverse
order. The script `upload-duplex.lftp` tests this.

PDF files with the prefix `Receipt` will be split into several documents with
exactly one page per document. This is used to scan a stack of single page
documents in one run.

# TODO

 * Reload `config.yml` automatically
