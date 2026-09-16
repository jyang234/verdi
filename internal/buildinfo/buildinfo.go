// Package buildinfo derives the one-line build identification string
// spec/uat-round-1 ac-1 requires (closes UAT-003: "Two builds were live
// during the UAT and disagreed about the store; nothing identified which
// one produced a result"). It is the single seam `verdi version`/`verdi
// --version` (cmd/verdi/version.go), `verdi serve`'s startup line
// (cmd/verdi/serve.go), and — a separate Fable lane — the workbench page
// footer all call, so the three surfaces can never disagree.
//
// Line is derived exclusively from runtime/debug.ReadBuildInfo: the main
// module's version and, when the binary was built inside a VCS checkout,
// a short revision and a "modified" marker. It never fabricates a
// version — when build info is unavailable it says so honestly.
package buildinfo

import "runtime/debug"

// shortRevisionLen is the number of leading hex characters of a VCS
// revision Line prints — long enough to be practically unique, short
// enough to read on one line, matching the conventional short-SHA length
// tools like `git rev-parse --short` settle on for a repository this
// size.
const shortRevisionLen = 12

// Line returns verdi's build identification line: the module version,
// a short VCS revision, and a "modified" marker when the checkout that
// produced this binary had uncommitted changes (vcs.modified). When
// runtime/debug.ReadBuildInfo cannot supply build info at all — a binary
// built without module/VCS context — it returns the honest placeholder
// "verdi (devel)" rather than a fabricated version.
func Line() string {
	return format(debug.ReadBuildInfo())
}

// format is Line's pure core, taking exactly what debug.ReadBuildInfo
// returns so it can be unit-tested with synthetic build info without
// depending on the test binary's own (environment-dependent) VCS stamps.
func format(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return "verdi (devel)"
	}

	version := info.Main.Version
	if version == "" {
		// Never fabricate a version: Go's own placeholder for "no
		// version could be determined" is "(devel)" (the same text
		// ReadBuildInfo itself uses for a main module built from source
		// rather than `go install pkg@version`).
		version = "(devel)"
	}
	line := "verdi " + version

	var revision string
	modified := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" {
		// No VCS settings embedded at all (e.g. built outside any git
		// checkout) — the version above is the whole honest answer.
		return line
	}

	line += " rev=" + shortRevision(revision)
	if modified {
		line += " modified"
	}
	return line
}

// shortRevision truncates rev to shortRevisionLen characters, never
// panicking or padding when rev is already shorter — it prints exactly
// what build info gave it, never a fabricated or invented value.
func shortRevision(rev string) string {
	if len(rev) > shortRevisionLen {
		return rev[:shortRevisionLen]
	}
	return rev
}
