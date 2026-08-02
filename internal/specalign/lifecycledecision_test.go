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
// FIX ROUND 1 REWRITE (task-6c re-review findings 2/3): the original
// version gated detection on PROVING a compared value's raw provenance
// (a chain back to artifact.DecodeSpec or an artifact.Status-typed
// declaration). The reviewer's own probe table proved that gate blind to
// several real reintroduction shapes — a nested selector (`s.FM.Status`,
// the exact form internal/residue/activespecs.go's own doc comment
// names as the shape the Task 6a migration removed), a switch on a
// nested selector, a range-loop variable's later `.Status` access, a
// map-range VALUE (as opposed to an indexed lookup), a `string(...)`-cast
// launder, and a comparand resolving through a same-file constant rather
// than a bare literal. The controller's decision: INVERT to fail-closed.
// The scanner no longer tries to prove a compared value IS raw before
// flagging it; it flags every `.Status` selector (any depth, any base
// type) used in a comparison/switch against one of the four persisted
// literals — or a same-file constant equal to one — and any value
// provably DERIVED from such a selector in the same function (a
// `string(...)` cast, a direct alias, or a map/slice-of-artifact.Status
// index/range value). What used to be a type-provenance GATE is now only
// used to widen that derivation tracking (container/param typing); it is
// never used to silently exempt a hit. Every real display consumer this
// broadening now (correctly, per the reviewer) catches is handled by an
// explicit, rationale-carrying allowlist entry instead — see
// lifecycleDecisionAllowlist below.
//
// Mechanical detection, mirroring vocabprose_test.go's own established
// idiom (single-file go/ast parse, per-file mechanical rules, disclosed
// limits rather than an adversarial-proof claim):
//
//   - FLAGGABLE (the thing being compared): any *ast.SelectorExpr whose
//     field name is exactly "Status", at ANY nesting depth and off ANY
//     base expression (s.Status, s.FM.Status, req.spec.Status, ...) —
//     unconditionally, never gated on the base's declared/inferred type.
//     Also flaggable: a same-function local identifier the scanner can
//     show, via simple sequential env tracking (mirroring the file's
//     prior version), was assigned directly from a flaggable expression,
//     from `string(<flaggable expression>)`, from indexing a
//     map/slice-of-artifact.Status, or from ranging over one (the VALUE
//     variable, not the key) — plus, unchanged from before, a function
//     parameter/receiver/local var explicitly typed artifact.Status
//     (refindex/status.go's mapStatusGroup(status artifact.Status)
//     shape, where the flaggable value is the bare parameter, no
//     selector at all).
//   - TARGET (what it's compared against): one of the four persisted
//     feature/story lifecycle literals — draft, accepted-pending-build,
//     closed, superseded — spelled either as a bare string literal, OR
//     as a same-FILE constant identifier (`const X = "closed"` or
//     `const X SomeType = "closed"`) whose own declared value resolves
//     to one of those four strings. Cross-package/qualified constants
//     (`pkg.SomeConst`) are NOT resolved — a disclosed limit, below.
//   - A comparison (==, !=) or switch case naming a TARGET against a
//     FLAGGABLE operand is a violation.
//   - internal/artifact (schema-compatibility validation) and internal/
//     specstate (the projection itself) are excluded WHOLESALE (skip
//     directories, never itemized) — the brief's own first two named
//     exceptions.
//   - Every other hit is either a NAMED allowlist entry (below) or a
//     reported violation. Allowlist entries come in two shapes (fix
//     round 1 finding 3): LINE-EXACT (file + line), reserved for the
//     TEMPORARY accept.go/supersede.go legacy-mutation entries where
//     precision is deliberately the point (their removal — the next
//     task's own retirement of the legacy gate — must visibly SHRINK the
//     table, never silently widen it via a stale function-scope match);
//     and FUNCTION-SCOPED (file + function/method name), used for every
//     other adjudicated exception — a legitimately-total mapping
//     function (mapStatusGroup) or a display/render function (the
//     board's own status-gated affordance checks, lint's status-in-path
//     validators) whose own internal line numbers shift on any ordinary,
//     unrelated edit; pinning those to an exact line makes the audit a
//     permanent tripwire on code nobody actually regressed. A
//     function-scoped entry is stale (and reported, same as a line-exact
//     one) when its named function no longer contains ANY flagged site —
//     never silently kept once its own reason for existing has moved or
//     been removed.
//
// Disclosed limits (this scanner is a ratchet against regression at the
// sites this migration actually touched, never an adversarial-proof
// gate — same posture as vocabprose_test.go's own file doc comment):
//
//   - CROSS-FUNCTION AND CROSS-PACKAGE LAUNDERS remain entirely
//     invisible: a `.Status` selector's value passed as a plain string
//     ARGUMENT into another function — in another file, another package,
//     or both — where the actual comparison happens is never traced.
//     This is exactly fix round 1 finding 1's own shape:
//     cmd/verdi/designfromstub.go passed `string(spec.Status)` as an
//     argument to internal/stubinstantiate.Instantiate, which itself
//     passes it on to SealedFeatureWallGuard's `status string` parameter,
//     which is where `status != "accepted-pending-build"` actually lived
//     — three function calls and a package boundary away from the
//     selector. No single-file scanner can trace that without a whole-
//     program call graph, which this audit deliberately does not build
//     (CLAUDE.md: no new dependencies — a real interprocedural analysis
//     would need golang.org/x/tools/go/{ssa,callgraph}, not stdlib
//     go/ast). This is WHY finding 1's fix is a CODE change (resolving
//     the effective status at the CLI call site, mirroring the board's
//     own already-correct call site) rather than a scanner enhancement:
//     the scanner cannot see that class of defect and a future one just
//     like it would again need a human (or a routed review finding) to
//     catch, not this audit.
//   - Same-file scope for BOTH the env-derivation tracking (a shadowed
//     re-declaration of the same name inside a nested block is not
//     modeled distinctly from its outer binding) and the constant table
//     (a constant declared in a DIFFERENT file of the same package is
//     invisible here, since this scanner walks and parses one file at a
//     time, mirroring vocabprose_test.go's own per-file architecture).
//   - Qualified (cross-package) constant comparands, e.g. `pkg.
//     SomeStatusConst`, are never resolved to their underlying string —
//     only a bare identifier looked up in the SAME file's own const
//     table.
//   - An aliased `artifact` import defeats the artifact.Status/
//     map[...]artifact.Status TYPE recognition this scanner still uses
//     for parameter/var/container seeding (though, per the fail-closed
//     inversion above, that recognition now only WIDENS what gets
//     flagged — a base selector like `x.Status` is caught regardless of
//     whether `x`'s own type was ever recognized at all).
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

