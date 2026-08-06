package policyintegration

import (
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/humanartifact"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// storedProfileFixtureCatalog is a HAND-BUILT governanceprincipal.Catalog
// matching baseStoreFiles' own solo-default profile's role and transition
// references — built directly here, never derived from any decoded
// Constitution. This is deliberate: DecodeStoredProfile takes the catalog
// as an injected dependency — the kernel owns implementation only and the
// governance vocabulary is project-registered, resolved "through an
// injected duplicate-free catalog" (GLG v3 dc-25; SI-18) — so a
// hand-built catalog is not a forgery vector the way a hand-built
// *policyartifact.Constitution or *Store is elsewhere in this table. See
// the audit row below for exactly where provenance IS enforced for a
// stored profile.
var storedProfileFixtureCatalog = governanceprincipal.Catalog{
	Roles:       []string{"author", "policy-owner"},
	Transitions: []string{"accept"},
}

// storedProfileFixtureData is the same solo-default profile artifact
// baseStoreFiles writes, standing alone so the audit row can decode it
// against storedProfileFixtureCatalog directly.
const storedProfileFixtureData = `---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: github-org, kind: forge}
role_mappings:
  - {role: author, trust_source: github-org, subjects: [alice]}
  - {role: policy-owner, trust_source: github-org, subjects: [alice]}
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
The solo operator profile.
`

// TestExportedAPIAudit_FailClosed is the cross-package audit item: every
// exported entry point across the four packages that could otherwise
// yield a value or a clean verdict from unprovenanced input fails
// closed — WITH ONE NAMED, DELIBERATE EXCEPTION (the DecodeStoredProfile
// row below), whose own case proves where provenance enforcement
// actually lives instead. Each case's deep grammar (WHY the specific
// input is invalid, or in the one exception's case, why it is validly
// accepted) is already pinned by that package's own unit tests — this
// table proves only that the cross-package SURFACE agrees: a zero-value
// or hand-built input never silently produces authority UNLESS the
// package's own design deliberately injects that exact input as a
// dependency.
func TestExportedAPIAudit_FailClosed(t *testing.T) {
	root := t.TempDir() // an empty, never-adopted root — reused across the projection cases

	tests := []struct {
		name string
		run  func() error
		// wantErr is true for every fail-closed case (the default this
		// table exists to prove) and false for the one row that is
		// SUPPOSED to succeed on a hand-built input by design.
		wantErr bool
	}{
		{
			// policyartifact's own seal discipline: checkSealed rejects
			// any value whose seal field is empty (seal.go). Pinned
			// deeply by policyartifact's own *_test.go seal-forgery
			// cases; this proves the exported Digest() surface itself.
			name:    "policyartifact.Policy{}.Digest() zero value",
			run:     func() error { _, err := (&policyartifact.Policy{}).Digest(); return err },
			wantErr: true,
		},
		{
			name:    "policyartifact.Overlay{}.Digest() zero value",
			run:     func() error { _, err := (&policyartifact.Overlay{}).Digest(); return err },
			wantErr: true,
		},
		{
			name:    "policyartifact.Exemption{}.Digest() zero value",
			run:     func() error { _, err := (&policyartifact.Exemption{}).Digest(); return err },
			wantErr: true,
		},
		{
			name:    "policyartifact.Constitution{}.Digest() zero value",
			run:     func() error { _, err := (&policyartifact.Constitution{}).Digest(); return err },
			wantErr: true,
		},
		{
			// GovernanceCatalog is the one egress a hand-built
			// Constitution could otherwise use to mint a vocabulary a
			// profile could validate against without ever having gone
			// through DecodeConstitution (constitution.go's own SI-21
			// forgery-posture comment).
			name:    "policyartifact.Constitution{}.GovernanceCatalog() zero value",
			run:     func() error { _, err := (&policyartifact.Constitution{}).GovernanceCatalog(); return err },
			wantErr: true,
		},
		{
			name:    "policyauthority.EffectivePolicy{}.Digest() zero value",
			run:     func() error { _, err := (&policyauthority.EffectivePolicy{}).Digest(); return err },
			wantErr: true,
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
			wantErr: true,
		},
		{
			// humanartifact.Contract{} with an empty Kind names no
			// recognized artifact family — contract.go's own Validate,
			// exercised here as the audit's "safe" case: it must error,
			// never silently accept an unrouted extension surface.
			name:    "humanartifact.Contract{}.Validate() zero value",
			run:     func() error { return (humanartifact.Contract{}).Validate() },
			wantErr: true,
		},
		{
			// instructionprojection.Generate/Verify on an empty root
			// both propagate policyauthority.ErrNotAdopted — the SAME
			// fact TestLegacyStore_NothingClaimsAuthority proves with
			// full nothing-written witnesses; this entry only confirms
			// the two functions are members of this fail-closed table.
			name:    "instructionprojection.Generate(empty root)",
			run:     func() error { _, err := instructionprojection.Generate(root); return err },
			wantErr: true,
		},
		{
			name:    "instructionprojection.Verify(empty root)",
			run:     func() error { _, err := instructionprojection.Verify(root); return err },
			wantErr: true,
		},
		{
			// THE ONE DELIBERATE EXCEPTION: policyartifact.DecodeStoredProfile
			// takes its governanceprincipal.Catalog as an injected
			// dependency — the kernel "owns implementation only" (GLG v3
			// dc-25) and profiles resolve "through an injected
			// duplicate-free catalog" (SI-18) — so a hand-built catalog
			// is accepted BY DESIGN here. Provenance enforcement
			// for a stored profile does not live at this decoder at all:
			// it lives at policyauthority.Load's own call site, which
			// obtains the catalog exclusively from
			// Constitution.GovernanceCatalog() — the sealed egress the
			// "zero value" row above already proves fails closed on any
			// non-decoded Constitution. A hand-built Catalog passed
			// directly to DecodeStoredProfile is therefore not a forgery
			// vector: nothing reachable from a real Load/Resolve chain
			// can hand it anything but a catalog that traces back to a
			// decoded, sealed Constitution.
			name: "policyartifact.DecodeStoredProfile(hand-built Catalog) — intentional DI, not a forgery vector",
			run: func() error {
				_, err := policyartifact.DecodeStoredProfile([]byte(storedProfileFixtureData), storedProfileFixtureCatalog)
				return err
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if tt.wantErr && err == nil {
				t.Fatalf("%s = nil error, want a fail-closed error", tt.name)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("%s = %v, want the intentional-DI success posture (nil error)", tt.name, err)
			}
		})
	}
}

// TestExportedAPIAudit_SentinelsWrapped spot-checks errors.Is over BOTH
// named sentinels policyauthority.Load exports (ErrNotAdopted and
// ErrIncompleteAdoption), each reached both directly and through
// instructionprojection's own wrapping — proving %w propagation held
// across the package boundary for every sentinel this audit claims to
// cover, not merely within policyauthority's own tests.
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

	incompleteRoot := t.TempDir()
	writeTree(t, incompleteRoot, map[string]string{
		".verdi/policy/policies/go-toolchain.md": `---
schema: verdi.policy/v1
id: policy/go-toolchain
kind: policy
title: "Go toolchain policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads: {}
---
A policy committed before the constitution manifest itself.
`,
	})

	_, err = policyauthority.Load(incompleteRoot)
	if !errors.Is(err, policyauthority.ErrIncompleteAdoption) {
		t.Fatalf("policyauthority.Load() error = %v, want errors.Is(_, ErrIncompleteAdoption)", err)
	}

	_, err = instructionprojection.Verify(incompleteRoot)
	if !errors.Is(err, policyauthority.ErrIncompleteAdoption) {
		t.Fatalf("instructionprojection.Verify() error = %v, want errors.Is(_, ErrIncompleteAdoption) through the package boundary", err)
	}

	_, err = instructionprojection.Generate(incompleteRoot)
	if !errors.Is(err, policyauthority.ErrIncompleteAdoption) {
		t.Fatalf("instructionprojection.Generate() error = %v, want errors.Is(_, ErrIncompleteAdoption) through the package boundary", err)
	}
}
