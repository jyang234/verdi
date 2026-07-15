package main

// The showcase live-draft feature fixture (public rollout design §4.3:
// "one live draft on a design branch" — the lifecycle stage the committed
// tree cannot hold, since a draft never lands on main, VL-004). The
// canonical LoanServ live draft is payoff-quote-portal (jira:LOAN-1533),
// authored on design/payoff-quote-portal and vetted as showcase content
// (design §4.1/§4.2): it is the draft a README reader is pointed at, and
// the draft-surface coverage the deletion of the old committed draft
// (new-feature-x, Task 1.3) left unrestored.
//
// Unlike provision_draftboards.go's deliberately minimal fixtures, this
// draft is exemplary and full: an object model at the showcase bar, its
// open questions worked on BOTH of VL-017's legal paths (one resolved in
// place, one carried onto the spec as a declared open_questions object),
// and a proposal-tier diagram (VL-021: derived_from a real corpus diagram
// + a well-formed sha256 digest). Every name/text below is bound by
// e2e/tests/fixtures.ts (SHOWCASE_DRAFT_*) — change them together.
//
// Rendering under /b/: the payoff draft is served from its branch's
// managed worktree (branchboard.go), and a freshly cut worktree carries no
// mutable zone (data/ is gitignored, so `git worktree add` checks out none
// of it) — its open-question STICKIES would not render. So this provisioner
// PRE-CUTS the worktree at wtmanager's deterministic path and seeds its
// mutable-zone annotation stream; at serve time EnsureWorktree finds the
// path already present and reuses it, stickies and all. The pre-cut lives
// entirely inside the gitignored data zone, so the serving checkout's
// `git status` is undisturbed (the draft-boards ac-2 invariance).
//
// Runs AFTER provisionDraftBoards, and restores the serving checkout to
// designBranch when done, so the board suite's authoring fixture is
// untouched.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/diagrambase"
)

const (
	showcaseDraftName    = "payoff-quote-portal"
	showcaseDraftBranch  = "design/" + showcaseDraftName
	showcaseDraftDiagram = "payoff-quote-flow"
	// showcaseDraftBaseDiagram is the real corpus diagram the proposal
	// derives from (diagrams/loansvc-topology.mermaid, a flowchart on main
	// and so on this branch) — VL-021 resolves derived_from.ref against it.
	showcaseDraftBaseDiagram = "loansvc-topology"

	// showcaseDraftOQCarried is oq-1's declared text AND the still-open
	// question sticky's body, byte-identical — VL-017's carried path keys
	// on that exact-text match (vl017.go carriedAsOpenQuestion).
	showcaseDraftOQCarried = "does a payoff quote's good-through date have to honor a rate lock that expires inside the quote window?"
	// showcaseDraftOQResolved is a question settled in place (a resolved
	// sticky) rather than carried — VL-017's other legal path.
	showcaseDraftOQResolved = "should the payoff quote require identity re-verification before it is shown?"
)

