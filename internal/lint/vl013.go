package lint

import (
	"fmt"
	"strings"

	"github.com/OWNER/verdi/internal/gitx"
)

// vl013 enforces "nothing under .verdi/data/ is ever git-tracked (git add
// -f is intent; lint catches it)" (02 §Lint rules; 01 §notes).
type vl013 struct{}

func (vl013) ID() string { return "VL-013" }

func (vl013) Check(in *RunInput) []Finding {
	tracked, err := gitx.LsFiles(in.Ctx, in.Root)
	if err != nil {
		return []Finding{{Rule: "VL-013", Path: "", Message: fmt.Sprintf("listing git-tracked files: %v", err)}}
	}

	var findings []Finding
	for _, p := range tracked {
		if strings.HasPrefix(p, ".verdi/data/") {
			findings = append(findings, Finding{Rule: "VL-013", Path: p, Message: "file under .verdi/data/ is git-tracked; nothing under data/ may ever be committed"})
		}
	}
	return findings
}
