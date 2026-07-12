// Package dex implements `verdi dex build` (PLAN.md Phase 12, 05
// §Verdi-dex): a static site that is a pure function of the store's
// committed tree at a given commit — byte-identical across rebuilds of the
// same tree state, with every page honestly labeled with its temporal
// class (01 §Temporal classes) rather than silently claiming currency it
// cannot back up.
package dex

import (
	"context"
	"fmt"

	"github.com/OWNER/verdi/internal/forge"
	"github.com/OWNER/verdi/internal/index"
	"github.com/OWNER/verdi/internal/store"
)

// Options configures Build.
type Options struct {
	// Root is the store root (the directory containing .verdi/).
	Root string
	// OutDir is the directory the site is written to. Build does not
	// clean it first (a caller building into a fresh temp dir, or one
	// that wants to diff a rebuild against a previous one, controls that
	// itself); Build only ever creates and overwrites the paths it owns.
	OutDir string
	// Commit is the git revision the build stamps every living-gated
	// banner and pinned copy-reference with, and the upper bound the
	// changelog's git log walks from. Empty defaults to "HEAD". This is
	// the *only* clock Build reads — never time.Now() — so building the
	// same commit twice, even on different days, produces byte-identical
	// output (Phase 12's determinism requirement).
	Commit string
	// Forge is the optional forge port the story-page pending-supersession
	// flag reads open MRs through (V1-P8; 03 §The amendment ladder: "the
	// fold's input set includes open supersession MRs") — the ONE input to
	// this site that is not a function of the tree, which is exactly the
	// flag's point: the race window is an open-MR fact. nil (no forge
	// configured/reachable — every hermetic `go test`) renders the flag
	// disclosed-unproven on affected story pages rather than silently
	// absent. For a fixed forge state, output stays byte-identical across
	// rebuilds (constitution 1's determinism is over the build's full
	// input set).
	Forge forge.Forge
	// DefaultBranch is the branch open supersession MRs are enumerated
	// against (the MR target, resolved by the caller the same way the
	// gates resolve it). Empty is treated like a nil Forge: candidates
	// cannot be enumerated, so the flag is disclosed-unproven.
	DefaultBranch string
}

// Build renders and writes the full dex site to opts.OutDir.
func Build(ctx context.Context, opts Options) error {
	if opts.Root == "" {
		return fmt.Errorf("dex: Build: Root must not be empty")
	}
	if opts.OutDir == "" {
		return fmt.Errorf("dex: Build: OutDir must not be empty")
	}
	commitArg := opts.Commit
	if commitArg == "" {
		commitArg = "HEAD"
	}

	stamp, err := resolveBuildStamp(ctx, opts.Root, commitArg)
	if err != nil {
		return err
	}

	ix, err := index.Build(opts.Root)
	if err != nil {
		return fmt.Errorf("dex: building index: %w", err)
	}

	pages, err := loadArtifactPages(opts.Root, ix)
	if err != nil {
		return err
	}

	services, err := store.DiscoverServices(opts.Root)
	if err != nil {
		return fmt.Errorf("dex: discovering services: %w", err)
	}

	known := knownRefs(ix)

	lens, err := computeLensData(ctx, opts.Root, opts.Forge, opts.DefaultBranch, pages)
	if err != nil {
		return err
	}

	for _, p := range pages {
		if err := writeArtifactPage(ctx, opts.OutDir, opts.Root, stamp.SHA, stamp, ix, known, lens, p); err != nil {
			return err
		}
	}
	if err := writeExemptionPages(opts.OutDir, stamp, pages, lens.exemptions, known); err != nil {
		return err
	}
	if err := writeStoryAxis(opts.OutDir, stamp, pages); err != nil {
		return err
	}
	if err := writeExternalPages(opts.OutDir, stamp, ix, known, services); err != nil {
		return err
	}
	if err := writeKindAxis(opts.OutDir, stamp, pages); err != nil {
		return err
	}
	if err := writeContractsAxis(opts.OutDir, stamp, services); err != nil {
		return err
	}
	if err := writeServiceAxis(opts.OutDir, stamp, services, pages); err != nil {
		return err
	}
	if err := writeChangelog(ctx, opts.Root, opts.OutDir, stamp, stamp.SHA); err != nil {
		return err
	}
	if err := writeDisclosuresPage(ctx, opts.Root, opts.OutDir, stamp); err != nil {
		return err
	}
	if err := writeSearchIndex(opts.OutDir, stamp, ix); err != nil {
		return err
	}
	if err := writeHome(opts.OutDir, stamp); err != nil {
		return err
	}
	if err := writeStaticAssets(opts.OutDir); err != nil {
		return err
	}

	return nil
}

// knownRefs is the set of every ref dex renders a permalink page for
// (every indexed entry, committed-zone and external alike) — used to
// decide whether a link/backlink's ref resolves to a real dex page.
func knownRefs(ix *index.Index) map[string]bool {
	m := make(map[string]bool)
	for _, e := range ix.All() {
		m[e.Ref] = true
	}
	return m
}
