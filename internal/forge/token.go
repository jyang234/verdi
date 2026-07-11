package forge

import "regexp"

// commentTokenRe matches a body beginning with `[vd:<object-id>]` (02
// §Record schemas' comment-token grammar). Object ids are kebab-case with
// a typed prefix (R4-I-13: `ac-`/`co-`/`dc-`/`oq-`), but this parses the
// token grammar only, not id well-formedness or whether the id names a
// real object on any particular spec: this package has no access to spec
// content, and a resolved token's id is validated against the target
// spec's own declared objects by the caller (artifact.DeclaredObjectIDs),
// never here.
var commentTokenRe = regexp.MustCompile(`^\[vd:([^\]\s]+)\]`)

// ParseCommentToken extracts the object id from a comment body beginning
// with a `[vd:<object-id>]` token — forge-agnostic (05: "identical on
// GitLab and GitHub"), so parsing lives here once rather than in every
// consumer (cmd/verdi/gate_threads.go, internal/mcpserve) — CLAUDE.md:
// shared code lives in a shared internal/ package. ok is false if body
// carries no such prefix — the caller's cue to route the comment to the
// unanchored inbox tray (05 §Review stickies and forge round-trip) rather
// than treat it as resolvable.
func ParseCommentToken(body string) (objectID string, ok bool) {
	m := commentTokenRe.FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	return m[1], true
}
