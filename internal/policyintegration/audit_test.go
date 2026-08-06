package policyintegration

import (
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/humanartifact"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// TestExportedAPIAudit_FailClosed is the cross-package audit item: every
// exported entry point across the four packages that could otherwise
// yield a value or a clean verdict from unprovenanced input fails
// closed. Each case's deep grammar (WHY the specific input is invalid)
// is already pinned by that package's own unit tests — this table
// proves only that the cross-package SURFACE agrees: a zero-value,
// hand-built, or otherwise non-provenanced input never silently
// produces authority.
func TestExportedAPIAudit_FailClosed(t *testing.T) {
	root := t.TempDir() // an empty, never-adopted root — reused across the projection cases

	tests := []struct {
		name string
		run  func() error
	}{
		{
			// policyartifact's own seal discipline: checkSealed rejects
			// any value whose seal field is empty (seal.go). Pinned
			// deeply by policyartifact's own *_test.go seal-forgery
			// cases; this proves the exported Digest() surface itself.
			name: "policyartifact.Policy{}.Digest() zero value",
			run:  func() error { _, err := (&policyartifact.Policy{}).Digest(); return err },
		},
		{
			name: "policyartifact.Overlay{}.Digest() zero value",
			run:  func() error { _, err := (&policyartifact.Overlay{}).Digest(); return err },
		},
		{
			name: "policyartifact.Exemption{}.Digest() zero value",
			run:  func() error { _, err := (&policyartifact.Exemption{}).Digest(); return err },
		},
		{
			name: "policyartifact.Constitution{}.Digest() zero value",
			run:  func() error { _, err := (&policyartifact.Constitution{}).Digest(); return err },
		},
		{
			// GovernanceCatalog is the one egress a hand-built
			// Constitution could otherwise use to mint a vocabulary a
			// profile could validate against without ever having gone
			// through DecodeConstitution (constitution.go's own SI-21
			// forgery-posture comment).
			name: "policyartifact.Constitution{}.GovernanceCatalog() zero value",
			run:  func() error { _, err := (&policyartifact.Constitution{}).GovernanceCatalog(); return err },
		},
		{
			name: "policyauthority.EffectivePolicy{}.Digest() zero value",
			run:  func() error { _, err := (&policyauthority.EffectivePolicy{}).Digest(); return err },
		},
		{
			// A hand-built Store (the unexported sealed marker never
			// set) must never satisfy Resolve's gate — resolve.go's own
			// "only Load produces a Store that satisfies Resolve's
			// gate" contract, proven here from OUTSIDE the package using
			// only exported fields.
			name: "policyauthority.Resolve(hand-built Store)",
			run: func() error {
				_, err := policyauthority.Resolve(&policyauthority.Store{
					Root:       root,
					Policies:   map[string]*policyartifact.Policy{},
					Overlays:   map[string]*policyartifact.Overlay{},
					Exemptions: map[string]*policyartifact.Exemption{},
					Profiles:   map[string]*policyartifact.StoredProfile{},
				})
				return err
			},
		},
		{
			// humanartifact.Contract{} with an empty Kind names no
			// recognized artifact family — contract.go's own Validate,
			// exercised here as the audit's "safe" case: it must error,
			// never silently accept an unrouted extension surface.
			name: "humanartifact.Contract{}.Validate() zero value",
			run:  func() error { return (humanartifact.Contract{}).Validate() },
		},
		{
			// instructionprojection.Generate/Verify on an empty root
			// both propagate policyauthority.ErrNotAdopted — the SAME
			// fact TestLegacyStore_NothingClaimsAuthority proves with
			// full nothing-written witnesses; this entry only confirms
			// the two functions are members of this fail-closed table.
			name: "instructionprojection.Generate(empty root)",
			run:  func() error { _, err := instructionprojection.Generate(root); return err },
		},
		{
			name: "instructionprojection.Verify(empty root)",
			run:  func() error { _, err := instructionprojection.Verify(root); return err },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil {
				t.Fatalf("%s = nil error, want a fail-closed error", tt.name)
			}
		})
	}
}

// TestExportedAPIAudit_SentinelsWrapped spot-checks errors.Is over the
// two named sentinels policyauthority.Load exports, reached both
// directly and through instructionprojection's own wrapping — proving
// %w propagation held across the package boundary, not merely within
// policyauthority's own tests.
func TestExportedAPIAudit_SentinelsWrapped(t *testing.T) {
	root := t.TempDir()

	_, err := policyauthority.Load(root)
	if !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("policyauthority.Load() error = %v, want errors.Is(_, ErrNotAdopted)", err)
	}

	_, err = instructionprojection.Verify(root)
	if !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("instructionprojection.Verify() error = %v, want errors.Is(_, ErrNotAdopted) through the package boundary", err)
	}

	_, err = instructionprojection.Generate(root)
	if !errors.Is(err, policyauthority.ErrNotAdopted) {
		t.Fatalf("instructionprojection.Generate() error = %v, want errors.Is(_, ErrNotAdopted) through the package boundary", err)
	}
}
