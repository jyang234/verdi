package publicrelease

import (
	"encoding/json"
	"testing"
)

func TestClosedTenProducerDeclarations(t *testing.T) {
	base, err := readDeclarations(checkBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups(base)) < 6 {
		t.Fatal("lost independent package scopes")
	}
	for _, tc := range []struct {
		name   string
		change func(*declarations)
	}{
		{"missing", func(d *declarations) { d.Producers = d.Producers[1:] }},
		{"duplicate", func(d *declarations) { d.Producers[1] = d.Producers[0] }},
		{"unknown", func(d *declarations) { d.Producers[0].ID = "unknown" }},
		{"wrong kind", func(d *declarations) { d.Producers[0].Kind = "static" }},
		{"wrong AC", func(d *declarations) { d.Producers[0].AC = "ac-99" }},
		{"absent mutation witness", func(d *declarations) {
			delete(d.Tests, "V:internal/sealedexec:TestPublicControllerConsolidationWitness")
		}},
		{"missing required name", func(d *declarations) {
			r := d.Tests[d.Producers[0].Checks[0]]
			r.Required = nil
			d.Tests[d.Producers[0].Checks[0]] = r
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := readDeclarations(checkBytes)
			if err != nil {
				t.Fatal(err)
			}
			tc.change(&d)
			b, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := readDeclarations(b); err == nil {
				t.Fatal("accepted malformed declarations")
			}
		})
	}
}

func TestFinalFrameBoundCorrectionsAreRequired(t *testing.T) {
	d, err := readDeclarations(checkBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []struct {
		producer string
		roots    []string
	}{
		{"public-execution-contract:ac-2:behavioral", []string{"A:internal/verdiproto:TestControllerSuccessFrameBoundIsExclusiveAndReusable", "A:internal/verdiproto:TestControllerSuccessMalformedRenderRefuses"}},
		{"public-execution-contract:ac-5:behavioral", []string{"A:internal/verdiproto:TestControllerSuccessFrameBoundIsExclusiveAndReusable", "A:internal/verdiproto:TestControllerSuccessMalformedRenderRefuses", "V:internal/sealedexec:TestPublicControllerConsolidationBoundCorrection"}},
		{"public-execution-contract:ac-5:static", []string{"V:internal/sealedexec:TestPublicControllerConsolidationBoundCorrection"}},
	} {
		t.Run(scope.producer, func(t *testing.T) {
			found := map[string]bool{}
			for _, p := range d.Producers {
				if p.ID == scope.producer {
					for _, key := range p.Checks {
						found[key] = true
					}
				}
			}
			for _, key := range scope.roots {
				r, ok := d.Tests[key]
				if !found[key] || !ok {
					t.Errorf("missing required correction %s", key)
					continue
				}
				if len(r.Required) == 0 {
					t.Errorf("missing required correction descendants %s", key)
				}
			}
		})
	}
}
