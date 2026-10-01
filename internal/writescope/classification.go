package writescope

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
)

// Effect classifies a function by what it can do to a git repository.
type Effect string

// The two effects.
const (
	// Mutating changes a ref, HEAD, the index, a linked worktree, a
	// remote, the object store, or working-tree files through git, or
	// writes under a repository's git directory.
	Mutating Effect = "mutating"
	// ReadOnly changes none of those.
	ReadOnly Effect = "read_only"
)

// Classified is one function and its effect. Func is the function's full
// name with the module path trimmed: "internal/gitx.AddAll",
// "(internal/gitx.DiffEntry).Pure", "(*internal/x.T).M".
type Classified struct {
	Func   string
	Effect Effect
}

var funcNameRE = regexp.MustCompile(`^(\(\*?)?[a-z][A-Za-z0-9_./-]*\.[A-Za-z_][A-Za-z0-9_]*(\)\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// Classification returns every exported internal/gitx function and method
// (with every other shape a caller can run code through: methods of
// unexported types and of interfaces, and exported variables of function
// type), and every function outside gitx that writes under a repository's
// git directory, each classified mutating or read-only. The gitx half matches
// the census of 2026-09-30 (twenty of gitx's functions mutate; object
// writes and git apply count, because each is a git write even where its
// ritual's declaration owns the effect elsewhere). The non-gitx half is
// the execution-workspace reconciler's and garbage collector's writes to
// worktree administrative entries (story dc-2, parent dc-7).
func Classification() []Classified {
	mut := func(name string) Classified { return Classified{Func: name, Effect: Mutating} }
	ro := func(name string) Classified { return Classified{Func: name, Effect: ReadOnly} }
	return []Classified{
		ro("(internal/gitx.DiffEntry).Pure"),
		// The observer gitx calls before each command it runs: its
		// implementations live outside gitx, where reachability follows
		// them like any other code.
		ro("(internal/gitx.Observer).Observe"),
		ro("(internal/gitx.Reachability).String"),
		mut("internal/gitx.AddAll"), // add -A
		mut("internal/gitx.AddPaths"),
		ro("internal/gitx.AheadBehind"),
		ro("internal/gitx.AncestryPathChanges"),
		mut("internal/gitx.ApplyPatch"), // git apply: working-tree files
		ro("internal/gitx.Blame"),
		ro("internal/gitx.BlobAt"),
		mut("internal/gitx.BuildTreeWithFile"), // objects through a scratch index
		ro("internal/gitx.CanonicalRemoteIdentity"),
		mut("internal/gitx.Checkout"), // HEAD switch behind a dirty-tree refusal
		mut("internal/gitx.CheckoutExisting"),
		mut("internal/gitx.CheckoutNewBranch"),
		mut("internal/gitx.CheckoutNewBranchFrom"),
		ro("internal/gitx.CommitDate"),
		ro("internal/gitx.CommitDateOnly"),
		ro("internal/gitx.CommitDates"),
		ro("internal/gitx.CommitExists"),
		ro("internal/gitx.CommitIdentityAvailable"),
		mut("internal/gitx.CommitTree"), // commit object only
		ro("internal/gitx.CommonDir"),
		ro("internal/gitx.ConfigValue"),
		mut("internal/gitx.CreateCommit"), // records the whole index
		mut("internal/gitx.CreateCommitPaths"),
		ro("internal/gitx.CurrentBranch"),
		ro("internal/gitx.DefaultBranch"),
		mut("internal/gitx.DeleteBranch"),
		mut("internal/gitx.DeleteMergedBranch"),
		ro("internal/gitx.DiffNameStatus"),
		ro("internal/gitx.DiffNameStatusCopies"),
		mut("internal/gitx.FastForwardOnly"), // merge --ff-only on the runway
		ro("internal/gitx.FirstParentBlobLanding"),
		ro("internal/gitx.FirstParentPathCommits"),
		ro("internal/gitx.HasLocalBranch"),
		ro("internal/gitx.HasRemote"),
		ro("internal/gitx.HasRemoteTrackingBranch"),
		ro("internal/gitx.HashObject"), // without -w
		ro("internal/gitx.IsAncestor"),
		ro("internal/gitx.IsShallow"),
		ro("internal/gitx.LastCommit"),
		ro("internal/gitx.LocalBranches"),
		ro("internal/gitx.Locate"),
		ro("internal/gitx.Log"),
		ro("internal/gitx.LsFiles"),
		ro("internal/gitx.LsFilesWithUntracked"),
		ro("internal/gitx.LsTree"),
		ro("internal/gitx.LsTreeEntries"),
		ro("internal/gitx.LsTreeEntriesIncludingTrees"),
		ro("internal/gitx.MergeBase"),
		ro("internal/gitx.MergeBaseCommit"),
		ro("internal/gitx.PathExistsAt"),
		ro("internal/gitx.PickaxeCommit"),
		mut("internal/gitx.Push"), // remote ref plus branch.<b>.* upstream config
		ro("internal/gitx.ReachableFromHEAD"),
		ro("internal/gitx.RemoteDesignBranches"),
		ro("internal/gitx.RemoteURL"),
		ro("internal/gitx.RepoPrefix"),
		ro("internal/gitx.ResetShallowCache"), // clears an in-process cache only
		ro("internal/gitx.ResolveExactRef"),
		ro("internal/gitx.RevParse"),
		ro("internal/gitx.Show"),
		ro("internal/gitx.StagedPaths"),
		ro("internal/gitx.StatusDirty"),
		ro("internal/gitx.StoreRelativePaths"),
		ro("internal/gitx.TrackedChanges"),
		ro("internal/gitx.UntrackedPaths"),
		mut("internal/gitx.UpdateRef"), // create-only ref write
		ro("internal/gitx.ValidateFullOID"),
		ro("internal/gitx.WithObserver"),
		mut("internal/gitx.WorktreeAdd"),
		mut("internal/gitx.WorktreeAddDetached"),
		ro("internal/gitx.WorktreeChangedPaths"),
		ro("internal/gitx.WorktreeList"),
		mut("internal/gitx.WorktreeRemove"),
		mut("internal/gitx.WriteBlob"), // object only

		// Outside gitx: renames and deletes $GIT_COMMON_DIR/worktrees/<id>/
		// administrative entries with package os (execworkspace/reconcile.go).
		mut("(*internal/execworkspace.GitReconciler).ReconcileUnit"),
		// Outside gitx: deletes execution-unit linked worktrees whose
		// identity it reads from administrative entries, and reconciles
		// those entries through the reconciler (execworkspace/gc.go).
		mut("internal/execworkspace.GC"),
	}
}

// MutatingFuncs returns the names list classifies mutating, sorted.
func MutatingFuncs(list []Classified) []string {
	var out []string
	for _, c := range list {
		if c.Effect == Mutating {
			out = append(out, c.Func)
		}
	}
	sort.Strings(out)
	return out
}

// ValidateClassification reports the first problem with list: an empty
// list, a malformed name, an unknown effect, or a name classified twice.
func ValidateClassification(list []Classified) error {
	if len(list) == 0 {
		return errors.New("writescope: the classification is empty")
	}
	seen := map[string]bool{}
	for _, c := range list {
		if !funcNameRE.MatchString(c.Func) {
			return fmt.Errorf("writescope: classified name %q is not a function's full name (internal/pkg.F or (*internal/pkg.T).M)", c.Func)
		}
		switch c.Effect {
		case Mutating, ReadOnly:
		default:
			return fmt.Errorf("writescope: %s: unknown effect %q (want mutating or read_only)", c.Func, c.Effect)
		}
		if seen[c.Func] {
			return fmt.Errorf("writescope: %s is classified twice", c.Func)
		}
		seen[c.Func] = true
	}
	return nil
}
