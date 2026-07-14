---
id: obligation/board-editor--ac-2--static
kind: obligation
title: "The structural-op transform is a pure function whose table-driven tests pin dc-2's grammar byte-for-byte"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/board-editor" }
frozen: { at: 2026-07-14, commit: e65c4341b3724b921786cc764fbc98e854b7033c }
---
# The structural-op transform is a pure function whose table-driven tests pin dc-2's grammar byte-for-byte

The static evidence must show the op transform implemented as a pure function
(source bytes + operation in, source bytes out — no I/O, no clock, no
randomness) with table-driven unit tests that pin dc-2's grammar exactly:
add-node appends one `<id>["<label>"]` line with the lowest unused n<k> id;
connect appends one `<from> --> <to>` line; rename rewrites only the label
brackets at the node's defining occurrence and never the id; node delete
removes the defining line plus every edge line naming the node; edge delete
removes that one line. The tests must include the determinism property (the
same source and operation yield identical bytes across repeated calls) and
the negative paths: an operation against source outside the grammar's
flowchart subset (e.g. a sequenceDiagram body) returns a typed
ops-unavailable result — never a rewritten source — and no operation type
carries or accepts any position field (the request schema is strict-decoded
and a position key fails closed). Tests that assert rendered output instead
of the produced source bytes do not satisfy this obligation.
