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
