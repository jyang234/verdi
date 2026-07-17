package store

import (
	"path/filepath"
	"strings"
	"testing"
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
		{"DerivedRoot", DerivedRoot(root), "/store/.verdi/data/derived"},
		{"DerivedSpecDir", DerivedSpecDir(root, "spec--widget"), "/store/.verdi/data/derived/spec--widget"},
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
		{"SpecRelPath/active", SpecRelPath(ZoneActive, "widget"), ".verdi/specs/active/widget/spec.md"},
		{"SpecRelPath/archive", SpecRelPath(ZoneArchive, "widget"), ".verdi/specs/archive/widget/spec.md"},
		{"ActiveSpecRelPath", ActiveSpecRelPath("widget"), ".verdi/specs/active/widget/spec.md"},
		{"DeviationReportRelPath", DeviationReportRelPath(ZoneActive, "widget"), ".verdi/specs/active/widget/deviation-report.md"},
		{"DecisionConflictReportRelPath", DecisionConflictReportRelPath(ZoneActive, "widget"), ".verdi/specs/active/widget/decision-conflict-report.md"},
		{"AttestationDirRelPath", AttestationDirRelPath("story-7"), ".verdi/attestations/story-7"},
		{"DerivedSpecRelDir", DerivedSpecRelDir("spec--widget"), ".verdi/data/derived/spec--widget"},
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
		{"spec/active", SpecPath(root, ZoneActive, name), SpecRelPath(ZoneActive, name)},
		{"spec/archive", SpecPath(root, ZoneArchive, name), SpecRelPath(ZoneArchive, name)},
		{"deviation", DeviationReportPath(root, ZoneActive, name), DeviationReportRelPath(ZoneActive, name)},
		{"decisionconflict", DecisionConflictReportPath(root, ZoneActive, name), DecisionConflictReportRelPath(ZoneActive, name)},
		{"attestationdir", AttestationDir(root, "story-7"), AttestationDirRelPath("story-7")},
		{"derivedspecdir", DerivedSpecDir(root, "spec--widget"), DerivedSpecRelDir("spec--widget")},
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
