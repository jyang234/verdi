package store

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// TestAbsolutePaths locks the host-native, root-joined forms against an
// explicit slash template converted to the host separator, so the assertion
// states the intended .verdi layout independently of the implementation.
func TestAbsolutePaths(t *testing.T) {
	const root = "/store"
	tests := []struct {
		name      string
		got       string
		wantSlash string
	}{
		{"SpecDir/active", SpecDir(root, ZoneActive, "widget"), "/store/.verdi/specs/active/widget"},
		{"SpecDir/archive", SpecDir(root, ZoneArchive, "widget"), "/store/.verdi/specs/archive/widget"},
		{"SpecPath/active", SpecPath(root, ZoneActive, "widget"), "/store/.verdi/specs/active/widget/spec.md"},
		{"SpecPath/archive", SpecPath(root, ZoneArchive, "widget"), "/store/.verdi/specs/archive/widget/spec.md"},
		{"ActiveSpecDir", ActiveSpecDir(root, "widget"), "/store/.verdi/specs/active/widget"},
		{"ActiveSpecPath", ActiveSpecPath(root, "widget"), "/store/.verdi/specs/active/widget/spec.md"},
		{"ArchiveSpecDir", ArchiveSpecDir(root, "widget"), "/store/.verdi/specs/archive/widget"},
		{"ArchiveSpecPath", ArchiveSpecPath(root, "widget"), "/store/.verdi/specs/archive/widget/spec.md"},
		{"DeviationReportPath/active", DeviationReportPath(root, ZoneActive, "widget"), "/store/.verdi/specs/active/widget/deviation-report.md"},
		{"DeviationReportPath/archive", DeviationReportPath(root, ZoneArchive, "widget"), "/store/.verdi/specs/archive/widget/deviation-report.md"},
		{"DecisionConflictReportPath", DecisionConflictReportPath(root, ZoneActive, "widget"), "/store/.verdi/specs/active/widget/decision-conflict-report.md"},
		{"AttestationDir", AttestationDir(root, "story-7"), "/store/.verdi/attestations/story-7"},
		{"AttestationPath", AttestationPath(root, "story-7", "ac-2"), "/store/.verdi/attestations/story-7/ac-2.md"},
		{"ObligationDir", ObligationDir(root, "widget"), "/store/.verdi/obligations/widget"},
		{"ObligationPath", ObligationPath(root, "widget", "ac-2", "behavioral"), "/store/.verdi/obligations/widget/ac-2--behavioral.md"},
		{"WaiverDir", WaiverDir(root, "story-7"), "/store/.verdi/waivers/story-7"},
		{"WaiverPath", WaiverPath(root, "story-7", "ac-2"), "/store/.verdi/waivers/story-7/ac-2.md"},
		{"DerivedRoot", DerivedRoot(root), "/store/.verdi/data/derived"},
		{"DerivedSpecDir", DerivedSpecDir(root, "spec--widget"), "/store/.verdi/data/derived/spec--widget"},
		{"DesignProvenancePath/active", DesignProvenancePath(root, ZoneActive, "widget"), "/store/.verdi/specs/active/widget/design-provenance.jsonl"},
		{"DesignProvenancePath/archive", DesignProvenancePath(root, ZoneArchive, "widget"), "/store/.verdi/specs/archive/widget/design-provenance.jsonl"},
		{"WriterLockPath", WriterLockPath(root), "/store/.verdi/data/writer.lock"},
		{"DraftMutationDir", DraftMutationDir(root, "widget"), "/store/.verdi/data/draft-mutation/widget"},
		{"DraftMutationJournalPath", DraftMutationJournalPath(root, "widget"), "/store/.verdi/data/draft-mutation/widget/journal.json"},
		{"DraftMutationSpecStagePath", DraftMutationSpecStagePath(root, "widget"), "/store/.verdi/data/draft-mutation/widget/spec.new"},
		{"DraftMutationProvenanceStagePath", DraftMutationProvenanceStagePath(root, "widget"), "/store/.verdi/data/draft-mutation/widget/provenance.new"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := filepath.FromSlash(tt.wantSlash)
			if tt.got != want {
				t.Errorf("got %q, want %q", tt.got, want)
			}
		})
	}
}

