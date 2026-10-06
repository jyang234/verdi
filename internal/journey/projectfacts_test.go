package journey

import (
	"context"
	"reflect"
	"testing"
)

// TestProjectFacts (ledger SI-352, lane P1 (a)) is ProjectFacts's table:
// over facts GatherFacts gathered it projects exactly the record
// ProjectWith projects for the same ref, for a feature with every kind of
// implementer and for a story; and facts no gather produced (the zero
// value: no target, no repository) are refused, never projected.
func TestProjectFacts(t *testing.T) {
	repo := buildEventualFixtureRepo(t)
	cfg := openConfig(t, repo.Dir)
	ctx := context.Background()
	p := NewProjector()
	for _, tc := range []struct {
		name    string
		facts   func(t *testing.T) (Facts, string)
		wantErr bool
	}{
		{name: "a feature's gathered facts", facts: func(t *testing.T) (Facts, string) {
			f, err := p.GatherFacts(ctx, cfg, "spec/checkout")
			if err != nil {
				t.Fatalf("GatherFacts: %v", err)
			}
			return f, "spec/checkout"
		}},
		{name: "a story's gathered facts", facts: func(t *testing.T) (Facts, string) {
			f, err := p.GatherFacts(ctx, cfg, "spec/checkout-story-two")
			if err != nil {
				t.Fatalf("GatherFacts: %v", err)
			}
			return f, "spec/checkout-story-two"
		}},
		{name: "zero facts", wantErr: true, facts: func(*testing.T) (Facts, string) { return Facts{}, "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts, ref := tc.facts(t)
			got, err := p.ProjectFacts(ctx, cfg, facts, Extras{})
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ProjectFacts(%+v) = %+v, want an error", facts, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ProjectFacts: %v", err)
			}
			want, err := p.ProjectWith(ctx, cfg, ref, Extras{})
			if err != nil {
				t.Fatalf("ProjectWith: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("ProjectFacts over gathered facts = %+v, want ProjectWith's %+v", got, want)
			}
		})
	}
}
