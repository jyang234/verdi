# Wave 6 Workbench Presentation Implementation Plan

> **For Claude Code:** begin every implementation unit with
> `/fable-orchestration`. FABLE remains chief architect and final producer-side
> judge. Dispatch non-frontend production to Sonnet, every frontend production
> or fix to a FABLE frontend worker, and accepted defect repairs to fresh Opus
> workers. Stop for independent Codex review; never claim Codex approval.

**Goal:** Deliver complete, accessible, responsive, live Wave 6 workbenches for
ASD, CI, GLG, and CSE over the same typed application cores used by CLI and MCP,
without browser-owned authority or a shadow database.

**Architecture:** Complete four missing browser-neutral predecessor seams and
the two ASD stop-gate corrections, then add four fully serialized
server-rendered workbench units. Reuse the Wave 3.5 four-area shell and
dependency-free JavaScript. Every read and action delegates to a typed
application operation; accepted Git, strict artifacts, policies, governance
kernels, execution receipts, and lifecycle derivations remain the only
authority.

**Tech stack:** Go, `internal/artifact`, feature application packages,
`internal/workbench`, `cmd/verdi`, server-rendered HTML, dependency-free
JavaScript, the shared dex stylesheet, hermetic `fixturegit`, built-binary Go
tests, live MCP tests, and Playwright.

**Authority:**
`docs/superpowers/specs/2026-08-29-wave-6-workbench-presentation-design.md`,
the four effective accepted feature specs, the Wave 5 CSE lifecycle design,
the Wave 3.5 pilot plan/report, orchestration Wave 6, SI-162–SI-168, and the
owner-approved Task 2 stop-gate corrections in SI-176–SI-177.

**Planning base:** `915529f792f7a672e9631f42909995b38ed12655`

**SI-177 amendment base:** `ab7518975b6621aceeef4607cca29d9a87cd75b7`

**Execution status:** The original planning authority and SI-176 amendment are
independently reviewed, owner-approved, and merged. Task 1B and Task 2 are
blocked until the consolidated SI-177 amendment passes its one independent
review and closure and its exact head merges to the configured default branch.
No implementation prompt may execute from an unmerged authority branch.

## Contents

1. Global constraints
2. Unit protocol and evidence bundle
3. Task 1 — ASD application predecessor
4. Task 1A — ASD browser-human authority correction
5. Task 1B — ASD writer-process transaction correction
6. Task 2 — ASD workbench
7. Task 3 — Constitution application predecessor
8. Task 4 — Constitution workbench
9. Task 5 — GLG predecessors
10. Task 6 — GLG workbench
11. Task 7 — CSE human-proof coordinator
12. Task 8 — CSE workbench
13. Task 9 — Wave 6 integration
14. Planning authority review gate
15. Exact Claude Code handoff template

## Global constraints

- Units are serialized in the order below. Each starts from its predecessor's
  owner-merged exact head and receives one branch and pull request.
- Each unit is Tier 3. Use TDD: semantic RED, minimum GREEN, focused refactor,
  producer-side FABLE/Opus review, commit, then independent Codex review.
- Do not edit canonical specs. Stop with `NEEDS_CONTEXT` if binding semantics or
  a required typed predecessor cannot be composed without invention.
- UI adapters never derive domain semantics or write feature artifacts
  directly. There is one feature application operation per browser action.
- Frontend files, rendered markup, CSS, JavaScript, and Playwright are FABLE
  worker-owned for implementation and fixes.
- No network in tests. Fixtures are deterministic and live under `testdata/`.
- Preserve exact 0/1/2 classification in typed browser results.
- Preserve the closed route, actor, proof, refresh, accessibility, and
  performance contracts in the design.
- No push, PR, merge, rebase, amend of reviewed history, or next-unit work from
  a Claude implementation session.
- Task 2 remains paused until Tasks 1A and 1B are independently reviewed,
  owner-approved, and merged. Its frontend branch must restart from the Task 1B
  owner-merge exact head.

## Unit protocol and evidence bundle

| Task | Branch after predecessor merge | Risk | Report leaf | Commit subject |
|---:|---|---|---|---|
| 1 | `agent/wave6-asd-core` | Tier 3 | `task-1-asd-core-report.md` | `Complete AI-assisted design application operations` |
| 1A | `agent/wave6-asd-human-authority` | Tier 3 | `task-1a-asd-human-authority-report.md` | `Permit explicit browser-human draft mutations` |
| 1B | `agent/wave6-asd-writer-reentry` | Tier 3 | `task-1b-asd-writer-reentry-report.md` | `Permit mutations inside the writer process` |
| 2 | `agent/wave6-asd-workbench` | Tier 3 | `task-2-asd-workbench-report.md` | `Present synchronized AI-assisted design` |
| 3 | `agent/wave6-constitution-core` | Tier 3 | `task-3-constitution-core-report.md` | `Establish the constitution application workflow` |
| 4 | `agent/wave6-constitution-workbench` | Tier 3 | `task-4-constitution-workbench-report.md` | `Present the project constitution` |
| 5 | `agent/wave6-glg-core` | Tier 3 | `task-5-glg-core-report.md` | `Complete lifecycle readiness and recovery` |
| 6 | `agent/wave6-glg-workbench` | Tier 3 | `task-6-glg-workbench-report.md` | `Present governed lifecycle journeys` |
| 7 | `agent/wave6-cse-human-proof` | Tier 3 | `task-7-cse-human-proof-report.md` | `Share experiment human proof orchestration` |
| 8 | `agent/wave6-cse-workbench` | Tier 3 | `task-8-cse-workbench-report.md` | `Present comparative experiments` |