// showcaseDraftSpec is the exemplary draft feature spec (class feature,
// status draft, story jira:LOAN-1533). Its problem/outcome carry the
// snippets the e2e placards assert (SHOWCASE_DRAFT_PROBLEM_SNIPPET /
// _OUTCOME_SNIPPET), both ACs declare evidence kinds (VL-006/VL-020), and
// oq-1 carries showcaseDraftOQCarried verbatim (VL-017's carried path).
const showcaseDraftSpec = `---
id: spec/` + showcaseDraftName + `
kind: spec
class: feature
title: "Payoff quote portal"
status: draft
owners: [servicing-experience]
story: jira:LOAN-1533
problem: { text: "a borrower who has decided to pay off their loan cannot get a binding payoff quote in the borrower portal today: they call servicing, are read a figure over the phone, and by the time they wire funds the accrued interest has moved — so overpayments and shortfalls both land back on the servicing team as manual reconciliation", anchor: "#problem" }
outcome: { text: "a borrower requests a payoff quote in the portal and receives a figure that is good through a stated date, sourced live from the loan servicing balance and the document vault fee schedule, with no phone call and no manual reconciliation", anchor: "#outcome" }
impacts: [borrower-portal, loansvc, doc-vault]
declares:
  boundaries:
    - { from: borrower-portal, to: loansvc, via: sync }
    - { from: loansvc, to: doc-vault, via: sync }
acceptance_criteria:
  - { id: ac-1, text: "a borrower requests a payoff quote in the portal and receives a dated figure, good through a stated date, without contacting servicing", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "every quote the portal shows is reproducible from the loansvc payoff balance and the doc-vault fee schedule it was computed against", evidence: [static, behavioral], anchor: "#ac-2" }
open_questions:
  - { id: oq-1, text: "` + showcaseDraftOQCarried + `", anchor: "#oq-1" }
---
# Payoff quote portal

## Problem

A payoff is the one moment a borrower most needs an exact number and most
often gets an approximate one. The figure depends on interest accrued to the
settlement date and on fees the servicer can waive or add; read over the
phone, it is stale before the wire clears.

## Outcome

The portal computes the quote the same way the servicer would, live: the
current payoff balance from **loansvc** and the fee schedule from the
**document vault**, stamped with a good-through date the borrower can rely
on. No callback, no reconciliation ticket.

## ac-1

The borrower-facing path: request in the portal, dated quote back, no
servicing contact.

## ac-2

The auditability path: the shown quote reconstructs exactly from the balance
and fee schedule it was computed against — the same inputs, the same number.

## oq-1

Whether a payoff quote's good-through window may outlive a rate lock that
expires inside it is a pricing-policy question, not a UI one; it is carried
here for the product lead to settle before this feature accepts.
`

// showcaseDraftDiagramDoc renders the proposal-tier diagram: a class:
// proposal flowchart of the payoff-quote flow, deriving from the real
// corpus base (pinned at baseCommit). digest/source_digest are both
// well-formed sha256 (VL-021 format-checks both); source_digest is the
// REAL canonical-graph digest of the pinned base (diagrambase, the seam
// peek/reset would verify against), digest the base file body's content
// sha256.
func showcaseDraftDiagramDoc(baseCommit, digest, sourceDigest string) string {
	return `---
id: diagram/` + showcaseDraftDiagram + `
kind: diagram
class: proposal
title: "Payoff quote flow (proposed)"
status: proposed
owners: [servicing-experience]
derived_from: { ref: diagram/` + showcaseDraftBaseDiagram + `@` + baseCommit + `, digest: ` + digest + `, source_digest: ` + sourceDigest + ` }
---
graph TD
  borrower-portal -->|request: payoff quote| loansvc
  loansvc -->|read: payoff balance + accrued interest| loansvc
  loansvc -->|request: fee schedule| doc-vault
  loansvc -->|dated payoff quote, good-through| borrower-portal
`
}

// showcaseDraftAnnotations is the seeded mutable-zone annotation stream for
// the payoff draft's board (data/mutable/annotations/spec--<name>.jsonl).
// Three question/agent-task stickies board-anchored to the spec so the
// authoring wall renders them; two are VL-017's twin fixtures — a resolved
// question and an open question carried onto the spec. Their target.ref is
// pinned to the fixture commit (02 §Identity: a target ref is a pinned
// ref); VL-017 matches on kind/name, ignoring the pin. Author handles
// follow the corpus's first-name convention (spec--stale-decline.jsonl).
func showcaseDraftAnnotations(specCommit string) string {
	target := "spec/" + showcaseDraftName + "@" + specCommit
	return `{"id":"a-01J8Z0K3PAYQFFRESVEDAAAAAA","ts":"2026-07-14T09:02:00Z","author":"nadia","target":{"ref":"` + target + `","selector":{"heading":"outcome","quote":"good through a stated date","line":null}},"board":{"story":"` + showcaseDraftName + `","x":220,"y":80},"type":"question","body":"` + showcaseDraftOQResolved + `","status":"resolved"}
{"id":"a-01J8Z0K3PAYQFFCARRYEDAAAAA","ts":"2026-07-14T09:05:00Z","author":"omar","target":{"ref":"` + target + `","selector":{"heading":"oq-1","quote":"rate lock","line":null}},"board":{"story":"` + showcaseDraftName + `","x":120,"y":170},"type":"question","body":"` + showcaseDraftOQCarried + `","status":"open"}
{"id":"a-01J8Z0K3PAYQFFTASKAAAAAAAA","ts":"2026-07-14T09:08:00Z","author":"nadia","board":{"story":"` + showcaseDraftName + `","x":320,"y":210},"type":"agent-task","body":"draft the doc-vault retention note for generated payoff statements","status":"open"}
`
}

