package lint

import (
	"context"
	"fmt"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
)

// vl010 enforces "frozen artifacts are immutable: any diff touching a
// frozen file fails, except a pure rename within an active→archive move"
// (02 §Lint rules), diffing Context.DiffBase..HEAD (I-14:
// merge-base(HEAD, default branch), computed by the caller). When DiffBase
// is unknown (e.g. no default branch could be established), there is
// nothing to diff against, so this rule is silent rather than guessing —
// matching VL-004's same "can't prove it" posture.
//
// Frozen-ness is evaluated on the BASE side of the diff, not on the HEAD
// snapshot. 02's letter is "ANY diff touching a frozen file fails" — so a
// DELETION (the file is gone from HEAD entirely) and a modification that
// also STRIPS the `frozen:` stamp both have to fail, and neither is visible
// from HEAD: the base is the only side that still records the file was
// frozen before the diff.
type vl010 struct{}

func (vl010) ID() string { return "VL-010" }

func (vl010) Check(in *RunInput) []Finding {
	if in.LintCtx.DiffBase == "" {
		return nil
	}

	entries, err := gitx.DiffNameStatus(in.Ctx, in.Root, in.LintCtx.DiffBase, "HEAD")
	if err != nil {
		return []Finding{{Rule: "VL-010", Path: "", Message: fmt.Sprintf("computing diff %s..HEAD: %v", in.LintCtx.DiffBase, err)}}
	}

	var findings []Finding
	for _, e := range entries {
		// Added files cannot violate immutability (nothing existed before to
		// mutate); any status other than M/D/R is out of this rule's scope.
		switch e.Status {
		case "M", "D", "R":
		default:
			continue
		}

		// The base-side path: for M/D it is the (unchanged) path; for a
		// rename it is the pre-change OldPath — the side the base commit
		// holds, and the side whose frozen-ness governs the whole diff.
		basePath := e.Path
		if e.Status == "R" {
			basePath = e.OldPath
		}

		frozen, err := baseFrozen(in.Ctx, in.Root, in.LintCtx.DiffBase, basePath)
		if err != nil {
			// The diff named this path as changed on the base side, so its
			// base content must be readable; a failure here is operational,
			// surfaced as a finding (fail closed) rather than swallowed.
			findings = append(findings, Finding{Rule: "VL-010", Path: basePath, Message: fmt.Sprintf("reading base-side content of %s at %s: %v", basePath, in.LintCtx.DiffBase, err)})
			continue
		}
		if !frozen {
			continue
		}

		switch e.Status {
		case "M":
			findings = append(findings, Finding{Rule: "VL-010", Path: e.Path, Message: fmt.Sprintf("frozen file modified between %s and HEAD", in.LintCtx.DiffBase)})
		case "D":
			findings = append(findings, Finding{Rule: "VL-010", Path: e.Path, Message: fmt.Sprintf("frozen file deleted between %s and HEAD", in.LintCtx.DiffBase)})
		case "R":
			if e.Pure() && isActiveArchiveMove(e.OldPath, e.Path) {
				continue // 02's sole legal exception
			}
			findings = append(findings, Finding{Rule: "VL-010", Path: e.Path, Message: fmt.Sprintf("frozen file renamed from %s (not a pure active->archive move) between %s and HEAD", e.OldPath, in.LintCtx.DiffBase)})
		}
	}
	return findings
}

// baseFrozen reports whether the file at basePath carried a `frozen:` stamp
// as it existed at diffBase — the base side of the diff. It reads the
// historical content via `git show` and probes only the frontmatter's
// `frozen:` key through artifact.ProbeFrozen, the deliberately-tolerant
// historical-content probe (see its doc for why strict decode would be
// wrong here). Anything ProbeFrozen cannot probe at all — a non-markdown
// file, absent or unparseable frontmatter — reads as "not frozen": a
// non-artifact file cannot carry a stamp. Only the `git show` failure is an
// error: the diff itself named this path as existing on the base side.
func baseFrozen(ctx context.Context, root, diffBase, basePath string) (bool, error) {
	content, err := gitx.Show(ctx, root, diffBase, basePath)
	if err != nil {
		return false, err
	}
	frozen, err := artifact.ProbeFrozen(content)
	if err != nil {
		return false, nil // not a probeable markdown artifact ⇒ not frozen
	}
	return frozen != nil, nil
}

// isActiveArchiveMove reports whether oldPath -> newPath is a spec
// directory moving from specs/active/<name>/ to specs/archive/<name>/ with
// the same name and the same tail path (01 §Directory layout: "an
// active→archive move changes the path but never the ref").
func isActiveArchiveMove(oldPath, newPath string) bool {
	const activePrefix = ".verdi/specs/active/"
	const archivePrefix = ".verdi/specs/archive/"
	if !strings.HasPrefix(oldPath, activePrefix) || !strings.HasPrefix(newPath, archivePrefix) {
		return false
	}
	return strings.TrimPrefix(oldPath, activePrefix) == strings.TrimPrefix(newPath, archivePrefix)
}
