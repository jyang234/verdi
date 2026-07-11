package lint

// generatedAttrToken returns the git-attribute token VL-012 requires for
// committed-generated paths, per the manifest's `forge:` field (I-22:
// "gitlab-generated / linguist-generated"). This package only reads the
// already-decoded manifest field — the forge port itself (auto-detection
// from the remote URL when forge: is omitted) is another agent's work
// (PLAN.md I-22); an empty/omitted forge defaults to "gitlab-generated"
// here, matching 02 §Repository plumbing's own literal example.
func generatedAttrToken(forge string) string {
	if forge == "github" {
		return "linguist-generated"
	}
	return "gitlab-generated"
}

// generatedAttrPaths are the fixed set of committed-generated path patterns
// 02 §Repository plumbing enumerates verbatim.
var generatedAttrPaths = []string{
	".verdi/specs/*/*/board.json",
	".verdi/specs/*/*/rollup.json",
	".verdi/specs/*/*/deviation-report.md",
}
