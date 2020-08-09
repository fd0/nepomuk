Process PDF files uploaded by a scanner.

For testing, the script `upload.lftp` can be used to upload two PDF files with odd and even pages. The even pages are in reverse order. Sample files can be found in `testdata/`. The script is run like this:

    lftp -f upload.lftp

In order for this to work, the file names must start with the strings `duplex-odd` and `duplex-even`, and the upload must happen in that order (first odd then even).

After uploading and de-duplixing, the program `ocrmypdf` is run on the files.

The service can be run with systemd socket activation, sample unit files can be found in the `doc/` subdir.
