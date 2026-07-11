package lint

import (
	"fmt"
	"strings"
)

// vl012 enforces ".gitattributes marks all committed-generated paths with
// the configured forge's generated attribute" (02 §Lint rules;
// §Repository plumbing's literal three-line example).
type vl012 struct{}

func (vl012) ID() string { return "VL-012" }

func (vl012) Check(in *RunInput) []Finding {
	forge := ""
	if in.Snapshot.Manifest != nil {
		forge = in.Snapshot.Manifest.Forge
	}
	token := generatedAttrToken(forge)

	present := parseGitAttributes(in.Snapshot.GitAttributes)

	var missing []string
	for _, pattern := range generatedAttrPaths {
		if present[pattern] != token {
			missing = append(missing, pattern)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []Finding{{
		Rule:    "VL-012",
		Path:    ".gitattributes",
		Message: fmt.Sprintf("missing %q attribute line(s) for: %s", token, strings.Join(missing, ", ")),
	}}
}

// parseGitAttributes parses .gitattributes content into a pattern -> last-
// declared-attribute-token map. It recognizes only the single-token-per-
// line shape this store's own .gitattributes uses (`<pattern> <token>`);
// git's fuller attribute grammar (multiple tokens, "-attr", "attr=value")
// is out of scope — VL-012 only ever needs to check for the literal
// generated-attribute token.
func parseGitAttributes(data []byte) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		result[fields[0]] = fields[len(fields)-1]
	}
	return result
}
