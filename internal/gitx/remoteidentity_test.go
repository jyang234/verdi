package gitx

import (
	"strings"
	"testing"
)

func TestCanonicalRemoteIdentity_Happy(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"https", "https://github.com/Org/Repo.git", "github.com/Org/Repo"},
		{"https without .git", "https://github.com/Org/Repo", "github.com/Org/Repo"},
		{"scp-like", "git@github.com:Org/Repo.git", "github.com/Org/Repo"},
		{"scp-like without user", "github.com:Org/Repo.git", "github.com/Org/Repo"},
		{"ssh scheme", "ssh://git@github.com/Org/Repo.git", "github.com/Org/Repo"},
		{"git scheme", "git://github.com/Org/Repo.git", "github.com/Org/Repo"},
		{"http scheme", "http://github.com/Org/Repo.git", "github.com/Org/Repo"},
		{"uppercase host lowercased", "https://GitHub.COM/Org/Repo.git", "github.com/Org/Repo"},
		{"uppercase scheme", "HTTPS://GitHub.com/Org/Repo.git", "github.com/Org/Repo"},
		{"explicit port preserved", "ssh://git@forge.example.com:2222/org/repo.git", "forge.example.com:2222/org/repo"},
		{"explicit https port preserved", "https://forge.example.com:8443/org/repo.git", "forge.example.com:8443/org/repo"},
		{"trailing slash stripped", "https://github.com/Org/Repo.git/", "github.com/Org/Repo"},
		{"multiple trailing slashes stripped", "https://github.com/Org/Repo///", "github.com/Org/Repo"},
		{"userinfo stripped", "https://user@github.com/Org/Repo.git", "github.com/Org/Repo"},
		{"credentials stripped", "https://user:TOKEN@github.com/Org/Repo.git", "github.com/Org/Repo"},
		{"surrounding whitespace tolerated", "  https://github.com/Org/Repo.git\n", "github.com/Org/Repo"},
		{"nested path preserved", "https://gitlab.example.com/Group/Sub/Repo.git", "gitlab.example.com/Group/Sub/Repo"},
		{"only one .git suffix stripped", "https://github.com/Org/Repo.git.git", "github.com/Org/Repo.git"},
		{"dot-git inside a segment kept", "https://github.com/Org/Repo.github.git", "github.com/Org/Repo.github"},
		{"scp-like with deep path", "git@forge.example.com:group/sub/Repo.git", "forge.example.com/group/sub/Repo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CanonicalRemoteIdentity(tt.url)
			if !ok {
				t.Fatalf("CanonicalRemoteIdentity(%q) ok = false, want true", tt.url)
			}
			if got != tt.want {
				t.Fatalf("CanonicalRemoteIdentity(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// TestCanonicalRemoteIdentity_NeverEmitsCredentials is the security
// property GLG v3's own decision states directly ("journey projections
// contain no credentials, secrets, ..."): whatever else canonicalization
// does, no userinfo byte may survive into the identity.
func TestCanonicalRemoteIdentity_NeverEmitsCredentials(t *testing.T) {
	const secret = "s3cr3t-TOKEN"
	urls := []string{
		"https://user:" + secret + "@github.com/Org/Repo.git",
		"http://user:" + secret + "@github.com/Org/Repo.git",
		"ssh://user:" + secret + "@forge.example.com:2222/org/repo.git",
		"git://user:" + secret + "@github.com/Org/Repo.git",
		"https://" + secret + "@github.com/Org/Repo.git",
	}
	for _, u := range urls {
		got, ok := CanonicalRemoteIdentity(u)
		if !ok {
			t.Fatalf("CanonicalRemoteIdentity(%q) ok = false, want true", u)
		}
		if strings.Contains(got, secret) || strings.Contains(got, "user") || strings.Contains(got, "@") {
			t.Fatalf("CanonicalRemoteIdentity(%q) = %q, want no userinfo in the identity", u, got)
		}
	}
}

// TestCanonicalRemoteIdentity_SchemeAgnostic proves the identity property
// the journey record depends on: one repository reached over ssh and over
// https canonicalizes to the SAME identity, so two checkouts of one
// repository never produce different record digests merely because they
// spelled the remote differently.
func TestCanonicalRemoteIdentity_SchemeAgnostic(t *testing.T) {
	groups := [][]string{
		{
			"git@github.com:Org/Repo.git",
			"https://github.com/Org/Repo.git",
			"ssh://git@github.com/Org/Repo.git",
			"https://user:TOKEN@github.com/Org/Repo",
			"git://github.com/Org/Repo.git/",
		},
		{
			"ssh://git@forge.example.com:2222/org/repo.git",
			"https://forge.example.com:2222/org/repo",
		},
	}
	for _, group := range groups {
		first, ok := CanonicalRemoteIdentity(group[0])
		if !ok {
			t.Fatalf("CanonicalRemoteIdentity(%q) ok = false, want true", group[0])
		}
		for _, u := range group[1:] {
			got, ok := CanonicalRemoteIdentity(u)
			if !ok {
				t.Fatalf("CanonicalRemoteIdentity(%q) ok = false, want true", u)
			}
			if got != first {
				t.Fatalf("CanonicalRemoteIdentity(%q) = %q, want %q (same repository, different spelling)", u, got, first)
			}
		}
	}
}

// TestCanonicalRemoteIdentity_DifferentPortsAreDifferentRepositories is the
// negative twin of the scheme-agnostic property: an explicit port is part
// of a self-hosted forge's identity and must never be normalized away.
func TestCanonicalRemoteIdentity_DifferentPortsAreDifferentRepositories(t *testing.T) {
	a, okA := CanonicalRemoteIdentity("ssh://git@forge.example.com:2222/org/repo.git")
	b, okB := CanonicalRemoteIdentity("ssh://git@forge.example.com/org/repo.git")
	if !okA || !okB {
		t.Fatalf("CanonicalRemoteIdentity ok = %v/%v, want true/true", okA, okB)
	}
	if a == b {
		t.Fatalf("CanonicalRemoteIdentity collapsed an explicit port: both %q", a)
	}
}

func TestCanonicalRemoteIdentity_Negative(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"empty", ""},
		{"whitespace only", "   \n"},
		{"garbage word", "not-a-url"},
		{"unsupported scheme", "file:///srv/git/repo.git"},
		{"rsync scheme", "rsync://example.com/repo.git"},
		{"absolute local path", "/srv/git/repo.git"},
		{"relative local path", "../sibling/repo.git"},
		{"scheme with no host", "https:///Org/Repo.git"},
		{"scheme with no path", "https://github.com"},
		{"scheme with empty path", "https://github.com/"},
		{"scp-like with no host", "git@:Org/Repo.git"},
		{"scp-like with no path", "git@github.com:"},
		{"bare colon", ":"},
		{"bare at", "git@"},
		{"non-numeric port", "https://github.com:notaport/Org/Repo.git"},
		{"path that is only .git", "https://github.com/.git"},
		{"control character", "https://git\x00hub.com/Org/Repo.git"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := CanonicalRemoteIdentity(tt.url)
			if ok {
				t.Fatalf("CanonicalRemoteIdentity(%q) = %q, ok = true; want ok = false", tt.url, got)
			}
			if got != "" {
				t.Fatalf("CanonicalRemoteIdentity(%q) = %q on the not-ok path, want \"\"", tt.url, got)
			}
		})
	}
}

// TestCanonicalRemoteIdentity_Deterministic proves repeated calls agree —
// the record digest depends on it.
func TestCanonicalRemoteIdentity_Deterministic(t *testing.T) {
	const u = "ssh://git@forge.example.com:2222/Org/Repo.git"
	first, ok := CanonicalRemoteIdentity(u)
	if !ok {
		t.Fatalf("CanonicalRemoteIdentity(%q) ok = false", u)
	}
	for i := 0; i < 3; i++ {
		got, ok := CanonicalRemoteIdentity(u)
		if !ok || got != first {
			t.Fatalf("CanonicalRemoteIdentity(%q) call %d = (%q, %v), want (%q, true)", u, i, got, ok, first)
		}
	}
}
