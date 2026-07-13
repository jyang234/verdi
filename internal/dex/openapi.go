package dex

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// transcodeOpenAPI reads path (a discovered
// <service-root>/api/openapi.{yaml,yml,json} file, 05 §Verdi-dex mechanics)
// and returns its canonical JSON transcoding: openapi-renderer.js (the
// third of dex's three budgeted JS files) fetches this, never the original
// YAML, so the client never needs a YAML parser. .json sources are decoded
// and re-canonicalized the same way as .yaml/.yml sources, so the emitted
// bytes are a deterministic function of the document's content regardless
// of the committed file's own formatting or extension.
func transcodeOpenAPI(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("dex: reading OpenAPI doc %s: %w", path, err)
	}

	var generic interface{}
	if strings.HasSuffix(path, ".json") {
		if err := json.Unmarshal(data, &generic); err != nil {
			return nil, fmt.Errorf("dex: decoding OpenAPI JSON %s: %w", path, err)
		}
	} else {
		generic, err = artifact.DecodeYAMLLoose(data)
		if err != nil {
			return nil, fmt.Errorf("dex: decoding OpenAPI YAML %s: %w", path, err)
		}
	}

	out, err := canonjson.Marshal(generic)
	if err != nil {
		return nil, fmt.Errorf("dex: canonicalizing OpenAPI doc %s: %w", path, err)
	}
	return out, nil
}
