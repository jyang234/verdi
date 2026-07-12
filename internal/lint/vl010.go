package lint

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
)

// anyStatusLineRe / supersededStatusLineRe recognize a frontmatter status
// line and, specifically, a `status: superseded` one — the base and head
// sides of VL-010's round-5 status-only supersession exception (D-12): a
// diff that touches an otherwise-frozen spec on ONLY its status line,
// flipping it to `superseded`, is legal (the accept ritual's predecessor
// flip, cmd/verdi/accept.go), alongside the pre-existing active→archive
// rename exception.
var (
	anyStatusLineRe        = regexp.MustCompile(`^status:\s*"?[a-z][a-z-]*"?\s*$`)
	supersededStatusLineRe = regexp.MustCompile(`^status:\s*"?superseded"?\s*$`)
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
			// Round-5 exception (D-12): a status-only edit flipping the spec
			// to `superseded` is legal on an otherwise-frozen spec (the accept
			// ritual's predecessor flip). Verified by diffing the base/head
			// content and requiring exactly one changed line — the status line
			// — now reading `superseded`.
			if ok, cerr := isStatusOnlySupersededFlip(in.Ctx, in.Root, in.LintCtx.DiffBase, e.Path); cerr == nil && ok {
				continue
			}
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

// isStatusOnlySupersededFlip reports whether the change to path between
// diffBase and HEAD is exactly a status-line flip to `superseded` and
// nothing else (D-12). It reads both historical sides via `git show`,
// compares them line by line, and returns true only when precisely one line
// differs, that line is a frontmatter status line on the base, and its HEAD
// counterpart reads `status: superseded`. Any read failure is surfaced as an
// error so the caller can fall through to the ordinary frozen-modification
// finding rather than silently admitting the diff.
func isStatusOnlySupersededFlip(ctx context.Context, root, diffBase, path string) (bool, error) {
	baseContent, err := gitx.Show(ctx, root, diffBase, path)
	if err != nil {
		return false, err
	}
	headContent, err := gitx.Show(ctx, root, "HEAD", path)
	if err != nil {
		return false, err
	}
	baseLines := strings.Split(string(baseContent), "\n")
	headLines := strings.Split(string(headContent), "\n")
	if len(baseLines) != len(headLines) {
		return false, nil
	}
	diffIdx := -1
	for i := range baseLines {
		if baseLines[i] == headLines[i] {
			continue
		}
		if diffIdx != -1 {
			return false, nil // more than one line changed — not status-only
		}
		diffIdx = i
	}
	if diffIdx == -1 {
		return false, nil
	}
	return anyStatusLineRe.MatchString(baseLines[diffIdx]) && supersededStatusLineRe.MatchString(headLines[diffIdx]), nil
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
