// Package specsplice is the board's ONLY write path into a spec document
// (05 §Workbench "Authoring" — board editing IS spec editing): surgical
// byte-range splicing against the pristine whole-file buffer, per spike
// S7's binding findings (docs/spikes/v1/spike-s7-findings.md):
//
//   - never decode→struct→yaml.Marshal→reassemble (proven to churn ~60
//     lines of quote/flow/indent style for a one-field edit);
//   - never write through a SplitFrontmatter-style split/join reassembly
//     (proven to drop the newline before the closing delimiter);
//   - locate nodes by yaml.Node.Line/.Column converted to whole-file
//     coordinates, find their ends with quote-aware scans, batch edits
//     computed against one parse, and apply them tail-to-head;
//   - validate-before-write: strict re-decode of every spliced result
//     before it may touch the working tree (Validate).
//
// The technique is proven for flow-style scalars and flow containers —
// every 02 §Object model example — plus the two line-grained insertions a
// board edit needs (a new list element on its own line; a new top-level
// block before the closing delimiter). Block scalars (`|`, `>`) are out of
// scope and fail closed.
package specsplice
