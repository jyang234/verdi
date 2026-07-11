package dex

import (
	"path"
)

// permalinkURL returns ref's permalink URL — "/a/<kind>/<name>" for a
// committed-zone ref, "/a/svc/<service>/<artifact>[/<name>]" for an
// index-minted external ref (05 §Verdi-dex mechanics: "permalinks are
// /a/<kind>/<name> — refs, not paths, so links survive active->archive
// moves"). ref is already exactly the segments dex needs — index.Entry.Ref
// and artifact.Link.Ref both use this same "<kind>/<name>" (or
// "svc/<service>/...") shape.
func permalinkURL(ref string) string {
	return "/a/" + ref + "/"
}

// permalinkOutPath returns the output-directory-relative file path
// permalinkURL(ref) is served from: "a/<ref>/index.html" — a directory-form
// URL backed by an index.html, the shape every static host (GitLab/GitHub
// Pages included) serves without a rewrite rule.
func permalinkOutPath(ref string) string {
	return path.Join("a", ref, "index.html")
}

// resolvableLinkURL returns the permalink URL for l's ref if it resolves to
// one of dex's own pages (a committed-zone kind/name ref or a svc/...
// external ref, both minted into the index and so both permalinked), or
// ("", false) for a link this dex build has no page for — a story link
// (scheme:key tracker ref, 02 §Link taxonomy: "no inverse") or a dangling
// ref lint would flag but dex still renders honestly as plain text rather
// than a broken link.
func resolvableLinkURL(ref string, known map[string]bool) (string, bool) {
	if !known[ref] {
		return "", false
	}
	return permalinkURL(ref), true
}

// breadcrumbEntry is one crumb in a page's breadcrumb: URL is "" for the
// final (current-page) crumb, matching how layout.go renders the last
// crumb as plain text rather than a self-link.
type breadcrumbEntry struct {
	Label string
	URL   string
}

// pageBreadcrumb builds an entry's breadcrumb from its kind (and, for
// specs, whether the source file lives under specs/active/ or
// specs/archive/ — the directory truth, not a guess from status: 01
// §Directory layout is what actually moves a spec at acceptance/closure,
// and I-17's "never guess, drift toward honest" ethos extends here), per
// 05 §Verdi-dex's by-kind IA grouping ("specs active/archive, decisions,
// diagrams, contracts and APIs").
func pageBreadcrumb(kind, title string, archived bool) []breadcrumbEntry {
	crumbs := []breadcrumbEntry{{Label: "Home", URL: "/"}}
	switch kind {
	case "spec":
		crumbs = append(crumbs, breadcrumbEntry{Label: "Specs", URL: "/by-kind/spec/"})
		if archived {
			crumbs = append(crumbs, breadcrumbEntry{Label: "Archive", URL: "/by-kind/spec/archive/"})
		} else {
			crumbs = append(crumbs, breadcrumbEntry{Label: "Active", URL: "/by-kind/spec/active/"})
		}
	case "adr":
		crumbs = append(crumbs, breadcrumbEntry{Label: "Decisions", URL: "/by-kind/adr/"})
	case "diagram":
		crumbs = append(crumbs, breadcrumbEntry{Label: "Diagrams", URL: "/by-kind/diagram/"})
	case "attestation":
		crumbs = append(crumbs, breadcrumbEntry{Label: "Attestations", URL: "/by-kind/attestation/"})
	case "waiver":
		crumbs = append(crumbs, breadcrumbEntry{Label: "Waivers", URL: "/by-kind/waiver/"})
	case "conflict":
		crumbs = append(crumbs, breadcrumbEntry{Label: "Conflicts", URL: "/by-kind/conflict/"})
	}
	crumbs = append(crumbs, breadcrumbEntry{Label: title, URL: ""})
	return crumbs
}

// externalBreadcrumb is pageBreadcrumb's counterpart for a svc/... external
// ref: Home > Services > <service> > <label>.
func externalBreadcrumb(service, label string) []breadcrumbEntry {
	return []breadcrumbEntry{
		{Label: "Home", URL: "/"},
		{Label: "Services", URL: "/by-service/"},
		{Label: service, URL: "/by-service/" + service + "/"},
		{Label: label, URL: ""},
	}
}
