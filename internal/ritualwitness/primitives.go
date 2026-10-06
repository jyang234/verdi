package ritualwitness

import (
	"path/filepath"
	"strings"
)

// primKind is what one of gitx's mutating primitives can do, and so which
// observed effects a logged call of it may explain (SI-325 (8)).
type primKind int

const (
	// primStage is `add`: index entries in its worktree.
	primStage primKind = iota + 1
	// primApply is `apply`: working-tree files only.
	primApply
	// primScratchTree is BuildTreeWithFile's read-tree, update-index and
	// write-tree, which run against a scratch index: objects only, never
	// the real index (rws-facts §4).
	primScratchTree
	// primCheckout is `checkout <ref>`: its worktree's HEAD and index.
	primCheckout
	// primCheckoutNew is `checkout -b <name> [<base>]`: it creates
	// refs/heads/<name> and switches its worktree's HEAD to it.
	primCheckoutNew
	// primCommit is `commit`: a commit object, which moves its worktree's
	// checked-out branch (or detached HEAD), and with a pathspec restages
	// those paths.
	primCommit
	// primCommitTree is `commit-tree`: a commit object only.
	primCommitTree
	// primBranchDelete is `branch -d <name>`: it deletes refs/heads/<name>.
	primBranchDelete
	// primFastForward is `merge --ff-only <commit>`: it moves its
	// worktree's checked-out branch to <commit>, with its index.
	primFastForward
	// primPush is `push`: the remote's refs, their remote-tracking
	// mirrors, and the pushed branch's upstream configuration.
	primPush
	// primUpdateRef is gitx.UpdateRef's `branch <name> <commit>`: it
	// creates refs/heads/<name> (ritual-write-scope-v3 dc-8).
	primUpdateRef
	// primWorktreeAdd is `worktree add`: a linked worktree at its path.
	primWorktreeAdd
	// primWorktreeRemove is `worktree remove <path>`.
	primWorktreeRemove
	// primWriteBlob is `hash-object -w --stdin`: a blob object only.
	primWriteBlob
)

// primitive is one of gitx's mutating functions, by its name in
// writescope.Classification, with the argv shape it issues (after git's
// global options) and what it can do.
type primitive struct {
	fn    string
	kind  primKind
	match func(args []string) bool
}

// mutatingPrimitives returns gitx's mutating functions as writescope
// classifies them (TestMutatingPrimitives_MatchTheClassification holds
// the two lists equal), each with the exact argv it issues: a logged call
// counts as a mutating primitive only when its argv has one of these
// shapes, so a read-only gitx call that happens to name a ref or path
// (RevParse, WorktreeList, HashObject without -w) attributes nothing.
func mutatingPrimitives() []primitive {
	checkout := func(a []string) bool { return len(a) == 2 && a[0] == "checkout" && !strings.HasPrefix(a[1], "-") }
	branchDelete := func(a []string) bool { return len(a) == 3 && a[0] == "branch" && a[1] == "-d" }
	return []primitive{
		{"internal/gitx.AddAll", primStage, exactArgv("add", "-A")},
		{"internal/gitx.AddPaths", primStage, func(a []string) bool { return len(a) >= 3 && a[0] == "add" && a[1] == "--" }},
		{"internal/gitx.ApplyPatch", primApply, exactArgv("apply")},
		{"internal/gitx.BuildTreeWithFile", primScratchTree, func(a []string) bool {
			return (len(a) == 2 && a[0] == "read-tree") ||
				(len(a) == 4 && a[0] == "update-index" && a[1] == "--add" && a[2] == "--cacheinfo") ||
				(len(a) == 1 && a[0] == "write-tree")
		}},
		{"internal/gitx.Checkout", primCheckout, checkout},
		{"internal/gitx.CheckoutExisting", primCheckout, checkout},
		{"internal/gitx.CheckoutNewBranch", primCheckoutNew, func(a []string) bool { return len(a) == 3 && a[0] == "checkout" && a[1] == "-b" }},
		{"internal/gitx.CheckoutNewBranchFrom", primCheckoutNew, func(a []string) bool { return len(a) == 4 && a[0] == "checkout" && a[1] == "-b" }},
		{"internal/gitx.CommitTree", primCommitTree, func(a []string) bool {
			return len(a) == 6 && a[0] == "commit-tree" && a[2] == "-p" && a[4] == "-m"
		}},
		{"internal/gitx.CreateCommit", primCommit, func(a []string) bool { return len(a) == 3 && a[0] == "commit" && a[1] == "-m" }},
		{"internal/gitx.CreateCommitPaths", primCommit, func(a []string) bool {
			return len(a) >= 5 && a[0] == "commit" && a[1] == "-m" && a[3] == "--"
		}},
		{"internal/gitx.DeleteBranch", primBranchDelete, branchDelete},
		{"internal/gitx.DeleteMergedBranch", primBranchDelete, branchDelete},
		{"internal/gitx.FastForwardOnly", primFastForward, func(a []string) bool { return len(a) == 3 && a[0] == "merge" && a[1] == "--ff-only" }},
		{"internal/gitx.Push", primPush, exactArgv("push", "--set-upstream", "origin", "HEAD")},
		{"internal/gitx.UpdateRef", primUpdateRef, func(a []string) bool {
			return len(a) == 3 && a[0] == "branch" && !strings.HasPrefix(a[1], "-") && !strings.HasPrefix(a[2], "-")
		}},
		{"internal/gitx.WorktreeAdd", primWorktreeAdd, func(a []string) bool {
			return len(a) == 4 && a[0] == "worktree" && a[1] == "add" && !strings.HasPrefix(a[2], "-")
		}},
		{"internal/gitx.WorktreeAddDetached", primWorktreeAdd, func(a []string) bool {
			return len(a) == 5 && a[0] == "worktree" && a[1] == "add" && a[2] == "--detach"
		}},
		{"internal/gitx.WorktreeRemove", primWorktreeRemove, func(a []string) bool { return len(a) == 3 && a[0] == "worktree" && a[1] == "remove" }},
		{"internal/gitx.WriteBlob", primWriteBlob, exactArgv("hash-object", "-w", "--stdin")},
	}
}

