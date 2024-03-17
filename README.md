# Nepomuk PDF Archive

This program implements an archive for scanned PDF documents. Each PDF file has
an auto-detected correspondent. For each correspondent a sub directory is
created and the PDF files for that correspondent are saved in the sub directory.

Within the subdir `.nepomuk`, additional files and directories are stored:

 * `incoming/` place new files here manually
 * `processed/` holds files optimized and OCRed before sorting
 * `db.json` contains data about the individual files

File names within `archive/Foo` (for correspondent called `Foo`) consist of the
date (`YYYY-MM-DD`) followed by the title, with the extension `.pdf`, for
example `2020-11-32 Title of the Document.pdf`. Internally, the archive system
identifies files based on the first four bytes of the SHA256 hash of its
contents. This ID is used to look up the file in the `db.json` file, which
contains additional metadata.

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