Task 9 is a controller integration gate, not a ninth feature commit.

For every unit, FABLE creates an ignored report under:

```text
.superpowers/sdd/2026-08-29-wave-6-workbench-presentation/<unit>-report.md
```

The report records:

- exact base/head/branch and clean-state proof;
- authority and predecessor reads;
- exact declared write set and exclusions;
- semantic RED and minimum GREEN output;
- all Opus findings and FABLE adjudications;
- correction and fresh Tier 3 Opus re-review evidence;
- focused race, vet, lint, format, diff, binary/wire, and Playwright gates;
- cannot-verify and disclosed-as-unproven items; and
- commit subject/footer and no-push/no-PR state.

Every implementation commit uses an imperative subject and exactly:

```text
Co-Authored-By: Codex Fable 5 <noreply@anthropic.com>
```

FABLE routes non-frontend production to `impl-sonnet-max`, the finding review
to `review-opus-max`, every accepted Tier 3 Critical/Important defect to a
fresh `fix-opus-max`, and the closure challenge to a fresh
`review-opus-max` distinct from both prior agents. FABLE adjudicates all four
outputs against authority before returning anything to Codex.

Every Opus review covers authority conformance, duplicated semantics, actor and
proof custody, strict decoding and aliasing, Git snapshot/stale-write behavior,
filesystem and zero-effect safety, CLI/MCP/browser parity, test bite and
determinism, and—when frontend is present—keyboard/focus, accessibility,
responsive layout, refresh races, and shadow persistence.

After every Playwright invocation in every UI unit, the report records this
scan and requires no output:

```bash
find e2e -type f \( -path '*/test-results/*' -o -path '*/playwright-report/*' -o -name '*.webm' -o -name '*.mp4' -o -name 'trace.zip' \) -print
```

## Task 1: Complete the ASD application core and adapter parity

**Owner:** Sonnet implementation worker under FABLE orchestration. Opus reviews
and fixes. No frontend files.

**Likely files:**

- Create `internal/designapp/*.go` and tests.
- Modify `internal/draftmutation/*.go` only for narrow shared ports or custody
  corrections proven necessary by RED.
- Modify `internal/designprovenance/*.go` only for the existing canonical read
  seam.
- Modify ASD command adapters under `cmd/verdi/design*.go` and built-binary
  tests.
- Modify `internal/mcpserve` registration/decoder/conformance tests for the six
  ASD operations.
- Modify `internal/specalign` and showcase mappings only when live fixed-set
  gates require the exact new inventory.
- Do not modify `internal/workbench`, assets, CSS, JavaScript, or Playwright.

**Required operations:** `get_board`, `get_design_context`,
`get_design_capabilities`, `mutate_draft`, `get_design_provenance`, and
`prepare_design_review`.

- [ ] Read effective ASD AC-2/AC-5–AC-8, artifact contracts, SI-123/SI-124 and
      SI-163, existing board/draft/provenance code, and CLI/MCP registry rules.
- [ ] Define consumer-owned ports in `internal/designapp`; do not move schema or
      mutation algorithms out of their existing owners.
- [ ] Write a semantic RED proving the missing `designapp` and adapter
      operations cannot satisfy one conformance suite.
- [ ] Implement strict, deep-copy-safe application request/results for all six
      operations.
- [ ] Route every CLI/MCP operation to the same application methods. MCP actor
      stays agent-controlled; browser actor is not implemented here.
- [ ] Prove bounded context, direct-edit disclosure, stale revision refusal,
      exact capability enforcement, deterministic review, and provenance only
      on explicit request.
- [ ] Leave the pre-existing shipped `boardspecapi.go` splice path byte-for-byte
      unchanged. Task 1 is limited to `designapp` plus CLI/MCP adapters; it must
      not connect the workbench to `designapp` or add a second board path.
- [ ] Run focused and full affected-package race tests, built-binary ASD tests,
      live MCP tests, vet, lint, gofmt, and diff checks.
- [ ] Complete FABLE/Opus producer review and stop for independent Codex review.

**Stop gate:** If semantic review, context, or actor attribution needs a new
schema/authority choice, stop before production edits and report it.

## Task 1A: Correct ASD browser-human mutation authority

**Owner:** Sonnet implementation worker under FABLE orchestration. Opus reviews
and fixes. Codex independently adjudicates the completed unit. No frontend
files.

**Authority:** Wave 6 design §§4.1 and 6.1.1; ASD AC-1/AC-3/AC-4/AC-7,
DC-9, CO-2–CO-6/CO-9; SI-163 and SI-176.

