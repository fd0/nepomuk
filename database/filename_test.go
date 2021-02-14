package database

import "testing"

func TestParseFilename(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string

		date, title string
	}{
		{
			filename: "2003-01-02 foobar title string with spaces.pdf",
			date:     "02.01.2003",
			title:    "foobar title string with spaces",
		},
		{
			filename: "2003-01-02 .pdf",
			date:     "02.01.2003",
			title:    "",
		},
		{
			filename: "2003-01-02 foo bar.pdf",
			date:     "02.01.2003",
			title:    "foo bar",
		},
		{
			filename: "2003-01-02.pdf",
			date:     "02.01.2003",
			title:    "",
		},
		{
			filename: "2003-01 abf9c1b9.pdf",
			title:    "2003-01 abf9c1b9",
		},
	}

	for _, test := range tests {
		// create local copy of test
		test := test

		t.Run("", func(t *testing.T) {
			t.Parallel()

			date, title, err := ParseFilename(test.filename)
			if err != nil {
				t.Fatal(err)
			}

			if date != test.date {
				t.Errorf("wrong date, want %q, got %q", test.date, date)
			}

			if title != test.title {
				t.Errorf("wrong title, want %q, got %q", test.title, title)
			}
		})
	}
}
