package workbench

// Static evidence for spec/case-file-flags (the same deliberately-minimal
// source-text witness posture badgesstatic_test.go and internal/
// wallbadge's TestLadderStaticCallSites established — coarse, legible
// checks of the exact claims the obligations name, not a call-graph
// prover).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCaseFileLadder_NoLocalRederivationInWorkbench is ac-1's static
// obligation (co-3, the ac-4 trap): the case file's spec-stale and
// pending-supersession values reach this package ONLY through the badge
// compute layer's attachment point (attachBadges → wallbadge.
// ComputeBadges, itself witnessed by internal/wallbadge's
// TestLadderStaticCallSites to call the dex lens's exact entry points).
// No file under internal/workbench may re-derive either flag: no call
// into decisionsweep, no second accepted-deviation counter, no second
// open-MR supersession fold (the marker shapes co-3 names).
func TestCaseFileLadder_NoLocalRederivationInWorkbench(t *testing.T) {
	forbidden := []string{
		"decisionsweep",            // the spec-stale scan's own package: only internal/wallbadge may call it
		"ScanSpecStale",            // the spec-stale entry point
		"PendingSupersession(",     // the supersession fold's entry point
		"FindingAcceptedDeviation", // an accepted-deviation counter's marker (co-3's "second counter")
		".Amended",                 // a supersession bucket fold's marker (co-3's "second open-MR fold")
		".Removed",
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		src := string(data)
		for _, bad := range forbidden {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %q — a ladder re-derivation marker; the case file's values must flow through wallbadge.ComputeBadges alone (co-3)", name, bad)
			}
		}
	}

	// The one legitimate route, present: badges.go calls
	// wallbadge.ComputeBadges (the compute layer's attachment point).
	badges, err := os.ReadFile("badges.go")
	if err != nil {
		t.Fatalf("reading badges.go: %v", err)
	}
	if !strings.Contains(string(badges), "wallbadge.ComputeBadges(") {
		t.Error("badges.go no longer calls wallbadge.ComputeBadges — the case file lost its one compute route")
	}
}

// TestSizeSmell_PureFunctionOfDeclaredInputs is ac-2's static obligation:
// the size-smell compute reads ONLY the spec frontmatter's declared AC
// count (its acCount parameter, counted by the caller from the decoded
// document) and the board layout package's declared geometry constants,
// against the declared reference constant — no file I/O, no stored card
// positions, no client-supplied value, no clock, no configuration knob.
// Witnessed on the compute's own source file: the named constants are
// present; every input-shaped escape hatch is absent.
func TestSizeSmell_PureFunctionOfDeclaredInputs(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "wallbadge", "sizesmell.go"))
	if err != nil {
		t.Fatalf("reading sizesmell.go: %v", err)
	}
	src := string(data)

	for _, want := range []string{
		"boardlayout.ZoneOriginY",       // the AC zone's top offset, by name
		"boardlayout.RowPitch",          // the declared row pitch, by name
		"ReferenceViewportHeight = 900", // dc-1's declared constant, a const not config
	} {
		if !strings.Contains(src, want) {
			t.Errorf("sizesmell.go does not read %s — the estimate must be a pure function of the declared constants (dc-1)", want)
		}
	}
	for _, bad := range []string{
		`"os"`, "os.ReadFile", "os.Getenv", // no file/env input beyond the caller's decoded count
		"boardio", "layout.json", "artifact.Position", // never stored/dragged positions (dc-1)
		`"time"`, `"math/rand"`, // no clock, no randomness (co-1)
		`"net/http"`, "Request", // no client-supplied value can reach the compute (ac-3)
	} {
		if strings.Contains(src, bad) {
			t.Errorf("sizesmell.go contains %q — an input beyond the declared constants and the caller's AC count (dc-1/co-1)", bad)
		}
	}
}

// TestSizeSmell_NothingConsumesTheBadge is dc-2/co-2's static half: the
// size-smell badge is an observation, never a rule — no gate, lint rule,
// or write path anywhere in the binary reads it. Witnessed by walking
// every non-test Go source under internal/ and cmd/ and asserting the
// badge's identifiers appear ONLY in its own compute package
// (internal/wallbadge). internal/workbench renders every badge through
// the generic badgeView shape without naming any source id, so a hit
// anywhere else is a consumer. cmd/e2eharness is excluded: it is the
// Playwright suite's fixture provisioner (test scaffolding named after
// the fixtures it authors), not a gate, lint rule, or write path.
func TestSizeSmell_NothingConsumesTheBadge(t *testing.T) {
	roots := []string{filepath.Join("..", ".."), filepath.Join("..", "..", "..")}
	moduleRoot := ""
	for _, r := range roots {
		if _, err := os.Stat(filepath.Join(r, "go.mod")); err == nil {
			moduleRoot = r
			break
		}
	}
	if moduleRoot == "" {
		t.Fatal("could not locate the module root from the package dir")
	}
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(moduleRoot, top), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				base := filepath.Base(path)
				if base == "wallbadge" || base == "e2eharness" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			src := string(data)
			for _, bad := range []string{"SizeSmell", "observe:size-smell"} {
				if strings.Contains(src, bad) {
					t.Errorf("%s names %q — size-smell is an observation nothing consumes (dc-2, co-2)", path, bad)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", top, err)
		}
	}
}

// TestSizeSmell_NoClientViewportFeedsBadgeState is ac-3's static
// obligation: the badge and its drawer content are produced entirely
// server-side, and no client script measures or injects a viewport
// dimension into badge state. Witnessed two ways: (a) in
// assets/boardspec.js, no source line that reads a viewport dimension
// (window.innerHeight and equivalents — legitimate for menu positioning
// and drag edge-scroll) also touches badge/stamp/drawer vocabulary; and
// (b) in badgerender.go, the data-badge-record attribute — the drawer's
// one content source — is written from the serialized derivation record
// alone (badgeRecordJSON over the compute's badgeView), so no drawer
// field can originate outside the record.
func TestSizeSmell_NoClientViewportFeedsBadgeState(t *testing.T) {
	js, err := os.ReadFile(filepath.Join("assets", "boardspec.js"))
	if err != nil {
		t.Fatalf("reading boardspec.js: %v", err)
	}
	viewportTokens := []string{"innerHeight", "innerWidth", "outerHeight", "outerWidth", "visualViewport", "screen.height", "screen.width"}
	badgeTokens := []string{"badge", "stamp", "smell", "drawer", "derivation"}
	for i, line := range strings.Split(string(js), "\n") {
		lower := strings.ToLower(line)
		hasViewport := false
		for _, v := range viewportTokens {
			if strings.Contains(line, v) {
				hasViewport = true
			}
		}
		if !hasViewport {
			continue
		}
		for _, b := range badgeTokens {
			if strings.Contains(lower, b) {
				t.Errorf("boardspec.js:%d reads a viewport dimension AND touches badge state (%q):\n%s", i+1, b, line)
			}
		}
	}

	render, err := os.ReadFile("badgerender.go")
	if err != nil {
		t.Fatalf("reading badgerender.go: %v", err)
	}
	src := string(render)
	if n := strings.Count(src, `data-badge-record="`); n != 1 {
		t.Errorf("badgerender.go writes data-badge-record %d times, want exactly 1 (the one drawer content source)", n)
	}
	if !strings.Contains(src, `data-badge-record="`+"` + esc(badgeRecordJSON(bd)) + `") {
		t.Error("badgerender.go's data-badge-record is not fed by badgeRecordJSON(bd) — a drawer field could originate outside the derivation record")
	}
}
