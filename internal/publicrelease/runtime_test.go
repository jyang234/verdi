package publicrelease

import (
	"encoding/json"
	"github.com/jyang234/verdi/internal/artifact"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeRequiresCompleteActualCutAndArmOutcomes(t *testing.T) {
	b, err := os.ReadFile("testdata/cuts.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []cutRow
	if err = artifact.DecodeStrictJSON(b, &rows); err != nil {
		t.Fatal(err)
	}
	if err = validateCuts(rows); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func([]cutRow) []cutRow
	}{
		{"omitted cut", func(r []cutRow) []cutRow { return r[1:] }}, {"duplicate", func(r []cutRow) []cutRow { r[1] = r[0]; return r }}, {"changed authority", func(r []cutRow) []cutRow { r[0].Classifier = "replay"; return r }}, {"translation", func(r []cutRow) []cutRow { r[5].Launches.Decode = 1; return r }}, {"missing cut process", func(r []cutRow) []cutRow { r[0].KillExit = 0; return r }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copyRows := append([]cutRow{}, rows...)
			if err := validateCuts(tc.change(copyRows)); err == nil {
				t.Fatal("accepted missing or changed cut")
			}
		})
	}
	arms, err := readArms("testdata/arm-events.jsonl")
	if err != nil || len(arms) != 25 {
		t.Fatalf("arms %d %v", len(arms), err)
	}
	if _, err = readArms("testdata/missing"); err == nil {
		t.Fatal("missing captures accepted")
	}
	if err = runtimeComplete(map[string]string{"claimed-pass": "yes"}); err == nil {
		t.Fatal("supplied pass accepted")
	}
}

func TestCopiedOutcomesRequireExactReplayAndZeroTranslation(t *testing.T) {
	fixture := "testdata/copy-control/candidate-controls"
	got, err := readCopy(fixture)
	if err != nil || got.Pair != "candidate" || !got.ReplayBytesEqual || got.TraceLaunches.Sealed != 1 {
		t.Fatalf("copy %+v %v", got, err)
	}
	for _, tc := range []struct {
		name, path string
		change     func(*rawOutcome)
	}{
		{"different handoff", "opposite-shipped-replay.json", func(r *rawOutcome) { r.Stdout = "changed" }},
		{"rewritten records", "opposite-shipped-replay.json", func(r *rawOutcome) { r.Records = []byte(`{"different":true}`) }},
		{"dirty git succeeds", "opposite-shipped-dirty-git.json", func(r *rawOutcome) { r.Exit = 0 }},
		{"translation", "nonblocking/outcome.json", func(r *rawOutcome) { r.Trace[0].Args = []string{"context", "owner", "decode"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "candidate-controls")
			if err := os.CopyFS(dir, os.DirFS(fixture)); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, tc.path)
			r, err := outcome(path)
			if err != nil {
				t.Fatal(err)
			}
			tc.change(&r)
			b, err := json.MarshalIndent(r, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = readCopy(dir); err == nil {
				t.Fatal("accepted changed copied result")
			}
		})
	}
}