// lifecycleFlag classifies an identifier's DERIVATION within one function
// body's local table (lifecycleFlagNone is the zero value: no recognized
// derivation, never itself flaggable — though a bare *ast.SelectorExpr
// named Status is ALWAYS flaggable regardless of this table; see the file
// doc comment's fail-closed inversion).
type lifecycleFlag int

const (
	lifecycleFlagNone lifecycleFlag = iota
	// lifecycleFlagStatus: this identifier's value was derived from a
	// flaggable Status expression (a direct alias, a string(...) cast, or
	// an index/range value out of a map/slice-of-artifact.Status), or is
	// itself explicitly typed artifact.Status (a parameter/var).
	lifecycleFlagStatus
	// lifecycleFlagContainer: this identifier is a map or slice whose
	// element type is artifact.Status — indexing or ranging over it
	// produces a lifecycleFlagStatus value.
	lifecycleFlagContainer
)

// TestLifecycleFlagNone_IsTheZeroValue pins lifecycleFlagNone's own
// contract (the doc comment above it): every unrecognized identifier in
// the classifier's per-function env map reads back as lifecycleFlagNone
// purely through Go's map zero-value semantics (env[name] on a missing
// key) — the code never spells the identifier out at a call site, so
// nothing else in this file exercises it directly. Fix round 1 (unrelated
// to this file's own diff — golangci-lint's `unused` flags any symbol not
// otherwise referenced, and this repo's policy is "unused code is
// deleted, not silenced"): this test is the reference, proving the
// zero-value contract rather than merely asserting it in prose.
func TestLifecycleFlagNone_IsTheZeroValue(t *testing.T) {
	var env map[string]lifecycleFlag
	if got := env["unrecognized"]; got != lifecycleFlagNone {
		t.Fatalf("env[missing] = %v, want lifecycleFlagNone (%v) — the classifier's own fail-closed default", got, lifecycleFlagNone)
	}
	if lifecycleFlagNone == lifecycleFlagStatus || lifecycleFlagNone == lifecycleFlagContainer {
		t.Fatalf("lifecycleFlagNone (%v) must be distinct from every recognized flag (status=%v, container=%v)", lifecycleFlagNone, lifecycleFlagStatus, lifecycleFlagContainer)
	}
}

