// Package jira is the Jira adapter for the story-provider port (04 §Jira
// adapter, PLAN.md Phase 11): it implements provider.StoryProvider against
// the Jira Cloud REST API v3.
//
//   - Resolve: GET /rest/api/3/issue/{key} maps key/summary/status/URL onto
//     provider.Story. URL is the human browse link, built as
//     BaseURL+"/browse/"+key — not the response's machine-facing "self"
//     REST URL, which is not a page a human should be sent to. See
//     Resolve's doc comment for why.
//   - PublishRollup writes two things: a machine field (the custom field
//     named by Config.RollupField) holding a compact JSON payload
//     ({commit, eligible, criteria:[{id,status}]}), idempotent on
//     (story, commit); and a human comment (the criteria table plus an
//     MR/pipeline link read from CI env vars, when present), posted only
//     when any AC status changed since the last publish. Change detection
//     reads the adapter's own field back first — an adapter-internal read
//     of adapter-owned state, not a second read of tracker-owned data (04
//     §Semantics). The very first publish for a story always counts as
//     "changed" and fires a comment (ledger I-26).
//
// Every HTTP failure is classified into 04's failure-table sentinels
// (provider.ErrNotFound / ErrUnauthorized / ErrUnavailable) so callers can
// apply 04's degrade/fail-loud/retry behavior without knowing anything
// about Jira's wire format.
//
// The adapter is tested hermetically: internal/provider/jira/jiratest hosts
// a minimal in-memory httptest-backed stand-in for the Jira API, used by
// this package's own tests and by cmd/verdi's rollup end-to-end tests. No
// live network is used anywhere (CLAUDE.md).
package jira
