package artifactview

import "testing"

const featureSpecYAML = `id: spec/stale-decline
kind: spec
class: feature
title: "Stale decline"
status: draft
owners: [platform-team]
story: jira:LOAN-1482
acceptance_criteria:
  - { id: ac-1, text: "does the thing", evidence: [static] }
dispositions:
  - { sticky: a-01J8Z0K3AAAAAAAAAAAAAAAAAA, disposition: open-question }
`

const adrYAML = `id: adr/0001-outbox-events
kind: adr
title: "Outbox events"
status: accepted
owners: [platform-team]
decided: 2026-01-01
frozen: { at: 2026-01-01, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
`

const obligationYAML = `id: obligation/fail-loud--ac-1--static
kind: obligation
title: "The gate refuses tracked binaries"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/fail-loud" }
frozen: { at: 2026-07-13, commit: c5e360a9ee5e9eb6089e54b772fa16959ada4662 }
`

func TestDecodeMeta_Happy(t *testing.T) {
	t.Run("spec", func(t *testing.T) {
		m, err := DecodeMeta("spec", []byte(featureSpecYAML))
		if err != nil {
			t.Fatalf("DecodeMeta: %v", err)
		}
		if m.Story != "jira:LOAN-1482" {
			t.Errorf("Story = %q, want jira:LOAN-1482", m.Story)
		}
		if len(m.Dispositions) != 1 || m.Dispositions[0].Sticky != "a-01J8Z0K3AAAAAAAAAAAAAAAAAA" {
			t.Errorf("Dispositions = %+v", m.Dispositions)
		}
		if m.Base.Title != "Stale decline" {
			t.Errorf("Base.Title = %q", m.Base.Title)
		}
	})
	t.Run("adr", func(t *testing.T) {
		m, err := DecodeMeta("adr", []byte(adrYAML))
		if err != nil {
			t.Fatalf("DecodeMeta: %v", err)
		}
		if m.Decided != "2026-01-01" {
			t.Errorf("Decided = %q", m.Decided)
		}
	})
	t.Run("obligation", func(t *testing.T) {
		m, err := DecodeMeta("obligation", []byte(obligationYAML))
		if err != nil {
			t.Fatalf("DecodeMeta: %v", err)
		}
		if m.ForKind != "static" {
			t.Errorf("ForKind = %q, want static", m.ForKind)
		}
		if m.Base.Title != "The gate refuses tracked binaries" {
			t.Errorf("Base.Title = %q", m.Base.Title)
		}
	})
}

func TestDecodeMeta_Negative(t *testing.T) {
	t.Run("unknown kind", func(t *testing.T) {
		if _, err := DecodeMeta("bogus", []byte(featureSpecYAML)); err == nil {
			t.Fatal("expected an error for an unhandled kind")
		}
	})
	t.Run("malformed frontmatter", func(t *testing.T) {
		if _, err := DecodeMeta("spec", []byte("not: valid: yaml: at: all:")); err == nil {
			t.Fatal("expected a decode error")
		}
	})
}
