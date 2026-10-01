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
// witness reports it with its count; spec/ritual-effect-witness empties it
// in the same change as the fixes and the behavioral witness that proves
// them, so no merged witness passes against a declaration a ritual does
// not meet. Check holds the list to the facts, ritual by ritual: a scoped
// declaration whose verbs reach gitx.CreateCommit (the whole-index commit)
// must be listed, and a listed ritual must still reach it. design start's
// --supersedes path is parsed inside the design start verb, so its entry
// sits in design_start.
func AwaitingFixes() []AwaitingFix {
	const carries = "commits every pre-staged index entry: git add -- <declared path> then a commit with no pathspec (UAT-036)"
	return []AwaitingFix{
		{Ritual: "constitution_propose", Path: "verdi context constitution propose", Defect: carries + "; also cuts a new branch from HEAD rather than the resolved default branch (parent dc-11)"},
	}
}
