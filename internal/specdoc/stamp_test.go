// internal/specdoc/stamp_test.go
package specdoc

import (
	"strings"
	"testing"
)

func TestEngineDigestIsStableAndPrefixed(t *testing.T) {
	a := EngineDigest()
	b := EngineDigest()
	if a != b {
		t.Fatalf("EngineDigest not stable: %q vs %q", a, b)
	}
	if !strings.HasPrefix(a, "sha256:") || len(a) != len("sha256:")+64 {
		t.Fatalf("EngineDigest = %q, want sha256:<64 hex>", a)
	}
}

func TestEngineDigestCoversSectionOrder(t *testing.T) {
	// The descriptor must include every kind's section order so that a
	// reorder changes the digest. Prove by digesting a descriptor with one
	// kind's order reversed and comparing.
	got := engineDescriptorDigest(engineDescriptor{ID: engineID, Version: engineVersion, Sections: map[Kind][]SectionID{KindSpec: {SectionOutcome, SectionProblem}}})
	if got == EngineDigest() {
		t.Fatalf("a different section order produced the same digest")
	}
}
