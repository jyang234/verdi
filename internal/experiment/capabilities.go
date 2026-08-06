package experiment

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// CapabilitiesSchema is the only accepted evaluator-capabilities schema
// identifier.
const CapabilitiesSchema = "verdi.experiment-evaluator-capabilities/v1"

// registeredProtocolVersions is the closed set of protocol version
// strings a capabilities response may declare support for (AC-3, DC-10).
// "verdi.experiment-evaluator/v1" is deliberately absent: OQ-1 leaves that
// protocol's scope unresolved, so it is not yet a name this package
// accepts anywhere.
var registeredProtocolVersions = map[string]bool{
	CapabilitiesSchema: true,
	ObservationSchema:  true,
}

// CapabilityMetric is one metric a capabilities response declares
// support for.
type CapabilityMetric struct {
	ID        string     `json:"id"`
	Type      MetricType `json:"type"`
	Unit      string     `json:"unit"`
	Direction Direction  `json:"direction"`
}

// Validate checks m's id, type, unit, and direction.
func (m CapabilityMetric) Validate() error {
	if err := ValidateID(m.ID); err != nil {
		return fmt.Errorf("experiment: capabilities.metrics: %w", err)
	}
	if err := m.Type.Validate(); err != nil {
		return fmt.Errorf("experiment: metric %q: %w", m.ID, err)
	}
	if err := ValidateUnit(m.Unit); err != nil {
		return fmt.Errorf("experiment: metric %q: %w", m.ID, err)
	}
	if err := m.Direction.Validate(); err != nil {
		return fmt.Errorf("experiment: metric %q: %w", m.ID, err)
	}
	return nil
}

// Capabilities is one verdi.experiment-evaluator-capabilities/v1 describe
// response (AC-3): the evaluator's declared protocol versions, metrics,
// guards, observers, workload inputs, environment dependencies, and
// access requirements.
type Capabilities struct {
	Schema           string             `json:"schema"`
	ProtocolVersions []string           `json:"protocol_versions"`
	Metrics          []CapabilityMetric `json:"metrics"`
	Guards           []string           `json:"guards,omitempty"`
	Observers        []string           `json:"observers,omitempty"`
	WorkloadInputs   []string           `json:"workload_inputs,omitempty"`
	Environment      []string           `json:"environment,omitempty"`
	RequiresNetwork  bool               `json:"requires_network"`
	RequiresElevated bool               `json:"requires_elevated"`
}

// DecodeCapabilities strict-decodes raw as a capabilities response and
// fully validates it.
func DecodeCapabilities(raw []byte) (Capabilities, error) {
	var c Capabilities
	if err := artifact.DecodeStrictJSON(raw, &c); err != nil {
		return Capabilities{}, fmt.Errorf("experiment: decoding capabilities: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Capabilities{}, err
	}
	return c, nil
}

// Validate checks the schema, the closed protocol-version set, unique
// metric ids, and the optional unique-id sets for guards, observers, and
// workload inputs.
func (c Capabilities) Validate() error {
	if c.Schema != CapabilitiesSchema {
		return fmt.Errorf("experiment: unknown capabilities schema %q, want %q", c.Schema, CapabilitiesSchema)
	}
	if len(c.ProtocolVersions) == 0 {
		return fmt.Errorf("experiment: capabilities.protocol_versions must be nonempty")
	}
	for _, v := range c.ProtocolVersions {
		if !registeredProtocolVersions[v] {
			return fmt.Errorf("experiment: capabilities.protocol_versions: unknown protocol version %q", v)
		}
	}

	metricIDs := make(map[string]bool, len(c.Metrics))
	for _, m := range c.Metrics {
		if err := m.Validate(); err != nil {
			return err
		}
		if metricIDs[m.ID] {
			return fmt.Errorf("experiment: capabilities.metrics: duplicate id %q", m.ID)
		}
		metricIDs[m.ID] = true
	}

	if err := uniqueIDs("capabilities.guards", c.Guards); err != nil {
		return err
	}
	if err := uniqueIDs("capabilities.observers", c.Observers); err != nil {
		return err
	}
	if err := uniqueIDs("capabilities.workload_inputs", c.WorkloadInputs); err != nil {
		return err
	}
	for i, e := range c.Environment {
		if e == "" {
			return fmt.Errorf("experiment: capabilities.environment[%d] must be nonempty", i)
		}
	}
	return nil
}

// uniqueIDs checks that every entry in ids is a well-formed id and that no
// entry repeats, naming field in any error.
func uniqueIDs(field string, ids []string) error {
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if err := ValidateID(id); err != nil {
			return fmt.Errorf("experiment: %s: %w", field, err)
		}
		if seen[id] {
			return fmt.Errorf("experiment: %s: duplicate id %q", field, id)
		}
		seen[id] = true
	}
	return nil
}
