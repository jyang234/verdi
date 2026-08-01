// The Task 6 brief's step 5 alignment test (merge-signaled spec
// acceptance, "Migrate indexes, residue, and workbench projections"):
// internal/specstate is now the ONE place every consumer routes feature/
// story lifecycle decisions through — no adapter reimplements
// reachability, and none trusts a persisted `status:` field alone. This
// file is the guard that keeps it that way: a source audit over every
// PRODUCTION .go file under cmd/ and internal/ for a raw feature/story
// lifecycle-status DECISION (a comparison or switch whose verdict a
// caller's control flow depends on) made directly off the persisted
// frontmatter field, rather than off specstate's projected Result.
//
// Mechanical detection, mirroring vocabprose_test.go's own established
// idiom (single-file go/ast parse, per-file mechanical rules, disclosed
// limits rather than an adversarial-proof claim):
//
//   - The scanner tracks, per function declaration, a small provenance
//     table seeded from parameter/receiver/local-var TYPES: a value typed
//     artifact.Status, or a *artifact.SpecFrontmatter/artifact.
//     SpecFrontmatter value's own .Status selector, or a value indexed out
//     of a map/slice whose element type is artifact.Status, or the direct
//     result of an artifact.DecodeSpec(...) call's .Status selector, are
//     all "raw" — the exact wire type spec frontmatter decodes to, before
//     any specstate resolution touches it.
//   - A raw-typed value compared (==, !=) against, or switched with a case
//     naming, one of the four persisted feature/story lifecycle values —
//     draft, accepted-pending-build, closed, superseded (internal/
//     specstate's own ArtifactStatus() enumerates exactly these four) — is
//     a violation: a decision made off the raw field.
//   - This is deliberately narrower than "every use of a field named
//     Status": a downstream projection struct (internal/workbench's
//     BoardProjection.Status, internal/dex's listItem.Status, internal/
//     lint's Document.Status, ...) is typed plain `string`, populated from
//     specstate's own resolved Result (or, for internal/lint, from an
//     explicit `string(fmv.Status)` cast at the ONE decode site,
//     internal/lint/walk.go) — never itself typed artifact.Status — so
//     comparisons against it are consuming an already-resolved value, not
//     deciding off the raw one, and are correctly not flagged. This is the
//     load-bearing distinction the whole audit turns on (see
//     TestScanLifecycleDecisions_Classifier's "display consumer" case).
//   - Component-class values ("active"), and every other kind's own status
//     vocabulary (diagram: proposed/accepted; ADR: accepted/superseded;
//     conflict: superseded/dismissed; annotation, waiver, evidence, ...)
//     are excluded by the same four-value literal set: "superseded" is
//     the only literal the feature/story set shares with another kind's
//     vocabulary (ADR, conflict), and every one of those lives inside
//     internal/artifact itself (the schema-validation package, allowed
//     wholesale below), so the shared literal never produces a false hit
//     on a different kind's decision elsewhere in the tree.
//   - Disclosed limits, same posture as vocabprose_test.go: single-
//     function-body scope (no cross-function/interprocedural tracing); an
//     aliased `artifact` import, or a value laundered through an
//     intermediate helper this scanner doesn't recognize (e.g. a wrapper
//     around DecodeSpec), would evade it. A ratchet against regression at
//     the sites this migration actually touched, not an adversarial-proof
//     gate.
package specalign

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// lifecycleStatusLiterals are the four persisted feature/story spec
// lifecycle values (internal/specstate/state.go's ArtifactStatus: "Closed
// has no legacy equivalent [for Unproven]; it maps to ... one of the four
// vocabulary values"). "proposed" and "unproven" are specstate's own
// projected vocabulary, never a persisted frontmatter value, so they are
// deliberately not scanned for.
var lifecycleStatusLiterals = map[string]bool{
	"draft":                  true,
	"accepted-pending-build": true,
	"closed":                 true,
	"superseded":             true,
}

// lifecycleRawKind classifies an identifier's provenance within one
// function body's local table (lifecycleRawNone is the zero value: no
// recognized raw provenance, never flagged).
type lifecycleRawKind int

