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
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
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

// modelAssignRe matches a source-level assignment whose target is a field
// named Model — `.Model =`, with `[^=]` after it to exclude the `==`/`>=`/`<=`
// comparison forms. It is ONLY a pre-filter over production files: a real
// assignment to Provenance.Model necessarily contains this text in
// gofmt-clean source (fmt-check enforces the single-space spacing), so a
// package with no match cannot host a bypass and is skipped unread by the
// type checker below. The actual decision is made by go/types, never by this
// regexp — the regexp exists to keep the (comparatively slow) type-checking
// scoped to the handful of packages that could possibly matter.
var modelAssignRe = regexp.MustCompile(`\.Model\s*=[^=]`)

// TestNoStrayProvenanceModelAssignment is ac-2's "no bypass" half. It proves
// that across all PRODUCTION source (excluding _test.go and
// internal/artifact/stamp.go itself), no file assigns to the Model field OF
// AN artifact.Provenance VALUE — the exact mutation StampProvenance performs
// — so StampProvenance stays the single writer of that field. A second such
// assignment anywhere in production would be a silent bypass of the seam.
//
// Mechanism (spec/model-digest's remediation dispatch: resolve the
// assignee's TYPE, do not pattern-match the field name). A coarse `.Model =`
// text scan — the shape this test originally shipped — is provably wrong: the
// vocabulary-surfaces work legitimately assigns *model.Model to unrelated
// Deps/HomeDeps.Model fields (internal/workbench/handler.go, directory.go),
// which a textual scan cannot tell apart from a Provenance.Model bypass and
// so reddened at the merged head. This version instead parses each candidate
// package with go/parser and type-checks it with go/types, resolving each
// `X.Model = …` assignment's RECEIVER type via types.Info.Selections; it
// trips ONLY when that receiver is artifact.Provenance (value or pointer).
// The stdlib source importer (importer.ForCompiler(fset, "source", nil))
// resolves this module's own internal/... packages from source — verified
// feasible against internal/workbench, the largest importer of this package
// — so no build-cache export data or golang.org/x/tools dependency is
// needed. This mirrors the repo's existing go/ast source-witness precedent
// (internal/specalign/gatecache_test.go, internal/showcasealign) and extends
// it with go/types receiver resolution.
//
// Limits, per the dispatch's "document the mechanism's limits":
//   - Only DIRECT field assignments are seen. A reflection-based write
//     (reflect.ValueOf(&p).Elem().FieldByName("Model").SetString(…)) carries
//     no `.Model =` text and is not policed — exactly as the spec's own
//     textual source-witness convention does not police it either.
//   - A candidate package that fails to type-check is a HARD failure here,
//     never a silent pass (CLAUDE.md: silence is never a pass): an unresolved
//     receiver type cannot be cleared to "not a Provenance".
//   - The four behavioral mint-path tests (report_test.go et al.)
//     independently prove each real site routes its digest through the seam;
//     this static witness is the no-undiscovered-FIFTH-site guard, not the
//     sole line of defense.
func TestNoStrayProvenanceModelAssignment(t *testing.T) {
	root := moduleRoot(t)
	const stampFile = "internal/artifact/stamp.go"
	const artifactPkgPath = "github.com/jyang234/verdi/internal/artifact"

	// 1. Pre-filter to the production package directories that contain at
	//    least one `.Model =` assignment (see modelAssignRe).
	candidateDirs := map[string]bool{}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
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
		if modelAssignRe.Match(data) {
			candidateDirs[filepath.Dir(path)] = true
		}
		return nil
	}); err != nil {
		t.Fatalf("walking module: %v", err)
	}

	dirs := make([]string, 0, len(candidateDirs))
	for d := range candidateDirs {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	// 2. Type-check each candidate package once, sharing a single source
	//    importer so common dependencies are resolved (and cached) once.
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "source", nil)

	var violations []string
	for _, dir := range dirs {
		files := parseProductionPackage(t, fset, dir)
		if len(files) == 0 {
			continue
		}
		relDir, rerr := filepath.Rel(root, dir)
		if rerr != nil {
			t.Fatalf("relpath %s: %v", dir, rerr)
		}
		importPath := "github.com/jyang234/verdi/" + filepath.ToSlash(relDir)

		info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
		var typeErrs []string
		conf := types.Config{
			Importer: imp,
			Error:    func(e error) { typeErrs = append(typeErrs, e.Error()) },
		}
		_, _ = conf.Check(importPath, fset, files, info)
		if len(typeErrs) > 0 {
			t.Fatalf("type-checking %s (a package carrying a .Model assignment) failed — the witness cannot resolve receiver types there and must not silently pass:\n%s", relDir, strings.Join(typeErrs, "\n"))
		}

		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for _, lhs := range assign.Lhs {
					sel, ok := lhs.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "Model" {
						continue
					}
					selection := info.Selections[sel]
					if selection == nil || !isArtifactProvenance(selection.Recv(), artifactPkgPath) {
						continue
					}
					pos := fset.Position(sel.Pos())
					rel, rerr := filepath.Rel(root, pos.Filename)
					if rerr != nil {
						rel = pos.Filename
					}
					rel = filepath.ToSlash(rel)
					if rel == stampFile {
						continue
					}
					violations = append(violations, fmt.Sprintf("%s:%d", rel, pos.Line))
				}
				return true
			})
		}
	}

	sort.Strings(violations)
	for _, v := range violations {
		t.Errorf("%s assigns to artifact.Provenance.Model outside %s — model-digest ac-2 requires StampProvenance to be the only writer of Provenance.Model", v, stampFile)
	}
}

// parseProductionPackage parses every build-eligible, non-test .go file in
// dir. build.Default.MatchFile honors build constraints so a platform- or
// tag-excluded file never derails type-checking; every file is added to the
// shared fset so positions and the importer line up.
func parseProductionPackage(t *testing.T, fset *token.FileSet, dir string) []*ast.File {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading dir %s: %v", dir, err)
	}
	var files []*ast.File
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		ok, merr := build.Default.MatchFile(dir, e.Name())
		if merr != nil {
			t.Fatalf("build-constraint check %s/%s: %v", dir, e.Name(), merr)
		}
		if !ok {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, 0)
		if perr != nil {
			t.Fatalf("parsing %s/%s: %v", dir, e.Name(), perr)
		}
		files = append(files, f)
	}
	return files
}

// isArtifactProvenance reports whether t is artifact.Provenance or a pointer
// to it — the receiver type an assignment must have to be a genuine bypass of
// StampProvenance. Any other .Model field owner (workbench.Deps,
// store.Config, *model.Model, …) returns false.
func isArtifactProvenance(t types.Type, pkgPath string) bool {
	if t == nil {
		return false
	}
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == pkgPath && obj.Name() == "Provenance"
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
