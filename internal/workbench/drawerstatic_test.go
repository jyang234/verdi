package workbench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDrawerRenderer_StaticEvidence is derivation-drawer ac-2's STATIC
// obligation: exactly ONE drawer-body renderer in internal/workbench,
// taking the canonical derivation record (badgeView, its local mirror) as
// its sole data input; no call from the drawer render path back into
// lint, decisionsweep, or evidence recomputation; and assets/boardspec.js
// free of any derivation-data templating — the client only toggles and
// positions the server-rendered hidden drawer element. The same
// deliberately-minimal source-text witness badgesstatic_test.go already
// established for this package.
func TestDrawerRenderer_StaticEvidence(t *testing.T) {
	// Exactly one renderer definition, package-wide, and exactly one call
	// site — writeBadgeButton (badgerender.go), which both the full page
	// and the post-mutation fragment reach through renderBoardRegion.
	goFiles, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	defs, calls := 0, 0
	var callFiles []string
	for _, f := range goFiles {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		defs += strings.Count(string(src), "func writeBadgeDrawer(")
		c := strings.Count(string(src), "writeBadgeDrawer(b, bd)")
		calls += c
		if c > 0 {
			callFiles = append(callFiles, f)
		}
	}
	if defs != 1 {
		t.Errorf("found %d definitions of writeBadgeDrawer, want exactly 1 (one renderer, ac-2)", defs)
	}
	if calls != 1 || len(callFiles) != 1 || callFiles[0] != "badgerender.go" {
		t.Errorf("writeBadgeDrawer is called %d times from %v, want exactly once from badgerender.go (the badge button's sibling emit)", calls, callFiles)
	}

	// The drawer renderer's sole data input is the record: its file
	// imports nothing but the escape and string primitives — no store
	// reads, no lint/decisionsweep/evidence recomputation, no clock
	// (ac-4's static obligation rides the same witness).
	src, err := os.ReadFile("drawerrender.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		`"github.com/jyang234/verdi/internal/lint"`,
		`"github.com/jyang234/verdi/internal/decisionsweep"`,
		`"github.com/jyang234/verdi/internal/evidence"`,
		`"github.com/jyang234/verdi/internal/wallbadge"`,
		`"os"`, `"time"`, "time.Now",
	} {
		if strings.Contains(string(src), forbidden) {
			t.Errorf("drawerrender.go contains %s — the drawer renderer must be a pure function of the record (no recomputation, no I/O, no clock)", forbidden)
		}
	}

	// boardspec.js never templates derivation data: it does not even READ
	// the serialized record (data-badge-record stays the server's opener
	// contract, consumed by tests and agents) — the client's whole drawer
	// role is toggling/positioning the server-rendered hidden sibling.
	js, err := os.ReadFile(filepath.Join("assets", "boardspec.js"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(js), "data-badge-record") {
		t.Error("assets/boardspec.js reads data-badge-record — the client must never template derivation data (dc-1)")
	}
}

// TestDrawerNoClock_StaticEvidence is derivation-drawer ac-4's STATIC
// obligation, workbench half: no wall-clock read and no timestamp
// formatting anywhere on the drawer render path (the renderer and the
// badge markup emit that hosts it). The wallbadge half — the judged-
// findings compute — is witnessed by that package's own
// TestJudgedSweep_StaticEvidence.
func TestDrawerNoClock_StaticEvidence(t *testing.T) {
	for _, f := range []string{"drawerrender.go", "badgerender.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"time.Now", `"time"`, ".Format(", "time.Time"} {
			if strings.Contains(string(src), forbidden) {
				t.Errorf("%s contains %q — no drawer render path may read or format a clock", f, forbidden)
			}
		}
	}
}
