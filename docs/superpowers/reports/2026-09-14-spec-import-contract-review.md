# Mechanical import contract review adjudication

Status: single genuine Claude Code Opus 5 review completed for contract/plan head
`2256d6a96f17304f89ea962dbc5a673271307bbb`; one main-authored correction prepared,
same-reviewer **CLOSED** on `ef52a6e083a6a4d49b078e4ba9a2f7e10c5d15cf`.
Main accepts closure; implementation may proceed under the reviewed contract.

## Provenance

The owner explicitly authorized the 15-file packet's transfer to Anthropic and
subsequent Verdi source/tests needed by assigned importer workers. The earlier
rejection is preserved; after permission the actual CLI review exited 0 with
result success/is_error false. Init and assistant model were `claude-opus-5`, tools
empty, session `8301939b-2ccd-49a9-8b10-ccacf79b3d1e`. Packet SHA-256:
`6a1ac178914b365f70784f026ff16f8cb2945fa366c9124822ca649437140b78`.
Local packet/stream/process/answer files are under
`.local/verdi-system/development/spec-import-f13-20260914/contract-review/`
in the workspace root. No other Claude model authored this contract.

## Main adjudication

| Finding | Decision and correction |
|---|---|
| B1 validation only at decoder | Accept. Request.Validate owns the complete input grammar and limits; both DecodeRequest and every Normalize/direct service/retry entry invoke it before request-derived paths. Explicitly distinguish validated Source.ID from display-only Label. Add malformed direct-struct cases. |
| B2 lint duplicate retained native source | The claimed runtime failure is refuted by current code: lint.walkDocuments and index both call artifact.ClassifyPath, which admits spec paths only under specs/active or specs/archive and excludes imports. Accept making this scan invariant explicit and pin it with native duplicate-ID Snapshot/ByRef/VL-002 tests. Do not add a second classifier. |
| B3 metadata/title/setext ambiguity | Accept. External leading frontmatter is retained-only opaque metadata removed from Markdown syntax parsing with original offsets preserved. Leading prose is retained; first real ATX or setext heading is title, both forms participate in child/peer rules. Add positive metadata/setext fixtures. |
| O1 evidence-only mapping | Accept. A metadata-only AC mapping retains automatic text/origin/spans and changes explicit evidence selections only; absent target refuses. |
| O2 template body placeholders | Accept. Candidate composition replaces statement bodies and removes orphaned placeholder AC body/stub. A narrow creation-only shared splice helper is allowed; no change to existing template bytes or ordinary mutation behavior. |
| O3 stubs disposition | Accept. External imported feature omits stubs and invents no decomposition. Current validateFeature checks entries when present and does not require a nonempty stubs list for draft creation; later acceptance rules remain. Task 2 tests this. |
| O4 dirty retry | Accept. After basic request validation, committed reconciliation precedes cleanliness/stale gates and reads no dirty candidate/config inputs. |
| O5 CAS loser | Accept. Failed create-only publication retries exact reconciliation once before reporting collision. |
| O6 actor structural pin | Inspected: the actual test is ExactlyOneProductionCaller and the existing workbench mintBrowserActor helper owns that call. Reuse this helper, preserve the test unchanged; no new constructor allowance needed. |
| O7 ASD unclassified versus import origin | Preserve ASD semantics and schema. Add adjacent verified import-record link in review UI and read-only CLI record command; explain separate creation provenance rather than pretending an ASD entry exists. |
| O8 binary identity | Accept. Preview/record bind engine_digest to actual executable SHA-256; same-release replay uses that binary. Different engine refuses automatic reconciliation and directs to historical read-only inspection. Hermetic binary-identity port permits stable tests. |
| O9 unresolved byte arithmetic | Accept. Complement is unresolved when RetainUnmapped false, retained-only when true; mapped union counts once, and three disposition totals sum exactly to source bytes. |
| O10 apply deferral disclosure | Accept. Result includes statements_deferred plus nonblocking statements-deferred disclosure, including already-created retries. |
| O11 F13 selection route parity | Accept. Pin extracted snapshot versus original plan lines 602–671 to identical selected digest, without asserting their different source metadata/request digests match. |
| O12 interval/unit counts | Clarify tests: 17 byte intervals, 8 mapped spans; separate eight line-accounting units. No source bytes or selectors change. |
| O13 source-like IDs | Accept an explicit blocking finding for the narrow leading ID-colon form until mapping resolves/preserves it; do not silently replace a recognizable source identifier with a generated ID. |

## Verification and authority boundary

The main inspected artifact.ClassifyPath, lint.walkDocuments/Snapshot, the existing
splice operations, scaffold template, validateFeature and browser actor helper/
structural pin. `go test ./internal/draftmutation -run '^TestNewUnauthenticatedHumanHasExactlyOneProductionCaller$' -count=1`
exited 0. This confirms the current caller inventory only, not the future importer.
Authored-document whitespace check passed before committing the correction.

The owner-adopted scope remains mechanical one-target import. These repairs fix
implementation details and executable tests inside that scope. They do not change
acceptance/evidence gates, claim full release validation or require another design
adoption. The same reviewer receives this one consolidated correction for closure;
any residual after closure returns to main adjudication, not a third review round.

## Closure disposition and bounded execution decisions

The same genuine Claude Code Opus 5 session returned CLOSED for `ef52a6e0`,
process exit 0, result success/is_error false. B1–B3 closed; O1–O13 disposed.
Main accepts the refutation of B2 and the complete closure. No third review round.
The local closure packet/result are beside the initial contract review artifacts.

Residuals explicitly adjudicated by main before dispatch:

1. Retain invalid-source / exit 2 for an unclosed leading frontmatter delimiter:
   it is an invalid selected serialization under this closed grammar, consistent
   with invalid UTF-8/source/envelope handling. Ordinary unresolved mapping is
   still a completed verdict. No code/contract change is needed.
2. The record CLI command is binding through the provenance section and Task 4;
   include it in actual usage/help/docs. Its omission from the introductory list
   is editorial, not permission to omit implementation.
3. Zero stubs remains required for imported external features. Task 2 must test
   both strict decode and existing candidate lint. A discovered conflict returns
   to main; never invent a decomposition to make creation pass.
4. For a source-ID-looking blocked list item, do not emit an automatic field.
   Its bytes are part of the ordinary unmapped complement (retained-only when
   RetainUnmapped=true, unresolved otherwise); the independent blocking finding
   prevents ready=true. The item's ordinal is still counted, so following
   generated IDs keep their source-position numbering. An explicit mapping to
   the source-declared ID over that item resolves the blocker; test this exact
   path and neighbouring IDs. This pins the implementation choice without a
   third review round or silent default.
5. The old policy.go test-name comment is pre-existing drift. Reuse the current
   mintBrowserActor helper and preserve the actual exactly-one structural test;
   no unrelated comment cleanup is assigned.

Source-transfer authorization now includes this contract packet and subsequent
Verdi source/tests needed for assigned importer work through genuine Claude Code,
excluding credentials/secrets and unrelated projects. No new design adoption is
required. Full importer/release evidence remains to be produced by Tasks 1–5.
