# Task 3 accepted handoff

**Status:** main accepts Task 3 at runtime/test head
`dca4015eddf9ea295dfd595450edf35cbda68e17`. Task 4 may consume the interfaces
below. The importer has no CLI/browser adapter yet; no release or MVP acceptance
is claimed.

**Base:** `1cb5882430d99eccfef962edbfd45e1af5803aa2`, the accepted Task 2 handoff.
Task 1/2 reviews remain closed. The only Task 2 runtime adjustment is the
previously assigned statement-first ordering of non-deferred prepared fields.

## Implementation and review provenance

All calls were genuine Claude Code, confirmed from actual model metadata:

- FABLE controller: `claude-fable-5-1`, session
  `63862906-74dc-42ad-b9c1-0a10917be18a`, continuing `/fable-orchestration`.
- Sonnet production: `claude-sonnet-5`; seven commits `d45c814b`, `4c2567a4`,
  `8a2fb872`, `327f3fde`, `351d99d1`, `547376cd`, `453c8b63`.
  The last commit adds missing test witnesses only.
- Independent review: `claude-opus-5`, session
  `67f0abc8-66b0-49f6-81eb-d31a75842295`, REVISE at `a59a34af`.
- Main authority records: `a59a34af`, `2e2db0dc`. The same reviewer CLOSED
  the narrow compiler-enum cross-reference at `2e2db0dc`.
- Distinct Opus fixer: session `7696afd8-d2aa-4035-80b3-f3ca90cd57cf`;
  commits `8346d526`, `76805af8`, `a57bdc1d`, `40628151`, `dca4015e`.
- Distinct Opus re-reviewer: session `1b49bb2b-896a-4168-9bb6-a9f2caf0587c`;
  ACCEPT of `2e2db0dc..dca4015e`, no blocking findings.

Main adjudicated every finding and verified the final clean tree, containment,
test output and previously failing probes. No history was amended or rewritten.

## Accepted behavior and corrections

Preview validates through Normalize before request-derived paths, prepares fields
through the shared helper, recomputes coverage, and binds the candidate/request,
HEAD, committed context, model and actual executable digest. It is read-only.

Apply uses the existing actor-policy dispatcher and checkout lock. It builds one
child commit through existing Git plumbing and publishes one create-only
`design/<slug>` ref, leaving caller checkout/index content intact. A retry checks
committed proof before dirty-context or policy/config reads and can return the
same already-created commit after an uncertain response. A moved branch,
mismatched request/actor or different binary does not reconcile automatically.

Finding 1: both HEAD and cleanliness are rechecked immediately before publication,
after the fault seam. Changed context refuses; the candidate is never re-parented.
Staging alone is no longer suggested as a remedy for dirty context. Independent
probes confirm refusal is recoverable through a new preview.

Finding 2: retained sources remain outside artifact index/lint/default design
context. The compiler's HEAD-tree channel also excludes `.verdi/imports/` before
reading bytes, reporting `spec-import-sidecar` in its complete partition.
SI-200 explicitly supplements the compiler authority design's §5.2 enum; existing
members, ASD reason and worktree-overlay precedence remain.

Finding 4: the sole record decoder checks its declared digest/identity shapes,
closed origins/dispositions/transforms/evidence, source membership and coverage
partition/count invariants. Main additionally found and routed the nested enum
and missing-coverage-entry cases; all now have real RED/GREEN witnesses. Valid
native, external, source-backed, evidence-only, user-added, deferred and retained
support-source records remain accepted.

ReadRecord checks committed source/candidate/record bindings, identifies the
imported revision after ordinary descendant edits, and separately discloses
whether the current spec still matches. The record is neither ASD creation
evidence nor acceptance evidence. Original full-file digests remain fingerprints;
only selected source bytes are retained. Decoding is not source authenticity or
semantic-completeness proof, nor a guarantee against arbitrary rewritten history.

## Frozen consumer interfaces

- `NewService() *Service`; adapters must use this production wiring.
- `(*Service).Preview(ctx, root, Request) (PreviewResult, error)`.
- `(*Service).Apply(ctx, root, Request, expectedDigest, draftmutation.Actor)
  (Result, error)`.
- `ReadRecord(ctx, root, branch, slug) (RecordView, error)`;
  `DecodeRecord([]byte) (Record, error)` is the sole stored-record decoder.
- `draftmutation.AuthorizeMutationPolicy` is the existing dispatch exported
  without changing its decisions. Browser adapters reuse mintBrowserActor;
  CLI adapters construct delegated actors, with no human bypass.

The contract owns JSON keys and error vocabulary. Record sources hold normalized
IDs/labels/ranges/digests, never the original full Source.Data. The committed
payloads are selected Snapshot.Data at the trusted store paths. Do not wrap
Apply in another writer lock or set Engine/Policy/Faults in production adapters.

## Verification

Main's final command at `dca4015e`, exit 0:
`go test ./internal/specimport ./internal/draftmutation ./internal/store ./internal/gitx ./internal/lint -count=1`
— packages 18.468s / 6.612s / 2.461s / 24.553s / 110.156s.

Final-head re-review: specimport 16.216s, contextcompile 12.078s, vet clean;
all supplied probes plus two new recovery/write-set probes pass. Final importer
race: 22.731s. Compiler race at its unchanged fix: 17.933s. Main independently
ran the combined owner/compiler probes with `-race`: specimport 2.784s and
contextcompile 1.443s, exit 0. Final `git diff --check` and tree status are clean.
Native duplicate-source tests prove one corpus spec and no VL-002 duplicate.

Process limitation: the producer captured real RED for foundations and ordering,
but initial Apply/ReadRecord code preceded their tests; early Apply failures were
fixture omissions. Later missing-witness tests passed immediately. The Opus
corrections and main-found gaps have genuine behavioral RED/GREEN evidence.
This history is preserved rather than described as uniformly test-first.

## Next task and remaining limits

Proceed with Task 4 CLI, then its review, then genuine FABLE 5.1 UI. Use the same
service; no adapter parser, policy matrix or draft writer. Present errors safely,
and distinguish retained coverage from mapping or semantic completeness. Preserve
explicit deferral disclosures on both created and already-created results.
Retry disclosures may omit corpus-derived nonblocking findings: retry reads no
new dirty-checkout corpus or policy. The compiler exclusion is scoped to its
HEAD-tree channel; no new declared-context pinning feature is authorized here.

Task 5 still requires whole-wave review, make verify, whole-repo race, a separately
identified binary and assisted F13 rehearsal. Same-release independent adoption
remains the user's subsequent acceptance step. Hosted testing remains deferred.
The installed binary and user ATC checkouts are untouched.

Full evidence is under the workspace's `.local/verdi-system/development/`
`spec-import-f13-20260914/execution/`: task3-resume, task3-review,
task3-authority-closure, task3-fix and its two follow-ups, task3-rereview,
task3-owner-final-probes.txt, and task3-owner-package-gate.txt.
