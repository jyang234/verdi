package artifact

// spec/model-digest ac-2's static evidence (source-witness tests in the
// style this repo already uses — the atomicfile CreateTemp witnesses,
// internal/wallbadge's TestLadderStaticCallSites, internal/workbench's
// evidenceslotstatic_test.go): every production artifact.Provenance{...}
// composite-literal mint routes its Model field through StampProvenance —
// no bypass, no undiscovered fifth site. Lives in package artifact (not
// align/commitdesign) so it can reuse seam_test.go's moduleRoot helper and
// read every mint site's source text from one place, mirroring the same
// package's own TestYAMLImportSeam module-wide scan convention.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// modelDigestMintSites enumerates model-digest ac-2's exact four
// production artifact.Provenance{...} construction sites (the frozen
// spec's own accounting, mirrored by
// .verdi/obligations/model-digest/ac-2--static.md) — paths relative to the
// module root. cmd/verdi/attest.go is deliberately absent:
// TestAttestGoMintsNoProvenance documents why, and
// TestProvenanceMintSites_ExactlyFour proves this list is not stale.
var modelDigestMintSites = []string{
	"internal/commitdesign/commitdesign.go",
	"internal/align/report.go",
	"internal/align/decision_report.go",
	"internal/align/diagram_report.go",
}

// TestProvenanceMintSites_RouteThroughStampProvenance proves each
// enumerated mint site (a) calls artifact.StampProvenance somewhere in its
// source, and (b) never sets Model: inline inside its own
// artifact.Provenance{...} composite literal(s) the way Digest:/Integrity:
// are set today — the exact "one seam, no surviving copies" shape
// spec/shared-homes ac-1's own static convention already established for
// a different seam in this codebase.
func TestProvenanceMintSites_RouteThroughStampProvenance(t *testing.T) {
	root := moduleRoot(t)
	for _, rel := range modelDigestMintSites {
		t.Run(rel, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				t.Fatalf("reading %s: %v", rel, err)
			}
			src := string(data)

			if !strings.Contains(src, "artifact.StampProvenance(") {
				t.Errorf("%s never calls artifact.StampProvenance — model-digest ac-2 requires every mint site to route its Model field through the seam", rel)
			}

			literals := provenanceLiterals(t, src, rel)
			if len(literals) == 0 {
				t.Fatalf("%s: found no artifact.Provenance{...} literal — modelDigestMintSites is stale (this file no longer mints one)", rel)
			}
			for _, lit := range literals {
				if strings.Contains(lit, "Model:") {
					t.Errorf("%s sets Model: inline inside an artifact.Provenance{...} literal — must be set only via StampProvenance, after construction:\n%s", rel, lit)
				}
			}
		})
	}
}

// provenanceLiterals extracts the brace-balanced text of every
// "artifact.Provenance{...}" composite literal in src (there may be more
// than one per file in principle; today each enumerated site has exactly
// one), so the inline-Model check above inspects exactly each literal's
// own fields — never an unrelated Model: key belonging to some other,
// later struct literal in the same file.
func provenanceLiterals(t *testing.T, src, rel string) []string {
	t.Helper()
	const marker = "artifact.Provenance{"
	var out []string
	searchFrom := 0
	for {
		idx := strings.Index(src[searchFrom:], marker)
		if idx < 0 {
			break
		}
		start := searchFrom + idx + len(marker) - 1 // index of the opening '{'
		depth := 0
		end := -1
		for j := start; j < len(src); j++ {
			switch src[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = j
				}
			}
			if end >= 0 {
				break
			}
		}
		if end < 0 {
			t.Fatalf("%s: unbalanced braces scanning an artifact.Provenance{ literal starting at byte %d", rel, start)
		}
		out = append(out, src[start:end+1])
		searchFrom = end + 1
	}
	return out
}

// TestProvenanceMintSites_ExactlyFour is the enumeration's own self-check
// (ac-2: "so that a future fifth mint site is caught by the same check
// rather than requiring this list to be rediscovered by hand"): exactly
// the four files in modelDigestMintSites construct artifact.Provenance{...}
// anywhere in the module's PRODUCTION (non-test) source. Test files are
// deliberately excluded from this count — ac-2's concern is production
// mint sites bypassing the seam; a test file directly constructing a
// Provenance{...} literal for decode/assertion fixtures (e.g.
// internal/artifact's own Validate tests, unqualified within the same
// package) is a different, legitimate thing this check does not police.
func TestProvenanceMintSites_ExactlyFour(t *testing.T) {
	root := moduleRoot(t)
	const marker = "artifact.Provenance{"
	got := map[string]bool{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(data), marker) {
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			got[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking module: %v", err)
	}

	want := map[string]bool{}
	for _, rel := range modelDigestMintSites {
		want[rel] = true
	}
	if len(got) != len(want) {
		gotList := make([]string, 0, len(got))
		for k := range got {
			gotList = append(gotList, k)
		}
		sort.Strings(gotList)
		t.Fatalf("found %d production file(s) constructing artifact.Provenance{...}: %v — want exactly the %d enumerated in modelDigestMintSites: %v", len(got), gotList, len(want), modelDigestMintSites)
	}
	for rel := range want {
		if !got[rel] {
			t.Errorf("enumerated mint site %s no longer constructs artifact.Provenance{...} in production source — update modelDigestMintSites", rel)
		}
	}
}

// TestNoStrayProvenanceModelAssignment is ac-2's "no bypass" half, scoped
// to production source (excluding _test.go — a test file legitimately
// building a *Provenance by hand for fixture/decode purposes is a
// different concern than a production mint site bypassing the seam;
// internal/store/open_test.go, for one example, contains the literal text
// ".Model = " only inside t.Fatal prose strings, not a real assignment,
// which is exactly the kind of false positive scoping to production files
// avoids): across every production .go file in the module except
// internal/artifact/stamp.go itself, no file assigns to a .Model field —
// the exact mutation StampProvenance performs. A second assignment site
// anywhere else in production code would be a silent bypass of the seam.
func TestNoStrayProvenanceModelAssignment(t *testing.T) {
	root := moduleRoot(t)
	const pattern = ".Model = "
	const stampFile = "internal/artifact/stamp.go"

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if rel == stampFile {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(data), pattern) {
			t.Errorf("%s assigns to a .Model field outside %s — model-digest ac-2 requires StampProvenance to be the only writer of Provenance.Model", rel, stampFile)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking module: %v", err)
	}
}

// TestAttestGoMintsNoProvenance documents and proves why
// cmd/verdi/attest.go is correctly absent from modelDigestMintSites (ac-2,
// and the frozen spec's own Ac 2 text): it mints only a Frozen stamp for an
// AttestationScaffold — a human claim, neither computed nor judged content
// — never a Provenance, so it was never in scope for a model digest and
// the enumeration's count stays four, never five.
func TestAttestGoMintsNoProvenance(t *testing.T) {
	root := moduleRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "cmd/verdi/attest.go"))
	if err != nil {
		t.Fatalf("reading cmd/verdi/attest.go: %v", err)
	}
	src := string(data)
	if strings.Contains(src, "artifact.Provenance{") {
		t.Fatal("cmd/verdi/attest.go now constructs an artifact.Provenance{...} literal — it must join modelDigestMintSites (ac-2's enumeration would become five, not four)")
	}
	if !strings.Contains(src, "NewFrozen(") {
		t.Fatal("cmd/verdi/attest.go no longer calls artifact.NewFrozen — re-verify the Problem section's own accounting (it mints only a Frozen stamp)")
	}
}