const (
	lifecycleRawNone lifecycleRawKind = iota
	// lifecycleRawSpec: a *artifact.SpecFrontmatter (or artifact.
	// SpecFrontmatter) value — its own .Status selector is the raw wire
	// field.
	lifecycleRawSpec
	// lifecycleRawStatus: an artifact.Status-typed value itself (a bare
	// identifier, e.g. a function parameter or a map/slice element).
	lifecycleRawStatus
	// lifecycleRawStatusContainer: a map or slice whose element type is
	// artifact.Status — indexing it produces a lifecycleRawStatus value.
	lifecycleRawStatusContainer
)

// lifecycleViolation is one raw feature/story lifecycle-status decision
// site: a comparison or switch case, at its own source line, naming one
// of lifecycleStatusLiterals against a raw-typed value.
type lifecycleViolation struct {
	File     string // module-root-relative, slash-separated
	Line     int
	Literals []string // the offending literal(s) this line names, sorted
	Snippet  string   // the enclosing line's trimmed source text, for the report
}

// isArtifactStatusType reports whether t is exactly the artifact.Status
// selector type.
func isArtifactStatusType(t ast.Expr) bool {
	sel, ok := t.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "artifact" && sel.Sel.Name == "Status"
}

// isSpecFrontmatterType reports whether t is artifact.SpecFrontmatter or
// *artifact.SpecFrontmatter.
func isSpecFrontmatterType(t ast.Expr) bool {
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	sel, ok := t.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "artifact" && sel.Sel.Name == "SpecFrontmatter"
}

// isStatusContainerType reports whether t is map[K]artifact.Status or
// []artifact.Status.
func isStatusContainerType(t ast.Expr) bool {
	switch tt := t.(type) {
	case *ast.MapType:
		return isArtifactStatusType(tt.Value)
	case *ast.ArrayType:
		return isArtifactStatusType(tt.Elt)
	}
	return false
}

// isDecodeSpecCall reports whether call is a call to artifact.DecodeSpec —
// the one decoder whose result's own .Status field is the raw feature/
// story wire value (every other kind's Decode* function produces a
// DIFFERENT kind's status vocabulary, out of this audit's scope by
// construction — see the file doc comment's literal-set argument).
func isDecodeSpecCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "artifact" && sel.Sel.Name == "DecodeSpec"
}

// lifecycleFuncEnv is one function body's provenance table: identifier
// name -> lifecycleRawKind, seeded from parameter/receiver types and
// updated, in source order, as assignments and var declarations are
// walked (ast.Inspect's pre-order traversal visits a block's statements in
// source order, which is sufficient for the straight-line code every real
// site in this tree uses — see the file doc comment's disclosed limits).
type lifecycleFuncEnv map[string]lifecycleRawKind

func (env lifecycleFuncEnv) seedFromType(names []*ast.Ident, typ ast.Expr) {
	var k lifecycleRawKind
	switch {
	case isArtifactStatusType(typ):
		k = lifecycleRawStatus
	case isSpecFrontmatterType(typ):
		k = lifecycleRawSpec
	case isStatusContainerType(typ):
		k = lifecycleRawStatusContainer
	default:
		return
	}
	for _, n := range names {
		if n.Name != "_" {
			env[n.Name] = k
		}
	}
}

// kindOf classifies expr's raw provenance against env's current table.
func (env lifecycleFuncEnv) kindOf(expr ast.Expr) lifecycleRawKind {
	switch v := expr.(type) {
	case *ast.Ident:
		return env[v.Name]
	case *ast.SelectorExpr:
		if v.Sel.Name != "Status" {
			return lifecycleRawNone
		}
		if base, ok := v.X.(*ast.Ident); ok && env[base.Name] == lifecycleRawSpec {
			return lifecycleRawStatus
		}
		return lifecycleRawNone
	case *ast.IndexExpr:
		if base, ok := v.X.(*ast.Ident); ok && env[base.Name] == lifecycleRawStatusContainer {
			return lifecycleRawStatus
		}
		return lifecycleRawNone
	case *ast.CallExpr:
		if isDecodeSpecCall(v) {
			return lifecycleRawSpec
		}
		return lifecycleRawNone
	}
	return lifecycleRawNone
}

// stringLiteralValue returns lit's unquoted string value and whether it is
// a member of lifecycleStatusLiterals.
func stringLiteralValue(expr ast.Expr) (string, bool) {
	bl, ok := expr.(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return "", false
	}
	return s, lifecycleStatusLiterals[s]
}

