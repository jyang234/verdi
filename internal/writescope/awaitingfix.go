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
// not meet.
func AwaitingFixes() []AwaitingFix {
	const carries = "commits every pre-staged index entry: git add -- <declared path> then a commit with no pathspec (UAT-036)"
	return []AwaitingFix{
		{Ritual: "design_start", Path: "verdi design start", Defect: carries},
		{Ritual: "design_start", Path: "verdi design start --supersedes", Defect: carries},
		{Ritual: "commit_to_design", Path: "verdi board commit, and the workbench's POST /board/{key}/commit", Defect: carries},
		{Ritual: "accept_diagram", Path: "verdi accept diagram/<name>", Defect: carries},
		{Ritual: "constitution_propose", Path: "verdi context constitution propose", Defect: carries + "; also cuts a new branch from HEAD rather than the resolved default branch (parent dc-11)"},
	}
}
