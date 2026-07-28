package lint

import (
	"fmt"

	"github.com/jyang234/verdi/internal/gitx"
)

// vl009 enforces "frozen artifacts carry valid frozen stamp and provenance
// where generated" (02 §Lint rules): the stamp's own shape (Frozen.Validate,
// which VL-001's DecodeStrict-only scope does not check — see doc.go's
// design note) plus, beyond what any Go-level shape check can see, that
// the stamped commit is reachable from HEAD in this repository's history.
//
// spec/evidence-resilience ac-3 (X-11b): this used to check mere object
// existence (gitx.CommitExists), a predicate a locally-dangling object —
// one that survived a rebase or a branch deletion in this exact worktree's
// object store, but that no branch or ref anywhere reaches — satisfies
// just as well as a genuinely pinned commit. A frozen.commit stamp
// pinning a commit that has already stopped being reachable is exactly
// the false green X-11 found: local lint passed while the pin no longer
// named real, retained history. Tightened to gitx.ReachableFromHEAD, which
// folds "does not exist at all" and "exists but unreachable" into the same
// honest false — closing the hole from both directions without narrowing
// what a legitimately reachable commit is allowed to be.
//
// P2-10b: gitx.ReachableFromHEAD is three-valued in a SHALLOW checkout —
// GitHub Actions' pull_request checkout is sometimes shallow even with
// fetch-depth: 0, and a genuinely-reachable frozen.commit beyond the horizon
// then reads as absent (PRs #186, #192). A would-be-negative in a shallow
// checkout is gitx.UnprovableShallow, which this rule renders as a
// disclosed-unproven NOTICE (SeverityDisclosure, VL-017's pattern: printed,
// exit-0) naming the stamp, the commit, and "shallow history cannot prove
// unreachability" — never a red — because shallow history can prove YES
// (reachable) but never a false NO. A full checkout's gitx.Unreachable still
// reds exactly as X-11b requires.
type vl009 struct{}

func (vl009) ID() string { return "VL-009" }

func (vl009) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Base.Frozen == nil {
			continue
		}

		if err := d.Base.Frozen.Validate(); err != nil {
			findings = append(findings, Finding{Rule: "VL-009", Path: d.RelPath, Message: err.Error()})
		} else {
			r, err := gitx.ReachableFromHEAD(in.Ctx, in.Root, d.Base.Frozen.Commit, "HEAD")
			switch {
			case err != nil:
				findings = append(findings, Finding{Rule: "VL-009", Path: d.RelPath, Message: fmt.Sprintf("checking frozen.commit %s: %v", d.Base.Frozen.Commit, err)})
			case r == gitx.Reachable:
				// Proven reachable — no finding.
			case r == gitx.UnprovableShallow:
				// P2-10b: this checkout is shallow, so a would-be "not reachable"
				// answer is not proof — the stamped commit can be genuinely
				// reachable in complete history yet sit beyond the horizon and
				// read as absent. Disclose it (VL-017's SeverityDisclosure
				// pattern: printed, never flips the exit) rather than redding
				// honest history content-dependently by horizon depth.
				findings = append(findings, Finding{Rule: "VL-009", Path: d.RelPath, Severity: SeverityDisclosure, Message: fmt.Sprintf("frozen.commit %s could not be proven reachable from HEAD: this checkout is shallow (git rev-parse --is-shallow-repository = true), and shallow history cannot prove unreachability — a commit genuinely reachable in complete history can sit beyond the horizon and read as absent. A full-history checkout proves it.", d.Base.Frozen.Commit)})
			default: // gitx.Unreachable — a full checkout's absence IS proof.
				findings = append(findings, Finding{Rule: "VL-009", Path: d.RelPath, Message: fmt.Sprintf("frozen.commit %s is not reachable from HEAD in this repository's history", d.Base.Frozen.Commit)})
			}
		}

		if d.Base.Provenance != nil {
			if err := d.Base.Provenance.Validate(); err != nil {
				findings = append(findings, Finding{Rule: "VL-009", Path: d.RelPath, Message: fmt.Sprintf("frozen and generated, but provenance is invalid: %v", err)})
			}
		}
	}
	return findings
}
