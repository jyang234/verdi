# Spike S6 findings — review-sticky forge round-trip

**Scope:** de-risks PLAN-V1.md §5 Phase V1-P0 for V1-P6/V1-P7 (`[vd:<object-id>]`
token grammar, 05-surfaces.md §Review stickies and forge round-trip;
02-artifact-contract.md §Record schemas "Comment-token grammar").
This is a spike: findings and captured JSON only, no shipped code, verdi/
untouched.

## Auth check (method step 1)

- `gh auth status`: **authenticated** — `github.com` account `jyang234`,
  token scopes `gist, read:org, repo, workflow`. No `delete_repo` scope
  (see cleanup note below).
- `glab auth status`: **glab is not installed** (`command not found: glab`;
  confirmed available via `brew info glab` but not present, and no
  `GITLAB_TOKEN`/similar in env). No working GitLab auth existed in this
  environment. Per method step 3, GitLab findings below are **doc-derived,
  UNVERIFIED against live** — never presented as captures.

**Live-verified: GitHub only. GitLab: doc-derived/unverified only.**

## Q1 — real JSON shapes for listing + posting inline comments

**GitHub (live-verified).** Two parallel comment systems exist and matter
for the port:

- Inline/diff comments: `GET/POST /repos/{owner}/{repo}/pulls/{pr}/comments`
  (REST). List and post use the same shape. Key fields: `id`, `body`,
  `path`, `commit_id`, `original_commit_id`, `line`, `original_line`,
  `side`, `position`, `original_position`, `pull_request_review_id`,
  `diff_hunk`. Posting requires `body`, `commit_id` (head sha), `path`,
  `line`, `side`.
- General/issue-level comments (not diff-anchored):
  `GET/POST /repos/{owner}/{repo}/issues/{pr}/comments` — much flatter
  shape, no `path`/`line`/`position` at all.
- Resolution state (`isResolved`) is **not present in the REST shape at
  all** — see Q2.

Captures: `captures/github/01-list-review-comments-REST.json` (post-both
comments), `02-list-issue-comments-REST.json`.