**Likely files:**

- Modify `internal/draftmutation/policy.go`, `service.go`, and focused tests.
- Modify `internal/designprovenance` schema/codec/record owners and tests.
- Modify directly affected `internal/designapp` and adapter conformance tests
  only for the V2 provenance projection and constructor-call inventory.
- Modify deterministic fixtures/goldens only when their exact V2 bytes move.
- Do not modify `internal/workbench`, `cmd/verdi/serve*`, assets, CSS,
  JavaScript, Playwright, accepted feature specs, this plan, or the ledger.

**Interfaces:**

- Produce `draftmutation.NewUnauthenticatedHuman() (Actor, error)` as the only
  public constructor for the explicit browser-human actor.
- Keep `Actor.Kind() == ActorHuman`, kernel unauthenticated attribution, and no
  harness/session, while sealing a private basis distinct from
  `NewTrustedHuman` with violated or unproven resolution.
- Preserve delegated-agent policy behavior and the existing no-bypass matrix
  for violated/unproven principal resolution.
- Add strict `verdi.design-provenance/v2`; keep V1 strict decode-only history;
  make all new writers emit V2.
- V2 forbids `policy_digest` and instead requires one non-null top-level
  `policy` object with exactly one arm:
  `{"state":"resolved","digest":"sha256:..."}` or
  `{"state":"not-applicable"}`. The latter forbids `digest` and is emitted
  only for the explicit browser-human actor when policy authority returns the
  canonical not-adopted condition. V1 retains required `policy_digest` and
  forbids `policy`; unknown, missing, null, duplicate, cross-arm, and trailing
  data fail closed in both versions.
- For explicit browser humans, a valid adopted effective policy contributes
  only its sealed digest; `design_assistance` presence/mode cannot authorize or
  refuse the mutation. Malformed adopted authority remains operational.

- [ ] RED the missing constructor, the no-policy browser refusal, and the V2
      schema symbols before production edits.
- [ ] RED a mode `off` browser-human mutation against valid adopted authority;
      it must currently refuse and later succeed while recording the effective
      digest.
- [ ] RED a valid adopted effective policy with no `design_assistance` payload;
      the browser-human mutation must succeed and record the effective-policy
      digest, while the delegated-agent path retains its missing-payload
      refusal.
- [ ] RED an unproven/violated `NewTrustedHuman` mutation under `off` and prove
      it remains refused after the correction.
- [ ] RED V1 history decode and mixed V1/V2 log continuity before version
      dispatch; prove exact historical bytes remain accepted and unchanged.
- [ ] Implement the sealed actor basis and actor-first authorization split.
- [ ] Implement one shared effective-policy identity resolution path. Treat
      only `policyauthority.ErrNotAdopted` as `not-applicable`; reject nil,
      malformed, unsealed, or otherwise failing adopted authority
      operationally.
- [ ] Implement strict V1/V2 dispatch, V2 deterministic encoding, closed policy
      arms, alias-safe projections, canonical round-trip validation, and own
      digest binding. Do not hash or synthesize a no-policy sentinel.
- [ ] Prove agent V2 entries always use `resolved`, browser-human absent-policy
      entries use `not-applicable`, and valid-policy browser-human entries use
      `resolved` without mode gating.
- [ ] Prove the sole production caller inventory for
      `NewUnauthenticatedHuman`: the resumed workbench handler is not present
      yet, so this unit exposes the constructor and structurally proves no
      current CLI/MCP/request-decoder path can call or mint it. Task 2 adds the
      one browser call and updates the inventory to exactly one.
- [ ] Run focused and full affected-package races, vet, lint, gofmt, strict
      fixture/golden ratchets, `internal/specalign`, `internal/showcasealign`,
      and diff checks.
- [ ] Complete FABLE/Opus producer review and stop for independent Codex review.

**Required semantic RED:**

```bash
GOCACHE=/private/tmp/verdi-wave6-task1a-gocache \
go test ./internal/draftmutation ./internal/designprovenance \
  -run 'Test.*(UnauthenticatedHuman|HumanPolicyPosture|DesignProvenanceV2)' \
  -count=1
```

Expected: compile/test failure on the missing constructor, V2 schema/policy
union, and no-policy browser authorization path. A sandbox-only cache failure
is not semantic RED evidence.

**Commit subject:** `Permit explicit browser-human draft mutations`

## Task 1B: Correct ASD writer-process transaction custody

**Owner:** Sonnet implements and fixes `internal/filelock` and
`internal/draftmutation` under FABLE orchestration. A FABLE frontend worker
owns the correction and any fix in `internal/workbench/boardspecapi.go` plus
its browser-handler tests. Opus independently reviews and fixes accepted
non-frontend defects; Codex independently adjudicates the completed unit. No
markup, CSS, JavaScript, asset, visual, or Playwright file changes.

**Authority:** Wave 6 design §6.1.2; ASD AC-2/AC-7/AC-8; I-12; SI-69 and
SI-177.

SI-177 supersedes SI-69 only for a registry-proven outer lock owned by the
caller process. Every foreign or unproven serve/writer holder retains SI-69's
existing refusal.

