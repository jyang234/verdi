package artifact

import "regexp"

// commentTokenRe is 02 §Record schemas' comment-token grammar: a forge MR
// inline comment whose body BEGINS with `[vd:<object-id>]` anchors that
// comment to the named spec object (§Object model), independent of which
// diff line it was left on. This is the single, authoritative
// implementation of that grammar, owned here beside the annotation schemas
// and the a-<ULID> helper the same 02 section defines — every consumer
// (internal/forge's gate/mcp parser and internal/workbench's board
// projection) parses through ParseCommentToken rather than re-deriving the
// pattern (CLAUDE.md: anything used by two or more packages lives in a
// shared internal/ package).
//
// UNIFICATION NOTE (W4 M-3 fix): two non-identical copies previously
// existed — internal/forge's permissive `^\[vd:([^\]\s]+)\]` and
// internal/workbench's stricter kebab `^\[vd:([a-z][a-z0-9]*-[a-z0-9]+(?:-[a-z0-9]+)*)\]`.
// This unifies on the PERMISSIVE grammar, resolving toward 02's text: 02
// defines the *token* grammar (`[vd:<object-id>]`) and separately defines
// object-id well-formedness (kebab-case with a typed prefix, R4-I-13);
// whether a parsed token names a REAL object is decided by resolving it
// against the target spec's DeclaredObjectIDs, never by the token parser.
// The workbench copy was the divergent one: it folded id-shape validation
// into the token regex, a job the grammar layer does not own — a token like
// `[vd:AC-2]` or `[vd:ac2]` is a well-formed token that simply fails to
// resolve (it routes to the inbox tray), not a non-token. Both old regexes
// already agreed on every input either accepted, because both post-filter
// the captured id against the spec's declared objects; unifying on the
// permissive form keeps that behaviour and removes the drift risk.
var commentTokenRe = regexp.MustCompile(`^\[vd:([^\]\s]+)\]`)

// ParseCommentToken extracts the object id from a comment body that BEGINS
// with a `[vd:<object-id>]` token. ok is false when the body carries no
// such leading token — the caller's cue to route the comment to the
// unanchored inbox tray (02 §Record schemas; 05 §Review stickies) rather
// than treat it as resolvable. Parsing the token grammar is all this does:
// it does not validate that the id is well-formed or names a real object —
// that is the caller's job via DeclaredObjectIDs.
func ParseCommentToken(body string) (objectID string, ok bool) {
	m := commentTokenRe.FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	return m[1], true
}
