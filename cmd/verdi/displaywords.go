// Display-prose composition helpers for CLI verdict/refusal lines
// (judged-cli-refusal-prose-class-state-words-still-bare; ledger L-M13(1):
// user-facing prose speaking a class word is display-layer and resolves
// through the model). Kept beside the verbs that speak them rather than in
// internal/model because only cmd/verdi's hand-written verdict prose uses
// the alternation form; the words themselves always come from the model's
// display chain (DisplayClass/DisplayClassPlural/DisplayState).
package main

// displayAlternation renders the singular/plural alternation some verdict
// lines speak — today's hand-written "stor(y/ies)" — from the display
// chain's own singular and plural words, so the alternation follows a
// renamed vocabulary instead of leaking the bare id. With no rename the
// output is byte-identical to the hand-written form:
//
//	story/stories          -> "stor(y/ies)"   (diverging tails, both shown)
//	Change Request/…quests -> "Change Request(s)" (plural extends singular)
//
// Purely a rendering of two already-resolved display words; never applied
// to identity-layer ids.
func displayAlternation(singular, plural string) string {
	if singular == plural {
		return singular
	}
	s, p := []rune(singular), []rune(plural)
	i := 0
	for i < len(s) && i < len(p) && s[i] == p[i] {
		i++
	}
	if i == len(s) {
		return singular + "(" + string(p[i:]) + ")"
	}
	return string(s[:i]) + "(" + string(s[i:]) + "/" + string(p[i:]) + ")"
}