**Likely files:**

- Modify `internal/filelock/filelock.go` and focused tests for synchronized,
  exact-file process-local ownership registration and its read-only query.
- Modify `internal/draftmutation/transaction.go` and focused tests for
  per-checkout in-process serialization and proven outer-lock reuse.
- Modify only `boardSpecServer.spliceSpec` and its focused tests under
  `internal/workbench` to run the surviving legacy `spec.md` splice inside
  `WithWriterLock` until Task 2 deletes that path.
- Modify `cmd/verdi/serve_integration_test.go` and narrowly related MCP
  integration fixtures/tests to prove a real served `mutate_draft` succeeds.
- Modify no other board handler, asset, CSS, JavaScript, Playwright, artifact
  schema, provenance schema, policy, actor, accepted spec, this plan, design,
  or ledger file.

**Interfaces:**

- Produce `filelock.HeldByCurrentProcess(path string) (bool, error)` as a
  read-only ownership query. It returns true only when `path` resolves to the
  exact still-open file identity registered by this process's successful
  `Acquire`; matching PID/start bytes without a registry entry return false.
- Keep `filelock.Acquire(path)` non-reentrant and `Release(file, path)` as the
  sole owner release. Registration begins only after successful acquisition
  and ends only when that exact registered handle is released.
- Keep `draftmutation.WithWriterLock`'s public signature unchanged. Its inner
  reuse is private and may occur only after ordinary acquisition returns
  `ErrHeld` and `HeldByCurrentProcess` proves ownership.
- Derive the in-process mutex key as the cleaned absolute writer-lock path only
  after the existing component-by-component symlink/non-directory checks. Do
  not resolve a forbidden symlink into an allowed alternate key.
- Keep `boardio` annotation JSONL outside the draft spec/provenance transaction;
  Task 1B makes no system-wide all-file serialization claim. The temporary
  legacy splice participates because it writes the same `spec.md` bytes that a
  draft transaction replaces.

- [ ] Read I-12, SI-69/SI-177, filelock acquisition/release/stale-takeover
      tests, draftmutation transaction/journal tests, `verdi serve`, standalone
      `verdi mcp`, and the live MCP `mutate_draft` adapter before editing.
- [ ] RED a process that acquires the checkout writer lock and then calls
      `WithWriterLock` on that checkout: base must return `ErrHeld`; GREEN must
      run the callback and leave the outer lock present and owned afterward.
- [ ] RED two concurrent `WithWriterLock` calls under one outer process-owned
      lock; prove their callbacks never overlap and both complete in a
      deterministic order controlled by the test. Also prove different
      checkout lock paths are not globally serialized.
- [ ] RED a legacy `boardSpecServer.spliceSpec` read/modify/write racing a live
      MCP `mutate_draft` on the same spec. At base, deterministically pause both
      after their reads and prove one silently loses the other's update; GREEN
      must run the legacy splice first and the MCP transaction second, preserve
      both ordered results, and leave design provenance matching the final spec.
- [ ] RED a forged lock file carrying the current PID/start but absent from the
      process-local registry; it must remain held/unproven and the mutation must
      refuse with zero journal/spec/provenance effects.
- [ ] Implement synchronized ownership registration using exact acquired file
      identity, not PID comparison. Refuse replaced, removed/recreated,
      unreadable, or path-mismatched lock files.
- [ ] Implement the per-canonical-lock-path transaction mutex. Hold it across
      validation, acquisition/reuse, callback/journal work, and any
      inner-acquired release. Release only a handle acquired by that invocation.
- [ ] Route only the legacy `spliceSpec` callback through `WithWriterLock` and
      retain its existing parse/edit/validate/atomic-write behavior inside the
      callback. Do not create provenance for that legacy path or change its
      response bytes; Task 2 still deletes it completely.
- [ ] Prove existing live foreign-holder refusal, malformed-lock refusal,
      symlink refusal, stale/dead-holder takeover, PID-reuse protection, and
      crash-retained journal recovery remain unchanged.
- [ ] Add one real `verdi serve` MCP-socket regression that sends a valid
      policy-authorized `mutate_draft`, observes the canonical clean result and
      exact spec/provenance mutation, and proves the outer writer lock remains
      continuously held. Its base RED must assert the exact typed
      `operational` / `io-failure` MCP classification and zero mutation, not
      merely error text. Retain the existing external built-binary mutation
      refusal while serve owns the lock.
- [ ] Run focused and full `internal/filelock`, `internal/draftmutation`,
      `internal/designapp`, `internal/mcpserve`, and `internal/workbench` races;
      the focused live serve/MCP integration race; `go vet` over affected
      packages; gofmt, golangci-lint, spec-align, showcase-align, and diff
      checks.
- [ ] Complete FABLE/Opus producer review and stop for independent Codex review.

**Required semantic RED:**

```bash
GOCACHE=/private/tmp/verdi-wave6-task1b-gocache \
go test ./internal/draftmutation ./internal/workbench ./cmd/verdi \
  -run 'Test(WithWriterLockReusesCurrentProcessHolder|LegacyBoardSpliceSerializesWithWriterTransaction|ServeMutateDraftUsesHeldWriterLock)' \
  -count=1
```

