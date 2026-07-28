package main

import "strings"

// Shell-word rendering for copyable command templates (CLAUDE.md: one file
// ≈ one topic). Extracted from closeprepare.go, which now consumes it as
// one of several concerns rather than owning it: the disposition-template
// worklist `verdi close --prepare` prints is only the FIRST caller, and the
// safe-set rules below are a shell-compatibility question, not a closure-
// preparation one.

// shellQuoteWord renders one argument as a copyable shell word. The
// artifact schema deliberately accepts arbitrary non-empty finding IDs, so
// presentation must quote rather than narrow that compatibility boundary.
//
// Quoting is single-quote based: a single-quoted word is literal in sh,
// bash, and zsh alike, and an embedded single quote is spliced out and back
// the standard '"'"' way. The only words emitted BARE are those made
// entirely of isSafeShellWordRune characters — see that function for what
// the safe set must survive.
func shellQuoteWord(word string) string {
	if word != "" {
		safe := true
		for _, r := range word {
			if !isSafeShellWordRune(r) {
				safe = false
				break
			}
		}
		if safe {
			return word
		}
	}
	return "'" + strings.ReplaceAll(word, "'", `'"'"'`) + "'"
}

// isSafeShellWordRune reports whether r may appear in a word emitted
// UNQUOTED. The set is the intersection of what every shell an operator may
// paste into treats as literal — deliberately narrower than POSIX alone
// allows, because the emitted template is copied into whatever shell the
// reader happens to run, not into /bin/sh by construction.
//
// '=' is EXCLUDED even though POSIX sh, bash, and dash all treat it as an
// ordinary literal character. zsh — the macOS default shell — applies
// EQUALS expansion to a word whose FIRST character is '=': `=ls` becomes
// the path of the ls command (witness: `/bin/zsh -c 'set -- =ls; printf %s
// "$1"'` prints /bin/ls, while the same line under /bin/sh prints =ls). A
// finding id is arbitrary text from the artifact schema, so an id beginning
// with '=' emitted bare would disposition a DIFFERENT id than the one
// printed. The rune leaves the safe set entirely rather than being
// special-cased in first position: this predicate is per-rune by
// construction, one character is the smallest fix, and the only cost is
// that words merely CONTAINING '=' are quoted too — quoting is always safe,
// leaving a word bare never is.
func isSafeShellWordRune(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' ||
		strings.ContainsRune("_@%+:,./-", r)
}
