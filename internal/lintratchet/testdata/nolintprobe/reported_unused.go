package nolintprobe

// ungated stays reported under a directive naming only an ungated linter:
// the witness neither counts nor refuses it.
var ungated int //nolint:unused
