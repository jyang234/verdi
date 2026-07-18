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
		// Model present (spec/model-digest ac-1): same sha256:<64 hex> shape
		// Digest/Integrity already validate against, no new vocabulary.
		{Generator: "align", Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "sha256:" + hex64, Model: "sha256:" + hex64},
		// Model absent (omitempty): a pre-model-digest artifact stays valid.
		{Generator: "align", Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "sha256:" + hex64, Model: ""},
	}
	for _, p := range cases {
		if err := p.Validate(); err != nil {
			t.Fatalf("Validate(%+v): %v", p, err)
		}
	}
}

func TestProvenance_Validate_Negative(t *testing.T) {
	cases := []Provenance{
		{Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "sha256:" + hex64},                                           // missing generator
		{Generator: "g", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "sha256:" + hex64},                                          // missing version
		{Generator: "g", Version: "v0", Digest: "sha256:" + hex64},                                                                 // missing inputs
		{Generator: "g", Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}},                                                      // neither digest nor integrity
		{Generator: "g", Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "not-sha256"},                                // malformed digest
		{Generator: "g", Version: "v0", Inputs: []string{"nonsense"}, Digest: "sha256:" + hex64},                                   // malformed input
		{Generator: "g", Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "sha256:" + hex64, Model: "not-sha256"},      // malformed model digest
		{Generator: "g", Version: "v0", Inputs: []string{"spec/foo@3e91ab2"}, Digest: "sha256:" + hex64, Model: "sha256:tooshort"}, // malformed model digest, wrong length
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

// TestIsBareFilename tables the containment predicate both the model kernel
// rule and designscaffold's LoadTemplate guard share (judged-template-
// filename-escapes-templates-dir): a bare filename is accepted; anything
// that could escape a fixed join directory — empty, . / .., a "/" or "\\"
// separator, or an absolute path — is rejected, the same judgment on every
// OS.
func TestIsBareFilename(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"story.md", true},
		{"custom-feature.md", true},
		{"feature", true},
		{".hidden.md", true}, // a leading-dot filename is still bare (not "." / "..")
		{"", false},
		{".", false},
		{"..", false},
		{"sub/story.md", false},
		{"../story.md", false},
		{"../../evil.md", false},
		{"a/b", false},
		{`sub\story.md`, false}, // backslash rejected too, so a shared store is portable
		{"/abs/story.md", false},
		{"/etc/passwd", false},
	}
	for _, tc := range cases {
		if got := IsBareFilename(tc.in); got != tc.want {
			t.Errorf("IsBareFilename(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
