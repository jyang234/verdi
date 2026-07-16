package main

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestClosedAwaitingArchiveSpec_Decodes proves the home-status-glance
// "closed awaiting archive" fixture (provision.go) is a well-formed,
// strict-decodable feature spec BEFORE the slow e2e suite ever provisions
// it: closed status with its required frozen stamp, an active-zone shape
// (the fixture is planted under .verdi/specs/active/ by provisionStore,
// never .verdi/specs/archive/ — this test only proves the frontmatter
// itself decodes; provision_test.go/main.go own the placement).
func TestClosedAwaitingArchiveSpec_Decodes(t *testing.T) {
	fm, _, err := artifact.SplitFrontmatter([]byte(closedAwaitingArchiveSpec))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		t.Fatalf("DecodeSpec: %v", err)
	}
	if spec.Class != artifact.ClassFeature {
		t.Fatalf("class = %q, want %q", spec.Class, artifact.ClassFeature)
	}
	if spec.Status != "closed" {
		t.Fatalf("status = %q, want %q", spec.Status, "closed")
	}
	if spec.Frozen == nil {
		t.Fatal("closed feature must carry a frozen stamp (internal/artifact requireFrozen), got nil")
	}
	if spec.ID != "spec/"+closedAwaitingArchiveName {
		t.Fatalf("id = %q, want %q", spec.ID, "spec/"+closedAwaitingArchiveName)
	}
}

// TestClosedAwaitingArchiveSpec_Negative_ComponentClassRejectsClosed
// guards the exact reasoning provision.go's own doc comment gives for
// choosing class: feature over class: component: a component's status
// enum has no "closed" value at all (internal/artifact/status.go's
// specComponentStatuses), so swapping the class on this fixture's own
// frontmatter must fail closed rather than silently decode — the
// negative-path proof that the class choice is load-bearing, not
// incidental.
func TestClosedAwaitingArchiveSpec_Negative_ComponentClassRejectsClosed(t *testing.T) {
	componentDoc := `---
id: spec/rate-table-sunset
kind: spec
class: component
title: "Rate table sunset (negative fixture)"
status: closed
owners: [platform-team]
---
# Rate table sunset
`
	fm, _, err := artifact.SplitFrontmatter([]byte(componentDoc))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	if _, err := artifact.DecodeSpec(fm); err == nil {
		t.Fatal("DecodeSpec accepted a component spec with status: closed, want a fail-closed refusal")
	}
}
