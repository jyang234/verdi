---
id: obligation/init-wizard--ac-3--behavioral
kind: obligation
title: "Wizard promotion gates on the full runModelCheck core plus a model.yaml decode-compare, promotes by exactly one rename, and both a truncated-stdin abort and a simulated mid-write crash leave nothing at the real root"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/init-wizard" }
frozen: { at: 2026-07-21, commit: 2f5b1cae64f5345c42085e2794c2ad3d8f830976 }
---
# Wizard promotion gates on the full runModelCheck core plus a model.yaml decode-compare, promotes by exactly one rename, and both a truncated-stdin abort and a simulated mid-write crash leave nothing at the real root

The behavioral evidence must show built-binary tests (`cmd/verdi/init_test.go`)
proving three things against the real compiled binary.

First, a scripted `--wizard` run whose stdin is deliberately truncated
before every prompt is answered (an EOF mid-interview) must exit 2, and a
full directory listing of the target root afterward must show no `.verdi/`
entry at all and no leftover sibling temp directory — the
mid-interview-abort pin.

Second, a scripted, otherwise-complete `--wizard` run driven with the
disclosed, test-only `VERDI_INIT_SIMULATE_CRASH_AFTER=<staged-file>`
environment override set must exit 2 after the named file is staged but
before promotion, with the same "nothing at the real root, no temp litter"
assertion — the simulated-mid-write-crash pin.

Third, a real (non-injected) successful `--wizard` run's promotion step
must be shown to be exactly one `os.Rename` — proven by asserting the
staged sibling temp directory no longer exists immediately after a
successful run (rather than being copied-then-deleted) and that the real
`.verdi/model.yaml`, when one was staged, decodes via `model.DecodeModel`
to a value `reflect.DeepEqual` to the interview's own accumulated candidate
`*model.Model` — the W-4 decode-compare pin.

Green in CI's test step, as part of `make verify`.
