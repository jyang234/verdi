package publicrelease

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestRaceEventsPreserveOnlyExactApprovedVerdiSkips(t *testing.T) {
	data, err := os.ReadFile("testdata/historical-verdi-skips.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []historicalSkip
	if err = decodeDocument(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatal("missing approved historical cases")
	}
	for _, row := range rows {
		t.Run(row.Test, func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				change func(*historicalSkip)
				bad    bool
			}{
				{"exact historical reason", func(*historicalSkip) {}, false},
				{"wrong package", func(s *historicalSkip) { s.Package += "/other" }, true},
				{"wrong name", func(s *historicalSkip) { s.Test += "Other" }, true},
				{"unknown test", func(s *historicalSkip) { s.Test = "TestUnknown" }, true},
				{"wrong reason", func(s *historicalSkip) { s.Reason = strings.Replace(s.Reason, ": ", ": different ", 1) }, true},
				{"reason suffix", func(s *historicalSkip) { s.Reason += "another explanation\n" }, true},
				{"empty reason", func(s *historicalSkip) { s.Reason = "" }, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					s := row
					tc.change(&s)
					stream := historicalSkipStream(t, s)
					skips, err := raceEvents(bytes.NewReader(stream), false)
					if (err != nil) != tc.bad {
						t.Fatalf("skip accounting %+v: %v", skips, err)
					}
					if !tc.bad && (len(skips) != 1 || skips[0] != s) {
						t.Fatal("historical disclosure was lost or rewritten")
					}
				})
			}
			stream := historicalSkipStream(t, row)
			if _, err := raceEvents(bytes.NewReader(stream), true); err == nil {
				t.Fatal("Verdi skip admitted as ATC skip")
			}
			// Historical full-suite allowances cannot turn a required producer test's
			// skip into a pass. The same actual stream must fail the strict reader.
			if _, err := readTestEvents(bytes.NewReader(stream), 0, row.Package, []string{row.Test}); err == nil {
				t.Fatal("required test skip accepted")
			}
		})
	}
}

func historicalSkipStream(t *testing.T, row historicalSkip) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	for _, e := range []testEvent{
		{Action: "start", Package: row.Package},
		{Action: "run", Package: row.Package, Test: row.Test},
		{Action: "output", Package: row.Package, Test: row.Test, Output: row.Reason},
		{Action: "skip", Package: row.Package, Test: row.Test},
		{Action: "pass", Package: row.Package},
	} {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
	return b.Bytes()
}
