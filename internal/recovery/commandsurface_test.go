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

// readOnlyGitxCalls is Task 2's own command-surface allow-list (Step 15):
// every read-only gitx primitive facts.go, derive.go, schema.go, codec.go,
// and commandlog.go may legitimately call. Task 4's apply.go additionally
// allow-lists branchcut.Unwind and reclaim.Apply only — not this test's
// concern (a different file, added by a later task).
var readOnlyGitxCalls = map[string]bool{
	"CurrentBranch":           true,
	"RevParse":                true,
	"MergeBase":               true,
	"DefaultBranch":           true,
	"LocalBranches":           true,
	"StagedPaths":             true,
	"WorktreeChangedPaths":    true,
	"StatusDirty":             true,
	"LsTree":                  true,
	"HasRemoteTrackingBranch": true,
	"AheadBehind":             true,
	"WorktreeList":            true,
	// Show joins the read half's allow-list per R-RR3-15 (plan amendment):
	// read-only blob-content-at-a-ref plumbing, already on
	// internal/residue's own allow-list — specClassAt's own fallback
	// chain (facts.go) needs it to resolve a spec's class from a ritual
	// branch's or the default branch's own tree when it is not visible
	// on disk in the current checkout.
	"Show": true,
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

func sortedGitxAllowlistKeys() []string {
	out := make([]string, 0, len(readOnlyGitxCalls))
	for k := range readOnlyGitxCalls {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
