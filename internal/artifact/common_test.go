package artifact

import (
	"strings"
	"testing"
)

func TestLinkType_Valid(t *testing.T) {
	for _, lt := range []LinkType{
		LinkImplements, LinkResolves, LinkSupersedes, LinkExempts, LinkVerifies, LinkDerivedFrom,
		LinkAnnotates, LinkDependsOn, LinkStory, LinkImpacts, LinkChallenges,
	} {
		if !lt.Valid() {
			t.Fatalf("LinkType(%q).Valid() = false, want true", lt)
		}
	}
	if LinkType("bogus").Valid() {
		t.Fatal(`LinkType("bogus").Valid() = true, want false`)
	}
}

func TestLink_Validate_Happy(t *testing.T) {
	cases := []Link{
		{Type: LinkImplements, Ref: "spec/foo"},
		{Type: LinkSupersedes, Ref: "adr/0001-old"},
		{Type: LinkVerifies, Ref: "spec/foo"},
		{Type: LinkDerivedFrom, Ref: "spec/foo"},
		{Type: LinkAnnotates, Ref: "spec/foo"},
		{Type: LinkDependsOn, Ref: "spec/foo"},
		{Type: LinkImpacts, Ref: "spec/foo"},
		{Type: LinkChallenges, Ref: "adr/0001-old"},
		{Type: LinkStory, Ref: "jira:LOAN-1482"},
		{Type: LinkImpacts, Ref: "svc/loansvc/boundary-contract"},
		{Type: LinkImpacts, Ref: "svc/loansvc/obligations/audit-before-publish"},
		{Type: LinkExempts, Ref: "adr/0012-outbox-loansvc-events"},
		// closed-vocabulary edges targeting an object fragment (02 §Link taxonomy).
		{Type: LinkImplements, Ref: "spec/loan-update#ac-1"},
		{Type: LinkResolves, Ref: "spec/loan-update#oq-1"},
		{Type: LinkExempts, Ref: "spec/loan-update#dc-1"},
		{Type: LinkSupersedes, Ref: "spec/loan-update#ac-1"},
		{Type: LinkDependsOn, Ref: "spec/loan-update#ac-1"},
	}
	for _, l := range cases {
		t.Run(string(l.Type)+"/"+l.Ref, func(t *testing.T) {
			if err := l.Validate(); err != nil {
				t.Fatalf("Validate(%+v): %v", l, err)
			}
		})
	}
}

func TestLink_Validate_Negative(t *testing.T) {
	cases := []Link{
		{Type: "bogus", Ref: "spec/foo"},
		{Type: LinkImplements, Ref: ""},
		{Type: LinkImplements, Ref: "not-a-ref"},
		{Type: LinkStory, Ref: "spec/foo"},  // story must be scheme:key, not kind/name
		{Type: LinkStory, Ref: "LOAN-1482"}, // missing scheme
		// fragment-targeting edges outside the closed five-value vocabulary.
		{Type: LinkVerifies, Ref: "spec/loan-update#ac-1"},
		{Type: LinkAnnotates, Ref: "spec/loan-update#ac-1"},
		{Type: LinkImpacts, Ref: "spec/loan-update#ac-1"},
		{Type: LinkChallenges, Ref: "spec/loan-update#ac-1"},
		{Type: LinkDerivedFrom, Ref: "spec/loan-update#ac-1"},
	}
	for _, l := range cases {
		t.Run(string(l.Type)+"/"+l.Ref, func(t *testing.T) {
			if err := l.Validate(); err == nil {
				t.Fatalf("Validate(%+v): want error, got nil", l)
			}
		})
	}
}

// TestValidateLinkForKind_ObligationWidensVerifiesToFragment proves the one
// documented exception ValidateLinkForKind carves out (common.go's doc
// comment, spec/obligation-artifact DC-1/DC-2): a KindObligation owner's
// `verifies` link may target an object fragment, where Link.Validate()
// itself (and every other owner kind) rejects it as outside the closed
// five-value spec-object edge vocabulary.
func TestValidateLinkForKind_ObligationWidensVerifiesToFragment(t *testing.T) {
	l := Link{Type: LinkVerifies, Ref: "spec/loan-update#ac-1"}

	if err := ValidateLinkForKind(l, KindObligation); err != nil {
		t.Fatalf("ValidateLinkForKind(%+v, KindObligation): %v, want nil", l, err)
	}

	// The exact same link, for every OTHER owner kind, keeps
	// Link.Validate()'s existing rejection — the widening is scoped to
	// KindObligation alone, never bled into any other kind.
	for _, owner := range []Kind{KindSpec, KindAttestation, KindWaiver, KindADR, KindDiagram, KindConflict, KindReaffirmation} {
		if err := ValidateLinkForKind(l, owner); err == nil {
			t.Errorf("ValidateLinkForKind(%+v, %s): want error (unchanged Link.Validate() behavior), got nil", l, owner)
		}
	}
}

