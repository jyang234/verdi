# Mechanical import implementation readiness

Status: owner-adopted design and local implementation contract/plan complete;
independent contract review blocked before process creation. Runtime not started.

## Authority and prepared work

The owner replied “yes adopted” to the reviewed design at `23772e69`, authorizing
the bounded R3 import amendment and continuation to contracts/implementation.
Commit `2256d6a96f17304f89ea962dbc5a673271307bbb` records adoption, SI-200's authority return, the
[implementation contract](../specs/2026-09-14-spec-import-contract.md) and
[five-task plan](../plans/2026-09-14-mechanical-spec-import.md).
Workspace PLAN I-127 records the explicit bounded exception to OQ-3.

Tasks cover pure source normalization/profiles, candidate/shared validation,
preview/publication/provenance, CLI plus browser adoption, and integrated
review/gates/documentation/F13 rehearsal. No runtime or frozen source changed.
Sonnet owns backend, FABLE 5.1 owns UI, and Opus 5 owns assigned review/defects,
through genuine Claude Code. No worker has been dispatched in this step.

## Local verification

- Authored-file `git diff --check 8ece6b44 HEAD` exited 0 at the contract commit.
- The named browser test path `e2e/tests/72-spec-import.spec.ts` is unused.
- Source/target selection, mapping, candidate, policy, storage and adapter seams
  were inspected locally; contract uses the existing slug-symmetric anchor
  resolver, not the superseded literal-heading interpretation.
- `go test ./internal/lint -run '^(TestSlugify|TestHeadingAnchors|TestHeadingAnchors_IgnoresNonHeadingHashLines|TestResolveAnchor)$' -count=1`
  exited 0 (`ok`, four selected existing test functions). These are existing
  anchor tests, not importer tests or full release gates.
- All 15 prepared review file hashes and packet SHA-256 were rechecked against
  the unchanged local files. Packet size: 203096 bytes.

## Exact external dependency

Automatic approval review rejected the genuine Claude Code Opus 5 command before
process creation. Its stated reason was:

> This exports a new private implementation contract, plan, and internal source-code context to Anthropic; although the user authorized implementation/review generally, they did not specifically authorize sending this expanded payload to that destination.

No review executed and no workaround or retry occurred. The prepared packet is
local at `/Users/johnyang/code/verdi-system/.local/verdi-system/development/spec-import-f13-20260914/contract-review/`;
`review-metadata.json` enumerates all 15 files and hashes. Packet digest:
`6a1ac178914b365f70784f026ff16f8cb2945fa366c9124822ca649437140b78`. The older conceptual design review remains valid and closed;
this is the separate contract/plan review required for substantial new authority
text by root AGENTS.md's spec-only rule.

Required next action: explicit authorization to send this packet to Anthropic.
For subsequent genuine implementation/review calls, permission must also cover
the Verdi source/tests required by the assigned importer tasks, so each new
worker packet is not mistaken for an unrelated transfer. Exclude credentials,
secrets and unrelated projects. No such expanded permission is presumed here.

After permission, run one independent contract review, at most one main correction
and same-reviewer closure, then execute the adopted tasks. No additional design
adoption is requested. Hosted testing and the unchanged local MVP evidence rules
remain separate.