// lineSnippet returns lines[n-1] trimmed, "" if out of range.
func lineSnippet(lines []string, n int) string {
	if n < 1 || n > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[n-1])
}

// scanLifecycleDecisionsInFunc walks one function declaration's body for
// raw feature/story lifecycle-status decisions, updating env as it goes.
func scanLifecycleDecisionsInFunc(rel string, fset *token.FileSet, fn *ast.FuncDecl, lines []string) []lifecycleViolation {
	env := lifecycleFuncEnv{}
	if fn.Recv != nil {
		for _, f := range fn.Recv.List {
			env.seedFromType(f.Names, f.Type)
		}
	}
	if fn.Type.Params != nil {
		for _, f := range fn.Type.Params.List {
			env.seedFromType(f.Names, f.Type)
		}
	}
	if fn.Body == nil {
		return nil
	}

	var out []lifecycleViolation
	record := func(pos token.Pos, literals []string) {
		p := fset.Position(pos)
		out = append(out, lifecycleViolation{File: rel, Line: p.Line, Literals: literals, Snippet: lineSnippet(lines, p.Line)})
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			if len(v.Rhs) != 1 || len(v.Lhs) == 0 {
				return true
			}
			id, ok := v.Lhs[0].(*ast.Ident)
			if !ok || id.Name == "_" {
				return true
			}
			if k := env.kindOf(v.Rhs[0]); k != lifecycleRawNone {
				env[id.Name] = k
				return true
			}
			if cl, ok := v.Rhs[0].(*ast.CompositeLit); ok && cl.Type != nil && isStatusContainerType(cl.Type) {
				env[id.Name] = lifecycleRawStatusContainer
			}
		case *ast.DeclStmt:
			gd, ok := v.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return true
			}
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok && vs.Type != nil {
					env.seedFromType(vs.Names, vs.Type)
				}
			}
		case *ast.BinaryExpr:
			if v.Op != token.EQL && v.Op != token.NEQ {
				return true
			}
			if env.kindOf(v.X) == lifecycleRawStatus {
				if lit, inSet := stringLiteralValue(v.Y); inSet {
					record(v.Pos(), []string{lit})
				}
			} else if env.kindOf(v.Y) == lifecycleRawStatus {
				if lit, inSet := stringLiteralValue(v.X); inSet {
					record(v.Pos(), []string{lit})
				}
			}
		case *ast.SwitchStmt:
			if v.Tag == nil || env.kindOf(v.Tag) != lifecycleRawStatus || v.Body == nil {
				return true
			}
			for _, stmt := range v.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				var hits []string
				for _, ce := range cc.List {
					if lit, inSet := stringLiteralValue(ce); inSet {
						hits = append(hits, lit)
					}
				}
				if len(hits) > 0 {
					record(cc.Pos(), hits)
				}
			}
		}
		return true
	})
	return out
}

// scanLifecycleDecisions scans one production Go source file for raw
// feature/story lifecycle-status decisions, deduplicated and sorted by
// line (a single `if a || b` line naming two literals reports once, with
// both literals named).
func scanLifecycleDecisions(rel string, src []byte) ([]lifecycleViolation, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, rel, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", rel, err)
	}
	lines := strings.Split(string(src), "\n")

	byLine := map[int]*lifecycleViolation{}
	var order []int
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		for _, v := range scanLifecycleDecisionsInFunc(rel, fset, fn, lines) {
			if existing, ok := byLine[v.Line]; ok {
				existing.Literals = mergeSortedUnique(existing.Literals, v.Literals)
				continue
			}
			vCopy := v
			vCopy.Literals = mergeSortedUnique(nil, v.Literals)
			byLine[v.Line] = &vCopy
			order = append(order, v.Line)
		}
	}
	sort.Ints(order)
	out := make([]lifecycleViolation, 0, len(order))
	for _, ln := range order {
		out = append(out, *byLine[ln])
	}
	return out, nil
}

