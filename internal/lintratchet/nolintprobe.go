package lintratchet

// NolintProbeDir is the //nolint probe module, relative to the repository
// root (spec/strict-lint-gate-v2 ac-3, dc-4; ledger SI-335, SI-337): this
// package's TestLintStrict_NolintProbe runs the pinned golangci-lint over it,
// and internal/specalign's TestStrictLintExclusionsCounted holds its own
// directive reading to the same files, so both read the path from here. Each
// file but doc.go holds one gated finding under one directive shape, and is
// named suppressed_*.go when the pinned golangci-lint suppresses that
// finding, reported_*.go when it reports it; a name going on with
// contextcheck_ holds contextcheck's finding, any other gochecknoglobals'.
// (A comment line of this package must not begin with that directive's
// name, which golangci-lint would read as a bare directive.)
const NolintProbeDir = "internal/lintratchet/testdata/nolintprobe"
