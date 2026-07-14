package forge

import "github.com/jyang234/verdi/internal/artifact"

// ParseCommentToken extracts the object id from a comment body beginning
// with a `[vd:<object-id>]` token — forge-agnostic (05: "identical on
// GitLab and GitHub"), so gate (cmd/verdi/gate_threads.go) and mcp
// (internal/mcpserve) reach the grammar through this one forge-facing
// entry point. The grammar itself lives once in internal/artifact
// (02 §Record schemas' owner, beside the annotation schemas) — this
// delegates there rather than carrying a second copy (W4 M-3: the two
// non-identical regexes are unified on artifact.ParseCommentToken). ok is
// false if body carries no such leading token — the caller's cue to route
// the comment to the unanchored inbox tray (05 §Review stickies and forge
// round-trip) rather than treat it as resolvable.
func ParseCommentToken(body string) (objectID string, ok bool) {
	return artifact.ParseCommentToken(body)
}