// TestRelativePaths locks the store-relative forms to exact slash-canonical
// strings — the identifier contract git tree paths, derivation-record inputs,
// and lint keys depend on. A negative assertion (no backslash ever) guards
// against a future regression to filepath.Join, which would corrupt these
// identifiers on a non-slash host.
func TestRelativePaths(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"SpecDirRelPath/active", SpecDirRelPath(ZoneActive, "widget"), ".verdi/specs/active/widget"},
		{"SpecDirRelPath/archive", SpecDirRelPath(ZoneArchive, "widget"), ".verdi/specs/archive/widget"},
		{"SpecRelPath/active", SpecRelPath(ZoneActive, "widget"), ".verdi/specs/active/widget/spec.md"},
		{"SpecRelPath/archive", SpecRelPath(ZoneArchive, "widget"), ".verdi/specs/archive/widget/spec.md"},
		{"ActiveSpecRelPath", ActiveSpecRelPath("widget"), ".verdi/specs/active/widget/spec.md"},
		{"DeviationReportRelPath", DeviationReportRelPath(ZoneActive, "widget"), ".verdi/specs/active/widget/deviation-report.md"},
		{"DecisionConflictReportRelPath", DecisionConflictReportRelPath(ZoneActive, "widget"), ".verdi/specs/active/widget/decision-conflict-report.md"},
		{"AttestationDirRelPath", AttestationDirRelPath("story-7"), ".verdi/attestations/story-7"},
		{"DerivedSpecRelDir", DerivedSpecRelDir("spec--widget"), ".verdi/data/derived/spec--widget"},
		{"DesignProvenanceRelPath/active", DesignProvenanceRelPath(ZoneActive, "widget"), ".verdi/specs/active/widget/design-provenance.jsonl"},
		{"DesignProvenanceRelPath/archive", DesignProvenanceRelPath(ZoneArchive, "widget"), ".verdi/specs/archive/widget/design-provenance.jsonl"},
		{"WriterLockRelPath", WriterLockRelPath(), ".verdi/data/writer.lock"},
		{"DraftMutationRelDir", DraftMutationRelDir("widget"), ".verdi/data/draft-mutation/widget"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
			if strings.ContainsRune(tt.got, '\\') {
				t.Errorf("relative path %q must be slash-canonical, contains a backslash", tt.got)
			}
		})
	}
}

// TestConveniencesMatchGeneralForms proves the fixed-zone wrappers are exactly
// the general form with the corresponding zone constant — no independent copy
// of the layout that could drift.
func TestConveniencesMatchGeneralForms(t *testing.T) {
	const root = "/store"
	const name = "widget"
	pairs := []struct {
		name    string
		wrapper string
		general string
	}{
		{"ActiveSpecDir", ActiveSpecDir(root, name), SpecDir(root, ZoneActive, name)},
		{"ActiveSpecPath", ActiveSpecPath(root, name), SpecPath(root, ZoneActive, name)},
		{"ArchiveSpecDir", ArchiveSpecDir(root, name), SpecDir(root, ZoneArchive, name)},
		{"ArchiveSpecPath", ArchiveSpecPath(root, name), SpecPath(root, ZoneArchive, name)},
		{"ActiveSpecRelPath", ActiveSpecRelPath(name), SpecRelPath(ZoneActive, name)},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			if p.wrapper != p.general {
				t.Errorf("wrapper %q != general %q", p.wrapper, p.general)
			}
		})
	}
}

