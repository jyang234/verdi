package provider

import "context"

// StoryRef is a scheme-prefixed tracker reference, e.g. "jira:LOAN-1482" or
// "gitlab:platform#482" (04 §Reference scheme). The scheme selects the
// adapter at runtime via Registry.
type StoryRef string

// Story is the tracker-owned metadata Resolve returns for a StoryRef (04
// §The port).
type Story struct {
	Ref    StoryRef
	Title  string
	Status string
	URL    string
}

// CriterionStatus is one AC's fold outcome as published to the tracker (04
// §The port). Status is one of the fold's per-AC statuses — evidenced,
// violated, pending, no-signal, waived (03 §The fold) — distinct from
// evidence-record verdicts; it is a plain string here because the port is
// consumer-defined and deliberately untyped at this boundary, matching 04's
// normative Go verbatim.
type CriterionStatus struct {
	ID      string // "ac-2"
	Text    string
	Status  string // evidenced | violated | pending | no-signal | waived
	Summary string // one-line evidence summary
}

// Rollup is the payload PublishRollup writes back to the tracker (04 §The
// port): the story's current fold, keyed for idempotency on (Story,
// Commit).
type Rollup struct {
	Story    StoryRef
	Ref      string // git ref
	Commit   string
	Criteria []CriterionStatus
	Eligible bool
}

// StoryProvider is the story-provider port (04 §The port). Adapters
// implement it per tracker; the registry selects one by StoryRef scheme.
//
// Resolve is read-mostly and safe to cache (04 §Semantics: 15m default
// TTL). PublishRollup runs in CI only and must be idempotent on the key
// (Story, Commit): republishing an unchanged rollup is an update, never
// a duplicate.
//
// Adapters report failures using the sentinel errors in this package
// (ErrNotFound, ErrUnauthorized, ErrUnavailable) so callers can implement
// 04's failure table.
type StoryProvider interface {
	Resolve(ctx context.Context, ref StoryRef) (Story, error)
	PublishRollup(ctx context.Context, r Rollup) error
}
