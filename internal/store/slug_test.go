package store

import (
	"strings"
	"testing"
)

func TestRefSlug_TableDriven(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		want string
	}{
		{"spec example from 01 §notes", "feature/stale-decline", "feature--stale-decline"},
		{"already-lower simple name", "main", "main"},
		{"uppercase lowered", "Feature/Stale-Decline", "feature--stale-decline"},
		{"multiple slashes both mapped", "release/2026/q3", "release--2026--q3"},
		{"underscore preserved: it is inside [a-z0-9._-]", "feature/foo_bar", "feature--foo_bar"},
		{"bare underscore ref preserved", "ci_run", "ci_run"},
		{"space mapped to dash", "feature/foo bar", "feature--foo-bar"},
		{"dots and dashes preserved", "v1.2.3-rc.1", "v1.2.3-rc.1"},
		{"digits preserved", "story-1482", "story-1482"},
		{"exclamation mapped to dash", "feature/foo!bar", "feature--foo-bar"},
		{"colon mapped to dash", "jira:LOAN-1482", "jira-loan-1482"},
		{"empty string", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RefSlug(tc.ref)
			if got != tc.want {
				t.Fatalf("RefSlug(%q) = %q, want %q", tc.ref, got, tc.want)
			}
		})
	}
}

// TestRefSlug_Collision proves the normative hard-error posture (01
// §notes): two distinct refs that map to the same slug are never silently
// merged — CheckSlugCollisions fails, naming both refs.
func TestRefSlug_Collision(t *testing.T) {
	refs := []string{"feature/foo bar", "feature/foo!bar"}
	if RefSlug(refs[0]) != RefSlug(refs[1]) {
		t.Fatalf("test setup invalid: %q and %q must slug identically (got %q and %q)",
			refs[0], refs[1], RefSlug(refs[0]), RefSlug(refs[1]))
	}

	err := CheckSlugCollisions(refs)
	if err == nil {
		t.Fatal("CheckSlugCollisions: want a hard error on collision, got nil")
	}
	for _, ref := range refs {
		if !strings.Contains(err.Error(), ref) {
			t.Fatalf("CheckSlugCollisions error %q does not name colliding ref %q", err.Error(), ref)
		}
	}
}

// TestCheckSlugCollisions_NoCollision is the negative-of-the-negative: a
// set of refs that map to distinct slugs must not error, including a ref
// repeated verbatim (not a collision — it's the same ref).
func TestCheckSlugCollisions_NoCollision(t *testing.T) {
	refs := []string{"feature/stale-decline", "main", "release/2026-q3", "feature/stale-decline"}
	if err := CheckSlugCollisions(refs); err != nil {
		t.Fatalf("CheckSlugCollisions: want nil, got %v", err)
	}
}

// TestRefSlug_UnderscoreIsNotDash proves the alphabet is read correctly.
// 01 §notes defines the slug alphabet as [a-z0-9._-], which CONTAINS the
// underscore, so an underscore survives slugging and "ci_run" and "ci-run"
// are two different slugs. Collapsing them would be exactly the silent
// merge the same paragraph forbids ("Two refs that collide after mapping
// are a hard error naming both — never a silent merge").
func TestRefSlug_UnderscoreIsNotDash(t *testing.T) {
	underscored := RefSlug("ci_run")
	dashed := RefSlug("ci-run")
	if underscored == dashed {
		t.Fatalf("RefSlug(%q) and RefSlug(%q) both = %q: distinct refs silently merged into one slug", "ci_run", "ci-run", underscored)
	}
	if !strings.Contains(underscored, "_") {
		t.Fatalf("RefSlug(%q) = %q, want the underscore preserved", "ci_run", underscored)
	}
}

// TestCheckSlugCollisions_UnderscoreAndDashDoNotCollide is the collision
// checker's half of the same rule: two refs differing only in '_' vs '-'
// are not a collision, because the mapping keeps them distinct.
func TestCheckSlugCollisions_UnderscoreAndDashDoNotCollide(t *testing.T) {
	if err := CheckSlugCollisions([]string{"ci_run", "ci-run"}); err != nil {
		t.Fatalf("CheckSlugCollisions([ci_run ci-run]): want nil (distinct slugs), got %v", err)
	}
}
