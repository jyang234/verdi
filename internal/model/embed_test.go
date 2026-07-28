package model

import (
	"bytes"
	"reflect"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestCanonicalYAML_MatchesEmbedded proves CanonicalYAML() returns
// exactly the embedded canonical.yaml bytes (spec/init-wizard ac-2/ac-3,
// ledger L-N5): the wizard's own hand-built model.yaml serialization
// (internal/initwizard) starts from these bytes rather than a second,
// drift-prone copy of the classes:/lifecycle: block.
func TestCanonicalYAML_MatchesEmbedded(t *testing.T) {
	got := CanonicalYAML()
	if !bytes.Equal(got, canonicalYAML) {
		t.Fatalf("CanonicalYAML() does not match the embedded canonicalYAML bytes")
	}
}

// TestCanonicalYAML_ReturnsACopy proves a caller mutating the returned
// slice can never corrupt the shared embedded asset — Canonical()'s own
// doc comment makes the same "never a shared, cached pointer" promise
// for the decoded *Model; CanonicalYAML extends it to the raw bytes.
func TestCanonicalYAML_ReturnsACopy(t *testing.T) {
	got := CanonicalYAML()
	if len(got) == 0 {
		t.Fatal("CanonicalYAML() returned no bytes")
	}
	original := got[0]
	got[0] = original + 1 // mutate the returned copy

	again := CanonicalYAML()
	if again[0] != original {
		t.Fatalf("mutating a prior CanonicalYAML() result changed a later call's bytes: got %q, want the original %q — CanonicalYAML must return a defensive copy", again[0], original)
	}
}

// TestCanonicalYAML_DecodesToCanonical proves the exported bytes are
// exactly what Canonical() itself decodes from — CanonicalYAML is not a
// second, independently-drifting source.
func TestCanonicalYAML_DecodesToCanonical(t *testing.T) {
	decoded, err := DecodeModel(CanonicalYAML())
	if err != nil {
		t.Fatalf("decoding CanonicalYAML(): %v", err)
	}
	if !reflect.DeepEqual(*decoded, canonicalModel) {
		t.Fatalf("CanonicalYAML() decodes to a DIFFERENT Model than canonicalModel")
	}
}

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
