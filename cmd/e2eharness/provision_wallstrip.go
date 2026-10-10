package main

// The wall-strip-and-drawer fixture walls (spec/wall-strip-and-drawer-v2;
// ledger SI-368): four walls whose working trees hold each of Commit and
// push's change states (ac-3), and two whose drawer projections are empty
// and unavailable (ac-5). Each is an authoring wall on its own namesake
// branch, served at its writable path from a pre-cut managed worktree, as
// the wall-canvas walls are (provisionCanvasWallBranches): the uncommitted
// state lives in that worktree alone, so the serving checkout's git status
// stays clean for every other suite.
//
// The e2e spec files keep their own copies of the names and paths
// (fixtures.ts is F7's), so they are pinned by TestWallStripPaths.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/workbench"
	"github.com/jyang234/verdi/internal/wtmanager"
)

const (
	// wallStripTypedSpecName's working tree holds typed changes only: the
	// problem replaced and ac-2 added, the body untouched.
	wallStripTypedSpecName = "decline-changes-typed"
	// wallStripUnclassifiedSpecName's holds unclassified changes only: an
	// edit to the body text and an untracked note — zero recognized
	// operations, so the indicator must stay set.
	wallStripUnclassifiedSpecName = "decline-changes-unclassified"
	// wallStripMixedSpecName's holds both: the problem replaced, and the
	// untracked note.
	wallStripMixedSpecName = "decline-changes-mixed"
	// wallStripUnreadableSpecName's spec was never committed: HEAD has no
	// revision to compare against, so the comparison is unreadable and the
	// spec itself is listed as an untracked file.
	wallStripUnreadableSpecName = "decline-changes-unreadable"
	// wallStripEmptySpecName's working tree is clean and its spec has no
	// design-provenance record: the Provenance projection is empty, and
	// Commit and push has nothing to commit.
	wallStripEmptySpecName = "decline-drawer-empty"
	// wallStripUnavailableSpecName's committed design-provenance record does
	// not decode, so the Provenance and Review projections are unavailable,
	// each with the decode failure as its reason.
	wallStripUnavailableSpecName = "decline-drawer-unavailable"

	// wallStripNotePath is the untracked note the unclassified and mixed
	// walls carry, relative to the worktree.
	wallStripNotePath = "notes/retraction-copy.md"
)

// wallStripWall is one fixture wall: its spec name, the files committed
// on its namesake branch, and the files then written into its worktree
// and left uncommitted (both store-relative).
type wallStripWall struct {
	name        string
	committed   map[string]string
	uncommitted map[string]string
}

// branch is the wall's namesake design branch, the one branch on which
// its domain is live.
func (w wallStripWall) branch() string {
	return "design/" + w.name
}

// writablePath is the wall's address beneath /b/, where the e2e tests
// open it.
func (w wallStripWall) writablePath() string {
	return workbench.BranchBoardHref(w.branch(), w.name)
}

// wallStripSpecRel is a wall's spec path, relative to the store.
func wallStripSpecRel(name string) string {
	return filepath.FromSlash(store.ActiveSpecRelPath(name))
}

// wallStripWalls is every fixture wall, in provisioning order.
func wallStripWalls() []wallStripWall {
	spec := func(name string) map[string]string {
		return map[string]string{wallStripSpecRel(name): wallStripSpec(name)}
	}
	note := "Draft the retraction copy for the servicing console.\n"
	return []wallStripWall{
		{
			name:      wallStripTypedSpecName,
			committed: spec(wallStripTypedSpecName),
			uncommitted: map[string]string{
				wallStripSpecRel(wallStripTypedSpecName): wallStripWithAddedCriterion(wallStripWithNewProblem(wallStripSpec(wallStripTypedSpecName))),
			},
		},
		{
			name:      wallStripUnclassifiedSpecName,
			committed: spec(wallStripUnclassifiedSpecName),
			uncommitted: map[string]string{
				wallStripSpecRel(wallStripUnclassifiedSpecName): wallStripWithEditedProse(wallStripSpec(wallStripUnclassifiedSpecName)),
				filepath.FromSlash(wallStripNotePath):           note,
			},
		},
		{
			name:      wallStripMixedSpecName,
			committed: spec(wallStripMixedSpecName),
			uncommitted: map[string]string{
				wallStripSpecRel(wallStripMixedSpecName): wallStripWithNewProblem(wallStripSpec(wallStripMixedSpecName)),
				filepath.FromSlash(wallStripNotePath):    note,
			},
		},
		{
			name:        wallStripUnreadableSpecName,
			uncommitted: spec(wallStripUnreadableSpecName),
		},
		{
			name:      wallStripEmptySpecName,
			committed: spec(wallStripEmptySpecName),
		},
		{
			name: wallStripUnavailableSpecName,
			committed: map[string]string{
				wallStripSpecRel(wallStripUnavailableSpecName):                                                    wallStripSpec(wallStripUnavailableSpecName),
				filepath.FromSlash(store.DesignProvenanceRelPath(store.ZoneActive, wallStripUnavailableSpecName)): "this line is not a design-provenance entry\n",
			},
		},
	}
}

