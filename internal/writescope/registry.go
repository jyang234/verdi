package writescope

import (
	"fmt"
)

// RitualBoardCommitPush is the one ritual that may declare carried: the
// board's Commit and push, which commits and pushes the operator's whole
// working tree at their explicit request (parent dc-3, dc-11; owner
// decision 2026-09-30).
const RitualBoardCommitPush = "board_commit_push"

// Registry returns the write-scope declaration of every verb that mutates
// a git repository, one declaration per ritual. Each declaration states
// what its ritual does at this story's base, read through parent dc-7,
// except the owner-ruled fixes of story dc-3: design start (with
// --supersedes), the commit-to-design ritual, accept diagram, and
// constitution propose are declared scoped and sit in AwaitingFixes until
// spec/ritual-effect-witness fixes them. Paths are store-relative.
func Registry() []Declaration {
	return []Declaration{
		{
			// cmd/verdi/design.go (runDesignStart) and designsupersede.go:
			// cut design/<name> from the resolved base and switch to it,
			// write the spec directory, git add -- <dir>, then a commit
			// with no pathspec. Awaiting fix: the commit carries pre-staged
			// entries today (UAT-036).
			Ritual:     "design_start",
			Verbs:      []Verb{CLI("design start")},
			RefsCreate: []RefPattern{"refs/heads/design/*"},
			HeadSwitch: true,
			StagePaths: []PathPattern{".verdi/specs/active/*/"},
			IndexCarry: CarryScoped,
		},
		{
			// internal/stubinstantiate.CommitScaffoldBranch: a blob, a tree
			// built in a scratch index, commit-tree, and a create-only ref;
			// no switch, the caller's index untouched. The board's actions
			// under /b/{branch}/ are the same ritual served against the
			// branch's managed worktree, which the mount adds on first use
			// (ledger SI-314 (1)).
			Ritual: "scaffold_branch",
			Verbs: []Verb{
				CLI("design start --from-stub"),
				Workbench("/b/{branch}/board/spec/{name}/api/create"),
				Workbench("/b/{branch}/board/spec/{name}/api/revise"),
				Workbench("/b/{branch}/board/spec/{name}/api/stub-instantiate"),
				Workbench("/board/spec/{name}/api/create"),
				Workbench("/board/spec/{name}/api/revise"),
				Workbench("/board/spec/{name}/api/stub-instantiate"),
			},
			RefsCreate: []RefPattern{"refs/heads/design/*"},
			Worktrees:  []WorktreePattern{".verdi/data/worktrees/*"},
			StagePaths: []PathPattern{".verdi/specs/active/*/"},
			IndexCarry: CarryScoped,
		},
		{
			// internal/specimport (publish.go): the same plumbing, recording
			// the spec, its import record, and its source snapshots.
			Ritual: "spec_import",
			Verbs: []Verb{
				CLI("design import apply"),
				Workbench("/design/import/apply"),
				MCP("import_apply"),
			},
			RefsCreate: []RefPattern{"refs/heads/design/*"},
			StagePaths: []PathPattern{".verdi/specs/active/*/", ".verdi/imports/*/"},
			IndexCarry: CarryScoped,
		},
		{
			// cmd/verdi/buildstart.go: checkout -b feature/<name>; no commit.
			Ritual:     "build_start",
			Verbs:      []Verb{CLI("build"), CLI("feature")},
			RefsCreate: []RefPattern{"refs/heads/feature/*"},
			HeadSwitch: true,
			IndexCarry: CarryNoCommit,
		},
		{
			// cmd/verdi/close.go and closefeature.go: refuses any staged
			// entry before mutating, cuts close/<name> and switches to it,
			// stages the spec's active and archive directories, commits;
			// the failure unwind switches back and deletes close/<name>.
			Ritual:     "close",
			Verbs:      []Verb{CLI("close")},
			RefsCreate: []RefPattern{"refs/heads/close/*"},
			RefsDelete: []RefPattern{"refs/heads/close/*"},
			HeadSwitch: true,
			StagePaths: []PathPattern{".verdi/specs/active/*/", ".verdi/specs/archive/*/"},
			IndexCarry: CarryRefused,
		},
		{
			// internal/commitdesign: writes spec.md and board.json into a new
			// spec directory, git add -- <dir>, then a commit with no
			// pathspec on the checked-out branch. Awaiting fix (UAT-036).
			Ritual:     "commit_to_design",
			Verbs:      []Verb{CLI("board"), Workbench("/board/{key}/commit")},
			RefsMove:   []RefPattern{RefCheckedOut},
			StagePaths: []PathPattern{".verdi/specs/active/*/"},
			IndexCarry: CarryScoped,
		},
		{
			// cmd/verdi/policy.go: cut policy/adopt from the resolved base,
			// write the starter store, commit with a pathspec.
			Ritual:     "policy_adopt",
			Verbs:      []Verb{CLI("policy")},
			RefsCreate: []RefPattern{"refs/heads/policy/adopt"},
			HeadSwitch: true,
			StagePaths: []PathPattern{
				".verdi/constitution/consumers.json",
				".verdi/policy/constitution.md",
				".verdi/policy/policies/*",
				".verdi/policy/profiles/*",
			},
			IndexCarry: CarryScoped,
		},
		{
			// cmd/verdi/acceptdiagram.go: rewrites the diagram, git add --
			// <file>, then a commit with no pathspec on the checked-out
			// branch. Awaiting fix (UAT-036).
			Ritual:     "accept_diagram",
			Verbs:      []Verb{CLI("accept")},
			RefsMove:   []RefPattern{RefCheckedOut},
			StagePaths: []PathPattern{".verdi/diagrams/*"},
			IndexCarry: CarryScoped,
		},
		{
			// internal/constitutionapp (propose.go): on the branch the
			// request names, created from HEAD or checked out when it
			// exists, stages the one proposed policy file and commits with
			// no pathspec. Awaiting fix (UAT-036; it also stops cutting from
			// HEAD, a base the write scope does not express).
			Ritual:     "constitution_propose",
			Verbs:      []Verb{CLI("context constitution propose")},
			RefsCreate: []RefPattern{"refs/heads/*"},
			RefsMove:   []RefPattern{"refs/heads/*"},
			HeadSwitch: true,
			StagePaths: []PathPattern{
				".verdi/policy/exemptions/*",
				".verdi/policy/overlays/*",
				".verdi/policy/policies/*",
			},
			IndexCarry: CarryScoped,
		},
		{
			// internal/constitutionapp (evaluation_checkout.go): a detached
			// evaluation worktree under a fresh temporary directory, removed
			// after the impact review; submit-preparation runs the same
			// review.
			Ritual: "constitution_evaluation",
			Verbs: []Verb{
				CLI("context constitution impact-review"),
				CLI("context constitution submit-preparation"),
				MCP("constitution_impact_review"),
			},
			Worktrees:  []WorktreePattern{WorktreeTemp},
			IndexCarry: CarryNoCommit,
		},
		{
			// cmd/verdi/context_execution.go: materializes a detached unit
			// worktree (internal/execworkspace, with git apply inside it and
			// the reconciler's administrative-entry writes), then the
			// hand-back fast-forwards the runway's checked-out branch.
			Ritual:     "context_execution",
			Verbs:      []Verb{CLI("context execution")},
			RefsMove:   []RefPattern{RefCheckedOut},
			Worktrees:  []WorktreePattern{".verdi/data/execution/*"},
			IndexCarry: CarryNoCommit,
		},
		{
			// internal/experimentapp through internal/execworkspace: the
			// same unit worktree, without a hand-back.
			Ritual:     "execution_workspace",
			Verbs:      []Verb{CLI("experiment resume"), CLI("experiment start"), MCP("experiment")},
			Worktrees:  []WorktreePattern{".verdi/data/execution/*"},
			IndexCarry: CarryNoCommit,
		},
		{
			// internal/workbench (branchboard.go) through
			// internal/wtmanager.EnsureWorktree: every /b/{branch}/ route
			// adds the branch's managed worktree on first use. The board's
			// actions beneath the API route join their root ritual's
			// declaration instead (ledger SI-314 (1)).
			Ritual: "managed_worktree",
			Verbs: []Verb{
				Workbench("/b/{branch}/board/spec/{name}"),
				Workbench("/b/{branch}/board/spec/{name}/api/{action}"),
				Workbench("/b/{branch}/board/spec/{name}/document"),
				Workbench("/b/{branch}/board/spec/{name}/document/snapshot"),
				Workbench("/b/{branch}/board/spec/{name}/fragment"),
				Workbench("/b/{branch}/board/spec/{name}/peek"),
				Workbench("/b/{branch}/board/spec/{name}/pinsearch"),
				Workbench("/b/{branch}/board/spec/{name}/snapshot"),
			},
			Worktrees:  []WorktreePattern{".verdi/data/worktrees/*"},
			IndexCarry: CarryNoCommit,
		},
		{
			// cmd/verdi/gc.go: removes managed worktrees (wtmanager.GC) and
			// execution units (execworkspace.GC); with --reclaim-unmanaged
			// --apply, removes eligible registered worktrees and deletes
			// their merged branches (internal/reclaim).
			Ritual:     "gc",
			Verbs:      []Verb{CLI("gc")},
			RefsDelete: []RefPattern{"refs/heads/*"},
			Worktrees: []WorktreePattern{
				".verdi/data/execution/*",
				".verdi/data/worktrees/*",
				WorktreeRegistered,
			},
			IndexCarry: CarryNoCommit,
		},
		{
			// cmd/verdi/recover.go through internal/recovery.Apply: the
			// empty-cut unwind switches back and deletes one of the four
			// ritual branches (internal/branchcut); the reclaim choice is
			// gc's reclaim.
			Ritual:     "recover",
			Verbs:      []Verb{CLI("recover")},
			RefsDelete: []RefPattern{"refs/heads/*"},
			HeadSwitch: true,
			Worktrees:  []WorktreePattern{WorktreeRegistered},
			IndexCarry: CarryNoCommit,
		},
		{
			// internal/workbench (boardspecapi.go actionGitCommit): git add
			// -A, a commit with no pathspec, and a push when an origin
			// exists — the one declared carried ritual. Under /b/{branch}/
			// it is the same ritual in the branch's managed worktree, which
			// the mount adds on first use (ledger SI-314 (1); the owner's
			// 2026-09-30 decision covers the board's Commit and push
			// wherever it is served).
			Ritual: RitualBoardCommitPush,
			Verbs: []Verb{
				Workbench("/b/{branch}/board/spec/{name}/api/git-commit"),
				Workbench("/board/spec/{name}/api/git-commit"),
			},
			RefsMove:          []RefPattern{RefCheckedOut},
			Worktrees:         []WorktreePattern{".verdi/data/worktrees/*"},
			StagePaths:        []PathPattern{PathWholeTree},
			IndexCarry:        CarryCarried,
			UntrackedMayEnter: true,
			MayPush:           true,
		},
		{
			// internal/workbench (boardspecapi.go actionGitSwitch): refuses
			// a dirty tree (a staged entry included) before checking out.
			// Under /b/{branch}/ the action reaches the same checkout
			// statically but always refuses (the instance's fixed branch);
			// the mount adds the managed worktree first (ledger SI-314 (1)).
			Ritual: "board_switch",
			Verbs: []Verb{
				Workbench("/b/{branch}/board/spec/{name}/api/git-switch"),
				Workbench("/board/spec/{name}/api/git-switch"),
			},
			HeadSwitch: true,
			Worktrees:  []WorktreePattern{".verdi/data/worktrees/*"},
			IndexCarry: CarryRefused,
		},
	}
}

