package specimport

import (
	"bytes"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// DecodeRequest is the sole request decoder
// (spec-import-contract.md, "Shared internal interfaces": "DecodeRequest
// is the sole request decoder"). It enforces the raw envelope size cap,
// explicitly rejects a top-level JSON null or any other non-object
// top-level value (artifact.DecodeExactJSON alone would silently leave a
// top-level `null` decoded as a Request zero value rather than erroring —
// spec-import-contract.md: "fail closed explicitly, not via zero-value
// fallthrough"), decodes through the shared strict/exact JSON seam
// (duplicate keys, unknown fields, trailing data, invalid UTF-8), and
// finally calls Request.Validate so every entry path — decoded here or
// built directly as a Go struct literal — passes through the same gate.
//
// This uses artifact.DecodeExactJSON rather than artifact.DecodeClosedJSON:
// the contract calls out only the top-level-null/non-object case as
// needing an explicit check, and DecodeClosedJSON's stricter "no explicit
// null at any depth" would also reject a well-formed `"mappings": null`
// the contract never singles out as invalid. See the lane report's
// semantic-decisions section.
func DecodeRequest(data []byte) (Request, error) {
	if len(data) > MaxEnvelopeBytes {
		return Request{}, fmt.Errorf("%w: request envelope is %d bytes, over the %d byte limit", ErrInvalidRequest, len(data), MaxEnvelopeBytes)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return Request{}, fmt.Errorf("%w: request body must be a top-level JSON object, not null, an array, or a bare scalar", ErrInvalidRequest)
	}

	var req Request
	if err := artifact.DecodeExactJSON(data, &req); err != nil {
		return Request{}, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}
	if err := req.Validate(); err != nil {
		return Request{}, err
	}
	return req, nil
}
