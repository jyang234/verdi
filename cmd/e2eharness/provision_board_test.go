package main

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestBadgeSpecDecodes proves every wall-badge fixture instance the
// harness provisions is a VALID spec document (spec/badge-computes ac-5's
// fixtures must reach a live, renderable board — a decode failure would
// 500 the board before any badge could compute): the draft instances and
// the frozen sealed instance all strict-decode and validate, and the
// badge-triggering state (the dangling stub ref, the dangling decision
// link, the dangling top-level link) is present in each.
func TestBadgeSpecDecodes(t *testing.T) {
	cases := []struct {
		name       string
		spec       string
		status     string
		frozenLine string
	}{
		{"authoring draft", badgeWallSpecName, "draft", ""},
		{"review draft", badgeReviewSpecName, "draft", ""},
		{"sealed", badgeSealedSpecName, "accepted-pending-build", "frozen: { at: 2024-01-01, commit: 0123456789abcdef0123456789abcdef01234567 }\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := badgeSpec(tc.spec, tc.status, tc.frozenLine)
			fmBytes, _, err := artifact.SplitFrontmatter([]byte(doc))
			if err != nil {
				t.Fatalf("SplitFrontmatter: %v", err)
			}
			fm, err := artifact.DecodeSpec(fmBytes)
			if err != nil {
				t.Fatalf("DecodeSpec: %v", err)
			}
			if fm.ID != "spec/"+tc.spec || string(fm.Status) != tc.status {
				t.Errorf("decoded id/status = %q/%q, want spec/%s / %s", fm.ID, fm.Status, tc.spec, tc.status)
			}
			if len(fm.Stubs) != 1 || fm.Stubs[0].Slug != "badge-orphan" || fm.Stubs[0].AcceptanceCriteria[0] != "ac-99" {
				t.Errorf("stubs = %+v, want the dangling badge-orphan → ac-99 fixture", fm.Stubs)
			}
			if len(fm.Links) != 1 || fm.Links[0].Ref != "spec/no-such-parent" {
				t.Errorf("links = %+v, want the dangling spec/no-such-parent depends-on", fm.Links)
			}
			if len(fm.Decisions) != 1 || len(fm.Decisions[0].Links) != 1 || fm.Decisions[0].Links[0].Ref != "adr/0099-no-such-adr" {
				t.Errorf("decisions = %+v, want dc-1 carrying the dangling exempts link", fm.Decisions)
			}
		})
	}
}

// TestBadgeSpecSealedRequiresFrozen is the negative path: the sealed
// status without its frozen stamp must FAIL validation (the frozenLine
// parameter is load-bearing, not decoration).
func TestBadgeSpecSealedRequiresFrozen(t *testing.T) {
	doc := badgeSpec(badgeSealedSpecName, "accepted-pending-build", "")
	fmBytes, _, err := artifact.SplitFrontmatter([]byte(doc))
	if err != nil {
		t.Fatalf("SplitFrontmatter: %v", err)
	}
	_, err = artifact.DecodeSpec(fmBytes)
	if err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Fatalf("DecodeSpec of an unfrozen sealed fixture: err = %v, want a frozen-stamp refusal", err)
	}
}
