package artifact

import "testing"

const componentSpecActiveYAML = `
id: spec/verdi-store-layout
kind: spec
class: component
title: "Store layout"
status: active
owners: [platform-team]
`

const componentSpecSupersededYAML = `
id: spec/legacy-cache-policy
kind: spec
class: component
title: "Legacy cache policy"
status: superseded
owners: [platform-team]
links:
  - { type: supersedes, ref: spec/verdi-store-layout }
`

const featureSpecDraftYAML = `
id: spec/new-feature-x
kind: spec
class: feature
title: "New feature X"
status: draft
owners: [platform-team]
story: jira:LOAN-9999
acceptance_criteria:
  - { id: ac-1, text: "does the thing", evidence: [static] }
`

const featureSpecAcceptedYAML = `
id: spec/stale-decline
kind: spec
class: feature
title: "Stale decline handling"
status: accepted-pending-build
owners: [platform-team]
story: jira:LOAN-1482
impacts: [loansvc, notification-svc]
context:
  - adr/0002-outbox-events@3e91ab2
declares:
  boundaries:
    - { from: loansvc, to: notification-svc, via: events }
acceptance_criteria:
  - { id: ac-1, text: "static check", evidence: [static] }
  - { id: ac-2, text: "static and behavioral", evidence: [static, behavioral] }
  - { id: ac-3, text: "behavioral only", evidence: [behavioral] }
  - { id: ac-4, text: "runtime", evidence: [runtime] }
dispositions:
  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: incorporated, where: "#ac-2" }
  - { sticky: a-01J8Z0K4BBBBBBBBBBBBBBBBBB, disposition: contradicted, note: "duplicates ac-1" }
  - { sticky: a-01J8Z0K5CCCCCCCCCCCCCCCCCC, disposition: open-question }
frozen: { at: 2026-05-14, commit: 3e91ab2 }
`

func TestDecodeSpec_Happy(t *testing.T) {
	cases := map[string]string{
		"component active":     componentSpecActiveYAML,
		"component superseded": componentSpecSupersededYAML,
		"feature draft":        featureSpecDraftYAML,
		"feature accepted":     featureSpecAcceptedYAML,
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			fm, err := DecodeSpec([]byte(y))
			if err != nil {
				t.Fatalf("DecodeSpec: %v", err)
			}
			if fm.ID == "" {
				t.Fatal("empty id")
			}
		})
	}
}

