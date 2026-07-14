package artifact

import "strings"

// ClassifyPath maps a .verdi/-relative slash path to the artifact kind it
// should decode as, per 01 §Directory layout's per-kind directories and 02
// §Kind registry's Dir column (spec/shared-homes ac-4, dc-3). It returns
// ok=false for every file that is not itself an indexed/lintable artifact
// — verdi.yaml, .gitignore, data/, and specs/*/*/{board.json,rollup.json,
// deviation-report.md} companion files.
//
// This is the ONE home for the classification table. It was previously
// hand-copied in internal/lint/walk.go and internal/index/walk.go, and the
// copies had already diverged: index's copy omitted the "reaffirmations/"
// case (both in classification and in its decodeEntry counterpart) while
// lint's own knownTopLevelEntries comment claimed the tables mirrored each
// other — the exact bug class that comment was written to memorialize.
// Both walks now call this function directly; they remain separate
// functions because their failure handling legitimately differs (lint
// tolerates and records a per-file decode error, index aborts the whole
// walk on the first bad file).
func ClassifyPath(rel string) (kind string, ok bool) {
	switch {
	case strings.HasPrefix(rel, "adr/") && strings.HasSuffix(rel, ".md"):
		return "adr", true
	case strings.HasPrefix(rel, "diagrams/") && strings.HasSuffix(rel, ".mermaid"):
		return "diagram", true
	case strings.HasPrefix(rel, "attestations/") && strings.HasSuffix(rel, ".md"):
		return "attestation", true
	case strings.HasPrefix(rel, "waivers/") && strings.HasSuffix(rel, ".md"):
		return "waiver", true
	case strings.HasPrefix(rel, "conflicts/") && strings.HasSuffix(rel, ".md"):
		return "conflict", true
	case strings.HasPrefix(rel, "reaffirmations/") && strings.HasSuffix(rel, ".md"):
		return "reaffirmation", true
	case strings.HasPrefix(rel, "obligations/") && strings.HasSuffix(rel, ".md"):
		return "obligation", true
	case (strings.HasPrefix(rel, "specs/active/") || strings.HasPrefix(rel, "specs/archive/")) &&
		strings.HasSuffix(rel, "/spec.md"):
		return "spec", true
	default:
		return "", false
	}
}
