package readinessload

import (
	"context"
	"strings"
	"testing"
)

// TestLoad_ObjectFollowsTheFamilyPosition is SI-345 (1) through the real
// loader (finding F4D-A1): stub slugs and object ids share one textual
// namespace, so a stub whose slug equals the declared decision dc-first
// is not that decision. Its stub-unreconciled row names no Object, while
// the rows whose family carries an object keep it.
func TestLoad_ObjectFollowsTheFamilyPosition(t *testing.T) {
	original := featureAlphaSpec(t)
	const stub = "{slug: alpha-story, acceptance_criteria: [ac-1]}"
	if !strings.Contains(original, stub) || !strings.Contains(original, "{id: dc-first,") {
		t.Fatal("feature-alpha no longer declares the alpha-story stub and the dc-first decision this test collides")
	}
	spec := strings.Replace(original, stub, "{slug: dc-first, acceptance_criteria: [ac-1]}", 1)
	repo := buildCompileRepo(t, map[string]string{".verdi/specs/active/feature-alpha/spec.md": spec})
	snap, err := Load(context.Background(), repo.Dir, "spec/feature-alpha", Options{BoardHref: boardHrefForTest})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for id, want := range map[string]string{
		"review/blocker/stub-unreconciled/dc-first": "",
		"review/blocker/outcome-floor/ac-1":         "ac-1",
		"success/coverage/ac-2":                     "ac-2",
		"shape/question/oq-1":                       "oq-1",
	} {
		if got := concernByID(t, snap, id).Object; got != want {
			t.Errorf("concern %q Object = %q, want %q", id, got, want)
		}
	}
	for _, c := range snap.AllConcerns {
		if c.Object == "dc-first" {
			t.Errorf("concern %q names the decision dc-first as its Object; no family carries a decision", c.ID)
		}
	}
}
