---
id: obligation/creation-form--ac-2--behavioral
kind: obligation
title: "The create action scaffolds a validated story on a fresh design branch via plumbing, guarded exactly like stub-instantiate"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/creation-form" }
frozen: { at: 2026-07-21, commit: cbd7eba9edb7770d0c4a10bd881b831c2feb48ba }
---
# The create action scaffolds a validated story on a fresh design branch via plumbing, guarded exactly like stub-instantiate

The behavioral evidence must show handler tests in
`internal/workbench` (beside the existing stub-instantiate coverage)
driving `POST /board/spec/{name}/api/create` end-to-end against a real
fixture repository. The happy path: submitted values plus at least one
chosen acceptance criterion produce exactly one new commit on a fresh
`design/<name>` branch whose spec strict-decodes with `class: story`,
carries an `implements` link to each chosen AC of the serving feature,
renders the submitted values verbatim in their template positions (no
`TODO` residue where a value was given), and leaves the serving
checkout's HEAD, working tree, and index untouched (asserted, not
assumed). The override leg: with a store `.verdi/templates/` override
for the story class's declared template, the created spec carries the
override's shape — the L-M12 property on the form path. The refusals,
each named: a wall that is not feature-class, a wall not at
accepted-pending-build, an already-existing `design/<name>` branch or
active spec name, a value key outside the enumerated descriptors, an AC
id the projection does not declare, and zero chosen ACs. The fail-closed
class gate: a template binding that renders a non-story class refuses
before any git object is written (`CheckClass`, stub-instantiate's
inherited posture). Green in CI's test step, as part of `make verify`.