Expected: the draftmutation test returns the existing live-holder refusal and
the live-serve mutation returns typed `operational` / `io-failure` before any
mutation, while the deterministic legacy-splice race loses one of the two
updates. A sandbox-only cache or local-socket denial is not semantic RED
evidence; rerun the identical command with the required local permissions.

**Commit subject:** `Permit mutations inside the writer process`

## Task 2: Build the ASD synchronized workbench

**Owner:** FABLE frontend worker. Sonnet may supply only non-visual handler
wiring if FABLE delegates it explicitly; FABLE owns every resulting UI fix.

**Likely files:**

- Modify `internal/workbench/boardspec*.go`, `handler.go`, `assets.go`, and tests.
- Add a route-scoped dependency-free ASD asset under the existing asset owner.
- Modify `cmd/verdi/serve*.go` only to inject `designapp` dependencies.
- Add `e2e/*design*` Playwright coverage and deterministic `testdata/` fixtures.
- Do not change feature schemas, mutation grammar, provenance grammar, context
  compilation, or semantic-review algorithms.
- Start only from the owner-merged Task 1B head. Call
  `draftmutation.NewUnauthenticatedHuman` only in the browser mutation adapter;
  do not reconstruct, wrap, or expose the actor through request data.
- Consume Task 1B's `WithWriterLock` behavior unchanged. Do not add a frontend
  lock, release the serve lock, bypass the transaction, or infer ownership from
  PID/lock-file bytes in the workbench.
- Delete the now-serialized legacy `spliceSpec` path in the same change that
  routes every domain mutation through `designapp`; do not retain it as a
  second serialized writer. Keep annotation append/graduation/deletion on the
  distinct `boardio` files and preserve their existing explicit
  post-clean-transaction ordering rather than folding them into the
  spec/provenance transaction.

- [ ] RED: a built handler/Playwright journey proves unsaved edits are currently
      lost or direct mutation bypasses `designapp`.
- [ ] Render revision, repository posture, four-area shell, exact-three queue,
      complete list, capabilities, and proposed/accepted posture.
- [ ] Put immediate/current concerns before distant downstream concerns, derive
      corrective guidance from exact source facts, explain stage sequencing,
      and label human review plainly while retaining formal secondary evidence.
- [ ] Validate slug/path grammar before authoring and preview exact graduation
      refs, paths, relationships, affected downstream facts, and unknowns before
      any durable mutation.
- [ ] Support capability-driven in-place correction of existing stubs and
      objects after creation through the same typed transaction.
- [ ] Atomically rewire every existing board mutation to `designapp` and delete
      the direct splice/write algorithm from `boardspecapi.go` in this unit. A
      merged workbench with both active paths is forbidden.
- [ ] Drive every mutation through `designapp` with explicit
      unauthenticated-human attribution when no real principal proof exists.
- [ ] Add unsaved-edit protection and conditional refresh preserving form
      state, focus, expansion, and last action result.
- [ ] Add on-demand provenance and semantic review; provenance is never
      authority and direct edits remain `unclassified`.
- [ ] Prove all six ASD operations through browser tests, including stale,
      invalid, inconclusive, keyboard, accessibility, 320px, 200% zoom,
      reduced-motion, and hidden-tab refresh cases.
- [ ] Prove no second mutation path remains and CLI/MCP conformance is unchanged.
- [ ] Run focused workbench/serve races, complete ASD Playwright, spec-align,
      showcase, vet, lint, gofmt, asset-size, response-size, and diff gates.
- [ ] Complete FABLE/Opus producer review and stop for independent Codex review.

## Task 3: Complete the Constitution application core

**Owner:** Sonnet under FABLE; Opus review/fixes. No frontend files.

**Likely files:**

- Create `internal/constitutionapp/*.go` and tests.
- Touch `internal/policyartifact`, `internal/policyauthority`,
  `internal/contextcompile`, or `internal/policyconflict` only for narrow
  consumer ports or proven custody defects; never duplicate their algorithms.
- Add equivalent CLI/MCP adapters and conformance tests.
- Update serialized inventories/spec-align/showcase only when required.

**Required operations:** inspect, propose, validate, impact-review, and
submit-preparation over one exact accepted/proposed Git identity.

- [ ] RED the missing propose-to-impact-review application flow.
- [ ] Resolve one exact accepted tree and one proposal identity per operation.
- [ ] Expose source layers, effective rule ledger, applicability derivation,
      conflict witnesses, exemptions, dispositions, and affected consumers
      without flattening provenance.
- [ ] Keep merge/approval outside the application operation. Preparation writes
      only the existing proposal artifacts.
- [ ] Add strict request/result, stale-head, corrupted-policy, unknown-scope,
      unavailable-judge, unauthorized-exemption, and zero-effect tests.
- [ ] Prove byte-equivalent CLI/workbench-capable records and agent-safe MCP
      read/validation/review projections over hermetic layered policy
      repositories. MCP structurally refuses commit, submission, approval,
      exemption ownership, and semantic disposition before store access.