// TestRelIsSlashOfAbsBelowRoot proves the anti-drift invariant the whole seam
// rests on: for every spec-directory family, the store-relative form equals
// the absolute form with the root stripped and separators slash-normalized.
// If the two families ever named different files, this fails — which is
// exactly the class of latent drift ADJ-71 set out to make impossible.
func TestRelIsSlashOfAbsBelowRoot(t *testing.T) {
	const root = "/store"
	const name = "widget"
	cases := []struct {
		name string
		abs  string
		rel  string
	}{
		{"specdir/active", SpecDir(root, ZoneActive, name), SpecDirRelPath(ZoneActive, name)},
		{"spec/active", SpecPath(root, ZoneActive, name), SpecRelPath(ZoneActive, name)},
		{"spec/archive", SpecPath(root, ZoneArchive, name), SpecRelPath(ZoneArchive, name)},
		{"deviation", DeviationReportPath(root, ZoneActive, name), DeviationReportRelPath(ZoneActive, name)},
		{"decisionconflict", DecisionConflictReportPath(root, ZoneActive, name), DecisionConflictReportRelPath(ZoneActive, name)},
		{"attestationdir", AttestationDir(root, "story-7"), AttestationDirRelPath("story-7")},
		{"derivedspecdir", DerivedSpecDir(root, "spec--widget"), DerivedSpecRelDir("spec--widget")},
		{"designprovenance/active", DesignProvenancePath(root, ZoneActive, name), DesignProvenanceRelPath(ZoneActive, name)},
		{"designprovenance/archive", DesignProvenancePath(root, ZoneArchive, name), DesignProvenanceRelPath(ZoneArchive, name)},
		{"writerlock", WriterLockPath(root), WriterLockRelPath()},
		{"draftmutation", DraftMutationDir(root, name), DraftMutationRelDir(name)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rel, ok := strings.CutPrefix(filepath.ToSlash(c.abs), "/store/")
			if !ok {
				t.Fatalf("absolute %q not rooted under /store/", c.abs)
			}
			if rel != c.rel {
				t.Errorf("relative form %q != slash(abs) below root %q", c.rel, rel)
			}
		})
	}
}

// TestAttestationPathEmptyRootDisplayForm locks the store-relative display
// behavior evidence disclosures depend on: an empty root drops the leading
// element, yielding the ".verdi/…"-rooted form a disclosure prints instead of
// a temp-dir- or checkout-rooted absolute path.
func TestAttestationPathEmptyRootDisplayForm(t *testing.T) {
	if got, want := AttestationPath("", "story-7", "ac-2"), filepath.FromSlash(".verdi/attestations/story-7/ac-2.md"); got != want {
		t.Errorf("AttestationPath(\"\", …) = %q, want %q", got, want)
	}
	if got, want := AttestationDir("", "story-7"), filepath.FromSlash(".verdi/attestations/story-7"); got != want {
		t.Errorf("AttestationDir(\"\", …) = %q, want %q", got, want)
	}
}

// TestObligationPathEmptyRootDisplayForm is ObligationPath/ObligationDir's
// own twin of TestAttestationPathEmptyRootDisplayForm (spec/obligation-seam
// ac-1/ac-5's shared path seam): an empty root drops the leading element,
// yielding the ".verdi/…"-rooted display form a refusal message names.
func TestObligationPathEmptyRootDisplayForm(t *testing.T) {
	if got, want := ObligationPath("", "widget", "ac-2", "behavioral"), filepath.FromSlash(".verdi/obligations/widget/ac-2--behavioral.md"); got != want {
		t.Errorf("ObligationPath(\"\", …) = %q, want %q", got, want)
	}
	if got, want := ObligationDir("", "widget"), filepath.FromSlash(".verdi/obligations/widget"); got != want {
		t.Errorf("ObligationDir(\"\", …) = %q, want %q", got, want)
	}
}

// TestWaiverPathEmptyRootDisplayForm is WaiverPath/WaiverDir's own twin of
// TestAttestationPathEmptyRootDisplayForm (spec/verb-surfaces ac-1): an
// empty root drops the leading element, yielding the ".verdi/…"-rooted
// display form a disclosure names.
func TestWaiverPathEmptyRootDisplayForm(t *testing.T) {
	if got, want := WaiverPath("", "story-7", "ac-2"), filepath.FromSlash(".verdi/waivers/story-7/ac-2.md"); got != want {
		t.Errorf("WaiverPath(\"\", …) = %q, want %q", got, want)
	}
	if got, want := WaiverDir("", "story-7"), filepath.FromSlash(".verdi/waivers/story-7"); got != want {
		t.Errorf("WaiverDir(\"\", …) = %q, want %q", got, want)
	}
}