func mergeSortedUnique(existing, add []string) []string {
	seen := map[string]bool{}
	for _, s := range existing {
		seen[s] = true
	}
	out := append([]string{}, existing...)
	for _, s := range add {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// lifecycleAllowEntry is one adjudicated exception to the audit: a raw
// feature/story lifecycle-status decision this repo's own review process
// has already looked at and, for the stated reason, decided to keep.
// Matched by EXACT (File, Line): a future edit that moves the decision to
// a different line re-reds the audit, forcing a conscious re-adjudication
// rather than a silent carry-forward.
type lifecycleAllowEntry struct {
	File      string
	Line      int
	Rationale string
}

// lifecycleDecisionAllowlist is the Task 6 brief step 5's own named
// allowlist: every raw feature/story lifecycle-status decision this audit
// finds outside internal/artifact and internal/specstate (both allowed
// wholesale below, never itemized), each with its own adjudicated reason.
// Task 6c report §"the complete allowlist with rationale per entry" mirrors
// this table.
var lifecycleDecisionAllowlist = []lifecycleAllowEntry{
	// cmd/verdi/accept.go and cmd/verdi/supersede.go: the legacy mutation
	// paths that physically WRITE the status: draft -> accepted-pending-
	// build (and, supersede.go, accepted-pending-build -> superseded)
	// flip, gated on reading the SAME raw field they are about to flip
	// (accept refuses a non-draft spec; supersede refuses a predecessor
	// not already accepted-pending-build). TEMPORARY: retired by the NEXT
	// task (the task-6c brief: "the legacy gate... the NEXT task
	// retires"), at which point these entries are deleted — this list
	// SHRINKS, never grows, as that retirement lands.
	{File: "cmd/verdi/accept.go", Line: 131, Rationale: "legacy accept mutation path: refuses a non-draft spec before flipping it; TEMPORARY, removed with the next task's accept.go retirement"},
	{File: "cmd/verdi/accept.go", Line: 276, Rationale: "legacy accept mutation path: self-validates the just-written flip landed accepted-pending-build; TEMPORARY, removed with the next task's accept.go retirement"},
	{File: "cmd/verdi/supersede.go", Line: 143, Rationale: "legacy supersede mutation path: idempotent already-superseded short-circuit; TEMPORARY, removed with the next task's supersede.go retirement"},
	{File: "cmd/verdi/supersede.go", Line: 146, Rationale: "legacy supersede mutation path: refuses a predecessor not already accepted-pending-build before flipping it; TEMPORARY, removed with the next task's supersede.go retirement"},
	{File: "cmd/verdi/supersede.go", Line: 180, Rationale: "legacy supersede mutation path: self-validates the just-written flip landed superseded; TEMPORARY, removed with the next task's supersede.go retirement"},

	// internal/residue/patternb.go archiveSpecClosedAt: a deliberately
	// preserved raw `status: closed` read, NOT migrated onto specstate —
	// see the disposition comment on archiveSpecClosedAt itself (patternb.
	// go lines ~39-49): routing this specific check through specstate
	// would silently drop archiveSpecClosedAt's own malformed-archive
	// hard-error behavior (TestArchiveSpecClosedAt_Negative), which an
	// author would want disclosed, not silently reclassified as "not yet
	// realized". TestLifecycleDecisionAllowlist_PatternBDispositionComment
	// below independently verifies that in-file comment still exists.
	{File: "internal/residue/patternb.go", Line: 125, Rationale: "archiveSpecClosedAt: raw status: closed read preserved deliberately for malformed-archive hard-error detection; disposition recorded in-file on the function"},

	// cmd/verdi/blastradius.go:112: the rung-4 quorum label's own display
	// filter — "affected in-flight or closed stories" governs only which
	// AC-2 cascade rows get PRINTED under the two-code-owner label; it
	// blocks nothing (the file's own doc comment: "this file computes and
	// PRINTS the quorum label only; nothing here blocks acceptance").
	// Adjudicated deferred at Task 6c's own re-review rather than migrated
	// in this pass — display-scoped.
	{File: "cmd/verdi/blastradius.go", Line: 112, Rationale: "rung-4 quorum label display filter only (never blocks acceptance); adjudicated deferred, display-scoped"},

	// internal/refindex/status.go mapStatusGroup: COMPONENT-class status
	// logic, not a feature/story decision — Task 6a narrowed this
	// function's only live caller (refindex.go's computeDefaultBranchEntries)
	// to class: component entries exclusively; the switch's feature/story-
	// shaped cases are retained defensively (the function stays total over
	// every artifact.Status value) but are never reached by a feature/story
	// spec in production. See the function's own doc comment for the full
	// disposition. The Task 6 brief itself names this exact file as the
	// "do not flag component handling" example.
	{File: "internal/refindex/status.go", Line: 30, Rationale: "mapStatusGroup: component-class status mapping, Task 6a narrowed to component-only callers (see in-file doc comment)"},
	{File: "internal/refindex/status.go", Line: 32, Rationale: "mapStatusGroup: component-class status mapping, Task 6a narrowed to component-only callers (see in-file doc comment)"},
	{File: "internal/refindex/status.go", Line: 36, Rationale: "mapStatusGroup: component-class status mapping, Task 6a narrowed to component-only callers (see in-file doc comment)"},
}

// lifecycleAllowed reports whether (file, line) is on the named allowlist.
func lifecycleAllowed(file string, line int) (bool, string) {
	for _, e := range lifecycleDecisionAllowlist {
		if e.File == file && e.Line == line {
			return true, e.Rationale
		}
	}
	return false, ""
}

// lifecycleDecisionSkipDir reports directories the audit deliberately does
// not scan: internal/artifact (schema-compatibility validation — the
// brief's own first named exception, allowed WHOLESALE, never itemized)
// and internal/specstate (the projection itself — the brief's second named
// exception), plus testdata/node_modules (fixtures, never production code).
func lifecycleDecisionSkipDir(root, path string) bool {
	switch filepath.Base(path) {
	case "testdata", "node_modules":
		return true
	}
	switch path {
	case filepath.Join(root, "internal", "artifact"),
		filepath.Join(root, "internal", "specstate"):
		return true
	}
	return false
}

// TestLifecycleDecisionSourceAudit is the Task 6 brief step 5 gate: zero
// unlisted raw feature/story lifecycle-status decisions in production code
// under cmd/ and internal/, outside internal/artifact and internal/
// specstate. A hit not on lifecycleDecisionAllowlist fails loud, naming
// every offending file:line so a future adapter cannot silently
// reintroduce status-only acceptance.
//
// guide-claim: N/A (self-hosted repo audit, no guide-claims row — mirrors
// TestRepoHygieneNoTrackedCompiledBinaries's own precedent)
func TestLifecycleDecisionSourceAudit(t *testing.T) {
	root := verdiRepoRoot

	seen := map[string]bool{} // "file:line" allowlist entries actually hit this run
	var unlisted []string

	for _, tree := range []string{"cmd", "internal"} {
		err := filepath.Walk(filepath.Join(root, tree), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if lifecycleDecisionSkipDir(root, path) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			rel, rerr := filepath.Rel(root, path)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			violations, serr := scanLifecycleDecisions(rel, src)
			if serr != nil {
				return serr
			}
			for _, v := range violations {
				if ok, _ := lifecycleAllowed(v.File, v.Line); ok {
					seen[fmt.Sprintf("%s:%d", v.File, v.Line)] = true
					continue
				}
				unlisted = append(unlisted, fmt.Sprintf("%s:%d: raw feature/story lifecycle decision on %v — %q — not on lifecycleDecisionAllowlist; route through internal/specstate's projected Result instead of the persisted status: field, or add a named, rationale-carrying allowlist entry if this is a genuinely adjudicated exception", v.File, v.Line, v.Literals, v.Snippet))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", tree, err)
		}
	}

	sort.Strings(unlisted)
	for _, msg := range unlisted {
		t.Error(msg)
	}

	// A stale allowlist entry (its file:line no longer produces a
	// violation — the decision moved, was removed, or was migrated onto
	// specstate) must not silently carry forward: it would let the SAME
	// file:line slot be reused by an unrelated future decision without
	// anyone re-adjudicating it. Report every entry not hit this run.
	for _, e := range lifecycleDecisionAllowlist {
		key := fmt.Sprintf("%s:%d", e.File, e.Line)
		if !seen[key] {
			t.Errorf("lifecycleDecisionAllowlist entry %s no longer matches a raw decision at that line (rationale: %q) — the allowlist is now stale here: remove the entry (this list must shrink, never silently drift)", key, e.Rationale)
		}
	}
}

// TestLifecycleDecisionAllowlist_PatternBDispositionComment independently
// verifies internal/residue/patternb.go still carries archiveSpecClosedAt's
// disposition comment (the task-6c brief's own instruction: "verify that
// in-file disposition comment still exists and reference it") — the
// allowlist entry above ASSERTS that comment's continued presence; this
// test PROVES it, so a future edit that quietly deletes the comment while
// leaving the raw read in place still fails loud, not just silently.
func TestLifecycleDecisionAllowlist_PatternBDispositionComment(t *testing.T) {
	path := filepath.Join(verdiRepoRoot, "internal", "residue", "patternb.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	for _, want := range []string{
		"deliberately NOT migrated onto",
		"hard-error behavior",
		"TestArchiveSpecClosedAt_Negative",
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("%s no longer contains %q — archiveSpecClosedAt's disposition comment (the reason its raw status: closed read is allowlisted) appears to have been removed or reworded; re-verify the disposition still holds and update lifecycleDecisionAllowlist's rationale to match", path, want)
		}
	}
}

// TestScanLifecycleDecisions_Classifier is the scanner's own happy/
// negative table, proving both failure modes the audit exists for
// (mirroring TestScanVocabProse_Classifier's precedent): a raw DecodeSpec-
// derived .Status comparison and a raw artifact.Status-typed switch both
// RED with the correct file:line; a downstream DISPLAY consumer (a plain
// string Status field on some other struct — the shape every migrated
// workbench/refindex/dex projection now has) comparing the identical
// literal stays GREEN, proving the audit distinguishes decisions from
// display rather than merely pattern-matching on the field name "Status";
// a raw artifact.Status compared against a literal OUTSIDE the feature/
// story set (component's "active", a different kind's own vocabulary) also
// stays GREEN.
func TestScanLifecycleDecisions_Classifier(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		wantLines []int
	}{
		{
			name: "RED: DecodeSpec-derived .Status compared to a raw literal",
			src: `package p
import "github.com/jyang234/verdi/internal/artifact"
func f(fm []byte) {
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return
	}
	if spec.Status != "draft" {
		return
	}
}
`,
			wantLines: []int{8},
		},
		{
			name: "RED: artifact.Status-typed switch",
			src: `package p
import "github.com/jyang234/verdi/internal/artifact"
func classify(status artifact.Status) string {
	switch status {
	case "accepted-pending-build":
		return "accepted"
	case "closed", "superseded":
		return "terminal"
	}
	return "other"
}
`,
			wantLines: []int{5, 7},
		},
		{
			name: "RED: artifact.Status map-indexed value compared",
			src: `package p
import "github.com/jyang234/verdi/internal/artifact"
func f(m map[string]artifact.Status, k string) bool {
	status := m[k]
	return status == "accepted-pending-build"
}
`,
			wantLines: []int{5},
		},
		{
			name: "GREEN: a display projection's plain string Status field, identical literal",
			src: `package p
type BoardProjection struct {
	Status string
}
func instantiable(p *BoardProjection) bool {
	return p.Status == "accepted-pending-build"
}
`,
			wantLines: nil,
		},
		{
			name: "GREEN: artifact.Status compared to a non-feature/story literal (component-only value)",
			src: `package p
import "github.com/jyang234/verdi/internal/artifact"
func f(status artifact.Status) bool {
	return status == "active"
}
`,
			wantLines: nil,
		},
		{
			name: "GREEN: DecodeSpec-derived value's OTHER field, not Status",
			src: `package p
import "github.com/jyang234/verdi/internal/artifact"
func f(fm []byte) bool {
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return false
	}
	return spec.Class == "feature"
}
`,
			wantLines: nil,
		},
		{
			name: "GREEN: a plain string parameter (already-resolved, laundered by the caller)",
			src: `package p
func guard(status string) bool {
	return status != "accepted-pending-build"
}
`,
			wantLines: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scanLifecycleDecisions("synth.go", []byte(tt.src))
			if err != nil {
				t.Fatalf("scanLifecycleDecisions: %v", err)
			}
			var gotLines []int
			for _, v := range got {
				gotLines = append(gotLines, v.Line)
			}
			if len(gotLines) != len(tt.wantLines) {
				t.Fatalf("scanLifecycleDecisions violations at lines %v, want %v (full: %+v)", gotLines, tt.wantLines, got)
			}
			for i, ln := range tt.wantLines {
				if gotLines[i] != ln {
					t.Fatalf("scanLifecycleDecisions violation[%d] line = %d, want %d (full: %+v)", i, gotLines[i], ln, got)
				}
			}
		})
	}
}
