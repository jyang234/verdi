package upstream

import (
	"fmt"
	"strings"
)

// CheckToolPin verifies that tool — a `flowmap`/`groundwork` pseudo-version
// string as recorded in a Graph's Tool field or a `<bin> version` output,
// e.g. "v0.0.0-20260707202836-cd38b1a56bb7" — was built from pinnedCommit
// (verdi.yaml's toolchain.commit). It compares the pseudo-version's
// trailing 12-hex-character revision segment against a same-length prefix
// of pinnedCommit (Go pseudo-versions always truncate the revision to 12
// hex characters, so pinnedCommit's usual 40-character form is compared by
// prefix).
//
// This is I-4's posture in code: "strict decode is the primary defense
// (constitution 5); additionally ... refuse a bundle whose recorded tool
// differs from the pinned commit."
func CheckToolPin(tool, pinnedCommit string) error {
	if tool == "" {
		return fmt.Errorf("upstream: CheckToolPin: recorded tool string is empty")
	}
	if pinnedCommit == "" {
		return fmt.Errorf("upstream: CheckToolPin: pinned commit is empty")
	}

	idx := strings.LastIndex(tool, "-")
	if idx < 0 {
		return fmt.Errorf("upstream: CheckToolPin: %q does not look like a pseudo-version (no '-' separator)", tool)
	}
	rev := tool[idx+1:]
	if rev == "" {
		return fmt.Errorf("upstream: CheckToolPin: %q does not look like a pseudo-version (empty revision segment)", tool)
	}

	pinned := strings.ToLower(pinnedCommit)
	rev = strings.ToLower(rev)
	n := len(rev)
	if n > len(pinned) {
		n = len(pinned)
	}
	if rev[:n] != pinned[:n] {
		return fmt.Errorf("upstream: recorded tool %q was built from commit %q, which does not match the pinned commit %q (I-4)", tool, rev, pinnedCommit)
	}
	return nil
}
