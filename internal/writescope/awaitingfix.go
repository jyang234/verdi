package writescope

// AwaitingFix is one ritual path whose declaration states the owner-ruled
// fix rather than what the path does at this base: the declaration it sits
// in, the invocation path, and the defect the fix removes.
type AwaitingFix struct {
	Ritual string
	Path   string
	Defect string
}

// AwaitingFixes returns the named, counted list of ritual paths awaiting
// the fix that makes them conform to their declaration (story dc-3). The
// witness reports it with its count. spec/ritual-effect-witness emptied
// it: each of story dc-3's five paths (design start, design start
// --supersedes, the commit-to-design ritual, accept diagram, constitution
// propose) left it in the same change as its fix, proven by
// TestUAT036_RitualsNeverCarryForeignEntries, so no merged witness passes
// against a declaration a ritual does not meet. The list only shrinks.
// Check holds it to the facts, ritual by ritual: a scoped declaration whose
// verbs reach gitx.CreateCommit (the whole-index commit) must be listed,
// and a listed ritual must still reach it.
func AwaitingFixes() []AwaitingFix {
	return []AwaitingFix{}
}
