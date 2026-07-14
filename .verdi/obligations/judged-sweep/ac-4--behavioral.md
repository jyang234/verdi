---
id: obligation/judged-sweep--ac-4--behavioral
kind: obligation
title: "A test SHA-256s the target diagram before/after a sweep and confirms byte-identity; another confirms the disclosure line"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/judged-sweep" }
frozen: { at: 2026-07-14, commit: 1d78c2776983c7c08ae4a065727a828b2fd28825 }
---
# A test SHA-256s the target diagram before/after a sweep and confirms byte-identity; another confirms the disclosure line

The behavioral evidence must show a test that computes the SHA-256 of a
real fixture diagram file's full bytes, runs a real sweep against it
(fake judge, real file on disk), re-reads the file, and asserts the
recomputed SHA-256 is byte-identical to the original — proving the sweep
never touched the diagram it read. A second test must assert the fixed
advisory/non-exhaustive disclosure line appears verbatim in a report with
at least one finding AND in a report with zero findings — proving a clean
sweep is never rendered as "nothing wrong here."
