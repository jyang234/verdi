package publicrelease

import (
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// decodeDocument guards these auxiliary whole-document readers before the
// shared strict schema decoder. A streaming decoder's More check alone cannot
// reject an unmatched closing delimiter after the first JSON value.
func decodeDocument(data []byte, target any) error {
	if !json.Valid(data) {
		return fmt.Errorf("invalid complete release JSON document")
	}
	return artifact.DecodeStrictJSON(data, target)
}