// TestValidateLinkForKind_ObligationStillRejectsBadRef proves the widening
// is narrow — non-empty, parseable ref only — not a blanket bypass: an
// obligation-owned verifies link with an empty or malformed ref still fails.
func TestValidateLinkForKind_ObligationStillRejectsBadRef(t *testing.T) {
	cases := []Link{
		{Type: LinkVerifies, Ref: ""},
		{Type: LinkVerifies, Ref: "not-a-ref"},
	}
	for _, l := range cases {
		t.Run(l.Ref, func(t *testing.T) {
			if err := ValidateLinkForKind(l, KindObligation); err == nil {
				t.Fatalf("ValidateLinkForKind(%+v, KindObligation): want error, got nil", l)
			}
		})
	}
}

// TestValidateLinkForKind_NonVerifiesUnchangedForObligation proves the
// widening is scoped to the verifies link TYPE too: an obligation-owned
// link of any other type still gets Link.Validate()'s ordinary behavior
// (including the closed edge vocabulary for a fragment target).
func TestValidateLinkForKind_NonVerifiesUnchangedForObligation(t *testing.T) {
	l := Link{Type: LinkAnnotates, Ref: "spec/loan-update#ac-1"}
	if err := ValidateLinkForKind(l, KindObligation); err == nil {
		t.Fatalf("ValidateLinkForKind(%+v, KindObligation): want error (non-verifies type, unchanged), got nil", l)
	}
}

func TestFrozen_Validate_Happy(t *testing.T) {
	f := Frozen{At: "2026-05-14", Commit: "3e91ab2"}
	if err := f.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestFrozen_Validate_Negative(t *testing.T) {
	cases := []Frozen{
		{At: "May 14 2026", Commit: "3e91ab2"},
		{At: "2026-05-14", Commit: "not-hex"},
		{At: "", Commit: "3e91ab2"},
		{At: "2026-05-14", Commit: ""},
	}
	for _, f := range cases {
		if err := f.Validate(); err == nil {
			t.Fatalf("Validate(%+v): want error, got nil", f)
		}
	}
}

func TestProvenance_Validate_Happy(t *testing.T) {
	cases := []Provenance{
		{Generator: "verdi-close", Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "sha256:" + hex64},
		{Generator: "align-judge", Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}, Integrity: "sha256:" + hex64},
		{Generator: "align", Version: "v0", Inputs: []string{".verdi/specs/active/foo/spec.md@3e91ab2"}, Digest: "sha256:" + hex64, Integrity: "sha256:" + hex64},
	}
	for _, p := range cases {
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate(%+v): %v", p, err)
		}
	}
}

func TestProvenance_Validate_Negative(t *testing.T) {
	cases := []Provenance{
		{Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "sha256:" + hex64},            // missing generator
		{Generator: "g", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "sha256:" + hex64},           // missing version
		{Generator: "g", Version: "v0", Digest: "sha256:" + hex64},                                  // missing inputs
		{Generator: "g", Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}},                       // neither digest nor integrity
		{Generator: "g", Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "not-sha256"}, // malformed digest
		{Generator: "g", Version: "v0", Inputs: []string{"nonsense"}, Digest: "sha256:" + hex64},    // malformed input
	}
	for i, p := range cases {
		if err := p.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, p)
		}
	}
}

var hex64 = strings.Repeat("ab", 32)

// TestDecodeStrict_AcceptsOptionalSchemaField proves Base's optional
// `schema:` key (phase 4 addition — see the field's doc comment) decodes
// strictly rather than tripping KnownFields, both when present and when
// absent, on every kind that embeds Base.
func TestDecodeStrict_AcceptsOptionalSchemaField(t *testing.T) {
	const withSchema = `---
id: adr/schema-field-present
kind: adr
title: "has a schema key"
status: proposed
owners: [platform-team]
schema: verdi.artifact/v1
---
`
	fm, err := DecodeADR(mustFrontmatter(t, withSchema))
	if err != nil {
		t.Fatalf("DecodeADR with schema field: %v", err)
	}
	if fm.Schema != "verdi.artifact/v1" {
		t.Fatalf("Schema = %q, want %q", fm.Schema, "verdi.artifact/v1")
	}

	const withoutSchema = `---
id: adr/schema-field-absent
kind: adr
title: "has no schema key"
status: proposed
owners: [platform-team]
---
`
	fm2, err := DecodeADR(mustFrontmatter(t, withoutSchema))
	if err != nil {
		t.Fatalf("DecodeADR without schema field: %v", err)
	}
	if fm2.Schema != "" {
		t.Fatalf("Schema = %q, want empty", fm2.Schema)
	}
}

func mustFrontmatter(t *testing.T, doc string) []byte {
	t.Helper()
	fm, _, err := SplitFrontmatter([]byte(doc))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	return fm
}
