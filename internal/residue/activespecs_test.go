package residue

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/specstate"
)

func TestWalkActiveSpecs_Happy(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                      "data/\n",
			".verdi/specs/active/widget/spec.md":     storySpecMD("widget", "accepted-pending-build", "feature-x"),
			".verdi/specs/active/gadget/spec.md":     featureSpecMD("gadget", "draft"),
			".verdi/specs/archive/old-thing/spec.md": storySpecMD("old-thing", "closed", "feature-x"),
		},
		Message: "seed active + archive specs",
	}})

	specs, err := walkActiveSpecs(repo.Dir)
	if err != nil {
		t.Fatalf("walkActiveSpecs: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("walkActiveSpecs = %+v, want exactly 2 (archive-zone spec excluded)", specs)
	}
	if specs[0].Name != "gadget" || specs[1].Name != "widget" {
		t.Fatalf("walkActiveSpecs names = [%s, %s], want sorted [gadget, widget]", specs[0].Name, specs[1].Name)
	}
	if specs[1].FM.Status != "accepted-pending-build" {
		t.Fatalf("widget status = %q, want accepted-pending-build", specs[1].FM.Status)
	}
}

func TestWalkActiveSpecs_NoActiveDir_NotAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatal(err)
	}
	specs, err := walkActiveSpecs(root)
	if err != nil {
		t.Fatalf("walkActiveSpecs(no active dir): unexpected error: %v", err)
	}
	if specs != nil {
		t.Fatalf("walkActiveSpecs(no active dir) = %+v, want nil", specs)
	}
}

// TestWalkActiveSpecs_Negative_ToleratesMalformedSpec proves a malformed
// spec.md elsewhere in the corpus is SKIPPED, never a hard failure — this
// is an audit pass over the whole store, and `verdi lint` (not this
// package) is the dedicated tool for surfacing a decode failure itself.
func TestWalkActiveSpecs_Negative_ToleratesMalformedSpec(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                  "data/\n",
			".verdi/specs/active/broken/spec.md": "not even frontmatter\n",
			".verdi/specs/active/widget/spec.md": storySpecMD("widget", "accepted-pending-build", "feature-x"),
		},
		Message: "seed one malformed spec alongside one valid spec",
	}})

	specs, err := walkActiveSpecs(repo.Dir)
	if err != nil {
		t.Fatalf("walkActiveSpecs: unexpected error over a malformed sibling spec: %v", err)
	}
	if len(specs) != 1 || specs[0].Name != "widget" {
		t.Fatalf("walkActiveSpecs = %+v, want exactly the one valid spec (widget)", specs)
	}
}

// TestExcludeSuperseded is a pure-fold unit test (dc-2's obligation
// per activespecs.go's own doc comment): excludeSuperseded takes no ctx and
// touches no Git — every case hand-builds the effective map an upstream
// caller (Scan, via effectiveStates) would have produced.
func TestExcludeSuperseded(t *testing.T) {
	in := []activeSpec{
		{Name: "a", FM: mustDecodeSpecFM(t, featureSpecMD("a", "accepted-pending-build"))},
		{Name: "b", FM: mustDecodeSpecFM(t, featureSpecMD("b", "superseded"))},
	}
	effective := map[string]specstate.State{
		"a": specstate.AcceptedPendingBuild,
		"b": specstate.Superseded,
	}
	out := excludeSuperseded(in, effective)
	if len(out) != 1 || out[0].Name != "a" {
		t.Fatalf("excludeSuperseded = %+v, want only %q kept", out, "a")
	}
}

