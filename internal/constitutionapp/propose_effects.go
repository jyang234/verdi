package constitutionapp

import (
	"bytes"
	"context"
)

const (
	repositoryEffectBranch   = "branch"
	repositoryEffectWorktree = "worktree"
	repositoryEffectIndex    = "index"
)

// proposeEffectTracker marks the boundary after which Propose can no longer
// promise a byte/state-equivalent refusal. It records only application-owned
// facts; failure re-observes the repository with an uncanceled context
// so cancellation of the failed operation cannot also hide its residue.
type proposeEffectTracker struct {
	initialBranch    string
	initialHead      string
	targetBranch     string
	targetHeadBefore string
	targetExisted    bool
	gitPath          string
	fullPath         string
	active           bool
	checkoutFailed   bool
	worktreeUnproven bool
	wrote            bool
	landedCommit     string
}

func newProposeEffectTracker(initialBranch, initialHead, targetBranch, targetHeadBefore string, targetExisted bool, gitPath, fullPath string) *proposeEffectTracker {
	return &proposeEffectTracker{
		initialBranch:    initialBranch,
		initialHead:      initialHead,
		targetBranch:     targetBranch,
		targetHeadBefore: targetHeadBefore,
		targetExisted:    targetExisted,
		gitPath:          gitPath,
		fullPath:         fullPath,
	}
}

func (t *proposeEffectTracker) beginCheckout() {
	t.active = true
}

func (t *proposeEffectTracker) checkoutRefused() {
	t.checkoutFailed = true
}

func (t *proposeEffectTracker) beginWorktreeMutation() {
	t.active = true
}

func (t *proposeEffectTracker) worktreeStateUnproven() {
	t.worktreeUnproven = true
}

func (t *proposeEffectTracker) wroteArtifact() {
	t.wrote = true
}

func (t *proposeEffectTracker) commitLanded(commit string) {
	t.landedCommit = commit
}

func (t *proposeEffectTracker) failure(ctx context.Context, svc Service, root string, typed *Error) *Error {
	if typed == nil || !t.active {
		return typed
	}
	cloned := *typed
	cloned.RepositoryEffects = t.observe(context.WithoutCancel(ctx), svc, root)
	return &cloned
}

func (t *proposeEffectTracker) observe(ctx context.Context, svc Service, root string) *RepositoryEffects {
	effects := &RepositoryEffects{
		Operation:        "propose",
		InitialBranch:    t.initialBranch,
		InitialHead:      t.initialHead,
		TargetBranch:     t.targetBranch,
		TargetHeadBefore: t.targetHeadBefore,
		LandedCommit:     t.landedCommit,
		WorktreePaths:    []string{},
		StagedPaths:      []string{},
		Unproven:         []string{},
	}

	currentBranch, err := svc.Git.CurrentBranch(ctx, root)
	if err != nil {
		appendUnproven(effects, repositoryEffectBranch)
		appendUnproven(effects, repositoryEffectWorktree)
	} else {
		effects.CurrentBranch = currentBranch
	}
	currentHead, err := svc.Git.RevParse(ctx, root, "HEAD")
	if err != nil {
		appendUnproven(effects, repositoryEffectBranch)
	} else {
		effects.CurrentHead = currentHead
	}
	if !t.targetExisted {
		created, branchErr := svc.Git.HasLocalBranch(ctx, root, t.targetBranch)
		if branchErr != nil {
			appendUnproven(effects, repositoryEffectBranch)
		} else {
			effects.BranchCreated = created
		}
	}

	staged, err := svc.Git.StagedPaths(ctx, root)
	if err != nil {
		appendUnproven(effects, repositoryEffectIndex)
	} else {
		effects.StagedPaths = append(effects.StagedPaths, staged...)
	}

	if t.checkoutFailed {
		// A failed checkout has no owner contract proving which tracked paths
		// Git may already have materialized. Branch/head and index are
		// re-observed above; the broader worktree remains explicitly unproven.
		appendUnproven(effects, repositoryEffectWorktree)
	}
	if t.worktreeUnproven {
		appendUnproven(effects, repositoryEffectWorktree)
	}
	if t.wrote {
		t.observeWrittenArtifact(ctx, svc, root, effects)
	}
	return effects
}

func (t *proposeEffectTracker) observeWrittenArtifact(ctx context.Context, svc Service, root string, effects *RepositoryEffects) {
	worktree, exists, err := readRegularFile(t.fullPath)
	if err != nil || !exists || effects.CurrentHead == "" {
		appendUnproven(effects, repositoryEffectWorktree)
		return
	}
	committed, err := svc.Git.Show(ctx, root, effects.CurrentHead, t.gitPath)
	if err != nil || !bytes.Equal(worktree, committed) {
		effects.WorktreePaths = append(effects.WorktreePaths, t.gitPath)
	}
}

func appendUnproven(effects *RepositoryEffects, dimension string) {
	for _, existing := range effects.Unproven {
		if existing == dimension {
			return
		}
	}
	effects.Unproven = append(effects.Unproven, dimension)
}
