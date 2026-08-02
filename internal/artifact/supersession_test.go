package artifact

import (
	"fmt"
	"strings"
	"testing"
)

// guide-claim: 8.3-supersession
func TestSupersession_Validate_Happy(t *testing.T) {
	s := Supersession{
		Carried:         []string{"ac-1", "co-3"},
		Amended:         []SupersessionNote{{ID: "ac-2", Note: "reworded for clarity"}},
		AmendedAdvisory: []SupersessionNote{{ID: "dc-4", Note: "non-reaffirming rewording"}},
		Removed:         []SupersessionNote{{ID: "ac-5", Note: "descoped"}},
		Added:           []string{"ac-6", "ac-7"},
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestSupersession_Validate_Negative(t *testing.T) {
	cases := []Supersession{
		{Carried: []string{"Not-Kebab"}},
		{Amended: []SupersessionNote{{ID: "ac-2", Note: ""}}},
		{Amended: []SupersessionNote{{ID: "bad id", Note: "x"}}},
		{AmendedAdvisory: []SupersessionNote{{ID: "dc-4", Note: ""}}},
		{Removed: []SupersessionNote{{ID: "ac-5", Note: ""}}},
		{Added: []string{"not valid!"}},
	}
	for i, s := range cases {
		if err := s.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, s)
		}
	}
}

// supersessionCardinalityTmpl is a minimal, schema-valid feature spec
// carrying a supersession: block, with its whole links: section supplied by
// the caller (empty string = no links: key at all).
const supersessionCardinalityTmpl = `
id: spec/loan-workflow-v2
kind: spec
class: feature
title: "Loan workflow v2 (supersession predecessor cardinality)"
status: draft
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "does the thing", evidence: [static] }
%ssupersession:
  added: [ac-1]
`

// TestDecodeSpec_SupersessionPredecessorCardinality proves the
// exactly-one-whole-spec-predecessor invariant a supersession: block
// carries: the block is a manifest ABOUT one named predecessor's objects,
// so zero predecessors leaves it un-anchored and two or more make it
// ambiguous (each predecessor would be credited the same single manifest).
// A FRAGMENT supersedes edge alongside it is a decision-level override, not
// a second whole-spec claim, and must not count either way.
func TestDecodeSpec_SupersessionPredecessorCardinality(t *testing.T) {
	cases := []struct {
		name    string
		links   string
		wantErr bool
	}{
		{
			name:    "exactly one whole-spec predecessor",
			links:   "links:\n  - { type: supersedes, ref: spec/loan-workflow }\n",
			wantErr: false,
		},
		{
			name: "one whole-spec predecessor plus a fragment override edge",
			links: "links:\n" +
				"  - { type: supersedes, ref: spec/loan-workflow }\n" +
				"  - { type: supersedes, ref: \"spec/rate-lock#dc-1\" }\n",
			wantErr: false,
		},
		{
			name: "one whole-spec predecessor plus an unrelated link type",
			links: "links:\n" +
				"  - { type: supersedes, ref: spec/loan-workflow }\n" +
				"  - { type: depends-on, ref: spec/rate-lock }\n",
			wantErr: false,
		},
		{
			name:    "zero supersedes links at all",
			links:   "",
			wantErr: true,
		},
		{
			name:    "no supersedes link, only another link type",
			links:   "links:\n  - { type: depends-on, ref: spec/rate-lock }\n",
			wantErr: true,
		},
		{
			name:    "only a FRAGMENT supersedes edge (a decision override, not a whole-spec claim)",
			links:   "links:\n  - { type: supersedes, ref: \"spec/loan-workflow#dc-1\" }\n",
			wantErr: true,
		},
		{
			name:    "only a NON-spec-kind supersedes edge",
			links:   "links:\n  - { type: supersedes, ref: adr/0002-outbox-events }\n",
			wantErr: true,
		},
		{
			name: "two whole-spec predecessors",
			links: "links:\n" +
				"  - { type: supersedes, ref: spec/loan-workflow }\n" +
				"  - { type: supersedes, ref: spec/rate-lock }\n",
			wantErr: true,
		},
		{
			name: "three whole-spec predecessors",
			links: "links:\n" +
				"  - { type: supersedes, ref: spec/loan-workflow }\n" +
				"  - { type: supersedes, ref: spec/rate-lock }\n" +
				"  - { type: supersedes, ref: spec/escrow-autopay }\n",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			y := fmt.Sprintf(supersessionCardinalityTmpl, tc.links)
			_, err := DecodeSpec([]byte(y))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("DecodeSpec: want a validation error, got nil\n%s", y)
				}
				if !strings.Contains(err.Error(), "supersession") {
					t.Fatalf("DecodeSpec error = %v, want it to name the supersession: block", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("DecodeSpec: %v\n%s", err, y)
			}
		})
	}
}

// TestWholeSpecSupersedesRefs is the shared predicate every consumer
// (artifact validation, internal/lint's VL-015, internal/specstate's corpus
// scan) resolves the "whole-spec predecessor" question through, rather than
// each re-deriving it.
func TestWholeSpecSupersedesRefs(t *testing.T) {
	cases := []struct {
		name  string
		links []Link
		want  []string
	}{
		{name: "no links", links: nil, want: nil},
		{
			name:  "one whole-spec supersedes edge",
			links: []Link{{Type: LinkSupersedes, Ref: "spec/loan-workflow"}},
			want:  []string{"spec/loan-workflow"},
		},
		{
			name:  "pinned whole-spec supersedes edge keeps its pin out of the name",
			links: []Link{{Type: LinkSupersedes, Ref: "spec/loan-workflow@3e91ab2"}},
			want:  []string{"spec/loan-workflow"},
		},
		{
			name:  "fragment supersedes edge is excluded",
			links: []Link{{Type: LinkSupersedes, Ref: "spec/loan-workflow#dc-1"}},
			want:  nil,
		},
		{
			name:  "non-spec kind is excluded",
			links: []Link{{Type: LinkSupersedes, Ref: "adr/0002-outbox-events"}},
			want:  nil,
		},
		{
			name:  "other link types are excluded",
			links: []Link{{Type: LinkDependsOn, Ref: "spec/loan-workflow"}},
			want:  nil,
		},
		{
			name:  "unparseable ref is excluded",
			links: []Link{{Type: LinkSupersedes, Ref: "svc/loansvc/openapi"}},
			want:  nil,
		},
		{
			name: "two whole-spec edges are both reported, in link order",
			links: []Link{
				{Type: LinkSupersedes, Ref: "spec/rate-lock"},
				{Type: LinkSupersedes, Ref: "spec/loan-workflow#dc-1"},
				{Type: LinkSupersedes, Ref: "spec/loan-workflow"},
			},
			want: []string{"spec/rate-lock", "spec/loan-workflow"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := WholeSpecSupersedesRefs(tc.links)
			if len(got) != len(tc.want) {
				t.Fatalf("WholeSpecSupersedesRefs = %v, want %v", got, tc.want)
			}
			for i, r := range got {
				if r.String() != tc.want[i] {
					t.Fatalf("WholeSpecSupersedesRefs[%d] = %q, want %q", i, r.String(), tc.want[i])
				}
			}
		})
	}
}