// ValidateRegistry reports the first problem with decls and awaiting:
// an invalid declaration, a ritual or verb declared twice, a carried
// declaration other than the board's Commit and push, or an awaiting-fix
// entry that names no scoped declaration.
func ValidateRegistry(decls []Declaration, awaiting []AwaitingFix) error {
	rituals := map[string]Declaration{}
	owner := map[Verb]string{}
	for _, d := range decls {
		if err := d.Validate(); err != nil {
			return err
		}
		if _, dup := rituals[d.Ritual]; dup {
			return fmt.Errorf("writescope: ritual %s is declared twice", d.Ritual)
		}
		rituals[d.Ritual] = d
		for _, v := range d.Verbs {
			if prev, dup := owner[v]; dup {
				return fmt.Errorf("writescope: verb %s is named by both %s and %s; each verb has exactly one declaration", v, prev, d.Ritual)
			}
			owner[v] = d.Ritual
		}
		if d.IndexCarry == CarryCarried && d.Ritual != RitualBoardCommitPush {
			return fmt.Errorf("writescope: %s declares carried; only %s may (parent dc-3, dc-11)", d.Ritual, RitualBoardCommitPush)
		}
	}
	paths := map[string]bool{}
	for _, a := range awaiting {
		d, ok := rituals[a.Ritual]
		if !ok {
			return fmt.Errorf("writescope: awaiting fix %q names unknown ritual %s", a.Path, a.Ritual)
		}
		if d.IndexCarry != CarryScoped {
			return fmt.Errorf("writescope: awaiting fix %q sits in %s, which is not declared scoped (spec/write-scope-registry dc-3)", a.Path, a.Ritual)
		}
		if a.Path == "" {
			return fmt.Errorf("writescope: an awaiting fix in %s names no invocation path", a.Ritual)
		}
		if a.Defect == "" {
			return fmt.Errorf("writescope: awaiting fix %q names no defect", a.Path)
		}
		if paths[a.Path] {
			return fmt.Errorf("writescope: awaiting fix %q is listed twice", a.Path)
		}
		paths[a.Path] = true
	}
	return nil
}
