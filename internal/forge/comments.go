// Comment-thread round-trip (V1-P7): three port methods list an MR/PR's
// full comment feed, post a token-bearing comment, and query per-thread
// resolution state — 05 §Review stickies and forge round-trip. Semantics
// (token grammar, round-trip behavior, thread-resolution readiness) are
// spec-fixed; the exact Go shapes below are this phase's invention — PLAN-V1
// §3 flagged the gap explicitly ("the exact Go method signatures are not
// spec-fixed... recorded as a phase-review invention ledger entry during
// V1-P7") and V1-P0's spike S6
// (docs/spikes/v1/spike-s6-findings.md, binding) prototyped both adapters'
// real APIs first. Ledgered at this phase's review as R4-I-26
// (PLAN-V1.md §7).
//
// Two comment universes exist on both forges (S6 finding): diff-anchored
// comments (GitHub `pulls/comments`, GitLab `DiffNote`) and general/thread
// comments (GitHub `issues/comments`, GitLab individual notes).
// ListComments merges both into the one feed 05 says the board "pulls...
// on every render" — classification into anchored-by-token vs unanchored
// (the inbox tray split) is the CALLER's job (ParseCommentToken, token.go),
// never the port's: the port returns the raw feed, unfiltered, so nothing
// is ever silently dropped before the caller even sees it.
//
// Resolution state is queried separately (GetThreadResolution) because its
// transport is NOT comparable across forges (S6 Q2, live-verified on
// GitHub): GitHub exposes `isResolved` only via a GraphQL `reviewThreads`
// query — REST `pulls/comments` carries no resolution field at all — while
// GitLab (doc-derived, UNVERIFIED against live — see this package's own
// adapters' doc comments) exposes `resolved`/`resolvable` directly on the
// REST Note. Both adapters hide this transport split entirely: the port
// never leaks GraphQL vs REST, `isResolved` vs `resolved`, or any other
// forge-native field name — only the derived shapes below.
package forge

// Comment is one entry in an MR/PR's full comment feed (ListComments).
// Body is byte-identical to what the forge stored — S6 Q3, live-verified
// on GitHub across post/resolve/push/force-push — so a `[vd:<object-id>]`
// token prefix (02 §Record schemas' comment-token grammar; ParseCommentToken)
// survives the round-trip untouched. Path/Line are display hints only (05:
// "position is derived at render time, never encoded in the token"): both
// are zero for a general (non-diff) comment, and Line is zero for a diff
// comment whose position was lost to a force-push/history rewrite (S6 Q4)
// even when Path is still known.
type Comment struct {
	// ID is the forge-native comment id, string-rendered (GitHub numeric
	// id, GitLab note id) — never compared across forges, only used to key
	// a fetched comment back to itself.
	ID string
	// ThreadID groups comments belonging to the same substantive thread —
	// the key ThreadResolution.ThreadID matches. "" for a comment that
	// belongs to no resolvable thread at all (a general/individual note;
	// the "two comment universes" finding above) — such a comment can
	// never gate on resolution and is exactly the inbox-tray population
	// once its token also fails to resolve.
	ThreadID string
	// Body is the raw comment text, untouched by this package.
	Body string
	// Author is the commenting user's forge-native handle.
	Author string
	// CreatedAt is the forge-reported creation timestamp, RFC3339, carried
	// through for disclosure/ordering only — never parsed or compared by
	// this package.
	CreatedAt string
	// Path is the commented file's path, "" for a general (non-diff)
	// comment.
	Path string
	// Line is the commented line in the current diff, best-effort:
	// forge-native re-anchoring across ordinary pushes (S6 Q4) may move
	// it; a force-push/history-rewrite that breaks the original commit's
	// ancestry zeroes it even when Path is still known. Display hint
	// only — never the token's anchor.
	Line int
}

// CommentTarget names where PostComment attaches a new comment in the
// diff. A nil target posts a general/thread comment, not anchored to any
// line — the durable `[vd:<object-id>]` token in the posted body is the
// real anchor either way (05: "position is derived at render time, never
// encoded in the token").
type CommentTarget struct {
	Path string
	Line int
}

// ThreadResolution is one substantive thread's resolution state (05
// §Review stickies and forge round-trip's "thread-resolution readiness...
// forge-native resolution state, deterministic on both GitLab and
// GitHub"). Both adapters return an entry ONLY for threads their forge
// itself treats as resolvable — GitHub: every GraphQL `reviewThreads` node
// (GitHub's model has no other kind of comment thread at all); GitLab:
// discussions whose notes carry `resolvable: true` per docs — excluding
// plain individual notes (the "two comment universes" finding above),
// which appear in ListComments' feed but never here. This is where
// "substantive" (05's own word) is operationalized: a thread this port
// surfaces here at all is substantive by construction; one that never
// appears here (a bare conversational comment) is not, and the merge gate
// (cmd/verdi/gate_threads.go) never blocks on it.
type ThreadResolution struct {
	// ThreadID matches every Comment.ThreadID belonging to this thread.
	ThreadID string
	// Resolved is the forge-native resolved state.
	Resolved bool
	// ResolvedBy is the resolving user's handle, "" if unresolved or the
	// forge did not report one.
	ResolvedBy string
}