// TestPolicyDispositionPath locks the host-native, root-joined form
// authority-design §8 fixes: "Semantic rulings live at
// .verdi/policy/dispositions/<name>.md".
func TestPolicyDispositionPath(t *testing.T) {
	got := PolicyDispositionPath("/store", "review-no-conflict")
	want := filepath.FromSlash("/store/.verdi/policy/dispositions/review-no-conflict.md")
	if got != want {
		t.Errorf("PolicyDispositionPath = %q, want %q", got, want)
	}
}

// TestPolicyDispositionRelPath locks the store-relative, slash-canonical
// twin, mirroring every other *RelPath accessor's own convention.
func TestPolicyDispositionRelPath(t *testing.T) {
	got := PolicyDispositionRelPath("review-no-conflict")
	want := ".verdi/policy/dispositions/review-no-conflict.md"
	if got != want {
		t.Errorf("PolicyDispositionRelPath = %q, want %q", got, want)
	}
	if strings.ContainsRune(got, '\\') {
		t.Errorf("PolicyDispositionRelPath %q must be slash-canonical, contains a backslash", got)
	}
}

// TestPolicyDispositionPath_RelIsSlashOfAbsBelowRoot is
// TestRelIsSlashOfAbsBelowRoot's own case for the new accessor pair — the
// same anti-drift invariant every other family in this file already proves.
func TestPolicyDispositionPath_RelIsSlashOfAbsBelowRoot(t *testing.T) {
	const root = "/store"
	abs := PolicyDispositionPath(root, "review-no-conflict")
	rel, ok := strings.CutPrefix(filepath.ToSlash(abs), "/store/")
	if !ok {
		t.Fatalf("absolute %q not rooted under /store/", abs)
	}
	if rel != PolicyDispositionRelPath("review-no-conflict") {
		t.Errorf("relative form %q != slash(abs) below root %q", PolicyDispositionRelPath("review-no-conflict"), rel)
	}
}

// TestPolicyDispositionPathEmptyRootDisplayForm is
// TestAttestationPathEmptyRootDisplayForm's own twin for the disposition
// accessor: an empty root drops the leading element, yielding the
// ".verdi/…"-rooted display form a refusal or disclosure names instead of a
// temp-dir- or checkout-rooted absolute path.
func TestPolicyDispositionPathEmptyRootDisplayForm(t *testing.T) {
	if got, want := PolicyDispositionPath("", "review-no-conflict"), filepath.FromSlash(".verdi/policy/dispositions/review-no-conflict.md"); got != want {
		t.Errorf("PolicyDispositionPath(\"\", …) = %q, want %q", got, want)
	}
}