func exactArgv(want ...string) func([]string) bool {
	return func(a []string) bool {
		if len(a) != len(want) {
			return false
		}
		for i := range want {
			if a[i] != want[i] {
				return false
			}
		}
		return true
	}
}

// globalFlag reports whether a is one of git's argument-less global
// options, which precede the subcommand.
func globalFlag(a string) bool {
	switch a {
	case "--literal-pathspecs", "--glob-pathspecs", "--noglob-pathspecs", "--icase-pathspecs",
		"--no-replace-objects", "--no-optional-locks", "--bare", "--no-pager", "-P":
		return true
	}
	return false
}

// splitGlobal removes git's global options from args, applying any -C to
// dir, and returns the directory the subcommand ran in and its argv. It
// reports false for an argv with no subcommand or a dangling option.
func splitGlobal(dir string, args []string) (string, []string, bool) {
	i := 0
	for i < len(args) {
		switch a := args[i]; {
		case a == "-c":
			if i+1 >= len(args) {
				return "", nil, false
			}
			i += 2
		case a == "-C":
			if i+1 >= len(args) {
				return "", nil, false
			}
			if filepath.IsAbs(args[i+1]) {
				dir = args[i+1]
			} else {
				dir = filepath.Join(dir, args[i+1])
			}
			i += 2
		case globalFlag(a):
			i++
		default:
			return dir, args[i:], true
		}
	}
	return "", nil, false
}

// loggedCall is one logged call of a mutating primitive: its canonical
// directory, its argv after global options, and its kind.
type loggedCall struct {
	Dir  string
	Args []string
	Kind primKind
}

// mutatingCall classifies one logged call: it reports false for a call
// whose argv matches no mutating primitive (every read-only gitx call).
func mutatingCall(c Call, prims []primitive) (loggedCall, bool) {
	dir, args, ok := splitGlobal(c.Dir, c.Args)
	if !ok {
		return loggedCall{}, false
	}
	for _, p := range prims {
		if p.match(args) {
			return loggedCall{Dir: canonicalPath("", dir), Args: args, Kind: p.kind}, true
		}
	}
	return loggedCall{}, false
}

// createdRef is the full refname a ref-creating call names, or "".
func (c loggedCall) createdRef() string {
	switch c.Kind {
	case primCheckoutNew:
		return "refs/heads/" + c.Args[2]
	case primUpdateRef:
		return "refs/heads/" + c.Args[1]
	}
	return ""
}

// deletedRef is the full refname a ref-deleting call names, or "".
func (c loggedCall) deletedRef() string {
	if c.Kind == primBranchDelete {
		return "refs/heads/" + c.Args[2]
	}
	return ""
}

// checkedOutRef is the branch a checkout call switches its worktree to,
// as a full refname, or "" for a call that does not name one.
func (c loggedCall) checkedOutRef() string {
	switch c.Kind {
	case primCheckoutNew:
		return "refs/heads/" + c.Args[2]
	case primCheckout:
		if !isObjectID(c.Args[1]) {
			return "refs/heads/" + c.Args[1]
		}
	}
	return ""
}

// checkoutTarget is what a checkout call switches its worktree to: a
// branch's full refname, or a commit id it detaches at; "" for any other
// call.
func (c loggedCall) checkoutTarget() string {
	switch c.Kind {
	case primCheckoutNew:
		return "refs/heads/" + c.Args[2]
	case primCheckout:
		if isObjectID(c.Args[1]) {
			return c.Args[1]
		}
		return "refs/heads/" + c.Args[1]
	}
	return ""
}

// pathspecs returns the pathspecs an add or commit call names, and
// whether it names any: `add -A` and a commit without "--" name none.
func (c loggedCall) pathspecs() ([]string, bool) {
	switch c.Kind {
	case primStage:
		if c.Args[1] == "--" {
			return c.Args[2:], true
		}
	case primCommit:
		if len(c.Args) > 3 && c.Args[3] == "--" {
			return c.Args[4:], true
		}
	}
	return nil, false
}

// addedCommit is the commit-ish a worktree-add call starts the worktree
// at, or "".
func (c loggedCall) addedCommit() string {
	if c.Kind != primWorktreeAdd {
		return ""
	}
	return c.Args[len(c.Args)-1]
}

// worktreePath is the canonical path a worktree call names, resolved
// against its own directory, or "".
func (c loggedCall) worktreePath() string {
	switch c.Kind {
	case primWorktreeAdd:
		if c.Args[2] == "--detach" {
			return canonicalPath(c.Dir, c.Args[3])
		}
		return canonicalPath(c.Dir, c.Args[2])
	case primWorktreeRemove:
		return canonicalPath(c.Dir, c.Args[2])
	}
	return ""
}

// fastForwardTarget is the commit a fast-forward call names, or "".
func (c loggedCall) fastForwardTarget() string {
	if c.Kind == primFastForward {
		return c.Args[2]
	}
	return ""
}
