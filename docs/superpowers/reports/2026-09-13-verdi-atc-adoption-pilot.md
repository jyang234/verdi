# Verdi-ATC / F13 adoption pilot

## Selection and status

The owner selected Verdi-ATC as the real project for the Verdi MVP adoption
pilot and suggested a remaining F13 implementation. The selected first work
item is **F13: make receipt verification usable by the review coordinator**.
This record selects the pilot and its entry conditions. It is not an accepted
Verdi specification, a sealed implementation Flight Plan, a completed adoption
journey, or authority to change an execution protocol.

The immediate design target is the ATC receipt-verification adapter. Its
current `VerifyReceipt` method accepts a receipt alone and deliberately returns
`ErrUnrepresentableRequest`. The existing Verdi verifier requires a complete
request binding the receipt, receipt-event acknowledgment, candidate, and proof
bundle. The adapter must carry and check the required inputs rather than invent
them or treat a provider summary as proof. The exact request representation,
controller/authority wiring, verdict mapping, and caller ownership need a
bounded design before implementation.

## Exact starting points

| Item | Selected baseline |
|---|---|
| Real project | Verdi-ATC, `github.com/jyang234/verdi-atc` |
| ATC source | `c346c005d117dfb090e1dedd5a89f45080875a5e`, branch `agent/local-build-first-20260912` |
| Existing ATC checkout | `/Users/johnyang/code/verdi-system/verdi-atc-boundary-stabilization-20260910` |
| Verdi tested source | `cdcab51acaa799de8f4df5de900f903d6319229d`; subsequent changes through `a3061123` are evidence-report-only |
| Verdi executable | `/Users/johnyang/code/verdi-system/verdi-mvp-release-readiness-20260913/.build/bin/verdi` |
| Verdi executable SHA-256 | `ff063b0cdcb710988e33d8ca6e53d2430764481296b5e4957f0f0fe2035fe2b3` |

The ATC checkout is clean. Its repository `main` remains at `5d08b6a`, behind
the selected development branch; do not silently use that older head or label
the development branch as landed on main. No ATC remote is configured, and
the selected checkout has no `.verdi/` store. Adoption setup is real work still
to perform. Use an isolated checkout of these real project sources; preserve
the original checkout, Git history, and installed Verdi/ATC pair.

The verified Verdi binary remains the adoption instrument for both journeys.
The ATC implementation candidate may change as the selected work is developed.
If the pilot exposes a Verdi defect requiring a new binary, identify and verify
the replacement release and repeat the acceptance journeys on that release.
Do not swap the installed ATC pin to imply paired-release validation.

## Existing F13 work and the remaining boundary

F13 already contains finding/adjudication validation, a pure candidate-bound
review machine, and a strict replayable review journal. These are not tasks to
implement again. The journal explicitly leaves fresh proof verification,
production storage/board/recorder wiring, and effect dispatch to future owners.
F12 still ends at its immutable candidate handoff.

The first pilot item addresses one prerequisite for those future owners:
faithfully requesting and consuming receipt verification through the existing
Verdi boundary. It does not promise full F13 orchestration, the missing alignment
machine surface, G2 recovery, or F14 countersigning, landing, and closure.
Production receipt-authority composition also has an unresolved trust-fact
boundary: the current handler constructs `source_kind: assembly-resolved`,
which its own encoder refuses. The design must identify an actually supported
source of authority or preserve an explicit unavailable/unproven result.
Changing the request signature alone cannot establish authoritative receipt
verification. Any needed authority correction must be identified and reviewed
in the design rather than disguised as request plumbing.

Fresh baseline check on the selected ATC head:

```text
go test -count=1 ./internal/gatekeeper ./internal/reviewlog
ok  github.com/jyang234/verdi-atc/internal/gatekeeper  0.414s
ok  github.com/jyang234/verdi-atc/internal/reviewlog   0.825s
exit 0
```

This is focused baseline evidence, not a new full ATC gate or integration proof.
The transcript is workspace-local at
`.local/verdi-system/development/mvp-readiness-20260913/verdi-atc-pilot/f13-existing-core-baseline.log`.

## Adoption journey

1. Initialize Verdi in an isolated checkout of the real ATC project and record
   its actual Git/default-branch situation. A local filesystem remote, if used,
   proves only local Git facts; it does not provide forge approvals.
2. Author the selected feature/work item in Verdi, trace its requirements to
   existing ATC and Verdi authority, save a supported board edit, and verify it
   after reload. Do not substitute this pilot brief for the product's artifacts.
3. Settle the bounded adapter design and author meaningful acceptance criteria
   and implementation obligations. A Verdi story needs its supported tracker
   reference/configuration; no fake tracker, human attribution, approval, or
   obligation vouch is silently introduced for an acceptance claim.
4. Follow the project's existing policy, review, and acceptance rules before
   entering its implementation path. Prepare the required sealed Flight Plan.
   Genuine Claude Code implementation begins with `/fable-orchestration`;
   FABLE uses 5.1, Opus uses 5, and the repository's Sonnet/backend and
   FABLE/frontend assignments remain in force. Preserve recorder requirements.
5. Implement the bounded item, inspect alignment and evidence, exercise a
   corrective path, and record all interventions and missing proof. Run the
   declared ATC checks, required full gates, and applicable pinned-binary checks.
6. Have an independent operator complete the second real-project journey using
   the same Verdi release and user-facing instructions. A rerun by the same
   development agent with its existing context is not independent acceptance.

The first journey is not yet complete. Real project policy/tracker setup,
the bounded design, governing acceptance, implementation, and independent
adoption evidence are still outstanding. This pilot does not require changing
the adopted MVP acceptance boundary or forcing authoritative closure.
Actual configured forge approvals/merge enforcement and CI evidence
production/retrieval remain deferred. If a selected step requires one of those
witnesses, that step remains incomplete until the witness exists.

## Source witnesses

- [R0–R3 verification and adoption limits](2026-09-13-mvp-release-rehearsal.md).
- [ATC F13 scope](/Users/johnyang/code/verdi-system/verdi-atc-boundary-stabilization-20260910/docs/superpowers/plans/2026-08-24-verdi-atc-stage-1-orchestration.md:602).
- [ATC receipt-only refusal](/Users/johnyang/code/verdi-system/verdi-atc-boundary-stabilization-20260910/internal/verdiproto/client.go:960).
- [Existing Verdi verification request](/Users/johnyang/code/verdi-system/verdi-mvp-release-readiness-20260913/internal/contextreceipt/schema.go:138).
- [Review-journal boundary](/Users/johnyang/code/verdi-system/verdi-atc-boundary-stabilization-20260910/docs/superpowers/plans/2026-09-12-f13-review-journal.md:18).
- [Execution-boundary gaps](/Users/johnyang/code/verdi-system/verdi-atc-boundary-stabilization-20260910/docs/design/execution-boundary-inventory.md:180).
- [Current receipt-authority limitation](/Users/johnyang/code/verdi-system/verdi-atc-boundary-stabilization-20260910/internal/executor/ownerimpl.go:1775).
