---
id: obligation/fail-loud--ac-2--static
kind: obligation
title: "Failure paths are honest in the code itself"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/fail-loud" }
frozen: { at: 2026-07-13, commit: 7f3c08d367dd4f76b45e982dc03813875e0e7a7c }
---
# Failure paths are honest in the code itself

The static evidence must show all three honesty repairs in source:
cascadecheck's loadActiveSpecTolerant tolerates ONLY fs.ErrNotExist (any
other read error wraps and propagates — the //nolint:nilerr blanket is
gone from the read branch); the four ErrBoardNotFound comparisons use
errors.Is, not ==; runtimeprobe's header states the transcription semantic
(emission success is exit 0 regardless of the stamped verdict, contrasted
with sync's evaluateBundle path).
