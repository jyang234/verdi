package mcpserve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/specimport"
)

// import_preview / import_apply (spec/spec-documents ac-9) wrap the frozen
// import contract (docs/superpowers/specs/2026-09-14-spec-import-contract.md)
// exactly as the CLI does: preview is read-only and takes no actor;
// apply mints a delegated-agent actor from harness/session (R-W3-4,
// SI-163: never a caller-supplied actor field) and recomputes the preview
// under the digest handshake. The record's actor.harness/actor.session
// are populated by specimport itself. Nothing here touches the request
// or result schemas.
//
// Both tools decode their OWN argument object with
// artifact.DecodeExactJSON rather than this package's default
// strictUnmarshal (decode.go): these are the import contract's write
// surface, where a duplicated argument key would silently select the
// recorded actor identity, so two spellings of one arguments object are
// refused here exactly as the contract refuses two spellings of one
// request (R-W3-10).

type importPreviewArgs struct {
	Request json.RawMessage `json:"request"`
}

type importApplyArgs struct {
	Harness       string          `json:"harness"`
	Session       string          `json:"session,omitempty"`
	PreviewDigest string          `json:"preview_digest"`
	Request       json.RawMessage `json:"request"`
}

var importPreviewDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// importSentinel maps a specimport sentinel to the CLI's closed code
// vocabulary (cmd/verdi/designimport.go designImportSentinels); the code
// prefixes every isError text so a harness can match refusals by name.
type importSentinel struct {
	err  error
	code string
}

var importSentinels = []importSentinel{
	{specimport.ErrInvalidRequest, "invalid-request"},
	{specimport.ErrInvalidSource, "invalid-source"},
	{specimport.ErrUnsupportedFormat, "unsupported-format"},
	{specimport.ErrInvalidModel, "invalid-model"},
	{specimport.ErrIdentityUnavailable, "identity-unavailable"},
	{specimport.ErrAuthorityInvalid, "authority-invalid"},
	{specimport.ErrIOFailure, "io-failure"},
	{specimport.ErrUnresolved, "unresolved"},
	{specimport.ErrDirtyContext, "dirty-context"},
	{specimport.ErrStalePreview, "stale-preview"},
	{specimport.ErrTargetExists, "target-exists"},
	{specimport.ErrPolicyForbidden, "policy-forbidden"},
	{specimport.ErrActorForbidden, "actor-forbidden"},
	{specimport.ErrImportRecordMissing, "provenance-mismatch"},
	{specimport.ErrProvenanceMismatch, "provenance-mismatch"},
}

func importToolError(tool string, err error) map[string]any {
	for _, s := range importSentinels {
		if errors.Is(err, s.err) {
			return toolError(fmt.Sprintf("%s: %s: %s", tool, s.code, err.Error()))
		}
	}
	return toolError(fmt.Sprintf("%s: io-failure: %s", tool, err.Error()))
}

// decodeImportRequest hands the caller's request bytes, untouched, to
// specimport's own strict decoder — the contract's sole decoder (R-W3-10:
// re-encoding the object first would erase the duplicate-key and
// invalid-UTF-8 evidence artifact.DecodeExactJSON refuses on, letting a
// mangled reading reach the write path and be attested by the record).
// json.RawMessage already holds the exact sub-document, and DecodeRequest
// does its own envelope cap, whitespace trim and top-level-object check.
func decodeImportRequest(tool string, raw json.RawMessage) (specimport.Request, map[string]any) {
	if len(raw) == 0 {
		return specimport.Request{}, toolError(tool + ": request is required")
	}
	req, err := specimport.DecodeRequest(raw)
	if err != nil {
		return specimport.Request{}, importToolError(tool, err)
	}
	return req, nil
}

// ImportPreview implements import_preview: a read-only preview of an
// import request. A completed preview with blocking findings is a result
// (ready:false), not an error (R-W3-5).
func (b *Backend) ImportPreview(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	if len(argsRaw) > specimport.MaxEnvelopeBytes {
		return toolError("import_preview: arguments exceed the 12 MiB import envelope")
	}
	var args importPreviewArgs
	if err := artifact.DecodeExactJSON(argsRaw, &args); err != nil {
		return toolError("import_preview: malformed arguments: " + err.Error())
	}
	req, failure := decodeImportRequest("import_preview", args.Request)
	if failure != nil {
		return failure
	}
	preview, err := specimport.NewService().Preview(ctx, b.Root, req)
	if err != nil {
		return importToolError("import_preview", err)
	}
	return toolJSON(preview)
}

// ImportApply implements import_apply: recomputes the preview, refuses on
// a changed digest or any blocking finding, and publishes the design
// branch with the import record under a delegated-agent actor.
func (b *Backend) ImportApply(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	if len(argsRaw) > specimport.MaxEnvelopeBytes {
		return toolError("import_apply: arguments exceed the 12 MiB import envelope")
	}
	var args importApplyArgs
	if err := artifact.DecodeExactJSON(argsRaw, &args); err != nil {
		return toolError("import_apply: malformed arguments: " + err.Error())
	}
	if strings.TrimSpace(args.Harness) == "" {
		return toolError("import_apply: harness is required")
	}
	if !importPreviewDigestRe.MatchString(args.PreviewDigest) {
		return toolError("import_apply: preview_digest must be 64 lowercase hex characters")
	}
	actor, err := draftmutation.NewDelegatedAgent(args.Harness, args.Session)
	if err != nil {
		return toolError("import_apply: " + err.Error())
	}
	req, failure := decodeImportRequest("import_apply", args.Request)
	if failure != nil {
		return failure
	}
	b.writeMu.Lock()
	defer b.writeMu.Unlock()
	result, err := specimport.NewService().Apply(ctx, b.Root, req, args.PreviewDigest, actor)
	if err != nil {
		return importToolError("import_apply", err)
	}
	return toolJSON(result)
}