func TestDecodeSpec_Negative(t *testing.T) {
	cases := map[string]string{
		"unknown class":                          "id: spec/foo\nkind: spec\nclass: bogus\ntitle: Foo\nstatus: draft\nowners: [x]\n",
		"component with story":                   "id: spec/foo\nkind: spec\nclass: component\ntitle: Foo\nstatus: active\nowners: [x]\nstory: jira:LOAN-1\n",
		"component with ACs":                     "id: spec/foo\nkind: spec\nclass: component\ntitle: Foo\nstatus: active\nowners: [x]\nacceptance_criteria:\n  - { id: ac-1, text: t, evidence: [static] }\n",
		"feature missing story":                  "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nacceptance_criteria:\n  - { id: ac-1, text: t, evidence: [static] }\n",
		"feature no ACs":                         "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nstory: jira:LOAN-1\n",
		"feature duplicate AC id":                "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nstory: jira:LOAN-1\nacceptance_criteria:\n  - { id: ac-1, text: a, evidence: [static] }\n  - { id: ac-1, text: b, evidence: [behavioral] }\n",
		"feature AC bad evidence kind":           "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nstory: jira:LOAN-1\nacceptance_criteria:\n  - { id: ac-1, text: a, evidence: [bogus] }\n",
		"feature unpinned context":               "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nstory: jira:LOAN-1\nacceptance_criteria:\n  - { id: ac-1, text: a, evidence: [static] }\ncontext:\n  - adr/0001-foo\n",
		"feature accepted missing frozen":        "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: accepted-pending-build\nowners: [x]\nstory: jira:LOAN-1\nacceptance_criteria:\n  - { id: ac-1, text: a, evidence: [static] }\n",
		"feature draft with frozen":              "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nstory: jira:LOAN-1\nacceptance_criteria:\n  - { id: ac-1, text: a, evidence: [static] }\nfrozen: { at: 2026-01-01, commit: 3e91ab2 }\n",
		"disposition incorporated without where": "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nstory: jira:LOAN-1\nacceptance_criteria:\n  - { id: ac-1, text: a, evidence: [static] }\ndispositions:\n  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: incorporated }\n",
		"disposition contradicted without note":  "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nstory: jira:LOAN-1\nacceptance_criteria:\n  - { id: ac-1, text: a, evidence: [static] }\ndispositions:\n  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: contradicted }\n",
		"disposition unknown value":              "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nstory: jira:LOAN-1\nacceptance_criteria:\n  - { id: ac-1, text: a, evidence: [static] }\ndispositions:\n  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: bogus }\n",
		"disposition duplicate sticky":           "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nstory: jira:LOAN-1\nacceptance_criteria:\n  - { id: ac-1, text: a, evidence: [static] }\ndispositions:\n  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: open-question }\n  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: open-question }\n",
		"disposition bad sticky shape":           "id: spec/foo\nkind: spec\nclass: feature\ntitle: Foo\nstatus: draft\nowners: [x]\nstory: jira:LOAN-1\nacceptance_criteria:\n  - { id: ac-1, text: a, evidence: [static] }\ndispositions:\n  - { sticky: not-a-ulid, disposition: open-question }\n",
		"unknown top-level field":                "id: spec/foo\nkind: spec\nclass: component\ntitle: Foo\nstatus: active\nowners: [x]\nbogus: 1\n",
	}
	for name, y := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeSpec([]byte(y)); err == nil {
				t.Fatalf("DecodeSpec(%s): want error, got nil", name)
			}
		})
	}
}

func TestAcceptanceCriterion_Validate_Happy(t *testing.T) {
	ac := AcceptanceCriterion{ID: "ac-1", Text: "does the thing", Evidence: []EvidenceKind{EvidenceStatic, EvidenceBehavioral}}
	if err := ac.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestAcceptanceCriterion_Validate_Negative(t *testing.T) {
	cases := []AcceptanceCriterion{
		{ID: "bad-id", Text: "t", Evidence: []EvidenceKind{EvidenceStatic}},
		{ID: "ac-1", Text: "", Evidence: []EvidenceKind{EvidenceStatic}},
		{ID: "ac-1", Text: "t", Evidence: nil},
		{ID: "ac-1", Text: "t", Evidence: []EvidenceKind{"bogus"}},
	}
	for i, ac := range cases {
		if err := ac.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, ac)
		}
	}
}

func TestDisposition_Validate_Happy(t *testing.T) {
	cases := []Disposition{
		{Sticky: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Disposition: DispositionIncorporated, Where: "#ac-2"},
		{Sticky: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Disposition: DispositionContradicted, Note: "duplicates ac-1"},
		{Sticky: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Disposition: DispositionOpenQuestion},
	}
	for _, d := range cases {
		if err := d.Validate(); err != nil {
			t.Fatalf("Validate(%+v): %v", d, err)
		}
	}
}

func TestDisposition_Validate_Negative(t *testing.T) {
	cases := []Disposition{
		{Sticky: "not-a-ulid", Disposition: DispositionOpenQuestion},
		{Sticky: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Disposition: "bogus"},
		{Sticky: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Disposition: DispositionIncorporated},
		{Sticky: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Disposition: DispositionContradicted},
	}
	for i, d := range cases {
		if err := d.Validate(); err == nil {
			t.Fatalf("case %d Validate(%+v): want error, got nil", i, d)
		}
	}
}

func TestBoundary_Validate(t *testing.T) {
	if err := (Boundary{From: "a", To: "b", Via: "events"}).Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := (Boundary{From: "a", To: "b"}).Validate(); err == nil {
		t.Fatal("Validate: want error for missing via, got nil")
	}
}
