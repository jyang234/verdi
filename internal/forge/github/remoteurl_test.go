package github

import "testing"

func TestOwnerRepoFromURL(t *testing.T) {
	cases := []struct {
		name        string
		url         string
		owner, repo string
		ok          bool
	}{
		// Happy path — the three forms git writes, with and without .git.
		{"https with .git", "https://github.com/jyang234/verdi.git", "jyang234", "verdi", true},
		{"https no .git", "https://github.com/jyang234/verdi", "jyang234", "verdi", true},
		{"scp with .git", "git@github.com:jyang234/verdi.git", "jyang234", "verdi", true},
		{"scp no .git", "git@github.com:jyang234/verdi", "jyang234", "verdi", true},
		{"ssh scheme", "ssh://git@github.com/jyang234/verdi.git", "jyang234", "verdi", true},
		{"trailing slash", "https://github.com/jyang234/verdi/", "jyang234", "verdi", true},
		{"trailing slash after .git", "https://github.com/jyang234/verdi.git/", "jyang234", "verdi", true},
		{"leading/trailing space", "  https://github.com/jyang234/verdi  ", "jyang234", "verdi", true},
		{"host case-insensitive, owner/repo case preserved", "https://GitHub.com/JYang/Verdi-CLI.git", "JYang", "Verdi-CLI", true},

		// Negative — nothing to identify, or not github.com.
		{"empty", "", "", "", false},
		{"gitlab", "https://gitlab.com/jyang234/verdi.git", "", "", false},
		{"enterprise custom domain (out of scope, like DetectKind)", "https://github.company.com/jyang234/verdi.git", "", "", false},
		{"host only", "https://github.com/", "", "", false},
		{"owner only, no repo", "https://github.com/jyang234", "", "", false},
		{"extra path segment", "https://github.com/jyang234/verdi/tree/main", "", "", false},
		{"empty owner", "https://github.com//verdi", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner, repo, ok := OwnerRepoFromURL(tc.url)
			if ok != tc.ok || owner != tc.owner || repo != tc.repo {
				t.Fatalf("OwnerRepoFromURL(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.url, owner, repo, ok, tc.owner, tc.repo, tc.ok)
			}
		})
	}
}
