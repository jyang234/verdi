package lint

// vl001 enforces "frontmatter present, decodes strictly against kind
// schema; the restricted dialect is enforced here" (02 §Lint rules) — the
// syntactic half of decode: frontmatter presence (SplitFrontmatter),
// unknown fields (KnownFields), and the restricted dialect (anchors,
// aliases, custom tags). See doc.go's design note for why this rule does
// not also run each kind's semantic Validate().
type vl001 struct{}

func (vl001) ID() string { return "VL-001" }

func (vl001) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered {
			continue
		}
		if d.DecodeErr != nil {
			findings = append(findings, Finding{Rule: "VL-001", Path: d.RelPath, Message: d.DecodeErr.Error()})
		}
	}
	return findings
}
