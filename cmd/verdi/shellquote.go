package main

import "strings"

// Shell-word rendering for copyable command templates (CLAUDE.md: one file
// ≈ one topic). Extracted from closeprepare.go, which now consumes it as
// one of several concerns rather than owning it: the disposition-template
// worklist `verdi close --prepare` prints is only the FIRST caller, and the
// safe-set rules below are a shell-compatibility question, not a closure-
// preparation one.

// shellQuoteWord renders one argument as a copyable POSIX-shell word. The
// artifact schema deliberately accepts arbitrary non-empty finding IDs, so
// presentation must quote rather than narrow that compatibility boundary.
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
// UNQUOTED.
func isSafeShellWordRune(r rune) bool {
	return r >= 'a' && r <= 'z' ||
		r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' ||
		strings.ContainsRune("_@%+=:,./-", r)
}
