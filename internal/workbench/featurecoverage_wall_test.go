// TestWallCoverageChips_SharedFunction (spec/index-coverage ac-1--
// behavioral) proves both halves of the obligation's claim:
//
//   - texts unchanged: served over two fixture walls, every coverage chip
//     renders byte-identically to a golden captured from the
//     pre-extraction projection.go (ce2f66db) — the scoping fixture's
//     "covered by 1 stub"/"no stub" and the multi-stub fixture's
//     "covered by 2 stubs" from two distinct stubs on one criterion;
//   - computed by the shared function: a structural scan of this
//     package's sources finds that every write of ACCoverage is either
//     the map's make() or an element set to len(<v>[<id>].Stubs) where
//     <v> is featurecoverage.Compute's result — no private recount
//     (ACCoverage[...]++, +=, a literal, or any other right-hand side).
//
// The golden half alone cannot tell delegation from a faithful copy (it
// passes on the pre-extraction code by construction); the structural half
// is what falsifies "the wall computes coverage outside the shared
// function".
//
// TestBuildProjection_ACCoverageMatchesFeatureCoverage compares
// buildProjection's counts with featurecoverage.Compute's on the same
// frontmatter; it proves equal VALUES, not delegation (a correct private
// recount would pass it too).
package workbench

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/featurecoverage"
	"github.com/jyang234/verdi/internal/fixturegit"
)

var coverageChipRe = regexp.MustCompile(`<span class="coverage-chip[^"]*" data-testid="coverage-[^"]*" data-coverage="[0-9]+">[^<]*</span>`)

// coverageMultiStubFixtureSpec is a feature-class wall whose ac-1 is
// covered by two DISTINCT stubs, ac-2 by one, and ac-3 by none — the
// "covered by N stubs" plural the scoping fixture never reaches. Its
// golden (testdata/wall-coverage-chips-multi.golden) was captured by
// serving this exact spec from the ce2f66db tree, whose projection.go
// still counted ACCoverage with its own per-stub loop.
const coverageMultiStubFixtureSpec = `---
id: spec/coverage-multi
kind: spec
class: feature
title: "Coverage multi-stub fixture"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "covered by two stubs", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "covered by one stub", evidence: [attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "uncovered", evidence: [attestation], anchor: "#ac-3" }
stubs:
  - { slug: stub-one, acceptance_criteria: [ac-1, ac-2] }
  - { slug: stub-two, acceptance_criteria: [ac-1] }
frozen: { at: 2026-07-12, commit: 6400db382876f416ed943f6b6e22954f9666fde3 }
---
# Coverage multi-stub fixture

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## ac-2

Prose.

## ac-3

Prose.
`

func TestWallCoverageChips_SharedFunction(t *testing.T) {
	walls := []struct {
		name   string
		slug   string
		spec   string
		golden string
	}{
		{"scoping fixture: one stub and none", "scoping-fixture", scopingProjectionFixtureSpec, "wall-coverage-chips.golden"},
		{"multi-stub fixture: two distinct stubs on one criterion", "coverage-multi", coverageMultiStubFixtureSpec, "wall-coverage-chips-multi.golden"},
	}
	for _, w := range walls {
		t.Run("texts unchanged/"+w.name, func(t *testing.T) {
			mainFiles := map[string]string{
				".verdi/specs/active/" + w.slug + "/spec.md": w.spec,
				".verdi/.gitignore":                          "data/\n",
			}
			repo := fixturegit.Build(t, []fixturegit.Layer{{Files: mainFiles, Message: "seed coverage-chip fixture"}})
			setDefaultBranchSymref(t, repo.Dir)

			h := newBoardTestHandler(repo.Dir)
			rec := getBoard(t, h, w.slug)
			if rec.Code != 200 {
				t.Fatalf("GET board = %d, want 200\n%s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()

			got := coverageChipRe.FindAllString(body, -1)
			if len(got) == 0 {
				t.Fatalf("no coverage chips found in served body:\n%s", body)
			}
			var gotJoined string
			for _, c := range got {
				gotJoined += c + "\n"
			}

			wantBytes, err := os.ReadFile(filepath.Join("testdata", w.golden))
			if err != nil {
				t.Fatalf("reading golden: %v", err)
			}
			if gotJoined != string(wantBytes) {
				t.Fatalf("served coverage chips changed from the pre-extraction golden %s.\n--- got ---\n%s--- want ---\n%s", w.golden, gotJoined, string(wantBytes))
			}
		})
	}

	t.Run("computed by featurecoverage.Compute", func(t *testing.T) {
		delegated, violations := scanACCoverageWrites(t, ".")
		for _, v := range violations {
			t.Errorf("ACCoverage written outside featurecoverage.Compute's result: %s", v)
		}
		if delegated == 0 {
			t.Fatal("found no ACCoverage element set from featurecoverage.Compute's result — the scan is broken or the wall no longer delegates")
		}
	})
}

// scanACCoverageWrites parses the non-test Go sources in dir and reports
// every statement that writes an ACCoverage field. delegated counts the
// element writes of the one allowed shape — `<x>.ACCoverage[<k>] =
// len(<v>[<k2>].Stubs)` where <v> was bound, in the same function, to a
// featurecoverage.Compute call. A make() of the whole map is allowed and
// not counted. Everything else is a violation: an element ++/--, a
// compound assignment, an element or whole-map assignment from any other
// expression, a composite-literal ACCoverage field, or a delete().
func scanACCoverageWrites(t *testing.T, dir string) (delegated int, violations []string) {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package sources: %v", err)
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				computed := computeResultVars(fn.Body)
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if n == nil {
						return false
					}
					at := fset.Position(n.Pos()).String()
					switch s := n.(type) {
					case *ast.IncDecStmt:
						if isACCoverageElem(s.X) || isACCoverageField(s.X) {
							violations = append(violations, at+": "+s.Tok.String()+" on ACCoverage")
						}
					case *ast.AssignStmt:
						for i, lhs := range s.Lhs {
							var rhs ast.Expr
							if len(s.Rhs) == len(s.Lhs) {
								rhs = s.Rhs[i]
							}
							switch {
							case isACCoverageElem(lhs):
								if s.Tok != token.ASSIGN || !isLenStubsOf(rhs, computed) {
									violations = append(violations, at+": ACCoverage element "+s.Tok.String()+" from a value other than len(<Compute result>[id].Stubs)")
									continue
								}
								delegated++
							case isACCoverageField(lhs):
								if s.Tok != token.ASSIGN || !isMakeCall(rhs) {
									violations = append(violations, at+": ACCoverage map "+s.Tok.String()+" from a value other than make()")
								}
							}
						}
					case *ast.KeyValueExpr:
						if id, ok := s.Key.(*ast.Ident); ok && id.Name == "ACCoverage" {
							violations = append(violations, at+": ACCoverage set in a composite literal")
						}
					case *ast.CallExpr:
						if id, ok := s.Fun.(*ast.Ident); ok && id.Name == "delete" && len(s.Args) > 0 && isACCoverageField(s.Args[0]) {
							violations = append(violations, at+": delete() on ACCoverage")
						}
					}
					return true
				})
			}
		}
	}
	return delegated, violations
}

