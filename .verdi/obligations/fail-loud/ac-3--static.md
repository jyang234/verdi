---
id: obligation/fail-loud--ac-3--static
kind: obligation
title: "mcpserve decodes what it owns strictly and can leave a trace"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/fail-loud" }
frozen: { at: 2026-07-13, commit: 7f3c08d367dd4f76b45e982dc03813875e0e7a7c }
---
# mcpserve decodes what it owns strictly and can leave a trace

The static evidence must show the fail-closed posture in source: one
strictUnmarshal helper (delegating to artifact.DecodeStrictJSON) decoding
every tool's arguments and LockInfo; additionalProperties: false emitted by
tooldefs' obj() on every tool schema; protocol envelopes left tolerant
(dc-2's split); Server.ErrLog wired to os.Stderr by verdi serve while the
stdio path keeps its own error inspection.
