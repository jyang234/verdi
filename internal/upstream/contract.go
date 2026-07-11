package upstream

import (
	"encoding/json"
	"fmt"

	"github.com/OWNER/verdi/internal/artifact"
)

const boundaryContractSchema = "flowmap.boundary/v1"

// HTTPEntrypoint is one `entrypoints.http[]` entry in a boundary contract.
type HTTPEntrypoint struct {
	Method string `json:"method"`
	Route  string `json:"route"`
	Tier   int    `json:"tier,omitempty"`
}

// ContractEntrypoints is a boundary contract's `entrypoints` object.
// Consumers' element shape was never populated in any spike S1 capture, so
// it decodes as raw JSON rather than a guessed struct (see Graph's doc
// comment on the same tradeoff).
type ContractEntrypoints struct {
	HTTP      []HTTPEntrypoint  `json:"http,omitempty"`
	Consumers []json.RawMessage `json:"consumers,omitempty"`
}

// NamedResource is a boundary contract's shape for `published`, `consumed`,
// and `external_dependencies` entries: a name plus the kind of surface it
// is. Inferred from spike S1's captured `groundwork diff` text output
// ("+ dependency audit-svc (http)"), which is the only direct evidence
// this package has for these arrays' element shape — none of the captured
// boundary-contract JSON files ever populated them. Strict decode fails
// loudly if a real contract's shape differs, which is the intended
// posture: a schema verdi does not own must never be silently guessed past
// (00 §Constitution 5).
type NamedResource struct {
	Name string `json:"name"`
	Kind string `json:"kind,omitempty"`
}

// BoundaryContract is `.flowmap/boundary-contract.json`'s decoded shape,
// schema `flowmap.boundary/v1` (PLAN.md §3: upstream's own fixed path,
// corrected by spike S1; "flowmap boundary has no stdout mode or output
// flag — it always writes there").
type BoundaryContract struct {
	Service              string              `json:"service"`
	SchemaVersion        string              `json:"schema_version"`
	Entrypoints          ContractEntrypoints `json:"entrypoints"`
	Published            []NamedResource     `json:"published,omitempty"`
	Consumed             []NamedResource     `json:"consumed,omitempty"`
	ExternalDependencies []NamedResource     `json:"external_dependencies,omitempty"`
	BlindSpots           []json.RawMessage   `json:"blind_spots,omitempty"`
}

// DecodeBoundaryContract strict-decodes and validates a boundary contract.
func DecodeBoundaryContract(data []byte) (*BoundaryContract, error) {
	var c BoundaryContract
	if err := artifact.DecodeStrictJSON(data, &c); err != nil {
		return nil, fmt.Errorf("upstream: decoding boundary contract: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Validate checks the schema literal and that Service is non-empty.
func (c BoundaryContract) Validate() error {
	if c.SchemaVersion != boundaryContractSchema {
		return fmt.Errorf("upstream: boundary contract schema_version %q, want %q", c.SchemaVersion, boundaryContractSchema)
	}
	if c.Service == "" {
		return fmt.Errorf("upstream: boundary contract has an empty service")
	}
	return nil
}
