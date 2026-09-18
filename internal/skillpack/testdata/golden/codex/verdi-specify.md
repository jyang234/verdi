---
name: verdi-specify
description: Turn a brief, a Markdown draft, or an existing spec file into a proposed verdi spec on its own design branch through the import contract — preview first, apply only after the human confirms the preview digest.
---
<!-- verdi:generated-skill host=codex skill=specify -->
<!-- verdi:engine-digest sha256:d805fb1ae60f08b6a2db8f122cd01f2c332314942b619debe5f6538bcb2e4856 -->
<!-- verdi:template-digest sha256:2a2f64296196cdac873198d00b8ef0c850977ddf5d7ea05bf6d08fd2ced41ee7 -->
<!-- verdi:render-commit 0123456789abcdef0123456789abcdef01234567 -->
<!-- verdi: this is a generated skill; edits here never change verdi, and any difference is reported as drift by `verdi harness check` until this file is regenerated with `verdi harness render`. -->

# verdi-specify

Use this skill when the user has a brief or a document and wants a verdi spec drafted from it. It writes only through the import contract (`import_preview` then `import_apply` over MCP, or `verdi design import preview` then `verdi design import apply` on the CLI). It never edits `.verdi/` directly.

## What the import contract guarantees

- Preview is read-only and deterministic: the same request over the same commit yields the same preview digest.
- Apply recomputes the preview and refuses when the digest changed (`stale-preview`), when a blocking finding remains (`unresolved`), or when the target already exists (`target-exists`).
- Every applied import records byte-offset provenance for each mapped span, coverage accounting for every source byte, the origin of each field (`copied-source`, `user-edited-source`, `user-added`, `native`), and the harness and session that proposed the mapping.

## Steps

1. Collect the source. For a file in the repository, run `verdi design import source --root <dir> --file <relative path>` (add `--start-line`/`--end-line` for a range) and keep the printed Source JSON. For text the user pasted, build a Source with `id` (lowercase slug), `label`, and base64 `data` yourself.
2. Compose the request: `schema: verdi.spec-import-request/v1`, `target` (`slug`, `class` feature or story, `title`, optional `story` parent), `format` (`markdown-v1` for ordinary headed Markdown, `native` for an existing verdi spec file, `manual-v1` when you supply every mapping), `primary` (the source id), `sources`, optional `mappings`, `retain_unmapped` (true only when the user disposes of the unmapped remainder as retained-only), `defer_statements` (true only when the user explicitly defers problem and outcome).
3. Preview. Call `import_preview` with the request (or `verdi design import preview --request -` with the JSON on stdin).
4. Show the human the preview: the `digest`, each field with its origin and evidence, every finding with its code and whether it blocks, and the coverage totals per source. If `ready` is false, fix the request (a mapping, a transform, a retained-only disposition) and preview again; never apply a not-ready preview.
5. Ask the human to confirm that exact digest. Do not apply on any other signal.
6. Apply. Call `import_apply` with the same request, `preview_digest` set to the confirmed digest, `harness` set to your harness id (`claude-code` or `codex`), and `session` set to your session id when you have one (or `verdi design import apply --request - --preview <digest> --harness <id>`).
7. Report the result: `status` (`created` or `already-created`), `branch`, `commit`, `spec_ref`, `board_path`, and any disclosure. Point the human at the board path and at `verdi design import record --branch <branch> --spec <slug>` for the provenance record.

## Refusals you must relay verbatim

`invalid-request`, `invalid-source`, `unsupported-format`, `invalid-model`, `identity-unavailable`, `authority-invalid`, `io-failure`, `unresolved`, `dirty-context`, `stale-preview`, `target-exists`, `policy-forbidden`, `actor-forbidden`, `provenance-mismatch`. That is the whole vocabulary: every refusal names one of these codes, and `io-failure` is also the code an error the contract maps to no named refusal arrives under. Each names its cause; do not retry blindly. A dirty checkout must be committed or cleaned by the human first.

```verdi-sequence
call import_preview
show
confirm
call import_apply
```
