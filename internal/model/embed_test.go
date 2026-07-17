package model

import (
	"reflect"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestCanonicalYAMLMatchesGoLiteral proves the embedded canonical.yaml
// (go:embed, embed.go) decodes to EXACTLY canonicalModel (canonical.go):
// the two must never silently drift apart (Task 5 Step 3's split
// rationale — canonical.go exists so validate.go's checkFrontier does
// not itself depend on this package's own embedded asset).
func TestCanonicalYAMLMatchesGoLiteral(t *testing.T) {
	decoded, err := DecodeModel(canonicalYAML)
	if err != nil {
		t.Fatalf("decoding embedded canonical.yaml: %v", err)
	}
	if !reflect.DeepEqual(*decoded, canonicalModel) {
		t.Fatalf("embedded canonical.yaml decodes to a DIFFERENT Model than canonical.go's canonicalModel literal:\nfrom YAML:  %+v\nGo literal: %+v", *decoded, canonicalModel)
	}
}

// TestCanonicalMatchesCode_States is spec/model-schema ac-2's states
// half: the canonical model's every declared lifecycle must have
// EXACTLY internal/artifact's own exported SpecFeatureStatuses() set as
// its states — through that exported accessor, never reflection on
// artifact's private specFeatureStatuses map, so a status added (or
// removed) in the Go code with no matching change here fails this test,
// and the reverse.
func TestCanonicalMatchesCode_States(t *testing.T) {
	want := artifact.SpecFeatureStatuses()

	canonical := Canonical()
	if len(canonical.Lifecycle) == 0 {
		t.Fatal("Canonical().Lifecycle is empty — nothing to compare")
	}
	for name, lc := range canonical.Lifecycle {
		got := append([]string(nil), lc.States...)
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("lifecycle %q states = %v, want internal/artifact.SpecFeatureStatuses() = %v", name, got, want)
		}
	}
}

// TestCanonical_FreshCopyPerCall proves Canonical() never hands back a
// shared, mutable singleton: two calls return distinct *Model values
// (so one caller mutating its copy can never affect another's), even
// though they are structurally equal.
func TestCanonical_FreshCopyPerCall(t *testing.T) {
	a := Canonical()
	b := Canonical()
	if a == b {
		t.Fatal("Canonical() returned the identical pointer twice — want a fresh Model per call")
	}
	if !reflect.DeepEqual(*a, *b) {
		t.Fatalf("Canonical() returned two structurally different Models:\na: %+v\nb: %+v", *a, *b)
	}
}
