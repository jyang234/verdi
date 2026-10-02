// Package nolintprobe is TestLintStrict_NolintProbe's fixture module
// (spec/strict-lint-gate-v2 ac-3, dc-4): each other file holds one
// package-level variable, gochecknoglobals' finding, under one //nolint
// shape. A file named suppressed_*.go holds a shape the pinned golangci-lint
// honours for gochecknoglobals, so its finding is suppressed; one named
// reported_*.go holds a shape it does not, so its finding is reported.
// internal/specalign's TestStrictLintExclusionsCounted holds its directive
// reading to the same names: it refuses or counts every directive of a
// suppressed file, and neither for a reported one. It is a module of its own
// under testdata/, so the repository's ./... never lints it.
package nolintprobe
