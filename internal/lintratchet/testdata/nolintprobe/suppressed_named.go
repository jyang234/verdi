package nolintprobe

// named is suppressed by a directive naming the gated linter, with a reason:
// the witness counts it.
var named int //nolint:gochecknoglobals // the probe's counted shape
