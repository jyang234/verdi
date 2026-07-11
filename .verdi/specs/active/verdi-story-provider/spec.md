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

Stories live in an external tracker; the spec owns the acceptance criteria.
The provider is a **read-mostly peer**: verdi resolves story metadata from it
and publishes rollups to it, one-way. The tracker never becomes a second
author of ACs — `PublishRollup` writes the AC list and statuses back so PMs
see everything in their habitat, but as a read model. Dual authoring is the
rot this boundary exists to prevent.

## Reference scheme

`story:` links are scheme-prefixed: `jira:LOAN-1482`, `gitlab:platform#482`.
The scheme selects the adapter at runtime from `verdi.yaml`'s `providers:`
map; VL-005 rejects unconfigured schemes. A new tracker is a new adapter
package, not a refactor.

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
    ID      string // "ac-2"
    Text    string
    Status  string // evidenced | violated | pending | no-signal | waived
    Summary string // one-line evidence summary
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
  pipelines and the closure MR's merge pipeline). Idempotent on the key
  `(story, commit)`: republishing is an update, never a duplicate. To decide
  whether the human comment fires, the adapter reads back its **own
  published field** first (read-before-write for change detection) — this is
  an adapter-internal read of adapter-owned state, and the narrower claim
  stands: `Resolve` remains the only read of *tracker-owned* data. Retry
  semantics are the CI job's retry plus idempotency; with no daemon there is
  no outbox to maintain.
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
    `{ commit, eligible, criteria: [{id, status}] }` — this is what the
    workflow validator reads (OQ-1);
  - a **human comment**, posted only when any AC status changed since the
    last publish: the criteria table plus a link to the MR/pipeline.
- **Secrets**: token via `VERDI_JIRA_TOKEN` (CI variable / local env). Only
  ids and URLs live in `verdi.yaml`; credentials are never committed.

## Testing

The port ships with a fake provider and a contract-test suite that every
adapter must pass (resolve happy path, not-found, publish idempotency,
comment-only-on-change). Fixtures are hermetic and committed, in the
verdi-go style, so the whole story pipeline tests without network.
