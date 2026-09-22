package recovery

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// readOnlyGitxCalls is Task 2's own command-surface allow-list (Step 15),
// trimmed (2B-F11) to EXACTLY the gitx names facts.go, derive.go,
// schema.go, codec.go, and commandlog.go actually call — not Step 15's
// own superset (which additionally named DefaultBranch and WorktreeList,
// both read-only but unused here). apply.go itself calls no gitx
// function directly at all (its own two executors, branchcut.Unwind and
// reclaim.Apply, do the git work) — TestCommandSurface_ApplyExecutorAllowList
// below is apply.go's own, separate, dc-4-scoped gate.
//
// Limitation (noted, not fixed): the walker below keys on the bare
// identifier "gitx", so `import gx "github.com/jyang234/verdi/internal/
// gitx"` followed by a call through that alias would evade both this
// list and the vacuity guard's own name-based check — only a wholesale
// rename of the import is caught (TestCommandSurface_OnlyReadOnlyGitxCallsInProductionSource's
// own vacuity guard fires when NO "gitx.*" call is found at all, which a
// consistently-aliased package still trips). internal/residue's own gate
// (commandsurface_test.go there) shares the same shape.
var readOnlyGitxCalls = map[string]bool{
	"AheadBehind":             true,
	"CurrentBranch":           true,
	"HasRemoteTrackingBranch": true,
	"LocalBranches":           true,
	"LsTree":                  true,
	"MergeBase":               true,
	"RevParse":                true,
	// Show joins the read half's allow-list per R-RR3-15 (plan amendment):
	// read-only blob-content-at-a-ref plumbing, already on
	// internal/residue's own allow-list — specClassAt's own fallback
	// chain (facts.go) needs it to resolve a spec's class from a ritual
	// branch's or the default branch's own tree when it is not visible
	// on disk in the current checkout.
	"Show":                 true,
	"StagedPaths":          true,
	"StatusDirty":          true,
	"WorktreeChangedPaths": true,
}

// TestCommandSurface_OnlyReadOnlyGitxCallsInProductionSource mirrors
// internal/residue/commandsurface_test.go's own walker (dc-4/ac-9's own
// static obligation): it parses every non-test .go file in this package
// and fails if any `gitx.<Ident>(...)` call it finds names a function
// outside readOnlyGitxCalls — in particular any mutating primitive
// (CheckoutNewBranch, Push, DeleteBranch, UpdateRef, AddPaths,
// CreateCommit, and so on). A newly-added gitx call in this package's
// production source that is not on the allow-list fails this test by
// construction, not by hoping a reviewer notices.
func TestCommandSurface_OnlyReadOnlyGitxCallsInProductionSource(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}

	found := map[string]bool{}
	var offenders []string

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || pkgIdent.Name != "gitx" {
				return true
			}
			fn := sel.Sel.Name
			found[fn] = true
			if !readOnlyGitxCalls[fn] {
				offenders = append(offenders, name+": gitx."+fn)
			}
			return true
		})
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf("internal/recovery's production source calls gitx function(s) outside the read-only allow-list %v (dc-4/ac-9: no new git primitive):\n%s",
			sortedGitxAllowlistKeys(), strings.Join(offenders, "\n"))
	}
	if len(found) == 0 {
		t.Fatal("no gitx.* calls found at all — this guard would pass vacuously; the parser/AST walk itself is broken")
	}
}

// TestCommandSurface_AllowListItselfHasNoMutatingNames is a narrow
// self-check that the allow list above was never accidentally widened to
// include a mutating primitive.
func TestCommandSurface_AllowListItselfHasNoMutatingNames(t *testing.T) {
	forbidden := []string{
		"WorktreeAdd", "WorktreeRemove", "WorktreeAddDetached", "Checkout", "CheckoutExisting",
		"CheckoutNewBranch", "CheckoutNewBranchFrom", "Push", "CreateCommit", "CreateCommitPaths",
		"AddAll", "AddPaths", "UpdateRef", "DeleteBranch", "DeleteMergedBranch", "ApplyPatch",
		"WriteBlob", "BuildTreeWithFile", "CommitTree", "HashObject",
	}
	for _, name := range forbidden {
		if readOnlyGitxCalls[name] {
			t.Fatalf("readOnlyGitxCalls wrongly allow-lists mutating primitive %q", name)
		}
	}
}

// TestCommandSurface_NoDirectExecInProductionSource proves this package
// never shells out on its own (co-1: no network in any test, and no
// second git-invocation path beside the gitx seam this projection's
// facts and Task 4's two executors are built on): no non-test .go file in
// this package imports os/exec.
func TestCommandSurface_NoDirectExecInProductionSource(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			if imp.Path.Value == `"os/exec"` {
				t.Fatalf("%s imports os/exec directly; every git invocation must go through internal/gitx", name)
			}
		}
	}
}

