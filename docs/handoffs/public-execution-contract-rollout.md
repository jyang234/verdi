# Public execution contract: paired rollout

This operating note applies the [ratified contract](../../.verdi/specs/active/public-execution-contract/spec.md), sections 4–6. It does not authorize publication or deployment. A release requires the completed checker report for the exact pair and the genuine `public-execution-contract-release` job; local results alone leave release CI unproven.

## Release inputs and evidence

Keep the two clean source commits and trees, measured ATC and Verdi executable SHA-256 values, exact `verdi context contract` bytes, governing story and ten obligation hashes, frozen fixture manifest, and executed validation/deletion witness together in the release report. The ten distinct `public-execution-contract:ac-N:{static,behavioral}` results must each satisfy their own scope. A broad job pass cannot replace a missing, stale, skipped, or unproven producer.

Both repositories must pass their ordinary `make verify` and full race suites. ATC must also pass `make boundary-check` against the measured candidate pin with all nine required tests and zero skips. Historical fixed-pin skips in the full suite remain explicitly disclosed. The final consolidated pair must repeat the all-operation exchanges, zero-translation launch trace, five crash cuts, and copied-store replay/refusal checks in both directions.

The manual workflow obtains ATC from an owner-provisioned private Git bundle using a protected secret URL, an approved bundle SHA-256, and exact baseline/candidate commits. It verifies and builds isolated sources. ATC has no configured remote; this process does not create one. The secret URL, private bundle, source checkouts/archives, and instrumentation source overlays must not be uploaded as evidence. Upload only explicitly selected source-free reports, declared evidence records, necessary runtime outcomes/traces, and appropriate logs; retain source identities as hashes.

Local checker output has local provenance. Workflow context in a report does not independently authenticate a GitHub run. CI evidence must be obtained from the actual matching workflow run and its uploaded artifacts. Provisioning the source bundle, publishing the implementation/workflow, and dispatching the release job require separate owner authorization. The workflow must be available on the default branch before dispatch.

## Upgrade

1. Finish or explicitly resolve incomplete flights using their original ATC/Verdi pair, then stop the old processes at a quiescent installation.
2. Retain the exact old ATC executable, its matching Verdi executable and verified pin as the rollback pair. Preserve historical stores unchanged.
3. Replace ATC and the pinned Verdi executable together with the exact pair identified by the accepted release report. Update every external FD3 host to v2: execution, context MCP, and receipt verification all move together.
4. Start new flights with the matched v2 pair. Completed-record replay remains subject to existing fresh-Git checks.

Do not use a pin swap to restart an in-flight lease. Existing records do not attest a transport version, and schema compatibility does not prove mixed-version in-flight recovery. Baseline ATC negotiates in dry run; with a changed pin, its actual-run path can reach runway/dispatch effects before an incompatible FD3 call fails. The new binary's pre-effect negotiation does not strengthen that old binary retroactively.

The standalone owner decode/encode commands remain compatibility and conformance tools. They are never an automatic runtime fallback. The pinned read-only context resolver remains an allowed child command. Zero owner translation launches establishes the structural change; it does not establish a numerical latency improvement.

## Rollback

Stop the new pair after finishing or explicitly resolving its incomplete flights using that pair. Restore the exact old ATC executable and its matching Verdi pin together. Keep historical stores unchanged. Two-way copied completed-store replay and fresh-Git refusal are release gates; failure leaves rollback unproven and prevents shipping under this no-migration contract.

## Recovery limits

The receipt event and receipt control fact are separate writes. The control fact atomically stores the event/receipt/ack triple before acknowledgment; it does not atomically commit the separate recorder event. A receipt alone cannot authorize candidate replay.

The accepted baseline comparison records resume-related refusal for the first four deterministic crash cuts and exact completed-handoff replay for the fifth. Earlier cuts can launch additional same-epoch starts before refusal. Retain the measured classifier, exit, durable records, and launch trace for each final-pair run. These results do not establish transparent continuation or exactly-once execution.

## Scope after release

The two implementation flights simplify the published execution boundary. F12 still ends at a durable candidate handoff. Receipt/checkpoint or expansion package extraction, F13/F14, and the repeated accepted-story-to-landing journey remain subsequent work. This note adds no provider, principal, claims authority, observable-write producer, persistence transaction, or review/landing behavior.
