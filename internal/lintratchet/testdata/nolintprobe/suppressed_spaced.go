package nolintprobe

// spaced is suppressed by a directive after "// ", which golangci-lint reads
// as a directive with no list: the witness refuses it.
var spaced int // nolint
