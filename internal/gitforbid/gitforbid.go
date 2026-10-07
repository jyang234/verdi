// Package gitforbid is the one list of git tokens that no governed git
// command log may carry, and the rule that matches them
// (spec/gitx-recorder-seam dc-1; spec/ritual-write-scope-v3 dc-9).
// Recovery's runtime refusal imports it, and so does the forbidden-token
// witness, so the two checks cannot drift apart.
package gitforbid

import "strings"

// Tokens returns the forbidden tokens, as a fresh slice on each call:
// ritual-write-scope-v3 ac-3's six (reset, restore, clean, stash, --force,
// update-ref), which are a floor, and the short force flag -f (ledger
// SI-224) that `git push -f`, `git branch -f` and `git checkout -f` all
// accept and "--force" alone does not catch.
func Tokens() []string {
	return []string{"reset", "restore", "clean", "stash", "--force", "-f", "update-ref"}
}

// Forbids reports whether argv, one git command's whole argument list
// after "git", contains a forbidden token. The check is over argv
// elements, never over the text inside one: an element matches a token
// when it equals it, or, for a token that begins with "--", when it starts
// with it ("--force" matches "--force-with-lease"). So `commit -m "reset
// the counter"` is not forbidden, because its message is one element that
// merely starts with a token. An exact one-word message ("commit -m
// reset") would be, since the check cannot tell a message from any other
// element.
func Forbids(argv []string) bool {
	for _, arg := range argv {
		if matches(arg) {
			return true
		}
	}
	return false
}

// matches reports whether arg, one whole argv element, matches a token by
// Forbids' rule. The prefix rule is kept to "--" tokens so that a plain
// token such as "reset" never matches an unrelated element that only
// starts with the same word.
func matches(arg string) bool {
	for _, tok := range Tokens() {
		if arg == tok {
			return true
		}
		if strings.HasPrefix(tok, "--") && strings.HasPrefix(arg, tok) {
			return true
		}
	}
	return false
}
