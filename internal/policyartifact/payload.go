package policyartifact

import (
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/jyang234/verdi/internal/artifact"
)

// Payload is one typed feature-specific policy payload (DC-23/OD-5:
// feature governance configuration is expressed as typed payloads inside
// this single policy-authority system). A payload owns its field schema;
// this package owns storage, decoding dispatch, resolution, identity,
// and digest. There is deliberately NO untyped map fallback: an
// unregistered payload kind fails decode closed, and registration
// supplies only a strict decoder — never a second resolver, hierarchy,
// or effective-policy function.
type Payload interface {
	// PayloadKind returns the registered payload kind key.
	PayloadKind() string
	// Validate checks the payload's own field semantics.
	Validate() error
}

// payloadDecoder strictly decodes one payload kind's YAML bytes.
type payloadDecoder func([]byte) (Payload, error)

var (
	payloadMu       sync.RWMutex
	payloadRegistry = map[string]payloadDecoder{}
)

// RegisterPayloadKind registers a typed decoder for a feature-specific
// payload kind, at init time only (OD-5: features own their payload
// fields, this system owns everything else). The design_assistance
// payload below is registered by this package itself because the
// accepted ASD specification already fixes its complete v1 field set —
// a transcription, not an ownership claim; a feature whose fields live
// in its own package (e.g. the CSE lane's experiment policy) registers
// from that package's init instead. Because the registry is
// process-global, WHICH kinds decode depends on which packages are
// linked into the binary; digests of successfully decoded artifacts are
// unaffected. Registering an empty kind, a nil decoder, or a kind twice
// is a programming error and panics.
func RegisterPayloadKind(kind string, decode func([]byte) (Payload, error)) {
	if kind == "" || decode == nil {
		panic("policyartifact: RegisterPayloadKind requires a kind and a decoder")
	}
	payloadMu.Lock()
	defer payloadMu.Unlock()
	if _, dup := payloadRegistry[kind]; dup {
		panic(fmt.Sprintf("policyartifact: payload kind %q registered twice", kind))
	}
	payloadRegistry[kind] = decode
}

// decodePayloads dispatches each payload node to its registered typed
// decoder. Unknown kinds fail closed.
func decodePayloads(nodes map[string]yaml.Node) (map[string]Payload, error) {
	out := make(map[string]Payload, len(nodes))
	for kind, node := range nodes {
		payloadMu.RLock()
		dec, ok := payloadRegistry[kind]
		payloadMu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("policyartifact: unknown payload kind %q (typed feature payloads must be registered; there is no untyped fallback)", kind)
		}
		n := node
		raw, err := yaml.Marshal(&n)
		if err != nil {
			return nil, fmt.Errorf("policyartifact: payload %s: re-encoding node: %w", kind, err)
		}
		p, err := dec(raw)
		if err != nil {
			return nil, fmt.Errorf("policyartifact: payload %s: %w", kind, err)
		}
		if p.PayloadKind() != kind {
			return nil, fmt.Errorf("policyartifact: payload %s: decoder returned kind %q", kind, p.PayloadKind())
		}
		if err := p.Validate(); err != nil {
			return nil, fmt.Errorf("policyartifact: payload %s: %w", kind, err)
		}
		out[kind] = p
	}
	return out, nil
}

// DesignAssistancePayloadKind is the ASD design-assistance payload's
// registered kind key (spec/ai-assisted-spec-design: "The
// design_assistance block is a typed ASD payload inside Context
// Integrity's single policy-authority system").
const DesignAssistancePayloadKind = "design_assistance"

// DesignAssistancePayload transcribes the ASD-owned field set the
// accepted ASD specification fixes for v1: "project policy selects
// exactly one of off, proposal-only, draft-write; layout is reserved for
// a future presentation extension and remains false in v1". ASD owns
// these fields; this registration only gives them their typed home.
type DesignAssistancePayload struct {
	Mode   string `yaml:"mode" json:"mode"`
	Layout bool   `yaml:"layout" json:"layout"`
}

// PayloadKind implements Payload.
func (p *DesignAssistancePayload) PayloadKind() string { return DesignAssistancePayloadKind }

// Validate implements Payload.
func (p *DesignAssistancePayload) Validate() error {
	switch p.Mode {
	case "off", "proposal-only", "draft-write":
	default:
		return fmt.Errorf("mode %q must be exactly one of off, proposal-only, draft-write", p.Mode)
	}
	if p.Layout {
		return fmt.Errorf("layout is reserved and must remain false in v1")
	}
	return nil
}

// decodeDesignAssistance strictly decodes the design_assistance payload.
func decodeDesignAssistance(raw []byte) (Payload, error) {
	var doc struct {
		Mode   *string `yaml:"mode"`
		Layout *bool   `yaml:"layout"`
	}
	if err := artifact.DecodeStrict(raw, &doc); err != nil {
		return nil, err
	}
	if doc.Mode == nil {
		return nil, fmt.Errorf("mode is missing")
	}
	if doc.Layout == nil {
		return nil, fmt.Errorf("layout is missing (it must be explicitly false in v1)")
	}
	return &DesignAssistancePayload{Mode: *doc.Mode, Layout: *doc.Layout}, nil
}

func init() {
	RegisterPayloadKind(DesignAssistancePayloadKind, decodeDesignAssistance)
}
