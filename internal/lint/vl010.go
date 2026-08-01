package lint

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// The status-line regexes below recognize a frontmatter status line and,
// specifically, the base/head sides of VL-010's two status-only exceptions on
// an otherwise-frozen spec:
//
//   - Round-5 supersession (D-12): a diff that touches an otherwise-frozen
//     spec IN PLACE on ONLY its status line, flipping it to `superseded`, is
//     legal (the accept ritual's predecessor flip, cmd/verdi/accept.go).
//   - Round-6 closure (D6-11): a spec.md moving specs/active→specs/archive
//     while its status line flips `accepted-pending-build`→`closed` and
//     nothing else changes is legal (the close ritual's archive step,
//     cmd/verdi/close.go, implementing 02 §Kind registry's `… → closed(archive)`
//     transition). The move is no longer the byte-identical R100 rename the
//     pure-rename exception covers, so this narrower exception admits it.
//
// Both reuse one line-diff core (statusOnlyFlip): exactly one changed line,
// that line a status line matching the expected base pattern, its head
// counterpart matching the expected terminal status.
// rootStorePrefix is the slash-path prefix of every artifact in the root
// store's own .verdi/ tree. VL-010's diff sweep is scoped to it so a
// whole-repo git diff never treats a nested/fixture store's frozen-stamped
// files (which always sit behind a directory prefix, e.g.
// examples/showcase/.verdi/…) as this store's frozen artifacts (R4-I-52).
const rootStorePrefix = ".verdi/"

var (
	anyStatusLineRe             = regexp.MustCompile(`^status:\s*"?[a-z][a-z-]*"?\s*$`)
	acceptedPendingStatusLineRe = regexp.MustCompile(`^status:\s*"?accepted-pending-build"?\s*$`)
	supersededStatusLineRe      = regexp.MustCompile(`^status:\s*"?superseded"?\s*$`)
	closedStatusLineRe          = regexp.MustCompile(`^status:\s*"?closed"?\s*$`)
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

		// VL-010 governs THIS store's frozen artifacts, which live under the
		// root store's own .verdi/ tree — exactly the subtree walk.go's
		// walkDocuments walks and every other (snapshot-based) rule is scoped
		// to. A whole-repo git diff also surfaces NESTED stores (a committed
		// fixture store such as examples/showcase, which carries its own
		// .verdi/verdi.yaml) and partial fixture overlays (testdata/violations/
		// **/.verdi/…), whose frozen-stamped files are fixtures, not this
		// store's artifacts; a nested .verdi/ tree always sits behind a
		// directory prefix, so the root store's own tree is exactly the diff
		// paths beginning ".verdi/". Restricting the sweep here keeps VL-010's
		// frozen-immutability reading faithful to the root store without
		// widening it to every .verdi/-shaped fixture the repo happens to
		// carry (R4-I-52).
		if !strings.HasPrefix(basePath, rootStorePrefix) {
			continue
		}

		protected, err := baseProtected(in.Ctx, in.Root, in.LintCtx.DiffBase, basePath)
		if err != nil {
			// The diff named this path as changed on the base side, so its
			// base content must be readable; a failure here is operational,
			// surfaced as a finding (fail closed) rather than swallowed.
			findings = append(findings, Finding{Rule: "VL-010", Path: basePath, Message: fmt.Sprintf("reading base-side content of %s at %s: %v", basePath, in.LintCtx.DiffBase, err)})
			continue
		}
		if !protected {
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
			if isActiveArchiveMove(e.OldPath, e.Path) {
				// Byte-identical move (the other quartet members, and any spec
				// already at status: closed before the move).
				if e.Pure() {
					continue
				}
				// Round-6 (D6-11): spec.md's status-only accepted-pending-build
				// →closed flip performed AS the archive move (cmd/verdi/close.go).
				// The only content change the move may carry; anything else fails.
				if ok, cerr := isStatusOnlyClosedArchiveFlip(in.Ctx, in.Root, in.LintCtx.DiffBase, e.OldPath, e.Path); cerr == nil && ok {
					continue
				}
			}
			// vocab:identity — status-flip grammar naming state enum VALUES (id-level transition record)
			findings = append(findings, Finding{Rule: "VL-010", Path: e.Path, Message: fmt.Sprintf("frozen file renamed from %s (not a pure active->archive move, nor a status-only accepted-pending-build->closed archive flip) between %s and HEAD", e.OldPath, in.LintCtx.DiffBase)})
		}
	}
	return findings
}

