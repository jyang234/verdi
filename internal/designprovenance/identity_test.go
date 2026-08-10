package designprovenance

import (
	"testing"

	"github.com/jyang234/verdi/internal/store"
)

func TestIdentityActiveArchiveAndExclusionReason(t *testing.T) {
	tests := []struct {
		zone string
		want string
	}{
		{store.ZoneActive, ".verdi/specs/active/widget/design-provenance.jsonl"},
		{store.ZoneArchive, ".verdi/specs/archive/widget/design-provenance.jsonl"},
	}
	for _, tt := range tests {
		t.Run(tt.zone, func(t *testing.T) {
			got, err := ResolveIdentity("spec/widget", tt.zone)
			if err != nil {
				t.Fatalf("ResolveIdentity: %v", err)
			}
			if got.Spec != "spec/widget" || got.RelPath != tt.want || got.ExclusionReason != ExclusionReason {
				t.Fatalf("identity = %+v", got)
			}
		})
	}
	if _, err := ResolveIdentity("adr/widget", store.ZoneActive); err == nil {
		t.Fatal("ResolveIdentity accepted a non-spec ref")
	}
	if _, err := ResolveIdentity("spec/widget", "mutable"); err == nil {
		t.Fatal("ResolveIdentity accepted an unknown zone")
	}
}

func TestFixedExclusionReason(t *testing.T) {
	if ExclusionReason != "design-provenance-sidecar" {
		t.Fatalf("ExclusionReason = %q", ExclusionReason)
	}
	if ContextUnavailableReason != "unavailable-before-context-compiler" {
		t.Fatalf("ContextUnavailableReason = %q", ContextUnavailableReason)
	}
}