// lifecycleViolation is one raw feature/story lifecycle-status decision
// site: a comparison or switch case, at its own source line and enclosing
// function, naming one of lifecycleStatusLiterals (bare or const-
// resolved) against a flaggable Status expression.
type lifecycleViolation struct {
	File     string // module-root-relative, slash-separated
	Line     int
	Func     string   // enclosing function/method name (FuncDecl.Name.Name)
	Literals []string // the offending literal(s) this line names, sorted
	Snippet  string   // the enclosing line's trimmed source text, for the report
}

// isArtifactStatusType reports whether t is exactly the artifact.Status
// selector type — used only to SEED param/var/container derivation
// tracking (widening what counts as flaggable), never to gate a bare
// Status selector (fix round 1's own inversion).
func isArtifactStatusType(t ast.Expr) bool {
	sel, ok := t.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == "artifact" && sel.Sel.Name == "Status"
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

// lifecycleFuncEnv is one function body's derivation table: identifier
// name -> lifecycleFlag, seeded from parameter/receiver/local-var types
// and updated, in source order, as assignments/declarations/range
// statements are walked (ast.Inspect's pre-order traversal visits a
// block's statements in source order, sufficient for the straight-line
// code every real site in this tree uses — see the file doc comment's
// disclosed limits).
type lifecycleFuncEnv map[string]lifecycleFlag

func (env lifecycleFuncEnv) seedFromType(names []*ast.Ident, typ ast.Expr) {
	var k lifecycleFlag
	switch {
	case isArtifactStatusType(typ):
		k = lifecycleFlagStatus
	case isStatusContainerType(typ):
		k = lifecycleFlagContainer
	default:
		return
	}
	for _, n := range names {
		if n.Name != "_" {
			env[n.Name] = k
		}
	}
}

// isFlaggable reports whether expr is a FLAGGABLE Status expression: any
// `.Status` selector at any depth (unconditional — the fail-closed
// inversion's own core rule), a same-function identifier this env has
// derived as lifecycleFlagStatus, a `string(<flaggable>)` cast, an index
// into a lifecycleFlagContainer, or a parenthesized flaggable expression.
func (env lifecycleFuncEnv) isFlaggable(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.SelectorExpr:
		return v.Sel.Name == "Status"
	case *ast.Ident:
		return env[v.Name] == lifecycleFlagStatus
	case *ast.IndexExpr:
		if base, ok := v.X.(*ast.Ident); ok {
			return env[base.Name] == lifecycleFlagContainer
		}
		return false
	case *ast.CallExpr:
		if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "string" && len(v.Args) == 1 {
			return env.isFlaggable(v.Args[0])
		}
		return false
	case *ast.ParenExpr:
		return env.isFlaggable(v.X)
	}
	return false
}

// isContainerExpr reports whether expr is a map/slice-of-artifact.Status
// value: a lifecycleFlagContainer identifier, or an inline composite
// literal of that type (a range statement's own source expression may be
// either shape).
func (env lifecycleFuncEnv) isContainerExpr(expr ast.Expr) bool {
	switch v := expr.(type) {
	case *ast.Ident:
		return env[v.Name] == lifecycleFlagContainer
	case *ast.CompositeLit:
		return v.Type != nil && isStatusContainerType(v.Type)
	case *ast.ParenExpr:
		return env.isContainerExpr(v.X)
	}
	return false
}

// resolveTargetInSet resolves expr to a lifecycleStatusLiterals member,
// either directly (a bare string literal) or through consts (a same-file
// constant declaration whose own value is that literal — the "package-
// const comparands" shape). Returns ("", false) for anything else,
// INCLUDING a literal or constant that resolves fine but names a value
// outside the four-member set (a different kind's own vocabulary word,
// or a component-only value like "active") — that is what keeps this
// scanner from flagging every kind's own status vocabulary, not any
// provenance gate.
func resolveTargetInSet(expr ast.Expr, consts map[string]string) (string, bool) {
	var s string
	switch v := expr.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}
		unquoted, err := strconv.Unquote(v.Value)
		if err != nil {
			return "", false
		}
		s = unquoted
	case *ast.Ident:
		resolved, ok := consts[v.Name]
		if !ok {
			return "", false
		}
		s = resolved
	default:
		return "", false
	}
	if !lifecycleStatusLiterals[s] {
		return "", false
	}
	return s, true
}