// wallStripProblem is the committed spec's problem text, which the typed
// and mixed walls replace.
const wallStripProblem = "a retracted decline notice still stands on some channels"

// wallStripSpec is a wall's committed spec: a draft feature with a
// problem, an outcome, and one criterion, each with its body section.
func wallStripSpec(name string) string {
	return `---
id: spec/` + name + `
kind: spec
class: feature
title: "Decline retraction (` + name + `)"
status: draft
owners: [platform-team]
problem: { text: "` + wallStripProblem + `", anchor: "#problem" }
outcome: { text: "every channel retracts a stale notice together", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a stale notice is retracted on every channel", evidence: [attestation], anchor: "#ac-1" }
---
# Decline retraction (` + name + `)

## Problem

A retracted notice can still stand on a channel that missed the retraction.

## Outcome

Every channel reads the retraction on its next fetch.

## ac-1

Prose.
`
}

// wallStripWithNewProblem replaces the problem's text: one typed change.
func wallStripWithNewProblem(spec string) string {
	return strings.Replace(spec, `text: "`+wallStripProblem+`"`, `text: "a retracted decline notice still stands on the mobile channel"`, 1)
}

// wallStripWithAddedCriterion adds ac-2 to the frontmatter alone (its
// anchor is optional, so the body needs no section): one typed change.
func wallStripWithAddedCriterion(spec string) string {
	return strings.Replace(spec, "anchor: \"#ac-1\" }\n", "anchor: \"#ac-1\" }\n  - { id: ac-2, text: \"a retraction is visible to audit\", evidence: [attestation] }\n", 1)
}

// wallStripWithEditedProse rewrites the problem's body text, leaving the
// frontmatter byte-identical: a change the semantic diff does not type.
func wallStripWithEditedProse(spec string) string {
	return strings.Replace(spec, "A retracted notice can still stand on a channel that missed the retraction.", "A retracted notice still stands on any channel that missed the retraction.", 1)
}

// provisionWallStrip gives each fixture wall its writable path, as
// provisionCanvasWallBranches does for the wall-canvas walls: it cuts the
// wall's namesake branch at the serving branch's tip (no checkout moves,
// nothing is pushed), pre-cuts that branch's managed worktree through the
// seam `verdi serve` itself uses (wtmanager.EnsureWorktree), commits the
// wall's committed files there, then writes its uncommitted files beside
// them. Serve-time EnsureWorktree finds the path present and reuses it, so
// the wall renders with exactly that working tree. Call it with the
// serving branch checked out.
func provisionWallStrip(ctx context.Context, storeRoot string) error {
	for _, w := range wallStripWalls() {
		if err := runGit(ctx, storeRoot, nil, "branch", w.branch(), designBranch); err != nil {
			return fmt.Errorf("wall-strip fixture %s: cutting %s: %w", w.name, w.branch(), err)
		}
		worktree, err := wtmanager.EnsureWorktree(ctx, storeRoot, w.branch())
		if err != nil {
			return fmt.Errorf("wall-strip fixture %s: pre-cutting %s's managed worktree: %w", w.name, w.branch(), err)
		}
		if len(w.committed) > 0 {
			if err := writeWallStripFiles(worktree, w.committed); err != nil {
				return fmt.Errorf("wall-strip fixture %s: %w", w.name, err)
			}
			if err := runGit(ctx, worktree, nil, "add", "-A"); err != nil {
				return fmt.Errorf("wall-strip fixture %s: %w", w.name, err)
			}
			if err := runGit(ctx, worktree, nil, "commit", "--quiet", "--no-verify", "-m", "design: "+w.name+" (wall-strip fixture)"); err != nil {
				return fmt.Errorf("wall-strip fixture %s: %w", w.name, err)
			}
		}
		if err := writeWallStripFiles(worktree, w.uncommitted); err != nil {
			return fmt.Errorf("wall-strip fixture %s: %w", w.name, err)
		}
	}
	return nil
}

// writeWallStripFiles writes files (store-relative) beneath root.
func writeWallStripFiles(root string, files map[string]string) error {
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", filepath.Dir(rel), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", rel, err)
		}
	}
	return nil
}
