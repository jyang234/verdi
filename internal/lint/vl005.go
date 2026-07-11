package lint

import (
	"fmt"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/store"
)

// vl005 enforces "feature spec has exactly one story: link with a
// configured scheme" (02 §Lint rules), reading I-24's resolution: the
// scalar `story:` field is canonical, and an optional mirroring `links[]`
// entry (type: story) must agree with it exactly.
type vl005 struct{}

func (vl005) ID() string { return "VL-005" }

func (vl005) Check(in *RunInput) []Finding {
	var findings []Finding
	schemes := configuredStorySchemes(in.Snapshot.Manifest)

	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil || d.Spec.Class != artifact.ClassFeature {
			continue
		}

		var storyLinks []string
		for _, l := range d.Base.Links {
			if l.Type == artifact.LinkStory {
				storyLinks = append(storyLinks, l.Ref)
			}
		}

		if len(storyLinks) > 1 {
			findings = append(findings, Finding{Rule: "VL-005", Path: d.RelPath, Message: fmt.Sprintf("more than one type:story link (%s) plus the scalar story: field %q — exactly one story association is allowed", strings.Join(storyLinks, ", "), d.Spec.Story)})
			continue
		}
		if len(storyLinks) == 1 && storyLinks[0] != d.Spec.Story {
			findings = append(findings, Finding{Rule: "VL-005", Path: d.RelPath, Message: fmt.Sprintf("links[] story entry %q disagrees with the canonical scalar story: field %q (I-24)", storyLinks[0], d.Spec.Story)})
			continue
		}

		scheme, _, _ := strings.Cut(d.Spec.Story, ":")
		if !schemes[scheme] {
			findings = append(findings, Finding{Rule: "VL-005", Path: d.RelPath, Message: fmt.Sprintf("story %q uses scheme %q, which verdi.yaml does not configure a provider for", d.Spec.Story, scheme)})
		}
	}
	return findings
}

// configuredStorySchemes returns the set of story-ref schemes verdi.yaml's
// providers: block configures. Only "jira" is modeled today
// (internal/store.ProvidersConfig); an absent manifest or providers block
// configures no scheme at all.
func configuredStorySchemes(m *store.Manifest) map[string]bool {
	schemes := map[string]bool{}
	if m == nil || m.Providers == nil {
		return schemes
	}
	if m.Providers.Jira != nil {
		schemes["jira"] = true
	}
	return schemes
}
