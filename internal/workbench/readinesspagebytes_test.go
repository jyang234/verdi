package workbench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

// readinessPageGoldenEnv re-captures the readiness page goldens when set
// to 1. A re-capture is a byte change of the page, which needs its own
// authority (SI-368 (14): the page keeps its words).
const readinessPageGoldenEnv = "VERDI_READINESS_PAGE_UPDATE"

// TestReadinessPage_BytesUnchangedByTheSharedRenderer pins the readiness
// page's own content, byte for byte, over every renderer fixture: the
// page's <main> region, captured before its body renderer was shared
// with the wall's Readiness tab (SI-368 (1), (14)), and the bar's
// "Open the wall →" control. The shared renderer must leave both
// unchanged. The top bar's other facts describe the serving checkout,
// not the derivation, so they are outside the pin.
func TestReadinessPage_BytesUnchangedByTheSharedRenderer(t *testing.T) {
	for _, tc := range []struct {
		name string
		snap readinesspilot.Snapshot
	}{
		{"mixed", readinessFixture()},
		{"with-role", readinessWithRoleFixture()},
		{"all-proven", readinessAllProvenFixture()},
		{"last-step", readinessLastStepFixture()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := renderReadinessFixture(t, tc.snap)
			start, end := strings.Index(page, "<main"), strings.Index(page, "</main>")
			if start < 0 || end < start {
				t.Fatalf("the page has no <main> region:\n%s", page)
			}
			got := page[start:end+len("</main>")] + "\n" + string(readinessWallLink(tc.snap)) + "\n"
			if !strings.Contains(page, string(readinessWallLink(tc.snap))) {
				t.Fatal("the page's bar does not carry its wall link")
			}
			path := filepath.Join("testdata", "readiness-page-"+tc.name+".golden.html")
			if os.Getenv(readinessPageGoldenEnv) == "1" {
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading the golden: %v", err)
			}
			if got != string(want) {
				at := 0
				for at < len(got) && at < len(want) && got[at] == want[at] {
					at++
				}
				t.Fatalf("the readiness page's bytes changed at byte %d (%s):\n got: %.160s\nwant: %.160s", at, path, got[at:], string(want)[at:])
			}
		})
	}
}