// provisionShowcaseDraft authors the payoff-quote-portal live draft on its
// own design branch, then pre-cuts and seeds the branch's managed worktree
// so its authoring board renders under /b/ with its open-question stickies.
func provisionShowcaseDraft(storeRoot string) error {
	// The pin commit and the base diagram bytes come from main, where the
	// corpus base lives — resolved before the branch is cut.
	mainSHA, err := gitOutput(storeRoot, "rev-parse", "main")
	if err != nil {
		return fmt.Errorf("resolving main HEAD for the proposal's base pin: %w", err)
	}
	baseRaw, err := os.ReadFile(filepath.Join(storeRoot, ".verdi", "diagrams", showcaseDraftBaseDiagram+".mermaid"))
	if err != nil {
		return fmt.Errorf("reading base diagram %s: %w", showcaseDraftBaseDiagram, err)
	}
	baseBody := diagramBodyBytes(baseRaw)
	sourceDigest, err := diagrambase.CanonicalGraphDigest(baseBody)
	if err != nil {
		return fmt.Errorf("computing proposal base source digest: %w", err)
	}
	sum := sha256.Sum256(baseBody)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	// Author the committed branch artifacts (spec + proposal diagram).
	if err := runGit(storeRoot, nil, "checkout", "--quiet", "-b", showcaseDraftBranch, "main"); err != nil {
		return fmt.Errorf("cutting %s: %w", showcaseDraftBranch, err)
	}
	files := map[string]string{
		filepath.Join(".verdi", "specs", "active", showcaseDraftName, "spec.md"): showcaseDraftSpec,
		filepath.Join(".verdi", "diagrams", showcaseDraftDiagram+".mermaid"):     showcaseDraftDiagramDoc(mainSHA, digest, sourceDigest),
	}
	for rel, content := range files {
		path := filepath.Join(storeRoot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", filepath.Dir(rel), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", rel, err)
		}
	}
	if err := runGit(storeRoot, nil, "add", "-A"); err != nil {
		return err
	}
	if err := runGit(storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: payoff-quote-portal live draft (showcase)"); err != nil {
		return err
	}
	// The fixture commit the stickies' target refs pin (a real commit on
	// this branch — the spec revision they annotate).
	specCommit, err := gitOutput(storeRoot, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("resolving the showcase-draft fixture commit: %w", err)
	}

	// Restore the serving checkout before touching worktrees: the branch
	// must not be checked out at the serving root for `git worktree add`.
	if err := runGit(storeRoot, nil, "checkout", "--quiet", designBranch); err != nil {
		return fmt.Errorf("restoring %s: %w", designBranch, err)
	}

	// Pre-cut the branch's managed worktree at wtmanager's deterministic
	// path (design/<name> -> .verdi/data/worktrees/<name>/), inside the
	// gitignored data zone. Serve-time EnsureWorktree reuses it.
	worktree := filepath.Join(storeRoot, ".verdi", "data", "worktrees", showcaseDraftName)
	if err := runGit(storeRoot, nil, "worktree", "add", "--quiet", worktree, showcaseDraftBranch); err != nil {
		return fmt.Errorf("pre-cutting %s worktree: %w", showcaseDraftBranch, err)
	}

	// Seed the worktree's mutable-zone annotation stream so the authoring
	// board renders its stickies (the zone is gitignored; nothing to add).
	annDir := filepath.Join(worktree, ".verdi", "data", "mutable", "annotations")
	if err := os.MkdirAll(annDir, 0o755); err != nil {
		return fmt.Errorf("creating showcase-draft annotations dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(annDir, "spec--"+showcaseDraftName+".jsonl"), []byte(showcaseDraftAnnotations(specCommit)), 0o644); err != nil {
		return fmt.Errorf("writing showcase-draft annotations: %w", err)
	}
	return nil
}

// diagramBodyBytes returns a diagram file's mermaid body — everything after
// its closing frontmatter fence — matching how diagrambase.Recover splits
// the pinned base before digesting it, so source_digest lines up.
func diagramBodyBytes(raw []byte) []byte {
	parts := strings.SplitN(string(raw), "\n---\n", 2)
	if len(parts) == 2 {
		return []byte(parts[1])
	}
	return raw
}