- [ ] Run affected races, full policy/context integration, binary/wire tests,
      vet, lint, gofmt, and diff checks.
- [ ] Complete FABLE/Opus producer review and stop for Codex.

**Stop gate:** Any new rule grammar, precedence, approval record, applicability
operator, or conflict semantics is `NEEDS_CONTEXT`.

**SI-179 closure correction (must merge before Task 4):**

- [ ] RED the nonempty-but-incomplete caller target case: two registered
      consumers exist, the caller supplies only the passing one, and the
      pre-correction packet incorrectly reports `ready_for_submission: true`.
- [ ] Create `internal/constitutionimpact` as the sole owner of strict
      `verdi.constitution-consumer-inventory/v1` at
      `.verdi/constitution/consumers.json` and canonical
      `verdi.constitution-impact-coverage/v1`; reuse the nested
      `contextcompile.Request`, execution-grant capability document, active
      action-subject vocabulary, and `policyconflict` result without copying
      their grammars or algorithms.
- [ ] Load accepted and proposed inventories at the same exact identities as
      the application operation; for a nonempty constitution layer diff,
      derive the sorted union and evaluate every member through the existing
      accepted-context conflict path. A removed row stays in the union.
- [ ] Prove the closed coverage states. Missing inventory/unknown evaluation is
      disclosed-unproven; duplicate, omitted, extra, or identity-mismatched
      rows are violated-with-witness; malformed present bytes are operational;
      only complete exact evaluation is proven.
- [ ] Demote request `targets` to supplemental preview/presentation inputs.
      They may add output but can never remove canonical rows, establish
      completeness, or improve submission readiness.
- [ ] Make `SubmitPreparation` require proven coverage plus passing canonical
      conflict verdicts. Pin missing inventory, removed-consumer union,
      malformed inventory, unknown evaluation, omitted/extra evaluation,
      supplemental-only success, no-layer-change, accepted/proposed identity,
      CLI/MCP byte parity, and zero-effect behavior.
- [ ] Keep `policyartifact`, `policyauthority`, `contextcompile`, and
      `policyconflict` behavior unchanged; this correction consumes their
      exported contracts and adds no reverse matcher or second evaluator.
- [ ] Run races for `internal/constitutionimpact`, `internal/constitutionapp`,
      `internal/contextcompile`, and `internal/policyconflict`; full
      `cmd/verdi` and `internal/mcpserve` conformance; spec-align, showcase,
      vet, lint, gofmt, and diff checks. Complete the Tier 3 FABLE/Opus chain
      and stop for independent Codex review.

## Task 4: Build the Constitution workbench

**Owner:** FABLE frontend worker.

**Precondition:** Task 3 and its SI-179 completeness correction are
owner-merged. The workbench consumes the canonical coverage witness through
`constitutionapp`; it cannot derive, substitute, or repair the affected set.

**Likely files:** create `internal/workbench/constitution*.go` plus tests,
route-scoped assets, `cmd/verdi/serve` injection, Playwright, and fixtures.

- [ ] RED missing live rule-ledger/impact-review behavior through the real
      handler.
- [ ] Render source layers, owners, scopes, accepted/proposed HEADs, effective
      rules, derivation trails, and three-valued conflict posture.
- [ ] Provide typed proposal editing, validation, impact review, and submission
      preparation through `constitutionapp` only.
- [ ] Keep unknown, opaque, stale, missing-judge, and unauthorized states
      visible; no favorable omission.
- [ ] Apply shared refresh, unsaved-edit, accessibility, responsive, and
      structural performance contracts.
- [ ] Playwright the complete Git-backed proposal journey plus mechanical
      conflict, semantic uncertainty, stale disposition, and no-effect refusal.
- [ ] Prove CLI/MCP/browser conformance and no UI approval/shadow policy store.
- [ ] Run full focused gates and stop for Codex after producer review.

## Task 5: Complete GLG Wave 5 predecessors

**Owner:** Sonnet under FABLE; Opus review/fixes. No frontend files.

**Likely files:**

- Complete `internal/journey` current/eventual derivation and tests.
- Create narrow schema owners for feature attestation, recovery projection, and
  journey metrics where the effective GLG spec requires them.
- Create `internal/journeyapp` as the application consumer.
- Add/complete CLI and MCP adapters, fixed-set inventories, conformance, and
  deterministic fixtures.
- Do not modify workbench, assets, CSS, JavaScript, or Playwright.

- [ ] RED every current deferral: eventual blockers, feature attestation,
      `verdi recover`, and metrics.
- [ ] Implement current/eventual readiness from canonical facts; remove the
      explicit `deriveEventual` deferral without display heuristics.
- [ ] Implement feature-attestation scaffolding/review without agent-authored
      human claims.
- [ ] Implement diagnosis-first recovery. Execute only approved reversible
      local actions through an existing typed executor and explicit
      confirmation; otherwise return guidance and witnesses.
- [ ] Implement metrics only from canonical observational events, with stable
      definitions/provenance and no gate effect or sensitive content.
