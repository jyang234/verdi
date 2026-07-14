---
id: obligation/verification-extractor--ac-2--behavioral
kind: obligation
title: "Scoped and unscoped truth regeneration are proven over canned captures and a fake exit failure"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/verification-extractor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# Scoped and unscoped truth regeneration are proven over canned captures and a fake exit failure

The behavioral evidence must show a test using `internal/upstream`'s
existing fake `Runner` seam (never a real `flowmap` binary) that: (1)
regenerates truth unscoped and decodes `testdata/svcfix-canned/graph.json`
(or an equivalent canned fixture) correctly; (2) regenerates truth with a
non-empty scope and asserts the fake runner observed an `-entry <scope>`
flag in the request it received; (3) asserts a non-zero fake exit code
surfaces as an operational error rather than a silent empty graph.
