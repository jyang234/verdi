package lint

import (
	"fmt"
	"strings"

	"github.com/OWNER/verdi/internal/gitx"
)

// vl010 enforces "frozen artifacts are immutable: any diff touching a
// frozen file fails, except a pure rename within an active→archive move"
// (02 §Lint rules), diffing Context.DiffBase..HEAD (I-14:
// merge-base(HEAD, default branch), computed by the caller). When DiffBase
// is unknown (e.g. no default branch could be established), there is
// nothing to diff against, so this rule is silent rather than guessing —
// matching VL-004's same "can't prove it" posture.
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

	frozen := frozenPathSet(in.Snapshot)

	var findings []Finding
	for _, e := range entries {
		switch e.Status {
		case "M":
			if frozen[e.Path] {
				findings = append(findings, Finding{Rule: "VL-010", Path: e.Path, Message: fmt.Sprintf("frozen file modified between %s and HEAD", in.LintCtx.DiffBase)})
			}
		case "R":
			if !frozen[e.Path] {
				continue
			}
			if e.Pure() && isActiveArchiveMove(e.OldPath, e.Path) {
				continue // 02's sole legal exception
			}
			findings = append(findings, Finding{Rule: "VL-010", Path: e.Path, Message: fmt.Sprintf("frozen file renamed from %s (not a pure active->archive move) between %s and HEAD", e.OldPath, in.LintCtx.DiffBase)})
		}
		// Added files cannot violate immutability (nothing existed before to
		// mutate). Deleted frozen files are out of this rule's tested scope
		// (see testdata/violations/VL-010, which exercises only the M case).
	}
	return findings
}

// frozenPathSet returns the RelPaths of every currently-decoded document
// that carries a frozen stamp.
func frozenPathSet(snap *Snapshot) map[string]bool {
	set := map[string]bool{}
	for _, d := range snap.Docs {
		if d.DecodeErr == nil && d.Base.Frozen != nil {
			set[d.RelPath] = true
		}
	}
	return set
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
