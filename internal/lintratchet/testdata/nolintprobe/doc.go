// Package nolintprobe is TestLintStrict_NolintProbe's fixture module
// (spec/strict-lint-gate-v2 ac-3, dc-4; ledger SI-335, SI-337): each other
// file holds one gated finding under one directive shape. A file named
// suppressed_*.go holds a shape the pinned golangci-lint honours, so its
// finding is suppressed; one named reported_*.go holds a shape it does not,
// so its finding is reported. A file whose name goes on with contextcheck_
// holds contextcheck's finding, a call from a function holding a context to
// one that starts afresh, under a shape on the callee's doc comment, which
// contextcheck reads itself; every other file holds gochecknoglobals'
// finding, a package-level variable. internal/specalign's
// TestStrictLintExclusionsCounted holds its directive reading to the same
// names: it refuses or counts every directive of a suppressed file, and
// neither for a reported one. It is a module of its own under testdata/, so
// the repository's ./... never lints it.
package nolintprobe
