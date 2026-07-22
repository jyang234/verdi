---
id: obligation/init-wizard--ac-1--behavioral
kind: obligation
title: "Bare verdi init writes exactly the README's .verdi/verdi.yaml skeleton, no model.yaml, and both paths refuse identically on any existing .verdi/ directory"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/init-wizard" }
frozen: { at: 2026-07-21, commit: 2f5b1cae64f5345c42085e2794c2ad3d8f830976 }
---
# Bare verdi init writes exactly the README's .verdi/verdi.yaml skeleton, no model.yaml, and both paths refuse identically on any existing .verdi/ directory

The behavioral evidence must show built-binary tests (`cmd/verdi/init_test.go`,
mirroring `cmd/verdi/model_test.go`'s `buildVerdiBinary` + `exec.Command`
style) driving the real compiled `verdi` binary with cwd set to an empty
temporary directory: a bare `verdi init` (no `--wizard`) exits 0, creates
exactly `.verdi/verdi.yaml` containing `schema: verdi.layout/v1` and nothing
else under `.verdi/` — no `model.yaml`, no `templates/`, no `specs/` — and
the resulting root passes `verdi model check` (exit 0, resolving to the
canonical model, matching `cmd/verdi/model_test.go`'s own
`TestModelCheck_NoModelYAML_OK` witness).

A second table-driven case must show both the bare form AND `--wizard`
refusing, exit 2, against a target directory that already carries a
`.verdi/` entry — one variant with a full `.verdi/verdi.yaml` manifest
present, one variant with only a stray, otherwise-empty `.verdi/` directory
and no manifest inside it — each refusal's stderr naming exactly what
already exists at the path, and a byte-for-byte proof that the pre-existing
`.verdi/` tree is completely untouched (mtime/content unchanged) by the
refused run.

Green in CI's test step, as part of `make verify`.
