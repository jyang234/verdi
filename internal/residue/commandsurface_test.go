package residue

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// readOnlyGitxCalls is ac-3's own exhaustive command-surface allow-list
// (ac-3's static obligation: "the list is worktree list, rev-parse/
// merge-base, and status checks only — never add, remove, or prune"),
// extended (dc-1's "one pass... shared") to cover every read-only gitx
// primitive this package's whole scan (not survey.go alone) legitimately
// calls: branch enumeration and tip-tree reads for AC-1/AC-2's own
// classification pass, in addition to AC-3's survey.
var readOnlyGitxCalls = map[string]bool{
	"LocalBranches": true, // enumerating close/* and all local branches
	"RevParse":      true, // resolving a branch/ref to its tip commit sha
	"IsAncestor":    true, // merge-base --is-ancestor: the merged/unmerged check
	"LsTree":        true, // archive-path presence at a ref, via git plumbing
	"WorktreeList":  true, // git worktree list --porcelain
	"StatusDirty":   true, // git status --porcelain: the clean/dirty signal
}

// TestCommandSurface_OnlyReadOnlyGitxCallsInProductionSource is ac-3's own
// static evidence, proven exhaustively rather than merely asserted: it
// parses every non-test .go file in this package and collects every
// `gitx.<Ident>(...)` call it finds, failing if ANY name outside
// readOnlyGitxCalls appears — in particular, gitx.WorktreeAdd,
// gitx.WorktreeRemove, gitx.Checkout, gitx.CheckoutNewBranch, gitx.Push,
// and any other git-MUTATING primitive gitx exposes. A newly-added gitx
// call in this package's production source that is not on the allow-list
// fails this test by construction, not by hoping a reviewer notices.
func TestCommandSurface_OnlyReadOnlyGitxCallsInProductionSource(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}

	var found = map[string]bool{}
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
		t.Fatalf("internal/residue's production source calls gitx function(s) outside the read-only allow-list %v (ac-3: never add/remove/prune):\n%s",
			sortedKeys(readOnlyGitxCalls), strings.Join(offenders, "\n"))
	}
	if len(found) == 0 {
		t.Fatal("no gitx.* calls found at all — this guard would pass vacuously; the parser/AST walk itself is broken")
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestCommandSurface_AllowListItselfHasNoMutatingNames is a narrow
// self-check that the allow list above was never accidentally widened to
// include a mutating primitive — a plain string check, not a parse, so it
// stays trivially correct even as gitx grows new mutating primitives this
// package must never call.
func TestCommandSurface_AllowListItselfHasNoMutatingNames(t *testing.T) {
	forbidden := []string{"WorktreeAdd", "WorktreeRemove", "Checkout", "CheckoutNewBranch", "Push", "CreateCommit", "AddAll", "UpdateRef"}
	for _, name := range forbidden {
		if readOnlyGitxCalls[name] {
			t.Fatalf("readOnlyGitxCalls wrongly allow-lists mutating primitive %q", name)
		}
	}
}
