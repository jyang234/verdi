package residue

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
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

func TestExcludeSuperseded(t *testing.T) {
	in := []activeSpec{
		{Name: "a", FM: mustDecodeSpecFM(t, featureSpecMD("a", "accepted-pending-build"))},
		{Name: "b", FM: mustDecodeSpecFM(t, featureSpecMD("b", "superseded"))},
	}
	out := excludeSuperseded(in)
	if len(out) != 1 || out[0].Name != "a" {
		t.Fatalf("excludeSuperseded = %+v, want only %q kept", out, "a")
	}
}

func TestActiveStatusByName(t *testing.T) {
	specs := []activeSpec{
		{Name: "a", FM: mustDecodeSpecFM(t, featureSpecMD("a", "accepted-pending-build"))},
		{Name: "b", FM: mustDecodeSpecFM(t, featureSpecMD("b", "draft"))},
	}
	got := activeStatusByName(specs)
	want := map[string]string{"a": "accepted-pending-build", "b": "draft"}
	if len(got) != len(want) || got["a"] != want["a"] || got["b"] != want["b"] {
		t.Fatalf("activeStatusByName = %+v, want %+v", got, want)
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
