---
id: spec/sync-local-flow
kind: spec
title: "Sync Local Flow"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-31
problem: { text: "verdi sync in a plain local checkout is only partially fixed and still fights the fold it feeds. A prior hot-fix (D6-14) taught the GitHub adapter to fall back to the git origin remote for the repository identifier when the CI env is absent, but when neither source resolves one, githubOwnerRepo (cmd/verdi/forgeboot.go) silently returns two empty strings and buildForge builds a live adapter around them anyway — a confusing network failure, not the legible refusal the rest of the toolchain gives. And once addressed, sync still only asks the forge for a bundle at HEAD's own exact commit, while the fold it feeds already accepts any record whose commit is an ancestor of HEAD (03 §The fold) — a routine, legitimately path-filtered commit makes sync refuse a bundle the fold would gladly accept (D6-32), the exact asymmetry round 6 worked around by cutting closure branches from the verified ancestor commit itself (ADJ-19)", anchor: problem }
outcome: { text: "verdi sync, run locally with no CI environment, resolves the GitHub repository from the git origin remote exactly as it already does when the CI env is absent, and now refuses legibly (operational, exit 2, naming every source tried) when neither resolves one — instead of quietly building a doomed adapter. Its bundle fetch walks the current commit's ancestry, nearest first, applying the same ancestor rule the fold's own reader already uses, so the nearest ancestor commit that actually carries a bundle — including the commit itself — is accepted and disclosed by name and distance, never demanding a HEAD-exact bundle the fold itself would not require. No new environment variable, config key, or flag is introduced; the explicit CI env, present, still wins byte-identically to today", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "verdi sync's GitHub forge identification derives the repository owner/repo from the git origin remote when the explicit CI env (GITHUB_REPOSITORY_OWNER / GITHUB_REPOSITORY) is absent, parsing every shape git writes for a github.com remote (https, ssh://, and scp-like git@github.com:owner/repo, each with or without a trailing .git) — the explicit CI env, when present, still wins byte-identically to today. When neither the env nor a resolvable origin identifies a repository, sync refuses operationally (exit 2), naming every source it tried, instead of silently addressing the forge with an empty owner or repo", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "verdi sync's bundle fetch accepts the nearest-ancestor evidence bundle on the current commit's history — the commit itself first, then its ancestors, nearest first — applying the same ancestor rule the fold already uses to accept records (03 §The fold: current is the latest record whose commit is an ancestor of C), rather than a second, parallel definition of ancestor. A bundle at the commit itself still wins, since a commit is its own ancestor under this rule; sync discloses which commit's bundle it accepted and how far back it walked, and a bounded walk that finds no bundle anywhere on the path refuses exactly as today, naming the range walked", evidence: [static, behavioral], anchor: ac-2 }
links:
  - { type: implements, ref: "spec/closure-ergonomics#ac-4" }
decisions:
  - { id: dc-1, text: "The ancestor rule sync's fetch conforms to is the fold's own, not a restatement: internal/evidence.LoadRecordsWithSources (internal/evidence/records.go's gitx.IsAncestor check) keeps a derived-tree commit directory only when it is commit itself or a real ancestor of commit, over gitx.IsAncestor's git merge-base --is-ancestor (internal/gitx/ancestry.go) — the same primitive, not a lookalike. Sync's fetch walk enumerates candidate ancestor commits via gitx's existing commit-history primitive (internal/gitx/log.go's Log, already most-recent-first) rather than a hand-rolled parent walk, so there is no second, possibly-disagreeing notion of ancestor anywhere in the tree. The walk is bounded — a live per-candidate forge call is not the free disk-directory filter LoadRecords performs, so an unbounded probe risks pathological API cost on a long-lived ref; the exact bound is a build-time judgment this story does not fix a number for, disclosed rather than silently chosen", anchor: dc-1 }
  - { id: dc-2, text: "The identifier refusal lands at the one shared construction seam, not a sync-local copy: buildForge/githubOwnerRepo (cmd/verdi/forgeboot.go) already serves eight verb files, but sync.go is the only one calling it directly and unconditionally — the other seven reach it through forgeBestEffort (cmd/verdi/gate_threads.go), which already declines gracefully (a nil forge, no error) via forgeCredentialsPresent whenever the identifier cannot be resolved, so none of those seven ever reaches a doomed buildForge call today and none is affected by tightening it. TestBuildForge_Happy's existing github case, which asserts an empty remote URL still builds a forge with no error (cmd/verdi/sync_helpers_test.go), encodes exactly the silent-empty-identifier gap this ac fixes and is a known, deliberate test update, not a regression guard to preserve", anchor: dc-2 }
  - { id: dc-3, text: "This story adds no new resolution semantics anywhere, reaffirming the feature's own dc-3: no new environment variable (only GITHUB_REPOSITORY_OWNER, GITHUB_REPOSITORY, and the existing git origin remote read are ever consulted), no new verdi.yaml key, and no new verdi sync flag. The explicit CI env is checked first and short-circuits identically to today whenever it resolves both fields — a regression obligation on every existing CI-env-present test, not a hope. Scope is GitHub-only, mirroring D6-14 itself: GitLab's numeric CI_PROJECT_ID has no URL-derived form (the D6-14 fix commit's own disclosed exclusion), so gitlab.go is untouched by this story", anchor: dc-3 }
