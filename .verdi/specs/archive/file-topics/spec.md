---
id: spec/file-topics
kind: spec
title: "File Topics"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-QH-4
problem: { text: "files hold topics they do not own and one package name misleads — the audit's remaining low-severity organizational findings, all confirmed. sync.go carries four store/forge bootstrap helpers (loadManifest, resolveRefCommit, buildForge, githubOwnerRepo) consumed by eight OTHER verb files, which point at sync.go as their home — the sync verb file became the binary's de facto manifest/forge module against dispatch.go's own charter. accept.go is 587 lines holding three subsystems: runAccept, the predecessor-supersession flip flow, and the ~180-line stub-match algorithm — whose test file stubmatch_test.go names a production file that does not exist. internal/runtime shadows the stdlib package name, and every one of its three import sites already pays an alias to call it what it is (runtimeprobe). And cmd/e2eharness — the weakest 700 lines in the repo — types its run-git closure three times (only one pinning deterministic dates), threads no context or client timeout through its exec/HTTP surface, installs signal handling only after build/provision so an early interrupt leaks the scratch dir, and names its board-fixture seeding provisionv2, which reads as a second provisioner and is not.", anchor: "#problem" }
outcome: { text: "every file owns one topic and no name misleads. The four bootstrap helpers live in their own cmd/verdi topic file; accept.go holds only runAccept, with stub-match in the production file its test always named and the supersession flow in its own; internal/runtime is internal/runtimeprobe and the three aliases are gone; e2eharness has one run-git helper carrying the deterministic-date env, context and a bounded client through its exec/HTTP surface, signal handling installed before any scratch state exists, and provision_board named for what it seeds. Every move is equivalence-preserving — no exported API, no behavior, no output changes except e2eharness's disclosed hygiene additions — proven by the untouched suites and the e2e gate.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the four store/forge bootstrap helpers (loadManifest, resolveRefCommit, buildForge, githubOwnerRepo) move verbatim from sync.go to their own topic file in cmd/verdi with a doc header naming the topic; sync.go keeps only sync's verb logic; gate_threads.go's stale points-at-sync.go comment is corrected; zero call-site behavior change (the cmd/verdi suite is the proof)", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "accept.go splits by topic: the stub-match subsystem (computeStubMatch and its helpers) moves verbatim to stubmatch.go — giving stubmatch_test.go its production twin — and the predecessor-supersession flip flow moves to its own file; accept.go retains runAccept; no signature, behavior, or message changes (the accept/supersession suites are the proof)", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "internal/runtime renames to internal/runtimeprobe (package runtimeprobe): the stdlib shadow is gone, the three import-site aliases are dropped as redundant, and no other package renames (the synonym pairs stay — the audit adjudicated them as churn)", evidence: [static, behavioral], anchor: "#ac-3" }
  - { id: ac-4, text: "cmd/e2eharness hygiene: one run-git helper carrying the deterministic GIT_*_DATE env replaces the three hand-typed closures; a context and a bounded http client thread through the exec/HTTP surface (waitHealthy gets a per-request timeout); signal handling installs before build/provision so an early interrupt cleans the scratch dir; provisionv2.go renames to say board fixtures; copyTree distinguishes absent (tolerated) from unreadable (error). The e2e suite passing unchanged is the equivalence proof; the hygiene additions are this story's disclosed behavior changes", evidence: [static, behavioral], anchor: "#ac-4" }
links:
  - { type: implements, ref: "spec/code-health#ac-5" }
decisions:
  - { id: dc-1, text: "moves are verbatim: ac-1 and ac-2 relocate code without rewriting it — same identifiers, same bodies, same error text — so the diff reads as pure relocation and the existing suites prove equivalence without new tests. New tests are owed only where ac-4 adds behavior (timeout, early-signal, copyTree error split)", anchor: "#dc-1" }
  - { id: dc-2, text: "the runtimeprobe rename is mechanical: directory + package clause + three import sites, nothing else — the audit measured the blast radius at exactly three imports, each already aliased runtimeprobe, so the rename deletes the aliases rather than adding churn", anchor: "#dc-2" }
  - { id: dc-3, text: "e2eharness stays a test-support tool held to the repo's rules, not gold-plated: no retry logic, no configurability, no logging framework — just the four hygiene gaps the audit witnessed, each the smallest fix (one helper, one ctx thread, one Notify move, one rename)", anchor: "#dc-3" }
constraints:
  - { id: co-1, text: "no network in any test; e2eharness's timeout/signal behavior is proven by the e2e gate still passing plus targeted unit tests where a helper is extractable without a browser", anchor: "#co-1" }
  - { id: co-2, text: "make verify green at every commit; one topic per commit (bootstrap file, accept split, rename, harness) so any regression bisects to one move", anchor: "#co-2" }
  - { id: co-3, text: "scope excludes everything the siblings own and everything adjudicated as churn: no dispatch-table refactor, no workbench split, no synonym renames, no shared-home extractions", anchor: "#co-3" }
