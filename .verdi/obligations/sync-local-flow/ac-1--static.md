---
id: obligation/sync-local-flow--ac-1--static
kind: obligation
title: "Table-driven tests cover every origin-remote URL shape, and the refusal names every source tried"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/sync-local-flow" }
frozen: { at: 2026-07-16, commit: 8e97d547d237e007d584e977f4eafdb73d69d59a }
---
# Table-driven tests cover every origin-remote URL shape, and the refusal names every source tried

The static evidence must show `internal/forge/github/remoteurl_test.go`'s
`TestOwnerRepoFromURL` still covers every `github.com` remote shape this ac
names — `https://github.com/OWNER/REPO(.git)?`,
`ssh://git@github.com/OWNER/REPO(.git)?`, and the scp-like
`git@github.com:OWNER/REPO(.git)?`, each with and without a trailing
`.git` — extended only if the build finds a real gap in that existing
coverage, never re-derived from scratch. It must further show
`cmd/verdi/sync_helpers_test.go`'s `TestGithubOwnerRepo` extended with the
neither-env-nor-origin-resolves case, asserting the function's new refusal
rather than the silently-returned empty pair that same test's own prior
"honest can't-identify case" comment names; a further unit test on the
refusal itself (wherever the build lands the refusing call) must assert
the error names every source it tried — both env vars
(`GITHUB_REPOSITORY_OWNER`, `GITHUB_REPOSITORY`) and the `origin` remote
URL or its documented absence — not merely that some error occurred.

Finally, it must show `TestBuildForge_Happy`'s existing github case
(`cmd/verdi/sync_helpers_test.go`) updated to match the new behavior
(dc-2): a suite that still asserts an empty remote URL builds a forge with
no error, left standing unchanged alongside a new refusal test elsewhere,
is a self-contradicting pair and does not satisfy this obligation — dc-2
is explicit that this is a known, deliberate test update the build makes,
not a regression guard to preserve.