constraints:
  - { id: co-1, text: "No network in any test: forge interactions run against httptest doubles (internal/forge/forgetest's shared adapter-conformance suite, internal/forge/fake) and fixturegit's real, deterministic local git plumbing for every ancestor topology — never a live dial to api.github.com or a real GitLab instance", anchor: co-1 }
  - { id: co-2, text: "Exit discipline is unchanged in shape: 0 clean (a bundle accepted), 1 verdict (evaluateTree/evaluateBundle's existing fail-verdict mapping, untouched by this story), 2 operational. This story adds exactly one new operational reason (the identifier unresolvable from either source) and re-words one existing operational message (no bundle found — now naming the ancestor range walked) without inventing a new exit code or a new verdict path", anchor: co-2 }
  - { id: co-3, text: "Every sync path this story does not target is byte-identical before and after: --produce, --produce-runtime, --force-local, and the CI-env-present identifier short-circuit are unchanged, proven by every pre-existing sync test (cmd/verdi/sync_test.go, sync_helpers_test.go, sync_regen.go's own tests) continuing to pass unmodified alongside this story's new negative-path coverage — a regression obligation, not a hope", anchor: co-3 }
---
# Sync Local Flow

## Problem

`verdi sync` in a plain local checkout is only *partially* fixed, and still
fights the fold it feeds.

A prior, standalone hot-fix (commit `7dcba91`, D6-14) already taught the
GitHub adapter to fall back to the git `origin` remote for the repository
identifier when GitHub Actions' own `GITHUB_REPOSITORY_OWNER` /
`GITHUB_REPOSITORY` env vars are absent — `githubOwnerRepo`
(`cmd/verdi/forgeboot.go`) prefers the env, then falls back per-field to
`forgegithub.OwnerRepoFromURL` (`internal/forge/github/remoteurl.go`), and
`cmdSync` already threads the resolved `origin` URL into it
(`cmd/verdi/sync.go`). That part of the mechanism, and its URL-shape
parsing, is already well covered by `TestGithubOwnerRepo`
(`cmd/verdi/sync_helpers_test.go`) and `TestOwnerRepoFromURL`
(`internal/forge/github/remoteurl_test.go`).

But when **neither** the env **nor** a resolvable `origin` identifies a
repository, `githubOwnerRepo` silently returns two empty strings, and
`buildForge` builds a live `forgegithub.Adapter` around them anyway — the
existing `TestGithubOwnerRepo` case even names this "the honest
can't-identify case," on the assumption that some caller declines to build
a doomed forge. That assumption is true for the other seven verb files that
reach `buildForge` through `forgeBestEffort`/`forgeCredentialsPresent`
(`cmd/verdi/gate_threads.go`), which pre-check resolvability and degrade to
"no live forge" gracefully. It is **not** true for `sync.go`, the one
direct, ungated caller: `cmdSync` calls `buildForge` unconditionally and
then dials the forge for real, producing the exact confusing failure D6-14
first reported (`GET .../repos///actions/runs?head_sha=...` → 404) the
moment `verdi sync` runs in a checkout with no configured `origin` and no CI
env — the plain local-checkout case this whole feature exists to serve.

