---
id: spec/forge-transport
kind: spec
title: "Forge Transport"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-QH-2
problem: { text: "the forge boundary silently truncates, never times out, discloses its decode policy in one adapter of three, and leaves I-4's promised bundle defense unwired. Every GitHub/GitLab list read returns one default page (30/20) with zero pagination handling anywhere — GetThreadResolution's GraphQL is hard-capped at first:100 with no pageInfo cursor and feeds gate_threads.go, so a PR with an unresolved thread beyond the cap can PASS the gate; ListOpenMRs feeds pendingsupersession.go, so a candidate can silently drop. Every adapter defaults to http.DefaultClient (Timeout: 0) and every CLI entrypoint passes bare context.Background(), so a stalled connection hangs forever — worst under the long-lived verdi serve. getJSON/postJSON are byte-identical between github and gitlab modulo one auth-header line, so any fix must land twice today (jira's doJSON is a third near-twin), and the tolerant-foreign-payload decode policy those helpers apply is documented only in jira.go. A 429 matches no sentinel, so an uncached rate-limited Resolve hard-fails instead of degrading. And upstream.CheckToolPin — I-4's \"additionally… refuse a bundle whose recorded tool differs from the pinned commit\" — is exported, documented, tested, and invoked by nothing.", anchor: "#problem" }
outcome: { text: "one transport seam, drained and bounded. A shared HTTP-JSON transport helper carries the tolerant-foreign-payload decode policy once (code-health dc-1, the ledger record), an auth-setter hook, a default deadline, and the status classifier; github, gitlab, and jira all ride it. Every list read drains to exhaustion — REST page walk, GraphQL pageInfo/endCursor for threads and their inner comments — proven against multi-page fakes whose distinguishing item sits past the first page. HTTP 429 classifies as unavailable so rate limits route to the degrade/retry path. And a fetched evidence bundle's recorded tool provenance is checked against the manifest pin at intake — the I-4 secondary defense finally wired, refusing a mismatch by naming both commits.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "one transport seam: a shared HTTP-JSON helper (get/post: build request, auth hook, deadline, status classify, tolerant-subset decode) consumed by the github and gitlab adapters and by jira's doJSON, carrying code-health dc-1's foreign-payload decode policy in its doc comment as the single disclosure site; the forge and provider contract suites pass unchanged against fake and real adapters (the equivalence proof) and no per-adapter transport copy survives", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "every list read drains: GitHub REST follows per_page + Link/page walk, GitLab follows page params, and GetThreadResolution's GraphQL walks pageInfo/endCursor for review threads AND their inner comments; proven hermetically with multi-page httptest fakes where the decisive item lies beyond page one — an unresolved thread at position >100 fails the gate check, and an open MR beyond the default page size is seen by the pending-supersession scan", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "every outbound call is bounded and rate limits degrade: the shared transport's client carries a default timeout (overridable via the existing injected-client fields), proven against a stalling handler; HTTP 429 classifies to the unavailable sentinel in the shared classifier (provider.ErrUnavailable on the tracker side, the forge side's transient refusal naming the status), so an uncached rate-limited call routes to the documented degrade/retry path instead of a hard failure", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "the I-4 secondary defense is wired: at fetched-evidence-bundle intake, the bundle's recorded tool provenance is checked with upstream.CheckToolPin against verdi.yaml's toolchain.commit before the bundle's records are accepted; a mismatch is refused naming the recorded and pinned commits (a canned mismatched bundle proves the refusal, a matching one passes). If intake discovers the fetched tree genuinely carries no tool provenance to check, that is surfaced to the orchestrator for adjudication — not silently skipped", evidence: [static, behavioral], anchor: "#ac-4" }
links:
  - { type: implements, ref: "spec/code-health#ac-2" }
decisions:
  - { id: dc-1, text: "the seam is a new leaf package (internal/httpjson or internal/forge's transport file — implementer picks the smallest home that avoids a provider→forge import; a provider package must not import the forge port just for transport). It owns: request build, per-call deadline via the client, auth via a caller-supplied header-setter, status classification via a caller-supplied classifier hook (forge and tracker keep their own sentinel taxonomies), and tolerant-subset JSON decode with the dc-1 policy prose. It does NOT own retries — no retry logic exists today and none is added (witness-scoped, code-health dc-2)", anchor: "#dc-1" }
  - { id: dc-2, text: "the default timeout is 30 seconds on the seam's default client, chosen as an obvious, generous ceiling for single REST/GraphQL calls (the CI job's own wall clock is the outer bound; serve gets liveness, not tuning). The existing injected HTTPClient fields keep working — a caller-supplied client is used as-is, so tests and future tuning override without code change. No per-call context plumbing changes in this story", anchor: "#dc-2" }
  - { id: dc-3, text: "GraphQL pagination walks BOTH cursors: reviewThreads(first:100, after:$cursor) and, for any thread whose comments.pageInfo.hasNextPage is true, the inner comments cursor too — an unresolved state can hide in an overflow comment page, and draining only the outer list would re-create the same silent-pass shape one level down. REST drains via per_page=100 plus the page walk (Link header on GitHub, x-next-page/page params on GitLab), stopping on the first short/empty page", anchor: "#dc-3" }
  - { id: dc-4, text: "CheckToolPin wires at the FetchEvidenceBundle intake (cmd/verdi/sync.go's fetch path materializes the derived tree): the recorded tool is read from the fetched bundle's own artifacts (graph.json's tool field — upstream.Graph.Tool) and checked before records are accepted. The exec path stays untouched — go run at the pinned commit is already safe by construction (the audit's adjudication). A missing tool field in a fetched bundle is a disclosed refusal-or-adjudication case per ac-4, never a silent skip", anchor: "#dc-4" }
constraints:
  - { id: co-1, text: "no network in any test: multi-page pagination against httptest fakes serving Link headers / page params / GraphQL pageInfo across calls; the timeout against a deliberately stalling handler with a short injected client; 429 against a canned rate-limit response; CheckToolPin against canned bundles (matching and mismatched pseudo-versions). The forge/provider contract suites (forgetest, providertest) run unchanged — any suite edit is scope creep", anchor: "#co-1" }
  - { id: co-2, text: "witness-scoped behavior change only (code-health dc-2): pagination drain, the timeout, 429 classification, and the bundle-pin refusal are the only behavior changes, each carried by a witness test; every existing happy-path response decodes byte-identically through the seam (the contract suites are the proof). make verify green at every commit", anchor: "#co-2" }
  - { id: co-3, text: "scope excludes the sibling stories: no shared-home extractions beyond the transport seam itself (shared-homes owns the digest/yaml/classify/criteria dups — including jira's criteriaChanged, which this story must not touch even while it edits jira.go's transport), no file moves or renames (file-topics)", anchor: "#co-3" }
frozen: { at: 2026-07-13, commit: 8ff365db1bc3f149f7b6475598a6cea01ad10fef, stub_matched: true }
---
# Forge Transport

## Problem

The forge boundary silently truncates, never times out, discloses its decode
policy in one adapter of three, and leaves I-4's promised bundle defense
unwired.

Every GitHub/GitLab list read returns one default page (30 on GitHub, 20 on
GitLab) with zero pagination handling anywhere in the tree.
GetThreadResolution's GraphQL is hard-capped at `first: 100` with no pageInfo
cursor and feeds gate_threads.go — a PR with an unresolved thread beyond the
cap can PASS the gate. ListOpenMRs feeds pendingsupersession.go — a
pending-supersession candidate beyond the first page silently drops.

Every adapter defaults to http.DefaultClient (Timeout: 0) and every CLI
entrypoint passes bare context.Background(), so a stalled connection hangs
forever — worst under the long-lived verdi serve. getJSON/postJSON are
byte-identical between the github and gitlab packages modulo one auth-header
line, so every one of these fixes would land twice today (jira's doJSON is a
third near-twin), and the tolerant-foreign-payload decode policy those
helpers all apply is documented only in jira.go. A 429 matches no sentinel,
so an uncached rate-limited Resolve hard-fails instead of degrading. And
upstream.CheckToolPin — I-4's "additionally… refuse a bundle whose recorded
tool differs from the pinned commit" — is exported, documented, tested, and
invoked by nothing.

## Outcome

One transport seam, drained and bounded. A shared HTTP-JSON transport helper
carries the tolerant-foreign-payload decode policy once (code-health dc-1),
an auth-setter hook, a default deadline, and the status classifier; github,
gitlab, and jira all ride it. Every list read drains to exhaustion — REST
page walk, GraphQL pageInfo/endCursor for threads and their inner comments —
proven against multi-page fakes whose decisive item sits past page one. HTTP
429 classifies as unavailable so rate limits route to the degrade/retry
path. And a fetched evidence bundle's recorded tool provenance is checked
against the manifest pin at intake — the I-4 secondary defense finally
wired, refusing a mismatch by naming both commits.

## AC-1

One transport seam. A shared HTTP-JSON helper — build request, auth hook,
deadline, status classify, tolerant-subset decode — consumed by the github
and gitlab adapters and by jira's doJSON, carrying code-health dc-1's
foreign-payload decode policy in its doc comment as the single disclosure
site. The forge and provider contract suites pass unchanged against fake and
real adapters (the equivalence proof), and no per-adapter transport copy
survives. Evidence: static + behavioral.

## AC-2

Every list read drains. GitHub REST follows per_page + the Link/page walk;
GitLab follows its page params; GetThreadResolution's GraphQL walks
pageInfo/endCursor for review threads AND their inner comments (dc-3).
Proven hermetically with multi-page httptest fakes where the decisive item
lies beyond page one: an unresolved thread at position >100 fails the gate
check, and an open MR beyond the default page size is seen by the
pending-supersession scan. Evidence: static + behavioral.

## AC-3

Every outbound call is bounded and rate limits degrade. The shared
transport's client carries a default timeout (dc-2), overridable via the
existing injected-client fields, proven against a stalling handler. HTTP 429
classifies to the unavailable sentinel in the shared classifier —
provider.ErrUnavailable on the tracker side, the forge side's transient
refusal naming the status — so an uncached rate-limited call routes to the
documented degrade/retry path instead of a hard failure. Evidence: static +
behavioral.

## AC-4

The I-4 secondary defense is wired. At fetched-evidence-bundle intake, the
bundle's recorded tool provenance is checked with upstream.CheckToolPin
against verdi.yaml's toolchain.commit before the bundle's records are
accepted; a mismatch is refused naming the recorded and pinned commits — a
canned mismatched bundle proves the refusal, a matching one passes. If
intake discovers the fetched tree genuinely carries no tool provenance to
check, that is surfaced to the orchestrator for adjudication, never silently
skipped. Evidence: static + behavioral.

## DC-1

The seam is a new leaf package (internal/httpjson, or internal/forge's own
transport file — the implementer picks the smallest home that avoids a
provider→forge import; a provider package must not import the forge port
just for transport). It owns request build, per-call deadline via the
client, auth via a caller-supplied header-setter, status classification via
a caller-supplied classifier hook (forge and tracker keep their own sentinel
taxonomies), and tolerant-subset JSON decode with the dc-1 policy prose. It
does NOT own retries — no retry logic exists today and none is added
(witness-scoped, code-health dc-2).

## DC-2

The default timeout is 30 seconds on the seam's default client — an obvious,
generous ceiling for single REST/GraphQL calls; the CI job's own wall clock
is the outer bound, and serve gets liveness, not tuning. The existing
injected HTTPClient fields keep working: a caller-supplied client is used
as-is, so tests and future tuning override without code change. No per-call
context plumbing changes in this story.

## DC-3

GraphQL pagination walks BOTH cursors: reviewThreads(first:100,
after:$cursor) and, for any thread whose comments.pageInfo.hasNextPage is
true, the inner comments cursor too — an unresolved state can hide in an
overflow comment page, and draining only the outer list would re-create the
same silent-pass shape one level down. REST drains via per_page=100 plus the
page walk (Link header on GitHub, x-next-page/page params on GitLab),
stopping on the first short or empty page.

## DC-4

CheckToolPin wires at the FetchEvidenceBundle intake — cmd/verdi/sync.go's
fetch path, where the derived tree materializes. The recorded tool is read
from the fetched bundle's own artifacts (graph.json's tool field,
upstream.Graph.Tool) and checked before records are accepted. The exec path
stays untouched: go run at the pinned commit is already safe by construction
(the audit's adjudication). A missing tool field in a fetched bundle is a
disclosed refusal-or-adjudication case per ac-4, never a silent skip.

## CO-1

No network in any test. Multi-page pagination against httptest fakes serving
Link headers / page params / GraphQL pageInfo across calls; the timeout
against a deliberately stalling handler with a short injected client; 429
against a canned rate-limit response; CheckToolPin against canned bundles
(matching and mismatched pseudo-versions). The forge/provider contract
suites (forgetest, providertest) run unchanged — any suite edit is scope
creep.

## CO-2

Witness-scoped behavior change only (code-health dc-2). Pagination drain,
the timeout, 429 classification, and the bundle-pin refusal are the only
behavior changes, each carried by a witness test; every existing happy-path
response decodes byte-identically through the seam — the contract suites are
the proof. make verify green at every commit.

## CO-3

Scope excludes the sibling stories: no shared-home extractions beyond the
transport seam itself — shared-homes owns the digest/yaml/classify/criteria
dups, including jira's criteriaChanged, which this story must not touch even
while it edits jira.go's transport — and no file moves or renames
(file-topics).
