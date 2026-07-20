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
			ok, err := gitx.ReachableFromHEAD(in.Ctx, in.Root, d.Base.Frozen.Commit, "HEAD")
			if err != nil {
				findings = append(findings, Finding{Rule: "VL-009", Path: d.RelPath, Message: fmt.Sprintf("checking frozen.commit %s: %v", d.Base.Frozen.Commit, err)})
			} else if !ok {
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