- [ ] Prove CLI/MCP/application parity, human-only refusals, no-guess recovery,
      privacy, deterministic output, and exact accepted/proposed posture.
- [ ] Run affected/full journey races, binary/wire tests, spec-align, vet, lint,
      gofmt, and diff checks.
- [ ] Complete producer review and stop for Codex.

**Stop gate:** If an action needs an executor or authentication seam not already
ratified, return guidance and `NEEDS_CONTEXT`; do not create one in an adapter.

## Task 6: Build the GLG lifecycle workbench

**Owner:** FABLE frontend worker.

**Likely files:** extend `internal/workbench/readiness*.go`; create
`journey*.go`, `attestation*.go`, `recovery*.go`, and `metrics*.go`; add
route-scoped assets, serve injection, Playwright, and deterministic fixtures.

- [ ] RED the startup-only pilot and incomplete eventual-readiness behavior.
- [ ] Preserve the four areas, queue/rail balance, exact-three preview, full
      inline list, plain/formal labels, and solo-author wording.
- [ ] Add conditional live current/eventual refresh without creating readiness
      state.
- [ ] Render journey, blockers, role obligations, attestation, recovery, and
      metrics from `journeyapp` only.
- [ ] Require explicit confirmation for every permissible recovery action;
      never render an unsupported action as executable.
- [ ] Prove metrics cannot alter ordering, gates, or readiness.
- [ ] Playwright story/feature, hidden-tab refresh, broader roles, attestation,
      interrupted recovery, unsafe refusal, keyboard, accessibility, responsive,
      and performance cases.
- [ ] Run all focused gates and stop for Codex after producer review.

## Task 7: Extract the CSE browser-neutral human-proof coordinator

**Owner:** Sonnet under FABLE; Opus review/fixes. No frontend files.

**Likely files:** `internal/experimentapp/*human*.go` or one narrower
consumer-owned shared package, `cmd/verdi/experiment_human.go`, paired tests,
and conformance fixtures. Do not edit experiment schemas or proof grammar.

- [ ] RED the inability to obtain/verify canonical CSE human proof outside the
      CLI without duplicating private adapter code.
- [ ] Extract exact challenge construction, rendering bytes, proof validation,
      historical accepted-policy lookup, mapping, and kernel resolution into one
      typed coordinator.
- [ ] Keep CLI behavior and bytes compatible.
- [ ] Refuse missing, malformed, short, long, foreign, stale, unmapped,
      ambiguous, and unreachable-historical-head proofs before mutation.
- [ ] Prove the seam cannot accept caller-created trust facts, sign, read keys,
      cache sessions, or expose human operations to MCP.
- [ ] Run experimentapp/cmd/MCP races, built-binary proof journeys, vet, lint,
      gofmt, and diff checks.
- [ ] Complete producer review and stop for Codex.

## Task 8: Build the CSE experiment workbench

**Owner:** FABLE frontend worker.

**Likely files:** create `internal/workbench/experiment*.go` plus tests,
route-scoped assets, serve wiring, Playwright, and deterministic evaluator/Git
fixtures. Application semantics remain in `experimentapp`.

- [ ] RED missing browser parity for the exact 15 CLI operations.
- [ ] Render registration, immutable inputs, capabilities, policy, schedule,
      run state, explanation, reproduction, ratification, capsule, release, and
      closure posture from application results.
- [ ] Keep the initial passive page to one accepted-snapshot projection. Load
      capsule, closure, and detailed run evidence as explicit on-demand
      projections when existing operations do not provide one aggregate read;
      do not compose duplicate accepted-tree enumerations into one projection.
- [ ] Implement offline proof UX: exact canonical challenge display/download,
      manual signing instruction, raw 64-byte signature upload, and zero
      credential access.
- [ ] Decode the shared input-bindings document through its existing owner for
      start/resume; no form-derived second mapping grammar.
- [ ] Keep publish-capsule and release-workspaces distinct and preserve
      first-writer-wins/cleanup retry behavior.
- [ ] Prove all 15 operations, invalid/inconclusive states, stale HEAD,
      protected paths, candidate failures, foreign proof, zero effects, and
      CLI/browser byte or typed-field parity.
- [ ] Prove MCP still exposes exactly `inspect`, `discover-capabilities`,
      `validate-draft`, `review-registration`, `status`, `explain-result`,
      `draft-definition`, `capture-candidate`, `start`, and `resume`, and no
      human-only or later-wave aliases.
- [ ] Apply complete accessibility, responsive, refresh, and performance tests.
- [ ] Run all focused gates and stop for Codex after producer review.

## Task 9: Integrate and close Wave 6

**Owner:** Codex integration gate. Claude may provide one read-only whole-wave
review after the integrated head and evidence package are frozen; Codex
adjudicates and authors any accepted bounded correction through the proper
FABLE/worker lane.

- [ ] Merge Tasks 1–8 in order; prove exact ancestry and clean state after each.
- [ ] Run fixed-set route, CLI, MCP, showcase, and vocabulary inventories.
- [ ] Run a four-feature browser journey in one hermetic repository proving
      shared shell, posture, refresh, human/agent boundaries, and no shadow
      state.
