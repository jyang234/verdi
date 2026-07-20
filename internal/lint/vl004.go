package lint

import "fmt"

// vl004 enforces "status transitions legal per kind; status: draft MUST
// NOT exist on the default branch" (02 §Lint rules), scoped per I-14:
// enforced only when linting the default branch itself or a change
// targeting it (Context.EnforceDraftGate); otherwise this is silently a
// warning, not a finding — always-enforcing would break ordinary design
// branches (PLAN.md I-14's rejected alternative).
type vl004 struct{}

func (vl004) ID() string { return "VL-004" }

func (vl004) Check(in *RunInput) []Finding {
	if !in.LintCtx.EnforceDraftGate() {
		return nil
	}

	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Kind != "spec" {
			continue
		}
		if d.Status == "draft" {
			// vocab:identity — frontmatter status-line echo (the rule's subject IS the literal line)
			findings = append(findings, Finding{Rule: "VL-004", Path: d.RelPath, Message: fmt.Sprintf("status: draft on the default branch (%s)", in.LintCtx.DefaultBranch)})
		}
	}
	return findings
}
