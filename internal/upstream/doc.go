// Package upstream execs the pinned verdi-go toolchain (module
// github.com/jyang234/golang-code-graph, binaries cmd/flowmap and
// cmd/groundwork) and strict-decodes its JSON output (PLAN.md §3, I-4).
// Verdi never imports verdi-go's internal packages — every fact this
// package knows about the toolchain's shapes comes from spike S1's
// captured, canned JSON (testdata/svcfix-canned) plus the binaries'
// documented --help usage. Unknown JSON fields and unknown enum values fail
// closed (CLAUDE.md: "Strict decode everywhere").
//
// Two upstream gotchas, both forced by spike S1, apply throughout this
// package:
//
//   - Flags must precede positional arguments in every invocation (Go's
//     stdlib flag package stops parsing at the first non-flag token); the
//     reverse order is an upstream exit-2 error. Request/buildArgv enforces
//     this ordering structurally — callers cannot get it wrong.
//   - `groundwork diff` has no --json mode (S1); verdi computes
//     derived/.../boundary-diff.json itself from two strict-decoded
//     boundary contracts (I-3, revised by S1) rather than parsing upstream's
//     plain-text diff output, which is a view, not a contract.
package upstream
