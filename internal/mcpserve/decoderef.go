package mcpserve

import (
	"encoding/json"

	"github.com/jyang234/verdi/internal/artifact"
)

// decodeRefArg runs the shared lone-ref-argument prologue every
// single-`ref`-field tool (get_artifact, get_board, get_links,
// list_annotations) starts with: strict-decode {"ref": "..."} via
// strictUnmarshal, reject an empty ref, then artifact.ParseRef it.
// toolName prefixes every error this returns — toolError's own
// tool-name-prefix convention (decode_test.go's
// TestToolArgsStrictDecode_UnknownField pins the "<tool>: " prefix) — so
// each call site's error text stays exactly what it was before extraction.
//
// On success ok is true and errResult is nil. On failure ok is false and
// errResult is the map[string]any a tool method should return immediately
// (ref is the zero artifact.Ref and must not be used). A tool with
// additional checks beyond the shared three — get_board's ref.Kind /
// ref.Object / ref.Pinned checks — runs them at its own call site, after
// decodeRefArg succeeds.
func decodeRefArg(toolName string, argsRaw json.RawMessage) (ref artifact.Ref, errResult map[string]any, ok bool) {
	var args struct {
		Ref string `json:"ref"`
	}
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return artifact.Ref{}, toolError(toolName + ": malformed arguments: " + err.Error()), false
	}
	if args.Ref == "" {
		return artifact.Ref{}, toolError(toolName + ": ref is required"), false
	}

	parsed, err := artifact.ParseRef(args.Ref)
	if err != nil {
		return artifact.Ref{}, toolError(toolName + ": " + err.Error()), false
	}
	return parsed, nil, true
}