// TestPolicyDispositionPath_EdgeCaseNamesAreJoinedNotValidated documents
// what this accessor pair does with a degenerate name. Like every sibling
// in paths.go — AttestationPath, ObligationPath, WaiverPath, ConflictPath —
// it is a PURE JOIN helper: it never validates its name, so a caller's
// empty, separator-bearing, traversal, or non-kebab name is joined and
// path-cleaned rather than refused here.
//
// That is deliberate, not a gap: the closed name grammar lives in
// policyartifact.ClassifyPolicyPath, the seam
// TestPolicyDispositionPath_AgreesWithPolicyArtifactGrammar pins and
// internal/policyauthority's loader actually consults. This test therefore
// records the join behavior AND shows that each degenerate name's resulting
// store path is refused by that grammar, so no such path can enter the
// constitution store through the accessor.
func TestPolicyDispositionPath_EdgeCaseNamesAreJoinedNotValidated(t *testing.T) {
	const policyPrefix = ".verdi/policy/"
	cases := []struct {
		name    string
		input   string
		wantRel string
		why     string
	}{
		{"empty", "", ".verdi/policy/dispositions/.md", "an empty name yields a bare extension, not an error"},
		{"nested separator", "sub/ruling", ".verdi/policy/dispositions/sub/ruling.md", "a separator nests rather than being rejected or escaped"},
		{"parent traversal", "../escape", ".verdi/policy/escape.md", "path.Join CLEANS the traversal, so the result leaves the dispositions directory"},
		{"non-kebab", "Review Ruling", ".verdi/policy/dispositions/Review Ruling.md", "case and spaces survive the join untouched"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rel := PolicyDispositionRelPath(tc.input)
			if rel != tc.wantRel {
				t.Errorf("PolicyDispositionRelPath(%q) = %q, want %q (%s)", tc.input, rel, tc.wantRel, tc.why)
			}
			if got, want := PolicyDispositionPath("/store", tc.input), filepath.FromSlash("/store/"+tc.wantRel); got != want {
				t.Errorf("PolicyDispositionPath(\"/store\", %q) = %q, want %q", tc.input, got, want)
			}
			if !strings.HasPrefix(rel, policyPrefix) {
				t.Fatalf("PolicyDispositionRelPath(%q) = %q, want a %q-prefixed path", tc.input, rel, policyPrefix)
			}
			if _, _, err := policyartifact.ClassifyPolicyPath(strings.TrimPrefix(rel, policyPrefix)); err == nil {
				t.Errorf("policyartifact.ClassifyPolicyPath accepted %q built from name %q; the closed grammar must refuse it", rel, tc.input)
			}
		})
	}
}

// TestPolicyDispositionPath_DotSegmentIsCleaned is the edge case above's
// one non-refusing member, separated because it proves the opposite half of
// the same "join, then clean" behavior: a leading "./" is cleaned away, so
// the result is the ordinary path for the bare name and the grammar accepts
// it. A caller must not read that as validation — only as path.Join's
// documented cleaning.
func TestPolicyDispositionPath_DotSegmentIsCleaned(t *testing.T) {
	if got, want := PolicyDispositionRelPath("./ruling"), PolicyDispositionRelPath("ruling"); got != want {
		t.Errorf("PolicyDispositionRelPath(%q) = %q, want it cleaned to %q", "./ruling", got, want)
	}
}

// TestPolicyDispositionPath_AgreesWithPolicyArtifactGrammar proves
// PolicyDispositionPath/RelPath never drift from
// policyartifact.DirDispositions or the policy-disposition/<name> id
// grammar policyartifact.ClassifyPolicyPath enforces: this store-level
// accessor and the constitution store's own closed directory grammar must
// always agree on which file backs which disposition id.
func TestPolicyDispositionPath_AgreesWithPolicyArtifactGrammar(t *testing.T) {
	const name = "review-no-conflict"
	rel := PolicyDispositionRelPath(name)
	const policyPrefix = ".verdi/policy/"
	if !strings.HasPrefix(rel, policyPrefix) {
		t.Fatalf("PolicyDispositionRelPath(%q) = %q, want a %q-prefixed path", name, rel, policyPrefix)
	}
	policyRel := strings.TrimPrefix(rel, policyPrefix)
	if !strings.HasPrefix(policyRel, policyartifact.DirDispositions+"/") {
		t.Fatalf("PolicyDispositionRelPath(%q) = %q, want it under policyartifact.DirDispositions (%q)", name, rel, policyartifact.DirDispositions)
	}
	kind, gotName, err := policyartifact.ClassifyPolicyPath(policyRel)
	if err != nil {
		t.Fatalf("policyartifact.ClassifyPolicyPath(%q): %v", policyRel, err)
	}
	if kind != policyartifact.KindDisposition {
		t.Fatalf("ClassifyPolicyPath(%q) kind = %q, want %q", policyRel, kind, policyartifact.KindDisposition)
	}
	if gotName != name {
		t.Fatalf("ClassifyPolicyPath(%q) name = %q, want %q", policyRel, gotName, name)
	}
}
