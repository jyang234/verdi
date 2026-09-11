# Public execution contract specification handoff

**Status:** owner-ratified design; reviewed Verdi story proposal ready for the
separately authorized acceptance PR. No runtime implementation, push, PR, merge,
or successful v2 release gate is claimed.

The [story](../../.verdi/specs/active/public-execution-contract/spec.md) carries
five acceptance criteria and ten elaborated static/behavioral obligations.
The [promotion record](../superpowers/specs/2026-09-10-public-execution-contract/promotion.md)
preserves the reviewed sources, maps every bounded amendment, and explains the
future paired evidence producers and existing obligation creation stamps.

## Exact identities

| Item | Identity |
|---|---|
| Verdi base | `8ab423fefa14f6cee3070ca96754928daf28c062` |
| Proposal branch | `design/public-execution-contract` |
| CLI scaffold commit | `f36be8e33f3cebea81b7c9690b5f2b8aeeeee320` |
| Initial consolidated story | `c4de51472d985102aa447939323e70a0b53e6040` |
| Final reviewed specification head | `06adefeea8d757f9a8f068f92f39e69bc7afe3f2` |
| Final reviewed tree | `f7358fac339dd788676b54bfb4a508843992f1a4` |
| Ratified ATC design | `a78ef6600246e6c5235162ed018ef0026958c6c2` |
| ATC ratification / IL-153 commit | `ecbddeb9d35e052994566257e8ed970d9ced05ad` |
| Candidate Verdi binary SHA-256 | `f60c534c73a7dcef3f95a1f20192d84c6db1fd784228a7ed6a995c598f3b59fd` |

This handoff is evidence-only, added after the reviewed specification head.
It changes none of the reviewed story, obligations, ledgers, or source archives.

## Independent review and adjudication

Claude Opus 5 reviewed one fixed packet, with safe mode and all tools disabled,
in session `2dcf80bf-bcef-4692-bed2-3a48abd0d407`. It had no repository access.
The initial review found one alleged blocker and two nonblocking observations.
Codex authored one correction pass; the same reviewer performed one closure.
Both processes exited 0 with `is_error:false`. Closure: **CLOSED; B1 withdrawn;
no remaining blocking concern**. No third review ran.

- B1 alleged that the obligations' `frozen` fields falsely claimed acceptance.
  Rejected after checking accepted obligation-artifact AC-1/DC-1 and
  obligation-seam AC-5: the kind requires creation stamps and permits pre-merge
  authoring. The paved CLI emitted them; default-branch reachability controls
  immutability. Added provenance explanation, retained every stamp. The reviewer
  withdrew the finding after receiving those missing authority excerpts.
- N1: labeled the five AC scope paragraphs; no contract text changed.
- N2: clarified one exact producer result per obligation within the shared job.
  Existing producer/source matching already refuses a missing per-obligation
  record. This does not add a public protocol surface.

The independent review is bounded to promotion and evidence design. Underlying
retained-source groups remain referenced, not newly clause-promoted. Source/Git
checks were performed by Codex; no independent repository traversal is claimed.

## Verification at the final specification head

Commands ran in the isolated Verdi worktree. Go used
`GOCACHE=/private/tmp/verdi-public-contract-ratification-20260910/go-cache`
because the sandbox refused writes to the default cache. No dependency or
runtime source changed to accommodate this environment setting.

| Check | Observed result |
|---|---|
| `go build -o .build/verdi ./cmd/verdi` | exit 0 |
| `./.build/verdi lint` | exit 0; 94 `disclosed-unproven` notices for absent mutable data; no other findings |
| `make spec-align` | exit 0; `ok github.com/jyang234/verdi/internal/specalign 102.532s`; no disclosed skips |
| `git diff --check 8ab423fefa14f6cee3070ca96754928daf28c062 HEAD` | exit 0, no output |
| Exact source/coverage check | both archives match reviewed Git objects; contract §§1–6 match verbatim; 23 amendment / 12 retained-source / 23 operation rows; 10/10 elaborated obligations |
| Local links | seven initial document links resolved; two handoff links resolved before commit; archived locators retain their pinned original base |
| `./.build/verdi journey --json spec/public-execution-contract` | exit 0; `proposed`, `relation:new`, `accepted_baseline:null`, clean head `06adefee` |

Journey discloses absent forge facts, author-vouch proof, future evidence, and
its current contributor limitations. Its ten `elaborated/producer-missing`
notices mean no matching evidence record exists. They do not mean the authored
fields are missing: the actual build-start precondition checks structural
elaboration, as `buildObligationQualityDebts` confirms. No runtime evidence is
fabricated to clear the advisory projection.

Full `make verify`, the full race suites, and ATC's new-pin no-skip boundary
matrix were not run for this specification-only preparation. They are not
claimed green. The future specification PR must pass its unconditional full
`merge-gate`; the two runtime flights and every contract §6 release gate remain
mandatory after acceptance. No Go, CI, policy, frozen incumbent, or parent
feature file changed.

## Evidence and next action

Local evidence: `/private/tmp/verdi-public-contract-ratification-20260910`.
Only visible review text, author dispositions and command results are evidence;
no hidden reasoning is included.

| File | SHA-256 |
|---|---|
| `review.md` | `a916f6b99c3fa5018e52c2d3952c51206f9b3613a5643aa73f3d747d9a704285` |
| `closure.md` | `98a0e4f2612c7d8ba99a009241e61dafdc3dfbf6a7f36d6b43a03d827e89dba8` |
| `adjudication.md` | `217305736447ae326b6c6da6421a4b16e9e4d901ccecf48219bf17250124e3c2` |
| `review-packet.txt` | `42c3044fdfa638bd0e1a41adf7c088ad0962c68516b1765729a7b9caf439f045` |
| `closure-packet.txt` | `e970698b503a5d793dd5481b6941d9990174168fca7e55b2b122d566048f4701` |
| `final-lint.log` | `4c50622b06a68e19687807acb47a17cb1b9790d0870558ca33e338c80bb581f3` |
| `final-spec-align.log` | `4f4cfcfc92b061bdddc04f8249ea9ea895748a02cbc4601d980f0d7c69bfcc0a` |
| `final-coverage.log` | `fbf5674023665b8985a62dbc169b9bc702b15ff9254d15c60a85f3f03c839820` |
| `final-journey.json` | `8bfc0c691d7d84862044b5e57a0bffb0d1a265583aa0aa09ee85b7b57e21cb11` |

The owner’s next authorization may cover pushing only this Verdi specification
branch, opening its PR, and merging it after required checks pass. Acceptance
is the story blob landing in the configured default branch, not this handoff
or the earlier ratification. ATC AGENTS explicitly requires separate external
action authorization; design §7 preserves it. Runtime work can follow that
acceptance gate under the already-authorized two-flight scope.

The Jira ref is unresolved local fake-provider bookkeeping; no real issue was
created or verified. The ATC ratification commit remains local on its existing
boundary branch; authorization for the specification PR does not implicitly
publish that separate accumulated ATC branch.
