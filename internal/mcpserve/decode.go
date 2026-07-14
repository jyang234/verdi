// Strict-decode posture for what mcpserve itself owns, ratified at
// spec/fail-loud dc-2: tool arguments (05's tool table publishes each
// schema) and the writer-lock file (lock.go writes it) decode STRICT —
// DisallowUnknownFields plus trailing-data rejection, so a typo'd field
// (target_reff) is refused NAMING the unknown field rather than silently
// dropped (ac-3). JSON-RPC protocol envelopes (wire.go's rpcRequest,
// server.go's tools/call name/arguments) stay TOLERANT — unknown members
// there are expected forward-compat, not a mistake to catch — and are left
// on bare json.Unmarshal, untouched by this file.
package mcpserve

import (
	"encoding/json"

	"github.com/jyang234/verdi/internal/artifact"
)

// strictUnmarshal decodes raw into dst with DisallowUnknownFields and
// trailing-data rejection. It delegates to internal/artifact's own
// DecodeStrictJSON (mcpserve already imports internal/artifact in nearly
// every tool_*.go file) rather than reimplementing the same
// json.NewDecoder posture a second time — CLAUDE.md: "Anything used by two
// or more packages lives in a shared internal/ package... never
// copy-paste". This wrapper's only job is to give every call site in this
// package one name to call and one place to point the ac-3/dc-2 doc
// comment at.
func strictUnmarshal(raw json.RawMessage, dst any) error {
	return artifact.DecodeStrictJSON(raw, dst)
}