// TestExcludeSuperseded_Negative_MissingFromEffective proves a name absent
// from the effective map (should never happen in production — Scan always
// builds effective from the exact same specs slice — but a defensive unit
// case) fails closed: the zero specstate.State ("") is not
// specstate.Superseded, so the spec is KEPT, never silently dropped.
func TestExcludeSuperseded_Negative_MissingFromEffective(t *testing.T) {
	in := []activeSpec{{Name: "a", FM: mustDecodeSpecFM(t, featureSpecMD("a", "accepted-pending-build"))}}
	out := excludeSuperseded(in, map[string]specstate.State{})
	if len(out) != 1 || out[0].Name != "a" {
		t.Fatalf("excludeSuperseded(no effective entry) = %+v, want %q kept (fail closed: never assume superseded)", out, "a")
	}
}

// TestSupersededNames mirrors TestExcludeSuperseded for the AC-2 exclusion
// lookup: also a pure fold over an effective map.
func TestSupersededNames(t *testing.T) {
	in := []activeSpec{
		{Name: "a", FM: mustDecodeSpecFM(t, featureSpecMD("a", "accepted-pending-build"))},
		{Name: "b", FM: mustDecodeSpecFM(t, featureSpecMD("b", "superseded"))},
	}
	effective := map[string]specstate.State{
		"a": specstate.AcceptedPendingBuild,
		"b": specstate.Superseded,
	}
	got := supersededNames(in, effective)
	if len(got) != 1 || !got["b"] {
		t.Fatalf("supersededNames = %+v, want only %q", got, "b")
	}
}

// TestEffectiveStates_Happy is effectiveStates' own fixturegit-backed
// integration proof (the map PRODUCER, Task 6a's own port to
// internal/specstate): an exact, already-committed, STATUSLESS active-zone
// spec resolves to specstate.AcceptedPendingBuild — never silently excluded
// the way a raw `FM.Status != "accepted-pending-build"` comparison would
// exclude an empty status. Proven across more than one spec in a single
// repo/call, so a reviewer can see effectiveStates hands every candidate to
// ONE ResolveMany call rather than looping Resolve per spec.
func TestEffectiveStates_Happy(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/.gitignore":                    "data/\n",
			".verdi/specs/active/scaffold/spec.md": featureSpecMD("scaffold", ""), // statusless
			".verdi/specs/active/widget/spec.md":   storySpecMD("widget", "accepted-pending-build", "scaffold"),
		},
		Message: "seed a statusless feature and an explicitly accepted story",
	}})
	setDefaultBranchSymref(t, repo.Dir, "main")

	specs, err := walkActiveSpecs(repo.Dir)
	if err != nil {
		t.Fatalf("walkActiveSpecs: %v", err)
	}

	got, err := effectiveStates(context.Background(), repo.Dir, specs)
	if err != nil {
		t.Fatalf("effectiveStates: %v", err)
	}
	if got["scaffold"] != specstate.AcceptedPendingBuild {
		t.Errorf("effectiveStates[scaffold] (statusless) = %q, want %q", got["scaffold"], specstate.AcceptedPendingBuild)
	}
	if got["widget"] != specstate.AcceptedPendingBuild {
		t.Errorf("effectiveStates[widget] = %q, want %q", got["widget"], specstate.AcceptedPendingBuild)
	}
}

// TestEffectiveStates_Empty proves the zero-spec case short-circuits to an
// empty, non-nil map without calling specstate (and therefore without
// needing a resolvable default branch) at all.
func TestEffectiveStates_Empty(t *testing.T) {
	got, err := effectiveStates(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("effectiveStates(no specs): unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("effectiveStates(no specs) = %+v, want empty", got)
	}
}

func TestActiveClassByName(t *testing.T) {
	specs := []activeSpec{
		{Name: "a", FM: mustDecodeSpecFM(t, featureSpecMD("a", "accepted-pending-build"))},
		{Name: "b", FM: mustDecodeSpecFM(t, storySpecMD("b", "accepted-pending-build", "a"))},
	}
	got := activeClassByName(specs)
	want := map[string]string{"a": "feature", "b": "story"}
	if len(got) != len(want) || got["a"] != want["a"] || got["b"] != want["b"] {
		t.Fatalf("activeClassByName = %+v, want %+v", got, want)
	}
}
