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

	// No client asset templates derivation data — boardspec.js nor any
	// asset added since (spec/wall-canvas-v2 co-1 ships new behaviour in
	// new assets): none even READS the serialized record
	// (data-badge-record stays the server's opener contract, consumed by
	// tests and agents) — the client's whole drawer role is
	// toggling/positioning the server-rendered hidden sibling.
	assets, err := workbenchAssets(os.DirFS("assets"))
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) == 0 {
		t.Fatal("no client asset was enumerated: the witness would be vacuous")
	}
	for _, name := range badgeRecordReaders(assets) {
		t.Errorf("assets/%s reads data-badge-record — the client must never template derivation data (dc-1)", name)
	}
}

// badgeRecordReaders names every asset handed to it that reads the
// serialized derivation record (data-badge-record), in input order.
func badgeRecordReaders(assets []workbenchAsset) []string {
	var out []string
	for _, a := range assets {
		if strings.Contains(string(a.data), "data-badge-record") {
			out = append(out, a.name)
		}
	}
	return out
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

// TestBadgeRecordReaders: the client-templating witness names every asset
// that reads the serialized derivation record — an asset added after this
// guard was written included — and none that does not.
func TestBadgeRecordReaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		assets []workbenchAsset
		want   []string
	}{
		{"clean", []workbenchAsset{{name: "boardspec.js", data: []byte("el.hidden = !el.hidden;\n")}}, nil},
		{"a new asset templating the record", []workbenchAsset{
			{name: "boardspec.js", data: []byte("el.hidden = !el.hidden;\n")},
			{name: "walltoolbar.js", data: []byte("JSON.parse(btn.getAttribute('data-badge-record'))\n")},
		}, []string{"walltoolbar.js"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := badgeRecordReaders(tc.assets)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("badgeRecordReaders = %v, want %v", got, tc.want)
			}
		})
	}
}
