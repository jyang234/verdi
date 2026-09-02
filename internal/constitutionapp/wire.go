package constitutionapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
)

// strictDecode decodes exactly one JSON document into v: unknown fields
// fail closed (root CLAUDE.md: "JSON via DisallowUnknownFields") and any
// byte remaining after the one top-level value is rejected as trailing
// data, the same discipline internal/canonjson.Marshal's own decode half
// already enforces for every other wire document in this repository.
func strictDecode(data []byte, v interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("constitutionapp: decoding request: %w", err)
	}
	if dec.More() {
		return errors.New("constitutionapp: trailing data after request document")
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
	return req, nil
}

// DecodeValidateRequest strict-decodes one verdi.constitution-validate-
// request/v1 document.
func DecodeValidateRequest(data []byte) (ValidateRequest, error) {
	var req ValidateRequest
	if err := strictDecode(data, &req); err != nil {
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
	return req, nil
}

// DecodeImpactReviewRequest strict-decodes one verdi.constitution-impact-
// review-request/v1 document.
func DecodeImpactReviewRequest(data []byte) (ImpactReviewRequest, error) {
	var req ImpactReviewRequest
	if err := strictDecode(data, &req); err != nil {
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
	return req, nil
}

// EncodeResult renders any of this package's five typed results (or a
// *Failure on the refusal path) as canonical JSON. Exported so both the CLI
// (cmd/verdi/context_constitution.go) and any future caller share the exact
// same encoding without reaching into internal/canonjson directly.
func EncodeResult(v interface{}) ([]byte, error) {
	return encodeResult(v)
}
