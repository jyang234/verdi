package experiment

import "fmt"

const (
	CapabilitiesSchema   = "verdi.experiment-evaluator-capabilities/v1"
	CapabilitiesSchemaV2 = "verdi.experiment-evaluator-capabilities/v2"
)

// registeredProtocolVersions is the closed set of protocol version
// strings a capabilities response may declare support for (AC-3, DC-10).
// V1 predecessor artifacts and the amended evaluator/observation protocols
// are the complete accepted set; unknown revisions fail closed.
var registeredProtocolVersions = map[string]bool{
	CapabilitiesSchema:      true,
	CapabilitiesSchemaV2:    true,
	EvaluatorProtocolSchema: true,
	ObservationSchema:       true,
	ObservationSchemaV2:     true,
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
	EvaluatorVersion string             `json:"evaluator_version,omitempty"`
	ProtocolVersions []string           `json:"protocol_versions"`
	Metrics          []CapabilityMetric `json:"metrics"`
	Guards           []string           `json:"guards,omitempty"`
	Observers        []string           `json:"observers,omitempty"`
	WorkloadInputs   []string           `json:"workload_inputs,omitempty"`
	Environment      []string           `json:"environment,omitempty"`
	RequiresNetwork  bool               `json:"requires_network"`
	RequiresElevated bool               `json:"requires_elevated"`
}

// capabilitiesDoc is the strict decode target: metrics is a pointer so an
// omitted (or explicit null) field is distinguishable from an explicitly
// present empty list, matching AC-3's "required response content; may be
// empty" grammar — the same presence trick observationDoc uses for
// disclosures. An evaluator that never mentions metrics has not answered
// the question; one that answers [] has.
type capabilitiesDoc struct {
	Schema           string              `json:"schema"`
	EvaluatorVersion string              `json:"evaluator_version,omitempty"`
	ProtocolVersions []string            `json:"protocol_versions"`
	Metrics          *[]CapabilityMetric `json:"metrics"`
	Guards           []string            `json:"guards,omitempty"`
	Observers        []string            `json:"observers,omitempty"`
	WorkloadInputs   []string            `json:"workload_inputs,omitempty"`
	Environment      []string            `json:"environment,omitempty"`
	RequiresNetwork  bool                `json:"requires_network"`
	RequiresElevated bool                `json:"requires_elevated"`
}

// DecodeCapabilities strict-decodes raw as a capabilities response and
// fully validates it (decodeStrictJSON: the shared strict seam plus this
// package's duplicate-key guard).
func DecodeCapabilities(raw []byte) (Capabilities, error) {
	var doc capabilitiesDoc
	if err := decodeStrictJSON(raw, &doc); err != nil {
		return Capabilities{}, fmt.Errorf("experiment: decoding capabilities: %w", err)
	}
	if doc.Metrics == nil {
		return Capabilities{}, fmt.Errorf("experiment: capabilities.metrics is missing (an explicitly empty list is [])")
	}
	c := Capabilities{
		Schema:           doc.Schema,
		EvaluatorVersion: doc.EvaluatorVersion,
		ProtocolVersions: doc.ProtocolVersions,
		Metrics:          *doc.Metrics,
		Guards:           doc.Guards,
		Observers:        doc.Observers,
		WorkloadInputs:   doc.WorkloadInputs,
		Environment:      doc.Environment,
		RequiresNetwork:  doc.RequiresNetwork,
		RequiresElevated: doc.RequiresElevated,
	}
	if err := c.Validate(); err != nil {
		return Capabilities{}, err
	}
	if c.Schema == CapabilitiesSchemaV2 {
		if err := requireCanonicalJSON(raw, c); err != nil {
			return Capabilities{}, fmt.Errorf("experiment: capabilities v2: %w", err)
		}
	}
	return c, nil
}

// Validate checks the schema, the closed protocol-version set, unique
// metric ids, and the optional unique-id sets for guards, observers, and
// workload inputs.
func (c Capabilities) Validate() error {
	if c.Schema != CapabilitiesSchema && c.Schema != CapabilitiesSchemaV2 {
		return fmt.Errorf("experiment: unknown capabilities schema %q", c.Schema)
	}
	if c.Schema == CapabilitiesSchemaV2 && c.EvaluatorVersion == "" {
		return fmt.Errorf("experiment: capabilities.evaluator_version must be nonempty for v2")
	}
	if c.Schema == CapabilitiesSchema && c.EvaluatorVersion != "" {
		return fmt.Errorf("experiment: capabilities.evaluator_version is not part of v1")
	}
	if len(c.ProtocolVersions) == 0 {
		return fmt.Errorf("experiment: capabilities.protocol_versions must be nonempty")
	}
	seenProtocols := make(map[string]bool, len(c.ProtocolVersions))
	for _, v := range c.ProtocolVersions {
		if !registeredProtocolVersions[v] {
			return fmt.Errorf("experiment: capabilities.protocol_versions: unknown protocol version %q", v)
		}
		if seenProtocols[v] {
			return fmt.Errorf("experiment: capabilities.protocol_versions: duplicate protocol version %q", v)
		}
		seenProtocols[v] = true
	}
	if c.Schema == CapabilitiesSchemaV2 {
		for _, required := range []string{EvaluatorProtocolSchema, ObservationSchemaV2} {
			if !seenProtocols[required] {
				return fmt.Errorf("experiment: capabilities.protocol_versions: v2 requires %q", required)
			}
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

// ValidateDefinitionCapabilities proves a locked definition's decision
// vocabulary is a member of the digest-pinned evaluator capabilities.
func ValidateDefinitionCapabilities(def Definition, c Capabilities) error {
	if err := def.Validate(); err != nil {
		return err
	}
	if c.Schema != CapabilitiesSchemaV2 {
		return fmt.Errorf("experiment: a V2 run requires capabilities schema %q", CapabilitiesSchemaV2)
	}
	if err := c.Validate(); err != nil {
		return err
	}
	guards := make(map[string]bool, len(c.Guards))
	for _, id := range c.Guards {
		guards[id] = true
	}
	metrics := make(map[string]CapabilityMetric, len(c.Metrics))
	for _, metric := range c.Metrics {
		metrics[metric.ID] = metric
	}
	for _, g := range def.Decision.Guards {
		if g.Bounded() {
			if !isHarnessMetric(g.ID) {
				if _, ok := metrics[g.ID]; !ok {
					return fmt.Errorf("experiment: bounded metric %q is absent from capabilities", g.ID)
				}
			}
			continue
		}
		if !guards[g.ID] {
			return fmt.Errorf("experiment: required guard %q is absent from capabilities", g.ID)
		}
	}
	pm := def.Decision.PrimaryMetric
	if builtin, ok := harnessMetric(pm.ID); ok {
		if pm.Type != builtin.Type || pm.Unit != builtin.Unit || pm.Direction != builtin.Direction {
			return fmt.Errorf("experiment: primary metric %q does not match its fixed harness definition", pm.ID)
		}
		return nil
	}
	metric, ok := metrics[pm.ID]
	if !ok {
		return fmt.Errorf("experiment: primary metric %q is absent from capabilities", pm.ID)
	}
	if metric.Type != pm.Type || metric.Unit != pm.Unit || metric.Direction != pm.Direction {
		return fmt.Errorf("experiment: primary metric %q does not match capabilities", pm.ID)
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