// baseProtected reports whether the file at basePath is IMMUTABLE as it
// existed at diffBase — the base side of the diff — under either of two
// independent signals (Task 4, the merge-signaled acceptance design's
// immutability keystone):
//
//  1. A `frozen:` stamp (the legacy signal, unchanged, renamed from
//     baseFrozen): probed tolerantly through artifact.ProbeFrozen, reading
//     ONLY the frontmatter's `frozen:` key (see its own doc for why strict
//     decode would be wrong for HISTORICAL content).
//  2. A strict-decoded feature or story spec — REGARDLESS of any frozen
//     stamp, or of a persisted status at all. Acceptance is now Git-derived
//     (internal/specstate): a feature/story spec.md that strict-decodes
//     cleanly at the diff base — the base being, by construction, either
//     the default branch tip or an ancestor of it — is an artifact this
//     store already treats as merge-accepted, whether or not it happens to
//     carry a frozen: stamp (new merge-accepted artifacts need none, per
//     the design's "Artifact compatibility": "do not require a
//     content-changing frozen stamp"). Without this second signal, a
//     statusless, unstamped feature/story that already landed on the
//     default branch could be freely edited or deleted by a later diff —
//     exactly the immutability gap this rule exists to close.
//
// A component spec strict-decoding cleanly is deliberately NOT protected
// by signal 2: component specs are "authored-living and never frozen" (01
// §Temporal classes; artifact/spec.go's validateComponent) — their only
// immutability signal remains an explicit frozen: stamp (signal 1),
// exactly as before Task 4.
//
// Anything neither ProbeFrozen nor DecodeSpec can make sense of — a
// non-markdown file, unparseable frontmatter, a non-spec artifact, a spec
// that fails strict validation — reads as "not protected": VL-010 governs
// frozen artifacts and Git-derived feature/story acceptance, not every
// file in the store. Only the `git show` failure is an error: the diff
// itself named this path as existing on the base side.
func baseProtected(ctx context.Context, root, diffBase, basePath string) (bool, error) {
	content, err := gitx.Show(ctx, root, diffBase, basePath)
	if err != nil {
		return false, err
	}
	if frozen, probeErr := artifact.ProbeFrozen(content); probeErr == nil && frozen != nil {
		return true, nil
	}
	// artifact.DecodeSpec expects frontmatter-only bytes (every real call
	// site splits first — internal/lint/walk.go's decodeDocument,
	// internal/specstate/resolve.go's own scanSuccessors); content here is
	// the FULL historical file (frontmatter delimiters plus body).
	if rawFM, _, splitErr := artifact.SplitFrontmatter(content); splitErr == nil {
		if fm, decodeErr := artifact.DecodeSpec(rawFM); decodeErr == nil {
			if fm.Class == artifact.ClassFeature || fm.Class == artifact.ClassStory {
				return true, nil
			}
		}
	}
	return false, nil
}

// isStatusOnlySupersededFlip reports whether the change to path between
// diffBase and HEAD is exactly a status-line flip to `superseded` and
// nothing else (D-12) — the round-5 in-place supersession exception (path
// unchanged on both sides). Any read failure is surfaced as an error so the
// caller can fall through to the ordinary frozen-modification finding rather
// than silently admitting the diff.
func isStatusOnlySupersededFlip(ctx context.Context, root, diffBase, path string) (bool, error) {
	baseContent, err := gitx.Show(ctx, root, diffBase, path)
	if err != nil {
		return false, err
	}
	headContent, err := gitx.Show(ctx, root, "HEAD", path)
	if err != nil {
		return false, err
	}
	return statusOnlyFlip(baseContent, headContent, anyStatusLineRe, supersededStatusLineRe), nil
}

// isStatusOnlyClosedArchiveFlip reports whether the rename oldPath→newPath
// between diffBase and HEAD carries exactly one content change: the spec's
// status line flipping `accepted-pending-build`→`closed` (D6-11) — the
// round-6 archive-move exception (the base holds the active/ copy at oldPath,
// HEAD the archive/ copy at newPath). Any read failure is surfaced as an
// error so the caller falls through to the ordinary finding.
func isStatusOnlyClosedArchiveFlip(ctx context.Context, root, diffBase, oldPath, newPath string) (bool, error) {
	baseContent, err := gitx.Show(ctx, root, diffBase, oldPath)
	if err != nil {
		return false, err
	}
	headContent, err := gitx.Show(ctx, root, "HEAD", newPath)
	if err != nil {
		return false, err
	}
	return statusOnlyFlip(baseContent, headContent, acceptedPendingStatusLineRe, closedStatusLineRe), nil
}

// statusOnlyFlip is the shared core of VL-010's two status-only exceptions
// (D-12 superseded, D6-11 closed): it compares baseContent and headContent
// line by line and returns true only when precisely one line differs, that
// line matches baseStatusRe on the base, and its head counterpart matches
// headStatusRe. Everything else must be byte-identical — the flip is the sole
// admissible content change on an otherwise-frozen spec.
func statusOnlyFlip(baseContent, headContent []byte, baseStatusRe, headStatusRe *regexp.Regexp) bool {
	baseLines := strings.Split(string(baseContent), "\n")
	headLines := strings.Split(string(headContent), "\n")
	if len(baseLines) != len(headLines) {
		return false
	}
	diffIdx := -1
	for i := range baseLines {
		if baseLines[i] == headLines[i] {
			continue
		}
		if diffIdx != -1 {
			return false // more than one line changed — not status-only
		}
		diffIdx = i
	}
	if diffIdx == -1 {
		return false
	}
	return baseStatusRe.MatchString(baseLines[diffIdx]) && headStatusRe.MatchString(headLines[diffIdx])
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