// fileConstTable collects every same-file, string-VALUED top-level const
// declaration (`const X = "y"` or `const X SomeType = "y"`, singly or
// inside a `const ( ... )` block) into name -> value, for
// resolveTargetInSet's "package-const comparands" case. Deliberately
// same-file only (the file doc comment's own disclosed limit) — this
// scanner parses and walks one file at a time.
func fileConstTable(f *ast.File) map[string]string {
	consts := map[string]string{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != len(vs.Values) {
				continue
			}
			for i, name := range vs.Names {
				bl, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || bl.Kind != token.STRING {
					continue
				}
				if s, err := strconv.Unquote(bl.Value); err == nil {
					consts[name.Name] = s
				}
			}
		}
	}
	return consts
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
func scanLifecycleDecisionsInFunc(rel string, fset *token.FileSet, fn *ast.FuncDecl, consts map[string]string, lines []string) []lifecycleViolation {
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
	funcName := ""
	if fn.Name != nil {
		funcName = fn.Name.Name
	}

	var out []lifecycleViolation
	record := func(pos token.Pos, literals []string) {
		p := fset.Position(pos)
		out = append(out, lifecycleViolation{File: rel, Line: p.Line, Func: funcName, Literals: literals, Snippet: lineSnippet(lines, p.Line)})
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
			switch {
			case env.isFlaggable(v.Rhs[0]):
				env[id.Name] = lifecycleFlagStatus
			case env.isContainerExpr(v.Rhs[0]):
				env[id.Name] = lifecycleFlagContainer
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
		case *ast.RangeStmt:
			if v.Value != nil && env.isContainerExpr(v.X) {
				if id, ok := v.Value.(*ast.Ident); ok && id.Name != "_" {
					env[id.Name] = lifecycleFlagStatus
				}
			}
		case *ast.BinaryExpr:
			if v.Op != token.EQL && v.Op != token.NEQ {
				return true
			}
			if env.isFlaggable(v.X) {
				if lit, inSet := resolveTargetInSet(v.Y, consts); inSet {
					record(v.Pos(), []string{lit})
				}
			} else if env.isFlaggable(v.Y) {
				if lit, inSet := resolveTargetInSet(v.X, consts); inSet {
					record(v.Pos(), []string{lit})
				}
			}
		case *ast.SwitchStmt:
			if v.Tag == nil || !env.isFlaggable(v.Tag) || v.Body == nil {
				return true
			}
			for _, stmt := range v.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				var hits []string
				for _, ce := range cc.List {
					if lit, inSet := resolveTargetInSet(ce, consts); inSet {
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
	consts := fileConstTable(f)

	byLine := map[int]*lifecycleViolation{}
	var order []int
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		for _, v := range scanLifecycleDecisionsInFunc(rel, fset, fn, consts, lines) {
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
// Exactly one of Line/Func is set (fix round 1 finding 3): Line (with
// Func == "") is LINE-EXACT — reserved for the TEMPORARY accept.go/
// supersede.go entries, where line-level precision is deliberately the
// point (their scheduled removal must visibly SHRINK this table). Func
// (with Line == 0) is FUNCTION-SCOPED — every other entry: a legitimately-
// total mapping function, or a display/render function, whose own
// internal line numbers are expected to move on ordinary, unrelated
// edits; pinning those to a line would make the audit a permanent
// tripwire on code nobody actually regressed.
type lifecycleAllowEntry struct {
	File      string
	Line      int    // line-exact match; 0 means "use Func instead"
	Func      string // function-scoped match; "" means "use Line instead"
	Rationale string
}

// matches reports whether v is covered by e.
func (e lifecycleAllowEntry) matches(v lifecycleViolation) bool {
	if e.File != v.File {
		return false
	}
	if e.Func != "" {
		return e.Func == v.Func
	}
	return e.Line == v.Line
}

// lifecycleDecisionAllowlist is the Task 6 brief step 5's own named
// allowlist: every raw feature/story lifecycle-status decision this audit
// finds outside internal/artifact and internal/specstate, each with its
// own adjudicated reason. Task 6c report §"the complete allowlist with
// rationale per entry" mirrors this table.
var lifecycleDecisionAllowlist = []lifecycleAllowEntry{
	// cmd/verdi/accept.go's legacy mutation path (the status: draft ->
	// accepted-pending-build flip, gated on reading that same raw field)
	// and cmd/verdi/supersede.go (the accepted-pending-build -> superseded
	// predecessor flip, invoked FROM WITHIN accept's own ritual — supersede.go
	// had no independent entry point) carried the five TEMPORARY line-exact
	// entries this table used to hold here. Task 7 (docs/superpowers/specs/
	// 2026-08-01-merge-signals-spec-acceptance-design.md) retires both in
	// the same commit — supersede.go's predecessor flip was never reachable
	// except through accept's own ritual, so the two retirements are one
	// atomic behavior change, not two — so all five raw decisions are gone
	// from the source, not merely re-routed, removed here rather than left
	// dangling (BINDING: never left dangling, never widened by a stale
	// match).

	// internal/residue/patternb.go archiveSpecClosedAt: a deliberately
	// preserved raw `status: closed` read, NOT migrated onto specstate —
	// see the disposition comment on archiveSpecClosedAt itself
	// (patternb.go, its own doc comment's final paragraph, added fix
	// round 1 finding 4 — findPatternB's doc comment above it carries the
	// original rationale too, but archiveSpecClosedAt now carries its own
	// copy so a reader of this function alone sees it): routing this
	// specific check through specstate would silently drop
	// archiveSpecClosedAt's own malformed-archive hard-error behavior
	// (TestArchiveSpecClosedAt_Negative), which an author would want
	// disclosed, not silently reclassified as "not yet realized".
	// FUNCTION-SCOPED: archiveSpecClosedAt is short and single-purpose,
	// but scoping it by function rather than line is still strictly safer
	// (fix round 1 finding 3's own uniform rule) and
	// TestLifecycleDecisionAllowlist_PatternBDispositionComment below
	// independently verifies the disposition text is still present.
	{File: "internal/residue/patternb.go", Func: "archiveSpecClosedAt", Rationale: "archiveSpecClosedAt: raw status: closed read preserved deliberately for malformed-archive hard-error detection; disposition recorded in the function's own doc comment"},

	// cmd/verdi/blastradius.go computeBlastRadius: this entry's own raw
	// decision is gone from the source — Fix round 1 (Task 7, docs/
	// superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md)
	// deletes blastradius.go outright (golangci-lint's `unused`: its sole
	// caller, accept.go's own retired ritual, no longer calls it, and no
	// other production caller exists). Removed here in the same fix as the
	// deletion, never left dangling.

	// internal/refindex/status.go mapStatusGroup: COMPONENT-class status
	// logic, not a feature/story decision — Task 6a narrowed this
	// function's only live caller (refindex.go's computeDefaultBranchEntries)
	// to class: component entries exclusively; the switch's feature/story-
	// shaped cases are retained defensively (the function stays total over
	// every artifact.Status value) but are never reached by a feature/story
	// spec in production. See the function's own doc comment for the full
	// disposition. The Task 6 brief itself names this exact file as the
	// "do not flag component handling" example. FUNCTION-SCOPED (fix
	// round 1 finding 3): this function is a legitimately-TOTAL mapping
	// over every artifact.Status value by charter — line-exact pins here
	// were themselves finding 3's own defect (a permanent tripwire on any
	// innocent edit to the function).
	{File: "internal/refindex/status.go", Func: "mapStatusGroup", Rationale: "mapStatusGroup: component-class status mapping, Task 6a narrowed to component-only callers (see in-file doc comment); legitimately-total over every artifact.Status value by charter"},

	// internal/workbench/boardspec.go loadBoard: proj.Status here is
	// EFFECTIVE state already resolved through specstate two lines
	// earlier in this SAME function (`string(st.ArtifactStatus())`,
	// boardspec.go's own board-mode/status-resolution block) and fed into
	// buildProjection as a plain value — a downstream DISPLAY consumer of
	// an already-migrated value, gating whether the creation-form fields
	// get attached, never a raw persisted-field read. Verified by hand
	// (task-6c fix round 1 report): loadBoard's own resolveState call
	// (boardspec.go, a few lines above) is the ONLY specstate resolution
	// in this function; proj.Status traces to its return value, not to
	// raw frontmatter. FUNCTION-SCOPED.
	{File: "internal/workbench/boardspec.go", Func: "loadBoard", Rationale: "proj.Status is EFFECTIVE state (string(st.ArtifactStatus())) resolved earlier in this same function via specstate; gates whether the creation-form fields are attached — display/affordance consumption of an already-migrated value, not a raw decision"},

	// internal/workbench/boardspecrender.go renderBoardRegion and
	// renderBoardDialogs: p.Status is BoardProjection.Status — declared
	// `Status string` (board.go), populated by buildProjection from the
	// effective status boardspec.go's loadBoard passes in (see the
	// loadBoard entry above) — never re-decoded here. Both functions are
	// pure HTML renderers (no I/O, no git, no artifact.DecodeSpec call
	// anywhere in this file) gating a card's Instantiate affordance and
	// the confirmation dialog's own attachment on the ALREADY-RESOLVED
	// projection field — display/affordance consumption, not a raw
	// decision. FUNCTION-SCOPED (two entries: the affordance gate and the
	// dialog-attachment gate are two separate functions).
	{File: "internal/workbench/boardspecrender.go", Func: "renderBoardRegion", Rationale: "p.Status is BoardProjection.Status, populated upstream from specstate's ArtifactStatus() (see boardspec.go's loadBoard entry above); gates the stub card's Instantiate affordance — pure HTML rendering, no re-decode"},
	{File: "internal/workbench/boardspecrender.go", Func: "renderBoardDialogs", Rationale: "p.Status is BoardProjection.Status, populated upstream from specstate's ArtifactStatus() (see boardspec.go's loadBoard entry above); gates whether the stub-instantiate confirmation dialog is attached — pure HTML rendering, no re-decode"},

	// internal/lint/vl002.go checkSpecPath: d.Status is lint.Document.
	// Status — declared `Status string` (document.go), populated by
	// walk.go's decodeDocument via an explicit `string(fmv.Status)` cast
	// at the ONE decode site (never re-decoded per-rule). VL-002 is a
	// STRUCTURAL self-consistency check — does a spec's OWN claimed
	// status agree with which directory (active/ vs archive/) it
	// physically lives in — not an acceptance decision: it never asks
	// "is this spec accepted", only "does this file's location match
	// what it says about itself", the same self-consistency register
	// internal/artifact's own schema validation occupies, just
	// implemented as a lint rule instead. FUNCTION-SCOPED.
	{File: "internal/lint/vl002.go", Func: "checkSpecPath", Rationale: "d.Status is lint.Document.Status (plain string, cast from raw frontmatter at the one decode site in walk.go); VL-002 validates path/status self-consistency of the artifact's OWN claim, never an acceptance decision — the same register internal/artifact's schema validation occupies"},

	// internal/lint/vl004.go (vl004).Check: d.Status != "draft" is a
	// PRE-FILTER identifying which documents carry the LEGACY "draft"
	// claim worth cross-checking against Git — VL-004's own charter (its
	// file doc comment: "this rule's own job narrows to exactly one
	// thing: when a legacy status: draft document's exact bytes are
	// ALREADY reachable from the configured default branch, disclose the
	// compatibility reading"). The actual VERDICT is decided entirely by
	// the specstate.Result switch a few lines below this filter (result.
	// State), never by d.Status again — verified by reading the whole
	// function (task-6c fix round 1 report). FUNCTION-SCOPED.
	{File: "internal/lint/vl004.go", Func: "Check", Rationale: "d.Status != \"draft\" is a pre-filter selecting which documents get the legacy-compatibility check; the actual verdict is decided by specstate.Result.State a few lines below, never by d.Status again — VL-004's own documented charter"},
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

	seenEntry := map[int]bool{} // index into lifecycleDecisionAllowlist actually hit this run
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
				matchedIdx := -1
				for i, e := range lifecycleDecisionAllowlist {
					if e.matches(v) {
						matchedIdx = i
						break
					}
				}
				if matchedIdx >= 0 {
					seenEntry[matchedIdx] = true
					continue
				}
				unlisted = append(unlisted, fmt.Sprintf("%s:%d: raw feature/story lifecycle decision in func %s on %v — %q — not on lifecycleDecisionAllowlist; route through internal/specstate's projected Result instead of the persisted status: field, or add a named, rationale-carrying allowlist entry if this is a genuinely adjudicated exception", v.File, v.Line, v.Func, v.Literals, v.Snippet))
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

	// A stale allowlist entry — line-exact whose line no longer produces a
	// violation, or function-scoped whose named function no longer
	// contains ANY flagged site (fix round 1 finding 3's own staleness
	// rule) — must not silently carry forward: it would let the SAME
	// line/function slot be reused by an unrelated future decision
	// without anyone re-adjudicating it. Report every entry not hit this
	// run.
	for i, e := range lifecycleDecisionAllowlist {
		if !seenEntry[i] {
			loc := fmt.Sprintf("line %d", e.Line)
			if e.Func != "" {
				loc = "func " + e.Func
			}
			t.Errorf("lifecycleDecisionAllowlist entry %s:%s no longer matches a raw decision (rationale: %q) — the allowlist is now stale here: remove the entry (this list must shrink, never silently drift)", e.File, loc, e.Rationale)
		}
	}
}

// TestLifecycleDecisionAllowlist_PatternBDispositionComment independently
// verifies internal/residue/patternb.go's archiveSpecClosedAt still
// carries its own disposition comment (fix round 1 finding 4: the prior
// version of this test pointed at "patternb.go lines ~39-49", which is
// actually findPatternB's doc comment, not archiveSpecClosedAt's own —
// archiveSpecClosedAt now carries its own copy of the disposition, added
// in the same fix round) — the allowlist entry above ASSERTS that
// comment's continued presence; this test PROVES it, so a future edit
// that quietly deletes the comment while leaving the raw read in place
// still fails loud, not just silently.
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
	// archiveSpecClosedAt's OWN doc comment block (not findPatternB's,
	// its neighbor above) must carry the phrases — the actual fix round 1
	// finding 4 assertion, not just "somewhere in the file".
	idx := strings.Index(string(src), "func archiveSpecClosedAt(")
	if idx < 0 {
		t.Fatalf("%s: archiveSpecClosedAt function not found", path)
	}
	// Its doc comment is the contiguous "//" block immediately above —
	// findPatternB's own doc comment ends with a blank (non-"//") line
	// before archiveSpecClosedAt's begins, so walking backward from idx
	// to the nearest non-"//"-prefixed line isolates it.
	before := string(src)[:idx]
	lines := strings.Split(strings.TrimRight(before, "\n"), "\n")
	var ownComment []string
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, "//") {
			break
		}
		ownComment = append([]string{trimmed}, ownComment...)
	}
	// Joined with a SPACE, not a newline, and comment markers stripped: a
	// phrase this check looks for may legitimately word-wrap across two
	// "//" lines in the source (as "deliberately NOT" / "migrated onto"
	// does here) — a newline-joined comparison would falsely miss it.
	var normalized []string
	for _, l := range ownComment {
		normalized = append(normalized, strings.TrimSpace(strings.TrimPrefix(l, "//")))
	}
	ownText := strings.Join(normalized, " ")
	for _, want := range []string{"deliberately NOT migrated onto", "hard-error behavior"} {
		if !strings.Contains(ownText, want) {
			t.Errorf("archiveSpecClosedAt's OWN doc comment (immediately above its func line) does not contain %q — it must carry its own copy of the disposition, not rely on findPatternB's neighboring comment (fix round 1 finding 4); own comment:\n%s", want, ownText)
		}
	}
}

// TestScanLifecycleDecisions_Classifier is the scanner's own happy/
// negative table, proving both failure modes the audit exists for
// (mirroring TestScanVocabProse_Classifier's precedent): a bare `.Status`
// selector comparison and an artifact.Status-typed switch both RED with
// the correct file:line; a downstream DISPLAY consumer stays GREEN only
// when explicitly outside the four-literal target set (component's
// "active") or genuinely untyped/unrelated — proving the scanner
// distinguishes TARGETS, not provenance, per the fail-closed inversion.
// Every row through "package-const comparand" mirrors one row of the
// task-6c fix round 1 reviewer's own probe table — each MISSED row is
// now CAUGHT.
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
			name: "RED (reviewer probe): nested selector s.FM.Status",
			src: `package p
type spec struct{ Status string }
type wrapper struct{ FM *spec }
func f(s wrapper) bool {
	return s.FM.Status != "accepted-pending-build"
}
`,
			wantLines: []int{5},
		},
		{
			name: "RED (reviewer probe): switch on nested selector",
			src: `package p
type spec struct{ Status string }
type wrapper struct{ FM *spec }
func f(s wrapper) string {
	switch s.FM.Status {
	case "closed":
		return "terminal"
	}
	return "other"
}
`,
			wantLines: []int{6}, // the case clause's own line, not the switch statement's
		},
		{
			name: "RED (reviewer probe): range-variable provenance, .Status used after the range",
			src: `package p
import "github.com/jyang234/verdi/internal/artifact"
func f(specs []artifact.SpecFrontmatter) int {
	n := 0
	for _, fm := range specs {
		if fm.Status == "accepted-pending-build" {
			n++
		}
	}
	return n
}
`,
			wantLines: []int{6},
		},
		{
			name: "RED (reviewer probe): map[string]artifact.Status RANGE value (not indexed)",
			src: `package p
import "github.com/jyang234/verdi/internal/artifact"
func f(m map[string]artifact.Status) int {
	n := 0
	for _, st := range m {
		if st == "closed" {
			n++
		}
	}
	return n
}
`,
			wantLines: []int{6},
		},
		{
			name: "RED (reviewer probe): string(...)-cast launder",
			src: `package p
import "github.com/jyang234/verdi/internal/artifact"
func f(fm *artifact.SpecFrontmatter) bool {
	s := string(fm.Status)
	return s == "draft"
}
`,
			wantLines: []int{5},
		},
		{
			name: "RED (reviewer probe): package-const comparand",
			src: `package p
import "github.com/jyang234/verdi/internal/artifact"
const wantAccepted = "accepted-pending-build"
func f(fm *artifact.SpecFrontmatter) bool {
	return fm.Status == wantAccepted
}
`,
			wantLines: []int{5},
		},
		{
			name: "now caught by design: a display projection's plain string Status field, identical literal (the fail-closed inversion has no free pass by field type — the reviewer's own point)",
			src: `package p
type BoardProjection struct {
	Status string
}
func instantiable(p *BoardProjection) bool {
	return p.Status == "accepted-pending-build"
}
`,
			wantLines: []int{6},
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
			name: "GREEN: a plain string parameter with no local Status derivation at all (finding 1's own cross-function shape: this scanner cannot and does not see it — the code fix is what closes it)",
			src: `package p
func guard(status string) bool {
	return status != "accepted-pending-build"
}
`,
			wantLines: nil,
		},
		{
			name: "GREEN: a same-file const NOT in the four-literal set never matches",
			src: `package p
import "github.com/jyang234/verdi/internal/artifact"
const wantActive = "active"
func f(fm *artifact.SpecFrontmatter) bool {
	return fm.Status == wantActive
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

// TestLifecycleAllowEntry_Matches is the allowlist-matching predicate's
// own happy/negative unit table (fix round 1 finding 3's new function-
// scoped shape): a line-exact entry matches only its exact line in its
// exact file; a function-scoped entry matches ANY violation in its named
// function regardless of line, and never matches a same-named function in
// a DIFFERENT file, nor a different function in its own file.
func TestLifecycleAllowEntry_Matches(t *testing.T) {
	lineExact := lifecycleAllowEntry{File: "cmd/verdi/accept.go", Line: 131}
	funcScoped := lifecycleAllowEntry{File: "internal/refindex/status.go", Func: "mapStatusGroup"}

	tests := []struct {
		name  string
		entry lifecycleAllowEntry
		v     lifecycleViolation
		want  bool
	}{
		{"line-exact: exact file+line matches", lineExact, lifecycleViolation{File: "cmd/verdi/accept.go", Line: 131, Func: "runAccept"}, true},
		{"line-exact: same file, different line does not match", lineExact, lifecycleViolation{File: "cmd/verdi/accept.go", Line: 132, Func: "runAccept"}, false},
		{"line-exact: same line, different file does not match", lineExact, lifecycleViolation{File: "cmd/verdi/supersede.go", Line: 131, Func: "runAccept"}, false},
		{"func-scoped: same file+func matches regardless of line", funcScoped, lifecycleViolation{File: "internal/refindex/status.go", Line: 999, Func: "mapStatusGroup"}, true},
		{"func-scoped: same file, different func does not match", funcScoped, lifecycleViolation{File: "internal/refindex/status.go", Line: 30, Func: "effectiveStatusGroup"}, false},
		{"func-scoped: same func name, different file does not match", funcScoped, lifecycleViolation{File: "internal/other/status.go", Line: 30, Func: "mapStatusGroup"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.matches(tt.v); got != tt.want {
				t.Errorf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}
