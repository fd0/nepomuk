package database

import "testing"

func TestParseFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string

		date, id, title string
		err             string
	}{
		{
			filename: "2003-01-02 foobar title string with spaces abf9c1b9.pdf",
			date:     "2003-01-02",
			title:    "foobar title string with spaces",
			id:       "abf9c1b9",
		},
		{
			filename: "2003-01-02 abf9c1b9.pdf",
			date:     "2003-01-02",
			title:    "",
			id:       "abf9c1b9",
		},
		{
			filename: "2003-01 abf9c1b9.pdf",
			err:      "invalid file name",
		},
		{
			filename: "2003-01-01 .pdf",
			err:      "invalid file name",
		},
		{
			filename: "2003-01-01.pdf",
			err:      "invalid file name",
		},
	}

	for _, test := range tests {
		// create local copy of test
		test := test

		t.Run("", func(t *testing.T) {
			t.Parallel()

			date, title, id, err := ParseFilename(test.filename)

			// run checks if an error is expected
			if test.err != "" {
				if err == nil {
					t.Fatalf("expected error %q for filename %v not found, got nil", test.err, test.filename)
				}

				if err.Error() != test.err {
					t.Fatalf("wrong error for filename %v returned: want %q, got %q", test.filename, test.err, err.Error())
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if date != test.date {
				t.Errorf("wrong date, want %q, got %q", test.date, date)
			}

			if title != test.title {
				t.Errorf("wrong title, want %q, got %q", test.title, title)
			}

			if id != test.id {
				t.Errorf("wrong id, want %q, got %q", test.id, id)
			}
		})
	}
}
