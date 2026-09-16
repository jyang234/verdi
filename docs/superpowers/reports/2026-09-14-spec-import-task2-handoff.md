# Mechanical spec import Task 2 — accepted candidate composition

Status: **ACCEPTED by main**, ready for FABLE integration and Task 3.
Runtime head: `8f124b9265c5bb0204efdec064bfbe45569f7201`.
Task base: `5a40fa8b2b7bf5e091c33ea04eb2e2b14e3ddc07`.
This report's commit changes documentation only.

## Delivered interface

- `specimport.Compose(ctx, root, request, plan) ([]byte, []Finding, error)`
  composes an external feature/story through existing scaffold and typed splice,
  or preserves valid native content byte-for-byte. Blocking findings return nil
  candidate bytes. No Git publication, CLI or browser import exists yet.
- `lint.CheckCandidate(ctx, root, relPath, content) ([]Finding, error)` reuses
  existing VL-002/003/005/006 over the corpus plus one in-memory Document.
  Candidate and corrupt-dependency failures block. Other corpus findings are
  separately disclosed, with original rule/path/fact, and do not determine
  candidate readiness. Existing ordinary lint decisions remain unchanged.
- Native candidates meet current requiredness/anchor/attestation rules, including
  old feature shapes, through existing VL-006 helpers. Native bytes are not
  rewritten to manufacture eligibility.
- Shared `prepareCandidateFields` supplies pair deferral and truthful generated
  origins/displaced-value disclosures. It supersedes missing-statement only;
  empty/ambiguous source findings remain blocking until explicitly corrected.
  Pair deferral replaces resolved statements, including explicit mappings; no
  alternate precedence rule was adopted. It makes no retention claim.
- External candidates mirror exact mapped text in frontmatter/body, use bare
  generated object anchors and resolving sections, and remove the canonical
  scaffold AC/stub and orphaned body together. Shared creation-only splice
  section helpers do not change the existing anchor resolver's semantics.

## Review adjudication and scope

Producer `92b1ed0d` needed correction: valid stories were refused, deferral
misattributed user text, ordering diverged, old native requirements were skipped,
and unrelated corpus findings were discarded. Main accepted B1/B2/M1/M2/M4,
required bare anchors (A1), and preserved explicit pair-deferral semantics (M3's
alternate precedence rejected). No new design adoption or rule relaxation occurred.

The first correction range ended at `b15993a5`. Independent re-review closed all
other corrections but found the criterion-side counterpart of the template stub
finding: renaming a generated criterion could retain it as a real requirement.
The final correction `b15993a5..8f124b92` restores an explicit refusal; fresh
independent closure accepted it. Reverting the production fix under current tests
fails exactly the two new criterion regressions.

Template compatibility is intentionally bounded. The existing `ac-1` scaffold
slot is replaced by mapped source requirements, including reworded defaults at
that slot; it is not imported source authority. Other template AC IDs and
non-placeholder stub slugs are refused with guidance, rather than silently
retained or deleted. Custom extension fields/body sections are preserved.
Required source criteria belong in the input spec/mappings, not in the reserved
scaffold slot. Task 5 onboarding must disclose this limitation. A refusal need
not enumerate every defect in one response. Duplicate heading slugs retain the
existing resolver's semantics; no new uniqueness or Markdown parser was added.

All 13 changed runtime/test paths remain in the original Task 2 write set:
`internal/specimport/compose*`, lint candidate files plus the narrow walk decode
extraction, and splice section files. No Task 1 source/profile or fixture changed.
F13 pinned bytes and coverage witnesses therefore remain unchanged.

## Genuine Claude Code provenance

Actual model IDs were checked in the saved stream metadata, not inferred from
commit attribution. Local evidence root:
`.local/verdi-system/development/spec-import-f13-20260914/execution/` in the
workspace, outside the repository.

| Role | Session or child | Actual model |
| --- | --- | --- |
| FABLE controller; actual /fable-orchestration invocation | 4e1e1a2b-0a8a-4bba-8bf1-da2b3ecd2fad | claude-fable-5-1 |
| Producer | a58580138f5fc5583 | claude-sonnet-5 |
| Initial reviewer | 09ae2fe9-a6db-47a0-9036-971b32de7b9c | claude-opus-5 |
| Assigned fixer, including template-stub correction | 6eb95222-4ceb-4186-8695-16e10c40a3f4 | claude-opus-5 |
| Independent re-review | b20abd1c-2651-4c4c-ae76-d7799354249b | claude-opus-5 |
| Fresh criterion fixer | d9a832a0-9195-4aff-9dc0-08cdc0d1f35b | claude-opus-5 |
| Fresh criterion closure | 4c316799-83e5-4665-a059-eed825e84867 | claude-opus-5 |

An earlier child was interrupted without Task 2 writes; FABLE explicitly
redispatched because its Agent API had no resume facility. No native Codex
subagent substituted for these roles. Some compound Claude shell commands were
refused and replaced with ordinary authorized operations. An unnecessary full
build was not available in that fix lane; the integrated build remains Task 5.

## Verification

- FABLE and initial reviewer reproduced the focused and complete affected suites.
- At `b15993a5`, independent re-review: full specimport/lint/splice suites passed
  (0.82s / 106.4s / 0.26s), vet/fmt clean. Subsequent runtime changes affect only
  external composition; lint/splice remain byte-identical to that checked head.
- At final `8f124b92`, fresh closure: full specimport package passed (0.432s),
  vet/fmt clean. Renamed/extra criterion refusals, canonical feature/story,
  no-stub templates, custom fields and original reviewer probes pass.
- Main at `b15993a5`: affected composition/candidate/section tests under race passed
  (specimport 2.128s, lint 1.541s, splice 1.290s).
- Main at final `8f124b92`: `go test -race ./internal/specimport -count=1` passed,
  `ok github.com/jyang234/verdi/internal/specimport 3.099s`.
- Original Compose authoring was not entirely test-first. Shared helpers were;
  later correction regressions captured actual RED before their fixes. No blanket
  original TDD claim is made. No full repository/browser release gate is claimed.

## Mandatory Task 3 handoff

Preview must Normalize the request itself, then use the shared candidate-ready
fields and recompute coverage from their spans through existing `buildCoverage`.
Never reuse stale Plan.Coverage after deferral. Displaced spans become retained
or unresolved under RetainUnmapped, with byte totals reasserted; generated fields
have no source span. Request mappings must remain available to provenance.

Pin statement-first final field ordering for non-deferred manual/F13 corrections
as well as deferral before hashing PreviewResult. A narrow adjustment to the
shared preparation helper is authorized in Task 3 if necessary; do not change
object insertion order or duplicate this logic in adapters. Native Plan findings
and all non-deferrable requirements must reach preview readiness.

Then implement the adopted actual-binary identity, clean-context preview,
actor-policy reuse, atomic exact-write-set commit/ref publication, dirty-safe
retry reconciliation, independently verified historical records and retained
source context exclusion. No Task 4 until its fixed-range review closes.
Hosted testing, installation replacement and user ATC writes remain deferred.
No usable importer/new release/MVP acceptance is claimed: two complete journeys
on the same new release, second independently run by the user, still remain.