Separately, once the repository is addressed, `verdi sync` still only asks
the forge for a bundle at HEAD's own **exact** commit
(`internal/forge/github.FetchEvidenceBundle`'s `head_sha=<commit>` query,
`internal/forge/github/github.go`) — while the fold it feeds already accepts
any record whose commit is an ancestor of the one being evaluated (03 §The
fold: "current ... whose commit is an ancestor of C" —
`internal/evidence.LoadRecordsWithSources`, `internal/gitx.IsAncestor`). A
routine, legitimately path-filtered commit (verify.yml's own header
discloses this as intentional, not a regression: "the fold's ancestor-based
'current' rule ... means an earlier code commit's evidence stays valid
until a newer one supersedes it") makes `sync` refuse a bundle the fold
would gladly fold (D6-32) — precisely the asymmetry round 6 worked around
by cutting closure branches from the verified ancestor commit itself rather
than fixing sync (ADJ-19).

## Outcome

`verdi sync`, run locally with no CI environment, resolves the GitHub
repository from the git `origin` remote exactly as it already does when the
CI env is absent (unchanged), and now **refuses legibly** — operational,
exit 2, naming every source it tried — when neither the env nor a
resolvable `origin` identifies one, instead of quietly building a doomed
adapter. Its bundle fetch walks the current commit's ancestry, nearest
first, applying the *same* ancestor rule the fold's own reader already uses
— so the nearest ancestor commit that actually carries a bundle, including
the commit itself, is accepted and disclosed by name and distance, never
demanding a HEAD-exact bundle the fold itself would not require. No new
environment variable, config key, or CLI flag is introduced anywhere; the
explicit CI env, when present, still wins byte-identically to today.

## AC-1

Forge identifier resolution and its refusal. `verdi sync`'s GitHub forge
identification derives the repository owner/repo from the git `origin`
remote when the explicit CI env is absent, parsing every shape git itself
writes for a `github.com` remote — `https://github.com/OWNER/REPO(.git)?`,
`ssh://git@github.com/OWNER/REPO(.git)?`, and the scp-like
`git@github.com:OWNER/REPO(.git)?` — exactly the enumeration
`OwnerRepoFromURL` already parses. The explicit CI env, when present, still
wins byte-identically to today: this is a regression obligation on the
existing env-wins behavior, not a hope. When neither source identifies a
repository, `verdi sync` refuses operationally (exit 2), naming every
source it tried (the two env vars, and the `origin` remote URL or its
absence) — never silently proceeding to address the forge with an empty
owner or repo, mirroring PR #102's legible-refusal posture for gate's
default-branch detection (`internal/lint.ResolveDefaultBranch`: env first,
local evidence as fallback, an unresolvable case names what was tried
rather than guessing).

**Static register:** table-driven unit tests over `OwnerRepoFromURL`
(`internal/forge/github/remoteurl_test.go`, already covers every URL shape
above with and without `.git`, trailing slashes, and case-insensitive host —
extended only if the build finds a real gap) and over `githubOwnerRepo`
(`cmd/verdi/sync_helpers_test.go`'s `TestGithubOwnerRepo`, extended with the
neither-resolves case asserting the new refusal rather than a silently
returned empty pair); a new unit test on the refusal itself, wherever the
build lands it, asserting the error names every source tried, plus the
`TestBuildForge_Happy` update dc-2 calls out by name.

**Behavioral register:** an integration test (`cmd/verdi/sync_test.go`,
hermetic — fixturegit plus `internal/forge/fake`) proving the explicit CI
env still wins byte-identically when both it and a resolvable `origin` are
present; a Go e2e test driving `cmdSync` itself — the real top-level entry
point `main` invokes — in a scratch fixturegit repo with no CI env and no
configured `origin` remote, asserting exit 2 and the named-sources refusal
message. This path is deliberately chosen for the built-binary register
because it resolves and refuses *before any network dial*, so it is
provable against the real entry point with zero network dependency (co-1);
a genuine successful forge round-trip stays proven only at the hermetic-fake
integration register, never by letting the built binary dial out for real.

## AC-2

Nearest-ancestor bundle resolution. `verdi sync`'s bundle fetch accepts the
nearest-ancestor evidence bundle on the current commit's history — the
commit itself first, then its ancestors, nearest first — applying the same
ancestor rule the fold already uses to accept records (dc-1), rather than a
second, parallel definition of "ancestor." A bundle at the commit itself
still wins when one exists, since a commit is its own ancestor under this
rule (`gitx.IsAncestor`'s own documented self-inclusive semantics) — the
walk starts there, so "nearest-ancestor" strictly includes HEAD, not just
its predecessors. `verdi sync` discloses which commit's bundle it accepted
and how many commits back it walked. Exhausting a bounded walk with no
bundle anywhere on the path refuses exactly as today's HEAD-exact refusal
does — naming the ref and the commit range walked — never silently
regenerating unless `--or-regen` is passed, and never succeeding at some
arbitrarily deep, undisclosed ancestor.

**Static register:** table-driven unit tests over the candidate-ancestor
enumeration/ordering helper, proving the commit itself is always the first
candidate and the remaining order is otherwise deterministic.

**Behavioral register:** integration tests (a new
`cmd/verdi/sync_ancestor_test.go`, hermetic — fixturegit topologies plus
`internal/forge/fake` seeded per candidate commit) over: a linear history
with the bundle at a named ancestor several commits back; a branched
history; a bundle present at HEAD itself (still wins, no walk needed); and
no bundle anywhere on the walked path (the existing refusal, message
updated to name the range walked) — each asserting which commit's bundle
was accepted and the disclosed distance walked.

## DC-1

One ancestor rule, shared, not restated. The rule sync's fetch conforms to
is the fold's own: `internal/evidence.LoadRecordsWithSources`
(`internal/evidence/records.go`, its `gitx.IsAncestor` check) keeps a
derived-tree commit directory only when it is the evaluated commit itself or
a real ancestor of it — over `gitx.IsAncestor`'s `git merge-base
--is-ancestor` (`internal/gitx/ancestry.go`), the same primitive, not a
lookalike. Sync's fetch walk enumerates candidate ancestor commits via
`gitx`'s existing commit-history primitive (`internal/gitx/log.go`'s `Log`,
already most-recent-first over a single rev with no path filter — exactly
rev's ancestor closure, nearest first) rather than a hand-rolled parent
walk, so there is structurally no second, possibly-disagreeing notion of
"ancestor" anywhere in the tree: git's own reachability concept is singular,
and both primitives are thin wrappers over it. The walk is bounded — a live
per-candidate forge call is not the free, already-on-disk directory filter
`LoadRecords` performs, so an unbounded probe risks pathological API cost or
rate-limiting on a long-lived ref. The exact bound is a build-time judgment
this story deliberately does not fix a number for; it is disclosed as an
open implementation choice rather than silently decided here.

## DC-2

The identifier refusal lands at the one shared construction seam, not a
sync-local copy. `buildForge`/`githubOwnerRepo` (`cmd/verdi/forgeboot.go`)
already serves eight verb files (its own header comment names them:
align.go, audit.go, buildstart.go, close.go, dex.go, design.go, feature.go,
gate.go/gate_threads.go, rollup.go, sync.go), but `sync.go` is the only one
calling `buildForge` directly and unconditionally
(`cmd/verdi/sync.go:cmdSync`). The other seven reach it through
`forgeBestEffort` (`cmd/verdi/gate_threads.go`), which already declines
gracefully — a nil forge, no error — via `forgeCredentialsPresent` whenever
the identifier cannot be resolved; none of those seven ever reaches a doomed
`buildForge` call today, so none is affected by tightening it. Concretely:
`TestBuildForge_Happy`'s existing github case, which asserts that an empty
remote URL still builds a forge with no error
(`cmd/verdi/sync_helpers_test.go`), encodes exactly the silent-empty-
identifier gap this ac fixes — it is a known, deliberate test update the
build makes, not a regression guard to preserve unchanged.

## DC-3

No new resolution semantics anywhere, reaffirming the parent feature's own
dc-3. No new environment variable is read (only `GITHUB_REPOSITORY_OWNER`,
`GITHUB_REPOSITORY`, and the existing git `origin` remote read are ever
consulted); no new `verdi.yaml` key; no new `verdi sync` flag. The explicit
CI env is checked first and short-circuits identically to today whenever it
resolves both fields — a regression obligation on every existing
CI-env-present test, not a hope. Scope is deliberately GitHub-only,
mirroring D6-14 itself: GitLab's numeric `CI_PROJECT_ID` has no URL-derived
form (the original D6-14 fix commit's own disclosed exclusion — "gitlab is
unchanged"), so `internal/forge/gitlab/gitlab.go` is untouched by this
story; a plain local-checkout gap on the GitLab side, if any, is out of this
story's declared scope, not silently narrowed.

## CO-1

No network in any test. Forge interactions run against `httptest` doubles
(`internal/forge/forgetest`'s shared adapter-conformance suite,
`internal/forge/fake`) and fixturegit's real, deterministic local git
plumbing for every ancestor topology this story adds — never a live dial to
`api.github.com` or a real GitLab instance, in any register.

## CO-2

Exit discipline is unchanged in shape: 0 clean (a bundle accepted), 1
verdict (`evaluateTree`/`evaluateBundle`'s existing fail-verdict mapping,
untouched by this story), 2 operational. This story adds exactly one new
operational reason (the identifier unresolvable from either source) and
re-words one existing operational message (no bundle found anywhere on the
walked path — now naming the ancestor range walked) without inventing a new
exit code or a new verdict path.

## CO-3

Every sync path this story does not target is byte-identical before and
after: `--produce`, `--produce-runtime`, `--force-local`, and the
CI-env-present identifier short-circuit are unchanged, proven by every
pre-existing sync test (`cmd/verdi/sync_test.go`, `sync_helpers_test.go`,
`sync_regen.go`'s own tests) continuing to pass unmodified alongside this
story's new negative-path coverage — a regression obligation, not a hope.
