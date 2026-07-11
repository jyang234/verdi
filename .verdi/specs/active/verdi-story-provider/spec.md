---
id: spec/verdi-story-provider
kind: spec
class: component
title: "Story provider: port, ref scheme, and the Jira adapter"
status: active
owners: [platform-team]
links:
  - { type: depends-on, ref: spec/verdi-evidence-model, note: "rollup shape" }
schema: verdi.provider/v1
---

# Story provider

## Purpose and direction of truth

Stories live in an external tracker; the story spec owns the acceptance
criteria it binds evidence to. The provider is a **read-mostly peer**: verdi
resolves story metadata from it and publishes rollups to it, one-way. The
tracker never becomes a second author of ACs — `PublishRollup` writes the AC
list and statuses back so PMs see everything in their habitat, but as a read
model. Dual authoring is the rot this boundary exists to prevent.

Under the two-level spec model (feature specs and story specs), tracker
binding is a story-level concern: the **story spec** carries the tracker
link that `PublishRollup` targets; a **feature spec**'s tracker link, when
present, names an epic or objective — grouping context for humans reading
the feature in its tracker habitat, never a rollup target. `PublishRollup`
therefore publishes **per story spec, one rollup per story**. Feature-level
rollup publication (an aggregate tracker view of a feature's stories) is
**explicitly deferred** — not built this round, not silently missing: a
feature's status is read from its own board/dex projection until a
feature-level publisher is specced.

## Reference scheme

`story:` links are scheme-prefixed: `jira:LOAN-1482`, `gitlab:platform#482`.
The scheme selects the adapter at runtime from `verdi.yaml`'s `providers:`
map. A new tracker is a new adapter package, not a refactor.

- On a **story spec**, `story:` is a **required** scalar: exactly one live
  tracker item per story — the target `PublishRollup` writes to.
- On a **feature spec**, `story:` is **optional**: when present it names an
  epic or objective in the tracker, not something verdi resolves rollups
  against. A feature spec carrying no `story:` link is not an error.

Reference-scheme validation — including rejection of unconfigured schemes
(VL-005) — has always lived in 02-artifact-contract, which owns the
link/reference model shared across all spec kinds; what round four moves is
only VL-005's **scope**, from the (v0) feature class to the story class
(R4-I-2), since the story class now carries the required scalar. This
document states only which spec class carries which `story:` obligation.

## The port

Consumer-defined, deliberately tiny:

```go
type StoryRef string // "jira:LOAN-1482"

type Story struct {
    Ref    StoryRef
    Title  string
    Status string
    URL    string
}

type CriterionStatus struct {
    ID         string   // "ac-2"
    Text       string
    Status     string   // evidenced | violated | pending | no-signal | waived
    Summary    string   // one-line evidence summary
    Implements []string // feature AC ids this story AC implements, via
                         // `implements` edges (spec realignment concept §1);
                         // empty when the story has no feature, or this AC
                         // implements none. Additive: adapters MAY ignore it.
}

type Rollup struct {
    Story    StoryRef
    Ref      string // git ref
    Commit   string
    Criteria []CriterionStatus
    Eligible bool
}

type StoryProvider interface {
    Resolve(ctx context.Context, ref StoryRef) (Story, error)
    PublishRollup(ctx context.Context, r Rollup) error
}
```

## Semantics

- **Resolve** — local surfaces cache results with a short TTL (default 15m)
  so lenses never hammer the tracker. On failure, degrade to displaying the
  raw ref; never block rendering.
- **PublishRollup** — runs in CI only (`verdi rollup --publish` on MR
  pipelines and the closure MR's merge pipeline), against a **story spec's**
  `story:` link; there is no feature-level call (see Purpose above — the
  feature-level aggregate is a named deferral). Idempotent on the key
  `(story, commit)`: republishing is an update, never a duplicate. To decide
  whether the human comment fires, the adapter reads back its **own
  published field** first (read-before-write for change detection) — this is
  an adapter-internal read of adapter-owned state, and the narrower claim
  stands: `Resolve` remains the only read of *tracker-owned* data. Retry
  semantics are the CI job's retry plus idempotency; with no daemon there is
  no outbox to maintain.
- **Upward mapping in the payload.** `Rollup.Criteria` entries now also
  carry `Implements` — the feature AC ids this story AC serves — so the
  tracker rendering can show the story's status alongside what it implements
  at the feature level. This is mechanically inert for `PublishRollup`
  itself: same key, same idempotency, same one-rollup-per-story write. It is
  an **additive** field on the payload; an adapter that does not render it
  MUST still accept and ignore it rather than fail the write.
- **Non-authoritative, off the critical path.** Tracker downtime never blocks
  ingestion, lint, the merge gate, or local work. The closure gate's
  tracker-side enforcement simply waits for the next successful publish.

| Failure            | Behavior                                             |
|--------------------|------------------------------------------------------|
| NotFound           | lint warning on the spec's `story:` link; ref shown raw |
| Unauthorized       | fail the publish job loudly (credential drift is a real error) |
| Unavailable/timeout| Resolve: degrade + cache stale; Publish: job retry   |

## Jira adapter

- **Resolve**: `GET /rest/api/3/issue/{key}` → key, summary, status, URL.
- **PublishRollup** writes two things:
  - a **machine field**: `providers.jira.rollup_field` (custom field id from
    `verdi.yaml`) set to a compact JSON payload
    `{ commit, eligible, criteria: [{id, status, implements}] }` —
    `implements` is the additive feature-AC-id list from
    `CriterionStatus.Implements` (empty array when none); this is what the
    workflow validator reads (OQ-1), and it MUST tolerate the field's
    presence without decode failure;
  - a **human comment**, posted when any AC status changed since the last
    publish — including the very first publish for a story, which always
    counts as changed (there is no prior baseline to diff against, so the
    PM learns the initial state): the criteria table plus a link to the
    MR/pipeline.
- **Secrets**: token via `VERDI_JIRA_TOKEN` (CI variable / local env). Only
  ids and URLs live in `verdi.yaml`; credentials are never committed.

## Testing

The port ships with a fake provider and a contract-test suite that every
adapter must pass (resolve happy path, not-found, publish idempotency,
comment-only-on-change). The resolve happy-path assertion is
harness-declared, not echo-based: the suite checks `Resolve`'s returned
URL against a per-adapter expectation the harness itself supplies, rather
than requiring every adapter to echo back an arbitrary seeded `Story.URL`
— a construct-from-config adapter (e.g. Jira, which builds a browse URL
from `base_url` plus the issue key) can never echo an arbitrary seeded
host. Fixtures are hermetic and committed, in the verdi-go style, so the
whole story pipeline tests without network.