frozen: { at: 2026-07-13, commit: efd8b5bcab91a2a5ee46c3e91e35a8fe5122369a, stub_matched: true }
---
# File Topics

## Problem

Files hold topics they do not own and one package name misleads — the
audit's remaining low-severity organizational findings, all confirmed.

sync.go carries four store/forge bootstrap helpers — loadManifest,
resolveRefCommit, buildForge, githubOwnerRepo — consumed by eight OTHER verb
files, which point at sync.go as their home: the sync verb file became the
binary's de facto manifest/forge module against dispatch.go's own charter.
accept.go is 587 lines holding three subsystems: runAccept, the
predecessor-supersession flip flow, and the ~180-line stub-match algorithm,
whose test file stubmatch_test.go names a production file that does not
exist. internal/runtime shadows the stdlib package name, and every one of
its three import sites already pays an alias to call it what it is. And
cmd/e2eharness types its run-git closure three times (only one pinning
deterministic dates), threads no context or client timeout through its
exec/HTTP surface, installs signal handling only after build/provision so an
early interrupt leaks the scratch dir, and names its board-fixture seeding
provisionv2 — which reads as a second provisioner and is not.

## Outcome

Every file owns one topic and no name misleads. The four bootstrap helpers
live in their own cmd/verdi topic file; accept.go holds only runAccept, with
stub-match in the production file its test always named and the supersession
flow in its own; internal/runtime is internal/runtimeprobe with the three
aliases gone; e2eharness has one run-git helper, context and a bounded
client through its exec/HTTP surface, signals installed before any scratch
state exists, and provision_board named for what it seeds. Every move is
equivalence-preserving, proven by the untouched suites and the e2e gate;
e2eharness's hygiene additions are the story's disclosed behavior changes.

## AC-1

The four store/forge bootstrap helpers move verbatim from sync.go to their
own topic file in cmd/verdi with a doc header naming the topic. sync.go
keeps only sync's verb logic. gate_threads.go's stale points-at-sync.go
comment is corrected. Zero call-site behavior change — the cmd/verdi suite
is the proof. Evidence: static + behavioral.

## AC-2

accept.go splits by topic: the stub-match subsystem moves verbatim to
stubmatch.go — giving stubmatch_test.go its production twin — and the
predecessor-supersession flip flow moves to its own file. accept.go retains
runAccept. No signature, behavior, or message changes; the
accept/supersession suites are the proof. Evidence: static + behavioral.

## AC-3

internal/runtime renames to internal/runtimeprobe (package runtimeprobe).
The stdlib shadow is gone and the three import-site aliases are dropped as
redundant. No other package renames — the synonym pairs stay, adjudicated
as churn (code-health dc-3). Evidence: static + behavioral.

## AC-4

cmd/e2eharness hygiene: one run-git helper carrying the deterministic
GIT_*_DATE env replaces the three hand-typed closures; a context and a
bounded http client thread through the exec/HTTP surface (waitHealthy gets
a per-request timeout); signal handling installs before build/provision so
an early interrupt cleans the scratch dir; provisionv2.go renames to say
board fixtures; copyTree distinguishes absent (tolerated) from unreadable
(error). The e2e suite passing unchanged is the equivalence proof; the
hygiene additions are this story's disclosed behavior changes. Evidence:
static + behavioral.

## DC-1

Moves are verbatim: ac-1 and ac-2 relocate code without rewriting it — same
identifiers, same bodies, same error text — so the diff reads as pure
relocation and the existing suites prove equivalence without new tests. New
tests are owed only where ac-4 adds behavior (timeout, early-signal,
copyTree error split).

## DC-2

The runtimeprobe rename is mechanical: directory + package clause + three
import sites, nothing else. The audit measured the blast radius at exactly
three imports, each already aliased runtimeprobe — the rename deletes the
aliases rather than adding churn.

## DC-3

e2eharness stays a test-support tool held to the repo's rules, not
gold-plated: no retry logic, no configurability, no logging framework —
just the four hygiene gaps the audit witnessed, each the smallest fix (one
helper, one ctx thread, one Notify move, one rename).

## CO-1

No network in any test. e2eharness's timeout/signal behavior is proven by
the e2e gate still passing plus targeted unit tests where a helper is
extractable without a browser.

## CO-2

make verify green at every commit; one topic per commit (bootstrap file,
accept split, rename, harness) so any regression bisects to one move.

## CO-3

Scope excludes everything the siblings own and everything adjudicated as
churn: no dispatch-table refactor, no workbench split, no synonym renames,
no shared-home extractions.