// applyExecutorAllowList is apply.go's own, separate, dc-4-scoped
// command-surface gate: the only calls apply.go's production source may
// make into a package capable of issuing a git command beyond a plain
// read (dc-4/ac-9: "the executors call only branchcut.Unwind and
// reclaim.Apply; no new git primitive"). branchcut.Unwind and
// reclaim.Apply are the two — and only two — MUTATING entry points; a
// fresh residue.Scan and a fresh reclaim.Compute (dispatch note (c): "the
// re-prove and the execution ... recompute reclaim.Compute over a fresh
// residue.Scan") are both read-only recomputations of the SAME two
// primitives internal/recovery's own Gather already calls in its
// ordinary read path (facts.go) — never a new git primitive of their
// own, and each already gated by its OWN package's command-surface test
// (internal/reclaim, internal/residue).
var applyExecutorAllowList = map[string]map[string]bool{
	"branchcut": {"Unwind": true},
	"reclaim":   {"Apply": true, "Compute": true},
	"residue":   {"Scan": true},
	// R-RR3-20 (review M3): the three executor packages alone export
	// only those four functions today, so a gate keyed on them could bite
	// only on a NEW export. The gate therefore also covers every other
	// package identifier apply.go could reach a write through: gitx
	// (allowed at exactly the read-only names, so gitx.DeleteBranch and
	// every other mutator is rejected HERE and not only by the
	// package-wide walker), and os / os/exec / filelock / wtmanager /
	// execworkspace / store, none of whose functions apply.go may call at
	// all. A future write path in this file therefore cannot land
	// ungated.
	"gitx":          readOnlyGitxCalls,
	"os":            {},
	"exec":          {},
	"filelock":      {},
	"wtmanager":     {},
	"execworkspace": {},
	"store":         {},
}

// TestCommandSurface_ApplyExecutorAllowList walks ONLY apply.go (Task 4's
// own file) and fails if it calls any function, on any of
// applyExecutorAllowList's own package identifiers, outside that map —
// so a future edit that reaches for a THIRD git-capable entry point (or
// widens beyond Unwind/Apply/Compute/Scan) fails this test by
// construction, not by hoping a reviewer notices.
func TestCommandSurface_ApplyExecutorAllowList(t *testing.T) {
	const file = "apply.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}

	found := map[string]bool{}
	var offenders []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		allowed, gated := applyExecutorAllowList[pkgIdent.Name]
		if !gated {
			return true
		}
		found[pkgIdent.Name] = true
		fn := sel.Sel.Name
		if !allowed[fn] {
			offenders = append(offenders, pkgIdent.Name+"."+fn)
		}
		return true
	})

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf("apply.go calls function(s) outside its own dc-4 executor allow-list %v:\n%s", applyExecutorAllowList, strings.Join(offenders, "\n"))
	}
	if !found["branchcut"] || !found["reclaim"] {
		t.Fatal("apply.go calls neither branchcut.* nor reclaim.* at all — this guard would pass vacuously")
	}
}

// TestCommandSurface_ApplyGateCoversEveryWriteCapablePackage is
// R-RR3-20's own self-check on the table above: the gate is only as wide
// as its key set, so every package identifier apply.go could reach a git
// or filesystem write through must be a key — and the three mutator
// names below must be REJECTED by the table rather than merely absent
// from apply.go today. (The walker's own biting power is proven by
// mutation in the fix round's report: os.RemoveAll and gitx.DeleteBranch
// injected into executeUnwind each fail
// TestCommandSurface_ApplyExecutorAllowList.)
func TestCommandSurface_ApplyGateCoversEveryWriteCapablePackage(t *testing.T) {
	for _, pkg := range []string{"gitx", "os", "exec", "filelock", "wtmanager", "execworkspace", "store", "branchcut", "reclaim", "residue"} {
		if _, gated := applyExecutorAllowList[pkg]; !gated {
			t.Fatalf("applyExecutorAllowList does not gate package %q; a write through it in apply.go would be invisible to this test", pkg)
		}
	}
	rejected := map[string]string{
		"gitx":      "DeleteBranch",
		"os":        "RemoveAll",
		"exec":      "Command",
		"wtmanager": "Remove",
	}
	for pkg, fn := range rejected {
		if applyExecutorAllowList[pkg][fn] {
			t.Fatalf("applyExecutorAllowList wrongly admits the mutating call %s.%s in apply.go", pkg, fn)
		}
	}
}

func sortedGitxAllowlistKeys() []string {
	out := make([]string, 0, len(readOnlyGitxCalls))
	for k := range readOnlyGitxCalls {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
