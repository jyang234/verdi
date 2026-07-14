package main

// The diagram editor's fixture provisioning (spec/board-editor co-4:
// "Playwright e2e under e2e/ drives the built binary against the vendored
// asset only"): class: proposal diagram artifacts on the design branch —
// a from-scratch proposal within the op grammar's flowchart subset (the
// ops/rail/save journeys), one outside it (the disclosed-unavailable
// journey), a pinned base plus a derived proposal whose derived_from
// digest is computed with the REAL formula (internal/diagrambase — the
// same seam the server verifies with), and a corrupted-digest twin (the
// disclosed-failure journey). Every name and body below is bound by
// e2e/tests/fixtures.ts — change them together.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/diagrambase"
)

const (
	diagramProposalName   = "editor-proposal"
	diagramOutsideOpsName = "editor-illustrative-ops"
	diagramBaseName       = "editor-base-topology"
	diagramDerivedName    = "editor-derived"
	diagramCorruptName    = "editor-derived-corrupt"
)

// diagramProposalBody is DIAGRAM_PROPOSAL_BODY (fixtures.ts): within the
// dc-2 op subset, with a comment and a blank line so byte preservation
// is observable through the page.
const diagramProposalBody = `flowchart TD
  loansvc["Loan service"]
  billing["Billing"]
  %% drafted on the wall
  loansvc --> billing
`

const diagramProposal = `---
id: diagram/` + diagramProposalName + `
kind: diagram
class: proposal
title: "Editor proposal"
status: proposed
owners: [platform-team]
---
` + diagramProposalBody

// diagramOutsideOps is a renderer-legal proposal OUTSIDE the op grammar's
// flowchart subset: the structural ops must be disclosed unavailable
// while the code pane stays live (ac-2).
const diagramOutsideOps = `---
id: diagram/` + diagramOutsideOpsName + `
kind: diagram
class: proposal
title: "Sequence sketch"
status: proposed
owners: [platform-team]
---
sequenceDiagram
  Applicant->>LoanSvc: apply
  LoanSvc->>Applicant: decline
`

// diagramBaseBody is DIAGRAM_BASE_BODY (fixtures.ts): the pinned base
// reset must reproduce byte-for-byte.
const diagramBaseBody = `graph TD
  loansvc --> notification-svc
  loansvc --> charge-svc
`

const diagramBase = `---
id: diagram/` + diagramBaseName + `
kind: diagram
title: "Editor base topology"
status: active
owners: [platform-team]
---
` + diagramBaseBody

// diagramDerivedBody is the derived proposal's working delta over the
// base: one extra proposed edge, so reset visibly discards something.
const diagramDerivedBody = diagramBaseBody + `  loansvc --> audit-svc
`

// cannedDiagramVerification is the rail's hermetic report
// (obligation ac-5--behavioral: "a canned verification report supplied
// through the dc-4 consumer port ... never a live flowmap run") for
// DIAGRAM_PROPOSAL only — every other editor fixture has no report, so
// its rail renders the disclosed verification-unavailable state.
const cannedDiagramVerification = `{
  "` + diagramProposalName + `": {
    "tier": "partial",
    "findings": [
      {"identity": "loansvc", "kind": "exists"},
      {"identity": "billing", "kind": "proposed-new"},
      {"identity": "audit-log", "kind": "contradicted", "witness": "abc1234"},
      {"identity": "charge-svc", "kind": "stale-base"}
    ]
  }
}
`

// runGitOut runs git in dir and returns its trimmed stdout — the output
// sibling of git.go's runGit, for the one provisioning step that needs a
// value back (resolving the base-pinning commit).
func runGitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), deterministicGitEnv...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v: %w", args, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// provisionDiagrams writes the editor fixtures onto the ALREADY CHECKED
// OUT design branch (provisionBoard leaves the store there) in two
// commits: the base and the un-derived proposals first, then — with that
// commit's SHA in hand — the derived proposals pinning it, their digests
// computed via the same diagrambase formula the server verifies with (a
// matching one for the good twin, a fixed corrupted one for the other).
// Returns the canned verification report's file path for the serve
// subprocess's env (VERDI_DIAGRAM_VERIFICATION).
func provisionDiagrams(scratch, storeRoot string) (verificationPath string, err error) {
	writeFiles := func(files map[string]string) error {
		for rel, content := range files {
			path := filepath.Join(storeRoot, rel)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("creating %s: %w", filepath.Dir(rel), err)
			}
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", rel, err)
			}
		}
		return nil
	}

	if err := writeFiles(map[string]string{
		filepath.Join(".verdi", "diagrams", diagramProposalName+".mermaid"):   diagramProposal,
		filepath.Join(".verdi", "diagrams", diagramOutsideOpsName+".mermaid"): diagramOutsideOps,
		filepath.Join(".verdi", "diagrams", diagramBaseName+".mermaid"):       diagramBase,
	}); err != nil {
		return "", err
	}
	if err := runGit(storeRoot, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: diagram editor fixtures (base + proposals)"); err != nil {
		return "", err
	}
	baseCommit, err := runGitOut(storeRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}

	digest, err := diagrambase.CanonicalGraphDigest([]byte(diagramBaseBody))
	if err != nil {
		return "", fmt.Errorf("computing base digest: %w", err)
	}
	derived := func(name, digest string) string {
		return `---
id: diagram/` + name + `
kind: diagram
class: proposal
title: "Derived target topology"
status: proposed
owners: [platform-team]
derived_from: { ref: diagram/` + diagramBaseName + `@` + baseCommit + `, digest: ` + digest + ` }
---
` + diagramDerivedBody
	}
	const corruptedDigest = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	if err := writeFiles(map[string]string{
		filepath.Join(".verdi", "diagrams", diagramDerivedName+".mermaid"): derived(diagramDerivedName, digest),
		filepath.Join(".verdi", "diagrams", diagramCorruptName+".mermaid"): derived(diagramCorruptName, corruptedDigest),
	}); err != nil {
		return "", err
	}
	if err := runGit(storeRoot, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: derived diagram proposals pinning the base"); err != nil {
		return "", err
	}
	if err := runGit(storeRoot, nil, "push", "--quiet"); err != nil {
		return "", err
	}

	verificationPath = filepath.Join(scratch, "diagram-verification.json")
	if err := os.WriteFile(verificationPath, []byte(cannedDiagramVerification), 0o644); err != nil {
		return "", fmt.Errorf("writing canned diagram verification: %w", err)
	}
	return verificationPath, nil
}
