---
id: spec/code-health
kind: spec
title: "Code Health"
owners: [platform-team]
class: feature
status: draft
problem: { text: "a seven-reviewer quality audit of main (ad012a6), pressure-tested by six adversarial verifiers against the specs and PLAN.md's ledger, confirmed real defects the gate does not see. The forge boundary silently truncates every paginated read — GetThreadResolution caps at 100 threads and feeds gate_threads.go, so a PR with an unresolved thread beyond the cap can PASS the gate; ListOpenMRs truncation can drop a pending-supersession candidate — while no outbound HTTP call carries any deadline, the tolerant decode posture is documented in one adapter of three, and I-4's promised refuse-a-mismatched-bundle defense (CheckToolPin) is built, tested, and never invoked. Shared logic has drifted into hand copies against the never-copy-paste rule: the atomic-write idiom four times (two beside the very helper they re-inline, all four missing fsync), the canonjson→sha256 digest tail ten times, the YAML-quote helper three times — and the twin classifyArtifactPath tables have SILENTLY DIVERGED: index omits reaffirmation while lint's comment still claims a mirror, the exact bug class walk.go's own comment memorializes. Honesty gaps: cascadecheck masks a permission error as a clean no-supersession pass; lint's own docs say fourteen rules while nineteen are registered; VL-019 exists in no ratified table. And a 21.8 MB compiled e2eharness binary is tracked at the repo root.", anchor: "#problem" }
outcome: { text: "every witnessed gap fails loud and every shared behavior has exactly one home. All forge/tracker list reads drain pagination and every outbound call carries a deadline, through one shared transport seam whose tolerant-foreign-payload decode policy is stated once and ratified; CheckToolPin guards the fetched-bundle path. The atomic write, the digest tail, the YAML quote, and the path-classification table each live in a single shared package — the classification divergence healed so index classifies reaffirmations — with extraction proven byte-identical where outputs are load-bearing. cascadecheck distinguishes absent from unreadable; stale counts are corrected; VL-019's provenance is ratified; mcpserve's verdi-owned decodes fail closed under a ledgered posture; no build output is tracked. And every finding the pressure test REFUTED is recorded here as a deliberate non-change, so the audit's negative space is as legible as its fixes.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "no build output is tracked: the 21.8 MB e2eharness binary is removed from git and ignored (the harness runs via `go run ./cmd/e2eharness` — playwright.config.ts — so the tracked artifact is pure drift), and a repo check proves no tracked file is a compiled binary", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "the forge boundary drains and bounds: every GitHub/GitLab/Jira list read follows pagination to exhaustion (REST page walk, GraphQL pageInfo cursor — killing the silent gate_threads >100-thread pass and the dropped pending-supersession candidate), every outbound HTTP call carries a deadline, HTTP 429 classifies as ErrUnavailable so rate limits route to the degrade/retry path, and all three adapters share one transport seam (getJSON/postJSON are today byte-identical modulo one auth line) carrying the tolerant-foreign-payload decode policy once, ratified in the ledger; CheckToolPin is invoked on the fetched-bundle intake path, closing I-4's promised secondary defense", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "one home per shared behavior, proven equivalent: a shared atomic-write helper (MkdirAll + temp + write + close + fsync + rename) replaces all four hand copies; a single digest helper replaces the ten canonjson→sha256→sha256-prefixed-hex copies with digest strings proven byte-identical; a single YAML double-quote helper replaces the three yamlDQ copies; the path-classification table moves to internal/artifact and BOTH walks consume it, with index gaining the reaffirmation case its table silently lost; commitdesign's titleCase (byte-indexing, not rune-safe) is deleted for designscaffold.HumanizeName; and the small intra-package dups collapse (evidence candidate-filter/dangling-check, align's byte-identical Preserve pair via one generic, provider's criteria-changed pair, mcpserve's four ref-decode prologues, cmd/verdi's five fold-load prologues)", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "witnessed honesty gaps fail loud: cascadecheck tolerates only os.IsNotExist — an IO/permission error surfaces as exit 2 instead of masking as a clean no-supersession pass; the four err==ErrBoardNotFound comparisons become errors.Is before a %w wrap can silently break them; runtimeprobe's emission-success-is-exit-0 semantic (verdi transcribes an external verdict, it does not compute one) is stated in its header and pinned by a fail-verdict test; mcpserve's verdi-owned decodes (tool args, LockInfo) fail closed with additionalProperties false in the tool schemas — envelopes stay tolerant per protocol — under a ledgered decision; dropped socket connections log to stderr; boardio's doc states the caller-holds-the-write-lock contract the workbench already honors; the three stale fourteen-rules comments plus testdata/violations/README.md are corrected; and VL-019's provenance is ratified in the 02 rule table alongside VL-015..018's R4-I-8 entry", evidence: [static, behavioral], anchor: "#ac-4" }
  - { id: ac-5, text: "files own one topic and names do not mislead: the four store/forge bootstrap helpers accreted in sync.go (loadManifest, resolveRefCommit, buildForge, githubOwnerRepo — consumed by eight other verb files) move to their own topic file; accept.go's cohabiting stub-match subsystem and supersession flow split out (giving stubmatch_test.go its missing production twin); internal/runtime renames to runtimeprobe, retiring the alias every import site already pays; and cmd/e2eharness gets one run-git helper, context and timeouts threaded through its exec/HTTP surface, signal handling installed before provisioning, and provisionv2 renamed to say board-fixtures", evidence: [static, behavioral], anchor: "#ac-5" }
decisions:
  - { id: dc-1, text: "tolerant decode for FOREIGN payloads is policy, not a violation: strict decode (DisallowUnknownFields + trailing-data rejection) is for verdi-owned artifacts and pinned upstream-CLI JSON; a forge/tracker API response is a foreign payload decoded as a tolerant subset — DisallowUnknownFields would turn every upstream field addition into a verdi outage. jira.go already states this; the fix is stating it ONCE at the shared transport seam for all three adapters and ratifying it in the ledger. This adjudicates the audit's strict-decode finding: the decode was right, the disclosure was missing", anchor: "#dc-1" }
  - { id: dc-2, text: "fixes are witness-scoped: behavior changes ONLY where the pressure test produced a witness (pagination truncation, the cascadecheck IO mask, 429 misrouting, index's missing reaffirmation case, the unwired CheckToolPin); every other change is an equivalence-preserving extraction whose load-bearing outputs (digest strings, YAML quoting, scaffold bytes) are proven identical by test. Three-valued honesty applied to refactoring: a claimed no-op without a byte-equivalence witness is unproven", anchor: "#dc-2" }
  - { id: dc-3, text: "the pressure test's REFUTED findings are deliberate non-changes, recorded so they are not re-litigated: the boardio lost-update race cannot occur (writeMu spans load→write in the one process writer.lock permits — the fix is stating that contract in boardio's doc, ac-4, not adding locks); board HTML escaping is correct at all 44 interpolations and boardspec.js is exercised by 30+ Playwright specs; the zip size cap is declined — CI is inside the trust anchor (00 §4, 03 §trust boundary); annotation TS and obligation frozen.at are declared stamps reaching no digest; align's twin reports diverge load-bearingly mid-body (only preamble/tail may share); the workbench stays one package (board files share only two symbols with the rest — split is feasible, deferred until a second consumer needs the projection; mcpserve's import of it is tolerated meanwhile); the synonym package renames and the dispatch-table refactor are declined as churn — the verb-inventory test already pins the drift the latter fears", anchor: "#dc-3" }
  - { id: dc-4, text: "YAML EMISSION gets a quoting seam, not a templating engine: internal/artifact (already the decode seam) gains the one double-quote helper, making the artifact package own serialization posture in both directions; but unifying the designscaffold and commitdesign scaffolds is declined — their outputs are structurally different specs by design (problem/outcome/stubs vs context/dispositions/provenance), and one template would trade real divergence for false uniformity. Only the casing helper is shared (ac-3)", anchor: "#dc-4" }
  - { id: dc-5, text: "dead defenses are wired or descoped, never left ambient: CheckToolPin wires into fetched-bundle intake (I-4's promised secondary defense — no §8 descope exists, and the exec path is already safe by construction via go run at the pinned commit, so the fetched path is the live gap); CachingProvider — constructed only by its own tests — stays unwired but its doc gains one line saying so, with single-flight and eviction recorded as prerequisites for whenever serve wires it. An exported capability nothing calls is a disclosure problem: the reader believes a defense exists", anchor: "#dc-5" }
constraints:
  - { id: co-1, text: "no network in any test: pagination drain is proven against multi-page httptest fakes (Link headers, GraphQL pageInfo), deadlines against a stalling httptest handler, 429 routing against a canned rate-limit response; byte-equivalence proofs (dc-2) are pure unit tests over committed fixtures", anchor: "#co-1" }
  - { id: co-2, text: "make verify is green at every commit and the gate never shrinks; extraction commits are small with imperative subjects, one shared-home per commit, so any equivalence regression bisects to one move", anchor: "#co-2" }
  - { id: co-3, text: "scope is the pressure-test survivor list only — nothing else enters. Explicitly OUT: the cosmetic declines in dc-3/dc-4, dex's inline page template, serve.go's linear ritual length, and rewriting scenario tests into tables (the audit found the table-driven rule alive in the core and softer in cmd; the resolution is ground-rules wording, not test churn)", anchor: "#co-3" }
stubs:
  - { slug: forge-transport, acceptance_criteria: [ac-2] }
  - { slug: shared-homes, acceptance_criteria: [ac-3] }
  - { slug: fail-loud, acceptance_criteria: [ac-1, ac-4] }
  - { slug: file-topics, acceptance_criteria: [ac-5] }
---
# Code Health

## Problem

A seven-reviewer quality audit of main (ad012a6), pressure-tested by six
adversarial verifiers against the specs and PLAN.md's ledger, confirmed real
defects the gate does not see.

The forge boundary silently truncates every paginated read.
GetThreadResolution caps at 100 threads (github.go GraphQL `first: 100`) and
feeds gate_threads.go — a PR with an unresolved thread beyond the cap can PASS
the gate. ListOpenMRs returns one default page (GitHub 30, GitLab 20) and
feeds pendingsupersession.go, so a pending-supersession candidate can silently
drop. No outbound HTTP call carries any deadline (http.DefaultClient
everywhere, zero WithTimeout on any forge/tracker path). The tolerant decode
posture is documented in one adapter of three (jira.go) and ratified nowhere.
And I-4's promised secondary defense — refuse a fetched bundle whose recorded
tool differs from the pin — exists as CheckToolPin (upstream/version.go),
tested, and is never invoked.

Shared logic has drifted into hand copies against the never-copy-paste rule.
The atomic-write idiom exists four times — boardio/boardstate.go and
boardio/graduate.go re-inline it in the same package as writeFileAtomic
(reposition.go), whose own comment names graduate.go as a user;
boardlayout/file.go is the fourth — with divergent behavior (only boardstate
does MkdirAll) and a uniform missing fsync. The canonjson→sha256→
"sha256:"+hex digest tail is hand-copied ten times across seven packages,
every copy cross-referencing another. The YAML double-quote helper exists
three times. And the twin classifyArtifactPath tables (lint/walk.go,
index/walk.go) have SILENTLY DIVERGED: index omits the reaffirmation case in
both classify and decodeEntry while lint's comment still claims a mirror —
the exact bug class lint/walk.go's knownTopLevelEntries comment memorializes
from the last time one walk was patched without its twin.

Honesty gaps: cascadecheck's loadActiveSpecTolerant returns nil-and-no-error
for ANY read failure, so a permission error masks as a clean no-supersession
pass — exit 0 where the contract demands exit 2. Four err==ErrBoardNotFound
comparisons break the day anyone %w-wraps the sentinel. internal/lint's own
doc says "the fourteen VL-001..VL-014 rules" while nineteen are registered
(three code sites plus testdata/violations/README.md), and VL-019 appears in
no ratified rule table — VL-015..018 have R4-I-8; VL-019 has only the
dogfooded obligation-artifact spec. And a 21.8 MB compiled e2eharness binary
is tracked at the repo root, un-ignored, while the harness actually runs via
`go run`.

## Outcome

Every witnessed gap fails loud and every shared behavior has exactly one home.
Forge/tracker reads drain pagination under deadlines through one transport
seam with a ratified decode policy; CheckToolPin guards fetched bundles. The
atomic write, digest tail, YAML quote, and classification table each live
once — the classification divergence healed — with extractions proven
byte-identical where outputs are load-bearing. cascadecheck distinguishes
absent from unreadable, stale counts are corrected, VL-019 is ratified,
mcpserve's verdi-owned decodes fail closed, no build output is tracked. And
every finding the pressure test refuted is recorded in dc-3 as a deliberate
non-change, so the audit's negative space stays as legible as its fixes.

## AC-1

No build output is tracked. The 21.8 MB e2eharness binary is removed from git
and ignored — playwright.config.ts launches the harness via
`go run ./cmd/e2eharness`, so the tracked artifact is pure drift that silently
stales on every source change. A repo check proves no tracked file is a
compiled binary. Evidence: static + behavioral.

## AC-2

The forge boundary drains and bounds. Every GitHub/GitLab/Jira list read
follows pagination to exhaustion — REST page walk with Link/`page=` handling,
GraphQL pageInfo/endCursor loop — killing the silent >100-thread gate pass and
the dropped pending-supersession candidate. Every outbound call carries a
deadline. HTTP 429 classifies as ErrUnavailable so a rate limit routes to the
degrade/retry path instead of a hard failure. All three adapters share one
transport seam — today getJSON/postJSON are byte-identical between github and
gitlab modulo one auth-header line, and jira's doJSON is a third near-twin —
carrying dc-1's decode policy once. CheckToolPin is invoked at fetched-bundle
intake, closing I-4's promised secondary defense. Evidence: static +
behavioral (multi-page/stall/429 httptest fakes per co-1).

## AC-3

One home per shared behavior, proven equivalent. A shared atomic-write helper
(MkdirAll + temp + write + close + fsync + rename) replaces all four hand
copies. A single digest helper replaces the ten canonjson→sha256→
"sha256:"+hex copies, digest strings proven byte-identical over committed
fixtures. A single YAML double-quote helper (dc-4's seam) replaces the three
yamlDQ copies. The path-classification table moves to internal/artifact and
both walks consume it — the walks stay separate, their failure handling
legitimately differs — with index gaining the reaffirmation case its copy
silently lost. commitdesign's titleCase (byte-indexing, not rune-safe) is
deleted for designscaffold.HumanizeName; the unicode divergence is unreachable
behind the slug regex, so this is drift removal, not a bug fix. The small
intra-package dups collapse: evidence's candidate-filter/dangling-check pair,
align's self-declared byte-identical Preserve pair via one generic, provider's
criteria-changed pair, mcpserve's four ref-decode prologues, cmd/verdi's five
fold-load prologues. Evidence: static + behavioral (byte-equivalence tests per
dc-2).

## AC-4

Witnessed honesty gaps fail loud. cascadecheck tolerates only os.IsNotExist —
an IO/permission error surfaces as exit 2, never as a clean no-supersession
pass (the malformed-spec tolerance stays: lint-store backstops it, and that
half of the audit finding was refuted). The four err==ErrBoardNotFound
comparisons become errors.Is. runtimeprobe's header states the
emission-success-is-exit-0 semantic — verdi transcribes an externally computed
verdict, it does not compute one, so a fail verdict exits 0 at emission and is
consumed downstream by the fold — and a test pins the fail-verdict case.
mcpserve's verdi-owned decodes (tool args, LockInfo) fail closed, with
additionalProperties false in tooldefs.go's schemas; protocol envelopes stay
tolerant; the split posture is ledgered. Dropped socket connections log to
stderr — the stdio path already inspects ServeConn's error; the socket path
discards it with no trace. boardio's doc states the caller-holds-the-write-
lock contract the workbench's writeMu already honors. The three stale
fourteen-rules comments and testdata/violations/README.md are corrected to
match the registered rule set. VL-019 gains its ratified rule-table row
alongside VL-015..018's R4-I-8 entry. Evidence: static + behavioral.

## AC-5

Files own one topic and names do not mislead. The four store/forge bootstrap
helpers accreted in sync.go — loadManifest, resolveRefCommit, buildForge,
githubOwnerRepo, consumed by eight other verb files that already point at
sync.go as their home — move to their own topic file in cmd/verdi. accept.go's
cohabiting subsystems split out: stub-match to stubmatch.go, giving
stubmatch_test.go its missing production twin, and the predecessor-
supersession flow to its own file; the spec locates the COMPUTATION in accept
(03 §Stub reconciliation) — only the file layout moves. internal/runtime
renames to runtimeprobe: all three import sites already alias it to exactly
that, paying for the stdlib shadow on every use. cmd/e2eharness gets one
run-git helper (three copies today, only one of which pins deterministic
dates), context and client timeouts threaded through its exec/HTTP surface,
signal handling installed before build/provision so an early interrupt cannot
leak the scratch dir, and provisionv2.go renamed to say what it is (board
fixtures, not a second provisioner). Evidence: static + behavioral.

## DC-1

Tolerant decode for foreign payloads is policy, not a violation. Strict decode
(DisallowUnknownFields + trailing-data rejection) is for verdi-owned artifacts
and pinned upstream-CLI JSON — surfaces verdi controls both sides of. A
forge/tracker API response is a foreign payload decoded as a tolerant subset:
DisallowUnknownFields there would turn every upstream field addition into a
verdi outage. jira.go already states this; github and gitlab apply it
silently. The fix is stating it once at the shared transport seam and
ratifying it in the ledger — adjudicating the audit's strict-decode finding as
"right decode, missing disclosure."

## DC-2

Fixes are witness-scoped. Behavior changes ONLY where the pressure test
produced a witness: pagination truncation, the cascadecheck IO mask, 429
misrouting, index's missing reaffirmation case, the unwired CheckToolPin.
Every other change is an equivalence-preserving extraction whose load-bearing
outputs — digest strings, YAML quoting, scaffold bytes — are proven identical
by test. Three-valued honesty applied to refactoring: a claimed no-op without
a byte-equivalence witness is unproven.

## DC-3

The pressure test's refuted findings are deliberate non-changes, recorded so
they are not re-litigated. The boardio lost-update race cannot occur: the
workbench dispatch holds writeMu across load→write (boardspec.go names exactly
this window) inside the one process writer.lock permits — the fix is stating
that contract in boardio's doc (ac-4), not adding locks. Board HTML escaping
is correct at all 44 interpolations, bodyHTML flows through the corpus render
path, and boardspec.js is exercised by 30+ Playwright specs — no render
rework. The zip size cap is declined: CI is inside the trust anchor (00 §4,
03 §trust boundary); a producer that can bomb a zip can already forge a
verdict. Annotation TS and obligation frozen.at are declared stamps reaching
no digest. align's twin reports diverge load-bearingly mid-body — only the
validation preamble and judge-runner defaulting may share. The workbench stays
one package: board files share only two symbols with the rest, so the split is
feasible, but it is deferred until a second consumer needs the projection —
mcpserve's import of it is tolerated meanwhile. The synonym package renames
and the dispatch-table refactor are declined as churn; the verb-inventory test
already pins the drift the latter fears.

## DC-4

YAML emission gets a quoting seam, not a templating engine. internal/artifact
— already the decode seam — gains the one double-quote helper, so the artifact
package owns serialization posture in both directions. Unifying the
designscaffold and commitdesign scaffolds is declined: their outputs are
structurally different specs by design (problem/outcome/stubs vs
context/dispositions/provenance), and one template would trade real divergence
for false uniformity. Only the casing helper moves (ac-3).

## DC-5

Dead defenses are wired or descoped, never left ambient. CheckToolPin wires
into fetched-bundle intake — I-4 promises "additionally… refuse a bundle whose
recorded tool differs from the pinned commit," no §8 descope exists, and the
exec path is already safe by construction (`go run` at the pinned commit), so
the fetched path is the live gap. CachingProvider — constructed only by its
own tests — stays unwired, but its doc gains one line saying so, and
single-flight plus eviction are recorded here as prerequisites for whenever
serve wires it. An exported capability nothing calls is a disclosure problem:
the reader believes a defense exists.

## CO-1

No network in any test. Pagination drain is proven against multi-page httptest
fakes (Link headers, GraphQL pageInfo), deadlines against a stalling httptest
handler, 429 routing against a canned rate-limit response. Byte-equivalence
proofs (dc-2) are pure unit tests over committed fixtures.

## CO-2

make verify is green at every commit and the gate never shrinks. Extraction
commits are small with imperative subjects, one shared-home per commit, so any
equivalence regression bisects to one move.

## CO-3

Scope is the pressure-test survivor list only — nothing else enters.
Explicitly out: the cosmetic declines in dc-3/dc-4, dex's inline page template
(a consistency nicety, not a dup), serve.go's linear ritual length, and
rewriting scenario tests into tables — the audit found the table-driven rule
alive in the core and softer in cmd/verdi; the resolution is ground-rules
wording, not test churn.
