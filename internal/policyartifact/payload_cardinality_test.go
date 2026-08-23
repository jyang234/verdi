package policyartifact

import (
	"fmt"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

const layeredCardinalityTestKind = "zz_layered_cardinality_test"

type layeredCardinalityTestPayload struct {
	Value string `json:"value" yaml:"value"`
}

func (p *layeredCardinalityTestPayload) PayloadKind() string { return layeredCardinalityTestKind }

func (p *layeredCardinalityTestPayload) Validate() error {
	if p.Value == "" {
		return fmt.Errorf("value is required")
	}
	return nil
}

func init() {
	RegisterLayeredPayloadKind(layeredCardinalityTestKind, func(raw []byte) (Payload, error) {
		var doc struct {
			Value *string `yaml:"value"`
		}
		if err := artifact.DecodeStrict(raw, &doc); err != nil {
			return nil, err
		}
		if doc.Value == nil {
			return nil, fmt.Errorf("value is missing")
		}
		return &layeredCardinalityTestPayload{Value: *doc.Value}, nil
	})
}

func TestLayeredPayloadCardinalityIsClosedAndSingletonRemainsDefault(t *testing.T) {
	tests := []struct {
		kind string
		want PayloadCardinality
	}{
		{kind: DesignAssistancePayloadKind, want: PayloadSingleton},
		{kind: layeredCardinalityTestKind, want: PayloadLayered},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			got, ok := RegisteredPayloadCardinality(tt.kind)
			if !ok || got != tt.want {
				t.Fatalf("RegisteredPayloadCardinality(%q) = %q, %t; want %q, true", tt.kind, got, ok, tt.want)
			}
		})
	}

	for _, cardinality := range []PayloadCardinality{PayloadSingleton, PayloadLayered} {
		if err := cardinality.Validate(); err != nil {
			t.Fatalf("%q.Validate() error = %v", cardinality, err)
		}
	}
	if err := PayloadCardinality("unknown").Validate(); err == nil {
		t.Fatal("unknown cardinality validated, want fail-closed error")
	}
}
