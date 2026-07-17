---
id: obligation/vocabulary-surfaces--ac-3--behavioral
kind: obligation
title: "An end-to-end test driving the real stdio MCP server over a vocab-rename store proves tool descriptions carry the model's class display names in place of today's literals"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/vocabulary-surfaces" }
frozen: { at: 2026-07-17, commit: 6fb386f1c7d53f9318519b7710144c9adcb4e33d }
---
# An end-to-end test driving the real stdio MCP server over a vocab-rename store proves tool descriptions carry the model's class display names in place of today's literals

The behavioral evidence must show a Go test driving the real MCP stdio
server end to end — mirroring `internal/mcpserve/
server_errlog_test.go`'s existing `ServeConn`-driving convention, never
a package-internal unit test standing in for it — against a fixture
store carrying a vocab-rename manifest, proving the server's tool-list
response (the description text `internal/mcpserve/tooldefs.go` already
carries, such as `get_context_bundle`'s reference to a "feature spec")
speaks the model's renamed class display word — "Initiative" in place
of "feature," for the `vocab-rename.yaml`-style fixture's own rename —
rather than today's literal. The description text must be read back
from the real tool-list response the server returns over the wire, not
from `tooldefs.go`'s Go value directly, so the proof covers the
resolution actually reaching a client. Green in CI's test step.
