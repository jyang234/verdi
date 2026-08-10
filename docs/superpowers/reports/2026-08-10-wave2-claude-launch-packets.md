# Wave 2 Claude Launch Packets

**Planning base:** `cb7cd6fb4e6123b21469a8ced886aae1f95f4398`

**Concurrency rule:** launch exactly two implementation units after this
planning/authority change is owner-merged. Do not launch the network or CSE
execution unit concurrently with them.

The repository permits four planning/review sessions, but the binding
orchestration gate permits at most two active implementation units. These two
units have disjoint primary ownership. Shared edits in `cmd/verdi` and journey
reason/golden files are small but must still be serialized at commit/push time.

## Packet A — ASD draft-mutation core and structured CLI

```text
You are the Sonnet implementation producer for ASD Wave 2
`draft-mutation-core` plus `structured-cli`.

Create a fresh isolated worktree and branch from the exact current origin/main
AFTER the owner has merged the planning PR containing:
  docs/superpowers/plans/2026-08-10-asd-draft-mutation-core.md
  successor-ledger entries SI-65..SI-70

Resolve and record the full 40-hex origin/main SHA before editing. Stop if the
plan and SI-65..SI-70 are not reachable from that SHA. Do not use uncommitted
files, another lane's branch, or a local-only authority document.

You are not alone in the repository. Preserve unrelated changes, do not revert
other agents, and stop on overlapping ownership.

Read completely before editing:
  /Users/johnyang/code/verdi-system/AGENTS.md
  /Users/johnyang/code/verdi-system/CLAUDE.md
  repository CLAUDE.md
  docs/superpowers/plans/2026-08-10-asd-draft-mutation-core.md
  .verdi/specs/active/ai-assisted-spec-design/spec.md
  orchestration Wave 2/shared-ownership/gate sections
  owner adjudications OD-3/5/8/10/12
  invention-ledger SI-9, SI-30/31/33/34/37 and SI-65..SI-70
  linked artifact, store-layout, policy-authority, governance-principal and
  human-artifact contracts named by the plan

Implement exactly the eight TDD tasks in the approved plan. The core belongs in
internal/draftmutation; the shared sidecar contract belongs in
internal/designprovenance; cmd/verdi is a thin adapter. Use the exact request,
operation, digest, provenance, policy, recovery, warning, path and exit
contracts recorded in SI-65..SI-70. In particular: the request carries a
digest-verified base snapshot for stale changed-target computation; the CLI is
always an unauthenticated delegated-agent and requires --harness (optional
--session), never environment-selected human/principal attribution; a direct
edit is the exact unclassified-gap arm; the request and every response carry
the plan's canonical exact checkout/branch/HEAD/spec identity; and recovery/
mutation uses only the checkout-wide data/writer.lock plus the ratified
data/draft-mutation root. Do not substitute a simpler two-file write or create
a per-spec lock.

Do not implement workbench or MCP adapters, capabilities/context compiler,
semantic review UI, Playwright changes, Git publication/governance verbs, a new
policy schema, a new principal type, arbitrary YAML operations, or
artifact.ClassifyPath coverage.

For each task: add happy and negative tests first, capture RED, implement the
minimum, capture GREEN, then commit with the plan's imperative subject. Fault-
inject every transaction durability boundary. Drive the actual built binary
for CLI behavior. No test may call a model, harness or network service.

Before handoff, self-review the exact diff against the plan's 51/51 coverage
witness and run fresh, unpiped:
  go test -race ./...
  make verify
  git diff --check
  git status --short

Push the branch and open one draft PR. Its body must include exact base/head,
the ASD spec path/blob/first-parent promotion, plan path and commit, SI IDs,
AC/DC/CO-to-test mapping, commit list, changed files, full command output,
three-valued disclosures and revert posture. Do not claim the deferred
workbench/MCP, capability/context or semantic-review portions complete.
```

## Packet B — GLG obligation quality

```text
You are the Sonnet implementation producer for GLG Wave 2
`obligation-quality`.

Create a fresh isolated worktree and branch from the exact current origin/main
AFTER the owner has merged the planning PR containing:
  docs/superpowers/plans/2026-08-10-glg-obligation-quality.md
  successor-ledger entries SI-71..SI-74

Resolve and record the full 40-hex origin/main SHA before editing. Stop if the
plan and SI-71..SI-74 are not reachable from that SHA. Do not use uncommitted
files, another lane's branch, or local-only authority.

You are not alone in the repository. Preserve unrelated changes, do not revert
other agents, and stop on overlapping ownership.

Read completely before editing:
  /Users/johnyang/code/verdi-system/AGENTS.md
  /Users/johnyang/code/verdi-system/CLAUDE.md
  repository CLAUDE.md
  docs/superpowers/plans/2026-08-10-glg-obligation-quality.md
  .verdi/specs/active/guided-lifecycle-governance/spec.md AC-2/DC-5/CO-1..7
  orchestration Wave 2/shared-ownership/gate sections
  owner adjudications OD-1/2/5/9/10
  invention-ledger SI-34, SI-47..SI-51, SI-56, SI-64 and SI-71..SI-74
  inherited obligation artifact/gate/seam/wall and merge-signaled authority
  binding 00..05 semantics named by the plan

Implement exactly the six TDD tasks in the approved plan. Use one strict
quality union, one evidence assessment, the existing evidence writer consuming
humanartifact, one pre-mutation build gate, and one journey consumer port.
Preserve failing evidence witnesses and waiver precedence while preventing
unelaborated positive evidence from satisfying a kind. Pin the exact planning
owner-merge commit as the prospective adoption cutoff: preserve historical
legacy folds at/behind it and refuse post-adoption positive satisfaction from
legacy absence. An elaborated block is only eligible: implement the plan's
exact producer/source matching and commit-based freshness checks; keep
attestation and dependency/environment/policy freshness disclosed unproven
until their missing receipts exist.

Do not edit any frozen obligation, spec, plan or ledger; add a new CLI/MCP verb,
lifecycle state, transition, store root or evidence-record obligation_id;
change feature obligations; implement an experimental advisory launch; touch
frontend presentation; or duplicate profile/renderer/fold/journey semantics.

For each task: add happy and negative tests first, capture RED, implement the
minimum, capture GREEN, then commit with the plan's imperative subject. Tests
must prove no Git mutation on build refusal and no network/model use.

Before handoff, self-review the exact diff against the plan's 26/26 coverage
witness and run fresh, unpiped:
  make test
  make fixture
  make spec-align
  go test -race ./...
  make verify
  git diff --check
  git status --short

Push the branch and open one draft PR. Its body must include exact base/head,
the GLG spec path/blob identity, plan path and commit, SI IDs, requirement-to-
test mapping, commit list, changed files, corpus witness for 282 legacy/27
marker files plus pre/post-adoption behavior, full command output, three-valued
disclosures and revert posture.
Do not claim experimental advisory execution, frontend presentation or frozen
legacy remediation complete.
```

## Queued packet — network enforcement

Do not launch until one Wave 2 slot is free. Its authority is
`docs/superpowers/specs/2026-08-10-default-deny-network-enforcement-design.md`
and SI-75..SI-76. The later producer owns only the Linux execution-workspace
backend and report contract, including the exact container-0 to invoking
effective uid/gid mappings, disabled setgroups and nil credential. CSE isolated
execution begins only after that backend is owner-merged and independently
reviewed.

## Controller/reviewer sequence

For each implementation PR, Codex prepares an immutable exact-head diff
package, performs one independent review against the pinned authority, posts
actionable PR comments, adjudicates every finding, and requests one closure
check after at most one correction pass. The user remains the merge owner.