**GitLab (doc-derived, unverified).** From `docs.gitlab.com/ee/api/discussions.html`:
list/create both return a `Discussion` object wrapping one or more `Note`s.
Diff-anchored notes are `type: DiffNote` and carry a `position` object with
`base_sha`/`start_sha`/`head_sha`, `old_path`/`new_path`, `old_line`/`new_line`,
and a `line_range` keyed by `line_code` (a hash, not a bare integer offset
like GitHub's `position`). Posting requires `body` plus a full `position`
hash — the caller must first fetch the MR's `diff_refs` (three shas) before
posting, a heavier precondition than GitHub's single `commit_id`.
Captures: `captures/gitlab/01-doc-derived-UNVERIFIED-list-discussions.json`,
`02-doc-derived-UNVERIFIED-post-discussion-request.json`.

## Q2 — comparable resolution-state shapes, or per-adapter normalization?

**Per-adapter normalization is required — confirmed live on GitHub, and the
REST/GraphQL split is a real, verified finding, not a hypothesis.**

- **GitHub:** REST `pulls/comments` has **no resolution field whatsoever**.
  Resolution (`isResolved`, `isOutdated`, `resolvedBy`) exists **only** via
  the GraphQL `reviewThreads` query (`PullRequest.reviewThreads.nodes[]`),
  and resolving a thread requires the GraphQL `resolveReviewThread`
  mutation (there is no REST equivalent). Verified live: queried
  `reviewThreads` before/after calling `resolveReviewThread` — see
  `captures/github/03-...before-resolve.json` and
  `04-...after-resolve.json` (`isResolved: false` → `true`,
  `resolvedBy: {"login":"jyang234"}`).
- **GitLab (doc-derived):** `resolved`/`resolvable`/`resolved_by`/`resolved_at`
  appear to be plain fields on the REST `Note` object itself (list, create,
  and the `PUT .../discussions/:id?resolved=true` response) — no GraphQL
  detour documented as required. **This is the doc-derived, unverified
  half** — GitLab's docs do not show a live capture, so this asymmetry
  (GitHub needs GraphQL, GitLab apparently doesn't) is disclosed as
  plausible-per-docs, not proven.

**Finding, stated plainly:** if GitLab's REST resolution field holds up
under live verification in V1-P7, the port's minimal contract is still
forced to the GraphQL-shaped lower common denominator on GitHub — i.e. the
GitHub adapter needs a GraphQL client/query path, not just REST, purely to
read/write thread resolution. This is a real adapter-asymmetry cost, not a
convenience the port can skip.

## Q3 — does a `[vd:ac-2]`-prefixed body survive byte-identical in the list response?

**Yes, confirmed live on GitHub, in every state tested** including after
the thread was resolved and after the comment became fully outdated/orphaned
(line nulled). Posted body:
`[vd:ac-2] outcome AC reads implementation-scoped — reword?` (note the
em-dash). Re-listed via REST after posting, after a line-shifting push,
after a content-rewriting push, and after a force-push that broke commit
ancestry — **body string was byte-identical in all four listings**
(`captures/github/01,05,07,09-*.json`; also mirrored in GraphQL captures
`03,04,06,08,10-*.json`). No forge-side normalization, whitespace
collapsing, or markdown mangling of the token was observed.

GitLab: not live-tested (no auth). Docs give no reason to expect the body
would be touched (it's an opaque markdown string field), but this is
**disclosed as unproven** for GitLab specifically, not extrapolated from
the GitHub result.

## Q4 — what happens when a push moves the commented line?

**Confirmed live on GitHub, and it is more nuanced than a binary
fresh/outdated split — three observed states across three pushes:**

1. **Push 1 (lines inserted above the commented line, content unchanged,
   same diff hunk):** GitHub's line-tracking algorithm **re-anchored the
   comment**. `line`/`position` updated from 9→12; `original_line`/
   `original_position` stayed pinned at 9; `outdated`/`isOutdated`
   stayed `false`. See `05-...after-push.json`, `06-...GraphQL-after-push.json`.
2. **Push 2 (the exact commented line's text itself rewritten, still same
   hunk shape):** No change at all vs. push 1 — `line` stayed 12,
   `outdated` stayed `false`. GitHub's tracking appears to key off diff-hunk
   continuity/line offset, **not** literal content match — editing the
   commented line's own text did not, by itself, trigger outdated. See
   `07-...after-line-rewrite.json`, `08-...GraphQL-after-line-rewrite.json`.
3. **Push 3 (force-push amending/rewriting commit history so the original
   commented commit is no longer an ancestor):** **This did trigger the
   outdated state.** `line: null`, `original_line: 9` (preserved),
   `position: 1` (GitHub's fallback/placeholder), `isOutdated: true`,
   `isCollapsed: true`, while `isResolved` stayed `true` (resolution
   survives independently of position loss). See
   `09-...after-force-push.json`, `10-...GraphQL-after-force-push.json`.

**This confirms the spec's premise directly:** line-position anchoring
(`line`/`position`) is not durable across arbitrary history changes — it
degrades to `null` under force-push/history-rewrite — while the comment
`body`, and therefore the `[vd:ac-2]` token, is completely unaffected by
any of the three pushes. The token is the only anchor that survived all
three scenarios; position did not survive the third. A surprising nuance
for the port: GitHub *does* opportunistically re-track position across
ordinary (non-rewriting) pushes, so "outdated" is not simply "any push
moved the line" — it is closer to "the diff/history relationship the
comment was pinned to no longer holds." The port must not assume outdated
fires on every push; it must read the actual `outdated`/`line` fields, not
infer staleness from push count.

GitLab: **inconclusive even from docs.** GitLab's docs confirm diff
comments "persist" across force-push/rebase/amend but do not document
whether/how `position`/`line_range` gets nulled or recomputed, and do not
show an explicit `outdated` boolean anywhere in the documented REST shape
(unlike GitHub's `outdated`/`isOutdated`). This is disclosed as unproven —
see `captures/gitlab/04-doc-derived-UNVERIFIED-position-staleness-notes.json`
for the detailed writeup, including the hypothesis (external knowledge, not
doc-confirmed here) that GitLab computes staleness client-side by comparing
`position.head_sha` to the MR's current `diff_refs.head_sha` rather than
exposing a server-computed flag. **This needs live GitLab verification in
V1-P7 before the port's GitLab adapter can be written with confidence** —
it is the single biggest open risk this spike did not close.

## Normalization the port will need

1. **Two comment universes per forge, one port surface.** Both forges
   distinguish diff-anchored comments (GitHub `pulls/comments`, GitLab
   `DiffNote`) from general/thread comments (GitHub `issues/comments`,
   GitLab `DiscussionNote`/`individual_note:true`). The port's `list`
   method needs to merge both into one feed (05-surfaces.md's "full comment
   feed" pulled every render) — token resolution and the inbox tray apply
   across both, per spec.
2. **Resolution state requires divergent transports.** GitHub: GraphQL only
   (`reviewThreads`/`resolveReviewThread`). GitLab: REST-only per docs
   (unverified). The port's resolution-state method cannot be a thin REST
   wrapper on both sides; the GitHub adapter needs a GraphQL leg.
3. **Position/outdated semantics are forge-native and non-comparable
   as data, only as an outcome.** GitHub exposes explicit
   `outdated`/`isOutdated` booleans plus `line`/`original_line`. GitLab
   (per docs) exposes raw shas (`position.head_sha` vs
   `diff_refs.head_sha`) with no boolean — staleness must be *derived* on
   the GitLab side, not read directly. The port's "is this comment's
   position stale" concept needs a per-adapter implementation; it cannot
   be a shared field-mapping.
4. **Token durability is the one thing that IS comparable across forges** —
   confirmed on GitHub, plausible-per-docs on GitLab: the `body` field is
   an opaque string on both, untouched by push/history events. This is
   exactly why the spec anchors on the token rather than position — this
   spike's central finding validates that design choice on live evidence
   (GitHub) and does not contradict it on GitLab (docs), though GitLab is
   not itself proven.
5. **`line_code`/`position_type`/three-sha preconditions (GitLab) vs.
   single `commit_id`+`line`+`side` (GitHub)** means the port's `post`
   method needs adapter-specific pre-fetch (GitLab: MR `diff_refs`; GitHub:
   PR head sha only) before constructing a post request — not a shared
   request builder.

## Recommended minimal port-method semantics (semantics only — Go shapes deferred to V1-P7)

- `ListComments(ctx, mrRef) ([]Comment, error)` — merges diff-anchored and
  general comments into one feed per MR/PR; each `Comment` carries at
  minimum: id, body (raw, untouched), author, forge-native diff-position-or-nil,
  a forge-native "is this position stale" tri-state (`fresh | stale |
  unknown` — `unknown` for forges/paths where staleness isn't determinable,
  e.g. GitLab pending live verification), and thread id (for resolution
  grouping).
- `PostComment(ctx, mrRef, body, target)` — `target` is either a diff
  anchor (path+line, forge translates to its own position preconditions
  internally — including any required sha pre-fetch) or nil for a
  general/thread comment. Returns the created comment including whatever
  forge-native id is needed to re-fetch it later.
- `GetThreadResolution(ctx, mrRef) ([]ThreadResolution, error)` — one
  entry per thread: thread id, resolved bool, resolvable bool,
  resolved-by (optional). Adapter-internal detail (GraphQL vs REST) is
  fully hidden; the port never leaks which transport was used.
- `ResolveThread(ctx, mrRef, threadID) error` — same hidden-transport rule.

None of the above should surface forge-native field names (e.g. `outdated`,
`isOutdated`, `line_code`) through the port interface — only the derived
tri-state and the body/token, since those are the only pieces the spec's
round-trip actually depends on (§Review stickies and forge round-trip: the
board re-resolves object anchoring from the token at render time, never
from stored position).

## Capture-file inventory

### GitHub — LIVE-VERIFIED (all files below are real API responses from a
throwaway private repo `jyang234/verdi-spike-s6-throwaway`, PR #1)

| File | Status | What it shows |
|---|---|---|
| `captures/github/01-list-review-comments-REST.json` | verified (live) | both inline comments just after posting; token body intact |
| `captures/github/02-list-issue-comments-REST.json` | verified (live) | the general/non-diff PR comment |
| `captures/github/03-review-threads-GraphQL-before-resolve.json` | verified (live) | both threads unresolved |
| `captures/github/04-review-threads-GraphQL-after-resolve.json` | verified (live) | `[vd:ac-2]` thread resolved via `resolveReviewThread` |
| `captures/github/05-list-review-comments-REST-after-push.json` | verified (live) | after push #1 (lines inserted above); line re-tracked 9→12 |
| `captures/github/06-review-threads-GraphQL-after-push.json` | verified (live) | same push, GraphQL view; still not outdated |
| `captures/github/07-list-review-comments-REST-after-line-rewrite.json` | verified (live) | after push #2 (commented line's own text rewritten); no change vs push 1 |
| `captures/github/08-review-threads-GraphQL-after-line-rewrite.json` | verified (live) | same, GraphQL view |
| `captures/github/09-list-review-comments-REST-after-force-push.json` | verified (live) | after push #3 (force-push/history rewrite); `line: null`, position lost, body intact |
| `captures/github/10-review-threads-GraphQL-after-force-push.json` | verified (live) | same, GraphQL view; `isOutdated: true`, `isResolved` still `true` |

### GitLab — DOC-DERIVED, UNVERIFIED (no live GitLab auth available; every
file below is representative JSON assembled from `docs.gitlab.com`, never a
captured response)

| File | Status | What it shows |
|---|---|---|
| `captures/gitlab/01-doc-derived-UNVERIFIED-list-discussions.json` | doc-derived, UNVERIFIED | representative discussions-list shape (DiffNote + DiscussionNote) |
| `captures/gitlab/02-doc-derived-UNVERIFIED-post-discussion-request.json` | doc-derived, UNVERIFIED | representative POST request body for an inline note |
| `captures/gitlab/03-doc-derived-UNVERIFIED-resolve-discussion-response.json` | doc-derived, UNVERIFIED | representative PUT resolve response |
| `captures/gitlab/04-doc-derived-UNVERIFIED-position-staleness-notes.json` | doc-derived, UNVERIFIED, and inconclusive from docs alone | explicit writeup that GitLab's docs do not confirm an outdated-flag mechanism; flagged as the top open risk for V1-P7 |

## Cleanup note (not a spec finding — an operational disclosure)

The throwaway GitHub repo `jyang234/verdi-spike-s6-throwaway` (private) was
created, used, and **could not be auto-deleted**: the authenticated `gh`
token lacks the `delete_repo` OAuth scope, and `gh auth refresh -h
github.com -s delete_repo` requires an interactive browser device-code
confirmation that this sandboxed session cannot complete (attempted; it
printed a one-time code and hung waiting on browser confirmation, killed
after 8s per the no-indefinite-hang rule). **The repo still exists** — it
is private, contains only this spike's throwaway content, and needs manual
deletion via `gh auth refresh -h github.com -s delete_repo` (interactive)
followed by `gh repo delete jyang234/verdi-spike-s6-throwaway --yes`, or
deletion via the GitHub web UI (Settings → Danger Zone) on
`https://github.com/jyang234/verdi-spike-s6-throwaway`.
