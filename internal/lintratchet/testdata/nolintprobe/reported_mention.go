package nolintprobe

// mentioned stays reported under a comment that mentions nolint mid-comment,
// which is no directive.
var mentioned int // a mention of nolint mid-comment is no directive