- [ ] Run every feature's complete Playwright suite, accessibility scan,
      320px/200% zoom matrix, hidden-tab refresh, and structural size budgets.
- [ ] After every Playwright invocation, require this recording-artifact scan to
      emit no paths:

  ```bash
  find e2e -type f \( -path '*/test-results/*' -o -path '*/playwright-report/*' -o -name '*.webm' -o -name '*.mp4' -o -name 'trace.zip' \) -print
  ```

- [ ] Run built-binary and live-MCP conformance across every operation.
- [ ] Run fresh foreground gates:

  ```bash
  go test -race ./...
  make verify
  make spec-align
  git diff --check <wave-6-base>..HEAD
  git status --short --branch
  ```

  If an unknown ambient process owns the default e2e port, do not terminate it.
  Rerun locally with a recorded alternate `VERDI_E2E_PORT_BASE` and disclose
  that local port-parity limitation; GitHub still must pass its exact checks.

- [ ] Audit production for direct browser artifact writes, duplicated semantic
      algorithms, browser storage, ambient identity, key access, new network
      calls, and unauthorized routes/operations.
- [ ] Freeze an immutable exact-head review package and request one independent
      whole-wave challenge.
- [ ] Adjudicate every finding; route accepted fixes by severity and frontend
      ownership; perform one closure check only.
- [ ] Publish only after all Critical/Important findings are closed and the
      owner approves.
- [ ] Do not start Wave 7 in this campaign.

## Planning authority review gate

The initial planning authority used this gate for the design, plan, and
SI-162–SI-168. Codex applied the same gate to the consolidated SI-176 amendment
before Task 1A. Before Task 1B receives an implementation prompt, Codex applies
it again to the consolidated SI-177 amendment across this design, this plan,
and the ledger, creating one immutable exact-range diff package with a
published SHA-256. Claude receives a read-only prompt that:

- names the exact base/head/range and diff-package path;
- requires `/fable-orchestration` only to structure the read-only challenge;
- forbids Git commands, edits, commits, pushes, PRs, and implementation work;
- requires full reading of workspace/check-out instructions, all three changed
  artifacts, the four effective specs, Wave 3.5 pilot plan/report,
  orchestration Wave 5–6, the CSE Wave 5 design, and cited ledger rows;
- checks stop-gate completeness, source coverage, public-surface inventions,
  authority ownership, FABLE/Sonnet/Opus/Codex role separation, verification,
  and whether the first implementation prompt is executable without semantic
  guessing; and
- returns `APPROVED` or `BLOCKED` with only authority-backed Critical/Important
  findings and separately labeled non-blocking Minors.

Codex adjudicates every finding and authors at most one correction pass because
this is spec-only authority. The same Claude reviewer receives one immutable
correction package and performs one closure check. There is no third review,
reviewer/fixer chain, or implementation before owner approval and merge of the
closed planning head.

## Exact Claude Code handoff template

After this plan is approved and merged, Codex gives Claude Code one unit at a
time using this form:

```text
/fable-orchestration

You are implementing Wave 6 Task <N>: <title> in the Verdi repository.

Read `/Users/johnyang/.claude/skills/fable-orchestration/SKILL.md` completely
before any other task action.

Use the exact owner-merged base and branch supplied below. Read workspace
AGENTS.md, checkout CLAUDE.md, the Wave 6 authority design, this implementation
plan, the effective feature specification sections named by Task <N>, the
invention-ledger rows cited by the task, and the immediately preceding unit's
ignored report before editing.

FABLE is chief architect and final producer-side judge. Dispatch non-frontend
implementation to Sonnet; dispatch every frontend implementation or fix to a
FABLE frontend worker; use an independent Opus finding reviewer; send accepted
Tier 3 defects to a fresh Opus fixer; then use a fresh Opus re-reviewer distinct
from both. Adjudicate every finding against binding authority. No third
producer-side review round.

Follow TDD: capture the named semantic RED before production code, implement
the smallest GREEN, run the task's exact focused gates, and write the ignored
evidence report. Stay inside the declared write set. Stop with NEEDS_CONTEXT
before editing if a missing authority/interface would require invention.

Commit with the task's exact imperative subject and:
Co-Authored-By: Codex Fable 5 <noreply@anthropic.com>

Do not push, open a PR, merge, rebase, amend reviewed history, start the next
task, or claim Codex approval. Return READY_FOR_CODEX_REVIEW with exact
status, risk tier, base/head/range, commits and footer proof, file list,
implemented contract, explicit exclusions, RED/GREEN/gates, finding-review
verdict, FABLE adjudications, correction range, fresh closure verdict, residual
risks, integration prerequisites, report path, clean status, and diff-check
evidence.

Base: <exact owner-merged SHA>
Branch: <exact isolated branch>
Task authority: <exact task section and citations>
Declared write set: <exact files/packages>
Required RED: <exact command and intended witness>
Required gates: <exact commands>
Commit subject: <exact subject>
```
