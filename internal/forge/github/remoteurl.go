package github

import "strings"

// OwnerRepoFromURL extracts the (owner, repo) pair from a GitHub remote URL
// — the `git remote get-url origin` value — so a local `verdi
// sync`/`close`/`gate`/`serve` can identify the repository WITHOUT the
// CI-only GITHUB_REPOSITORY / GITHUB_REPOSITORY_OWNER env vars being set
// (D6-14: those are GitHub Actions' own variables, absent in a developer
// shell). It accepts the three forms git writes for a github.com remote:
//
//	https://github.com/OWNER/REPO(.git)?
//	ssh://git@github.com/OWNER/REPO(.git)?
//	git@github.com:OWNER/REPO(.git)?        (scp-like)
//
// ok is false for a non-github.com URL — GitHub Enterprise Server on a
// custom domain is intentionally out of scope, matching forge.DetectKind's
// own github.com-substring limitation — or any URL that does not resolve to
// exactly OWNER/REPO. It is a pure string parse: no network, safe under
// `go test`. Owner/repo case is preserved (only the host match is
// case-insensitive).
func OwnerRepoFromURL(remoteURL string) (owner, repo string, ok bool) {
	const host = "github.com"
	s := strings.TrimSpace(remoteURL)
	i := strings.Index(strings.ToLower(s), host)
	if i < 0 {
		return "", "", false
	}
	rest := s[i+len(host):]
	rest = strings.TrimPrefix(rest, ":") // scp-like git@github.com:OWNER/REPO
	rest = strings.TrimPrefix(rest, "/") // https / ssh github.com/OWNER/REPO
	rest = strings.TrimSuffix(rest, "/")
	rest = strings.TrimSuffix(rest, ".git")
	rest = strings.TrimSuffix(rest, "/") // a trailing slash after .git, defensively

	owner, repo, found := strings.Cut(rest, "/")
	if !found || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return "", "", false
	}
	return owner, repo, true
}
