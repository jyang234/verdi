package constitutionapp

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// strictDecode decodes exactly one JSON request document into v under the
// CLOSED grammar every constitutionapp request envelope declares: unknown
// fields, DUPLICATE KEYS, explicit NULLS, invalid UTF-8, and any byte
// remaining after the one top-level value all fail closed.
//
// It consumes internal/artifact's shared seam rather than re-deriving the
// rules here (root CLAUDE.md: "schemas/decoding in internal/artifact";
// "never copy-paste across packages"). The previous local decoder enforced
// only unknown fields and trailing data, so `{"targets":[],"targets":[...]}`
// and `{"schema":null}` were both accepted — two spellings of one document
// meaning whatever the decoder happened to keep, under an envelope whose own
// doc comments claim an exact `.../request/v1` contract.
func strictDecode(data []byte, v interface{}) error {
	if err := artifact.DecodeClosedJSON(data, v); err != nil {
		return fmt.Errorf("constitutionapp: decoding request: %w", err)
	}
	return nil
}

// encodeResult renders v as canonical JSON (sorted keys, no HTML escaping,
// trailing newline) — the one encoding every CLI and MCP adapter uses, so
// two conformant adapters given identical inputs return byte-identical
// results (Wave 6 Task 3: "byte-equivalent CLI/workbench-capable records").
func encodeResult(v interface{}) ([]byte, error) {
	return canonjson.Marshal(v)
}

// DecodeInspectRequest strict-decodes one verdi.constitution-inspect-
// request/v1 document.
func DecodeInspectRequest(data []byte) (InspectRequest, error) {
	var req InspectRequest
	if err := strictDecode(data, &req); err != nil {
		return InspectRequest{}, err
	}
	if err := requireRequestSchema(req.Schema, InspectRequestSchema); err != nil {
		return InspectRequest{}, err
	}
	return req, nil
}

// DecodeValidateRequest strict-decodes one verdi.constitution-validate-
// request/v1 document.
func DecodeValidateRequest(data []byte) (ValidateRequest, error) {
	var req ValidateRequest
	if err := strictDecode(data, &req); err != nil {
		return ValidateRequest{}, err
	}
	if err := requireRequestSchema(req.Schema, ValidateRequestSchema); err != nil {
		return ValidateRequest{}, err
	}
	return req, nil
}

// DecodeProposeRequest strict-decodes one verdi.constitution-propose-
// request/v1 document.
func DecodeProposeRequest(data []byte) (ProposeRequest, error) {
	var req ProposeRequest
	if err := strictDecode(data, &req); err != nil {
		return ProposeRequest{}, err
	}
	if err := requireRequestSchema(req.Schema, ProposeRequestSchema); err != nil {
		return ProposeRequest{}, err
	}
	return req, nil
}

// DecodeImpactReviewRequest strict-decodes one verdi.constitution-impact-
// review-request/v1 document.
func DecodeImpactReviewRequest(data []byte) (ImpactReviewRequest, error) {
	var req ImpactReviewRequest
	if err := strictDecode(data, &req); err != nil {
		return ImpactReviewRequest{}, err
	}
	if err := requireRequestSchema(req.Schema, ImpactReviewRequestSchema); err != nil {
		return ImpactReviewRequest{}, err
	}
	return req, nil
}

// DecodeSubmitPreparationRequest strict-decodes one verdi.constitution-
// submit-preparation-request/v1 document.
func DecodeSubmitPreparationRequest(data []byte) (SubmitPreparationRequest, error) {
	var req SubmitPreparationRequest
	if err := strictDecode(data, &req); err != nil {
		return SubmitPreparationRequest{}, err
	}
	if err := requireRequestSchema(req.Schema, SubmitPreparationRequestSchema); err != nil {
		return SubmitPreparationRequest{}, err
	}
	return req, nil
}

// EncodeResult renders any of this package's five typed results (or a
// *Failure on the refusal path) as canonical JSON. Exported so both the CLI
// (cmd/verdi/context_constitution.go) and any future caller share the exact
// same encoding without reaching into internal/canonjson directly.
func EncodeResult(v interface{}) ([]byte, error) {
	return encodeResult(v)
}