// computeResultVars returns the names bound to a featurecoverage.Compute
// call anywhere in body.
func computeResultVars(body *ast.BlockStmt) map[string]bool {
	vars := map[string]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || len(as.Lhs) != len(as.Rhs) {
			return true
		}
		for i, rhs := range as.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok {
				continue
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Compute" {
				continue
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "featurecoverage" {
				continue
			}
			if id, ok := as.Lhs[i].(*ast.Ident); ok {
				vars[id.Name] = true
			}
		}
		return true
	})
	return vars
}

func isACCoverageField(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "ACCoverage"
}

func isACCoverageElem(e ast.Expr) bool {
	ix, ok := e.(*ast.IndexExpr)
	return ok && isACCoverageField(ix.X)
}

func isMakeCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := call.Fun.(*ast.Ident)
	return ok && id.Name == "make"
}

// isLenStubsOf reports whether e is len(<v>[<k>].Stubs) with <v> in vars.
func isLenStubsOf(e ast.Expr, vars map[string]bool) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "len" {
		return false
	}
	sel, ok := call.Args[0].(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Stubs" {
		return false
	}
	ix, ok := sel.X.(*ast.IndexExpr)
	if !ok {
		return false
	}
	v, ok := ix.X.(*ast.Ident)
	return ok && vars[v.Name]
}

// TestBuildProjection_ACCoverageMatchesFeatureCoverage is the differential
// half of ac-1--behavioral: buildProjection's ACCoverage must equal
// featurecoverage.Compute's own stub-half count for the SAME frontmatter,
// on both the plain fixture and the duplicate-entry fixture — the case
// that distinguishes "delegates to the shared function" from a
// re-implementation that merely produces the same TOTAL on the simple
// case. The stub input is featurecoverage.StubDecls, the shared
// stub-to-declaration step buildProjection itself calls.
func TestBuildProjection_ACCoverageMatchesFeatureCoverage(t *testing.T) {
	tests := []struct {
		name string
		fm   *artifact.SpecFrontmatter
	}{
		{"plain fixture", mustDecodeSpecForTest(t, scopingProjectionFixtureSpec)},
		{"duplicate-entry fixture", dupEntryStubFrontmatter()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := buildProjectionFM(tt.name, tt.fm, nil, nil, nil, nil, modeReadOnly)
			if err != nil {
				t.Fatalf("buildProjectionFM: %v", err)
			}
			ids := make([]string, len(tt.fm.AcceptanceCriteria))
			for i, ac := range tt.fm.AcceptanceCriteria {
				ids[i] = ac.ID
			}
			cov := featurecoverage.Compute(ids, featurecoverage.StubDecls(tt.fm.Stubs), nil)
			for _, id := range ids {
				want := len(cov[id].Stubs)
				if got := p.ACCoverage[id]; got != want {
					t.Errorf("ACCoverage[%s] = %d, want featurecoverage.Compute's %d", id, got, want)
				}
			}
		})
	}
}
