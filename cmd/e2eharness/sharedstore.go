package main

// The shared harness store's provisioning sequence, extracted from run()
// (main.go) so that every store claiming the shared store's SHAPE — the
// shared serve itself and any isolated fixture that must reproduce its
// exact posture (readinesspilotfixture.go) — is provisioned by ONE piece
// of code. Two hand-copied sequences would drift the moment one gained a
// provisioner the other lacked; the browser oracles pinned against the
// shared store would then silently stop describing the isolated one.

import (
	"context"
	"fmt"
	"path/filepath"
)

// sharedStore is what a fully provisioned shared-shape store hands
// `verdi serve`: the store root (serve's working directory) plus the
// three caller-owned side artifacts serve is pointed at — the canned
// review feed (VERDI_REVIEW_FEED), the canned diagram verification
// report (VERDI_DIAGRAM_VERIFICATION), and the readiness context request
// (--context-request).
type sharedStore struct {
	storeRoot            string
	feedPath             string
	verificationPath     string
	readinessRequestPath string
}

// provisionSharedStore builds the shared-shape store under scratch and
// returns its paths. The sequence — and every argument — is exactly
// run()'s historical order: provisionStore (examples/showcase on main),
// then afterMain, then the design-branch provisioners provisionBoard →
// provisionDiagrams → provisionFamilyBoardLinks → provisionDirectory →
// provisionDraftBoards → provisionShowcaseDraft → provisionReadiness.
//
// afterMain runs once, between the main-only base store and the first
// design-branch provisioner, while the checkout still sits on main with
// no design branch cut: main.go hands it the static dex build, which must
// keep reflecting main (provisionBoard's own doc: "It runs AFTER the dex
// site is built, so the static site keeps reflecting main"). nil skips
// the stage — an isolated fixture serves no dex site. Nothing here is
// reordered around it.
func provisionSharedStore(ctx context.Context, moduleRoot, scratch string, afterMain func(ctx context.Context, storeRoot string) error) (sharedStore, error) {
	storeRoot := filepath.Join(scratch, "store")
	if err := provisionStore(ctx, moduleRoot, storeRoot); err != nil {
		return sharedStore{}, fmt.Errorf("provisioning scratch store: %w", err)
	}

	if afterMain != nil {
		if err := afterMain(ctx, storeRoot); err != nil {
			return sharedStore{}, err
		}
	}

	// The v1 board fixtures land on a design branch AFTER the afterMain
	// stage (the dex build), so the static site keeps reflecting main
	// while `verdi serve`'s working tree sits on the design branch
	// (authoring mode's branch state — 05 §Workbench "Two modes").
	feedPath, err := provisionBoard(ctx, scratch, storeRoot)
	if err != nil {
		return sharedStore{}, fmt.Errorf("provisioning v1 board fixtures: %w", err)
	}

	// The diagram editor's fixtures (spec/board-editor) land on the same
	// design branch provisionBoard just checked out, plus the canned
	// verification report the rail consumes through its dc-4 port.
	verificationPath, err := provisionDiagrams(ctx, scratch, storeRoot)
	if err != nil {
		return sharedStore{}, fmt.Errorf("provisioning diagram editor fixtures: %w", err)
	}

	// The family-board-links fixtures (spec/family-board-links; see
	// provision_familyboardlinks.go) — the archived-match feature/story
	// pair, the instantiated-but-unlanded stub's own design branch, and
	// the dangling-implements-target story. Lands on the same design
	// branch provisionBoard/provisionDiagrams just used, restoring it
	// when done.
	if err := provisionFamilyBoardLinks(ctx, storeRoot); err != nil {
		return sharedStore{}, fmt.Errorf("provisioning family-board-links fixtures: %w", err)
	}

	// The directory-home ref fixtures (local-only / remote-only / empty /
	// doomed design branches) — after the board fixtures, restoring the
	// board suite's serving checkout when done.
	if err := provisionDirectory(ctx, storeRoot); err != nil {
		return sharedStore{}, fmt.Errorf("provisioning directory fixtures: %w", err)
	}

	// The draft-boards branch fixtures (spec/draft-boards; see
	// provision_draftboards.go) — cut from main after the afterMain stage
	// like the board fixtures above, restoring the serving checkout when
	// done.
	if err := provisionDraftBoards(ctx, storeRoot); err != nil {
		return sharedStore{}, fmt.Errorf("provisioning draft-boards fixtures: %w", err)
	}

	// The showcase live-draft feature (payoff-quote-portal) on its own
	// design branch — the "one live draft on a design branch" lifecycle
	// stage (see provision_showcase_draft.go). Runs last among the branch
	// provisioners; it pre-cuts and seeds its worktree and restores the
	// serving checkout to designBranch when done.
	if err := provisionShowcaseDraft(ctx, storeRoot); err != nil {
		return sharedStore{}, fmt.Errorf("provisioning showcase draft fixtures: %w", err)
	}

	// The readiness pilot consumes one strict design-phase context request
	// against the serving branch. Provision the existing policy fixture and
	// managed projection, then pass the caller-owned request to serve; the
	// snapshot itself remains startup-only and in memory.
	readinessRequestPath, err := provisionReadiness(ctx, moduleRoot, storeRoot)
	if err != nil {
		return sharedStore{}, fmt.Errorf("provisioning readiness fixtures: %w", err)
	}

	return sharedStore{
		storeRoot:            storeRoot,
		feedPath:             feedPath,
		verificationPath:     verificationPath,
		readinessRequestPath: readinessRequestPath,
	}, nil
}

// sharedServeEnv is the environment `verdi serve` gets over a shared-shape
// store: the ambient process environment (already CI-neutralized by
// main.go's neutralizeCIEnv) plus the three injection seams, in this
// order — the canned review feed (workbench.CommentFeed's canned-file
// implementation: REVIEW_SPEC reads as under MR review with the three
// fixtures.ts comments), the control server's open-MR feed the directory
// home consults per render (openmrfeed.go's httpOpenMRFeed, loopback
// only), and the canned diagram verification report the rail consumes —
// no network (CLAUDE.md). A pure function of its inputs, shared by the
// shared serve (main.go) and the readiness-pilot fixture
// (readinesspilotfixture.go) so their postures cannot drift; the ambient
// slice is never mutated.
func sharedServeEnv(ambient []string, store sharedStore, openMRFeedURL string) []string {
	env := make([]string, 0, len(ambient)+3)
	env = append(env, ambient...)
	return append(env,
		"VERDI_REVIEW_FEED="+store.feedPath,
		"VERDI_OPENMR_FEED="+openMRFeedURL,
		"VERDI_DIAGRAM_VERIFICATION="+store.verificationPath,
	)
}
