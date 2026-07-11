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
		{"underscore mapped to dash", "feature/foo_bar", "feature--foo-bar"},
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
