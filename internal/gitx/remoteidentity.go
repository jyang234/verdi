package gitx

import (
	"net/url"
	"regexp"
	"strings"
)

// hostRe is the host shape CanonicalRemoteIdentity admits: an ordinary DNS
// name or IPv4 literal. A bracketed IPv6 literal, an internationalized
// host in a non-ASCII spelling, or anything else deliberately fails
// closed (ok == false) rather than being half-normalized into an identity
// nobody can compare — an unknown identity is disclosable, a wrong one is
// not.
var hostRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]*[A-Za-z0-9])?$`)

// scpLikeRe matches git's scp-like remote syntax, `[user@]host:path` — the
// spelling `git@github.com:Org/Repo.git` uses. The host part admits no
// slash and no colon, so a URL with a scheme never reaches this branch and
// a path-only argument ("/srv/git/repo.git") never matches at all.
var scpLikeRe = regexp.MustCompile(`^(?:[^@/]*@)?([^@/:]+):(.*)$`)

// canonicalRemoteSchemes is the closed set of remote URL schemes
// CanonicalRemoteIdentity understands. Anything else (file://, rsync://, a
// helper transport) fails closed.
var canonicalRemoteSchemes = map[string]bool{
	"https": true,
	"http":  true,
	"ssh":   true,
	"git":   true,
}

// CanonicalRemoteIdentity reduces a git remote URL to a canonical, scheme-
// and credential-free repository identity of the form "host[:port]/path",
// returning ok == false for anything it cannot reduce.
//
// It exists because a raw remote URL is unfit to be a repository IDENTITY
// in a canonical artifact, for two independent reasons:
//
//   - It can carry credentials. `https://user:TOKEN@host/org/repo.git` is a
//     legal, common origin URL, and copying it into a record would serialize
//     a secret into a canonical, digest-bound, shareable artifact. GLG v3
//     (.verdi/specs/active/guided-lifecycle-governance-v3/spec.md) decides
//     that journey projections "contain no credentials, secrets, prompt
//     content, hidden reasoning, or unnecessary personal data". Userinfo is
//     therefore ALWAYS discarded — this function has no mode that keeps it.
//   - It is not stable per repository. One repository cloned over ssh
//     (`git@host:Org/Repo.git`) and over https
//     (`https://host/Org/Repo.git`) yields two different strings for the
//     same thing, so two checkouts would disagree on identity and on every
//     digest computed over it.
//
// The reduction: userinfo dropped; host lowercased (DNS is
// case-insensitive); an EXPLICIT port preserved (a self-hosted forge on
// :2222 is a different repository from one on the default port); one
// trailing ".git" and any trailing slashes stripped; path segment case
// preserved (forge path components are case-sensitive on most forges).
//
// Consumers: internal/journey's gatherRepositoryFacts, which stores the
// result as the record's canonical repository identity (AC-1) and, on
// ok == false, records the fact as unknown with a fixed disclosure —
// never the raw URL. gitx.RemoteURL itself is deliberately unchanged: its
// callers (cmd/verdi/sync.go's forge auto-detect, gate_threads.go) need
// the raw URL.
func CanonicalRemoteIdentity(rawURL string) (string, bool) {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return "", false
	}

	var host, port, path string
	if scheme, _, found := strings.Cut(raw, "://"); found {
		if !canonicalRemoteSchemes[strings.ToLower(scheme)] {
			return "", false
		}
		// Re-parse the whole string rather than hand-splitting it: net/url
		// already knows how to separate userinfo, host, port, and path, and
		// rejects an invalid port or a control character outright.
		u, err := url.Parse(raw)
		if err != nil {
			return "", false
		}
		host, port, path = u.Hostname(), u.Port(), u.EscapedPath()
	} else if m := scpLikeRe.FindStringSubmatch(raw); m != nil {
		// scp-like syntax has no port field at all (`host:2222/x` means the
		// path "2222/x", exactly as git itself reads it).
		host, port, path = m[1], "", m[2]
	} else {
		return "", false
	}

	if !hostRe.MatchString(host) {
		return "", false
	}
	if port != "" && !isDigits(port) {
		return "", false
	}

	path = strings.TrimLeft(path, "/")
	path = strings.TrimRight(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "", false
	}
	if !isPrintableASCII(path) {
		return "", false
	}

	identity := strings.ToLower(host)
	if port != "" {
		identity += ":" + port
	}
	return identity + "/" + path, true
}

// isDigits reports whether s is one or more ASCII digits — the port shape
// (net/url already rejects most malformed ports, but the scp-like branch
// never goes through it, so the check lives here for both).
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isPrintableASCII reports whether s consists solely of printable,
// non-space ASCII. A repository identity that reaches a canonical artifact
// must be comparable and safely renderable; anything outside that range
// fails closed rather than being emitted.
func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] <= ' ' || s[i] > '~' {
			return false
		}
	}
	return true
}
