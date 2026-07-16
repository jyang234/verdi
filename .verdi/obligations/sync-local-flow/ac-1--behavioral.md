---
id: obligation/sync-local-flow--ac-1--behavioral
kind: obligation
title: "A Go test proves the CI-env-wins regression and a scratch e2e test proves the local refusal with no network dial"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/sync-local-flow" }
frozen: { at: 2026-07-16, commit: 8e97d547d237e007d584e977f4eafdb73d69d59a }
---
# A Go test proves the CI-env-wins regression and a scratch e2e test proves the local refusal with no network dial

The behavioral evidence must show two things. First, an integration test
in `cmd/verdi/sync_test.go` (hermetic — fixturegit plus
`internal/forge/fake`) that constructs a checkout with BOTH the explicit
CI env (`GITHUB_REPOSITORY_OWNER`, `GITHUB_REPOSITORY`) set AND a
resolvable `origin` remote configured, and asserts the resolved identifier
is byte-identical to today's CI-env-wins behavior — proving the env still
wins, not merely that sync happens to succeed either way.

Second, a Go e2e test that drives `cmdSync` itself — the real top-level
entry point `main` invokes, never a package-internal helper called
directly — over a scratch fixturegit repo with no CI environment
variables set and no configured `origin` remote, asserting exit 2 and a
refusal message naming every source tried. This built-binary register is
deliberately chosen for the refusal path because it resolves and refuses
before any network dial (co-1): a genuine successful forge round-trip
must stay proven only at the hermetic-fake integration register above,
never by letting the built binary in the e2e test actually dial out. An
e2e test that asserts a successful sync instead of the exit-2 refusal, or
that requires network access to pass, does not satisfy this obligation.
