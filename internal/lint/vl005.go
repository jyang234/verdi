package lint

import (
	"fmt"
	"strings"

	"github.com/OWNER/verdi/internal/artifact"
)

// vl005 enforces "story spec has exactly one story: link with a configured
// scheme (moved from the feature class, R4-I-2); a feature spec's optional
// story: epic ref, when present, is validated against the same configured
// schemes" (02 §Lint rules, as amended). The story class carries the same
// I-24 mirroring machinery the feature class used to own (the scalar
// story: field is canonical, and an optional mirroring links[] entry
// (type: story) must agree with it exactly); the feature class's now-
// optional epic ref gets only the lighter scheme-configuredness check 02's
// amended row states, with no mirroring requirement of its own.
type vl005 struct{}

func (vl005) ID() string { return "VL-005" }

func (r vl005) Check(in *RunInput) []Finding {
	var findings []Finding
	schemes := in.Snapshot.Manifest.ConfiguredStorySchemes()

	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Spec == nil {
			continue
		}
		switch d.Spec.Class {
		case artifact.ClassStory:
			findings = append(findings, r.checkStory(d, schemes)...)
		case artifact.ClassFeature:
			findings = append(findings, r.checkFeatureEpicRef(d, schemes)...)
		}
	}
	return findings
}

// checkStory enforces the story class's canonical story: scalar (R4-I-2):
// exactly one story association (scalar plus an optional agreeing links[]
// mirror, I-24) with a configured scheme. DecodeSpec already guarantees the
// scalar is present and scheme:key-shaped for this class (spec.go's
// validateStory), so this rule's own job is the mirror-agreement and
// scheme-configuredness checks decode cannot see (verdi.yaml is lint-only
// context).
func (vl005) checkStory(d *Document, schemes map[string]bool) []Finding {
	var findings []Finding

	var storyLinks []string
	for _, l := range d.Base.Links {
		if l.Type == artifact.LinkStory {
			storyLinks = append(storyLinks, l.Ref)
		}
	}

	if len(storyLinks) > 1 {
		return []Finding{{Rule: "VL-005", Path: d.RelPath, Message: fmt.Sprintf("more than one type:story link (%s) plus the scalar story: field %q — exactly one story association is allowed", strings.Join(storyLinks, ", "), d.Spec.Story)}}
	}
	if len(storyLinks) == 1 && storyLinks[0] != d.Spec.Story {
		return []Finding{{Rule: "VL-005", Path: d.RelPath, Message: fmt.Sprintf("links[] story entry %q disagrees with the canonical scalar story: field %q (I-24)", storyLinks[0], d.Spec.Story)}}
	}

	scheme, _, _ := strings.Cut(d.Spec.Story, ":")
	if !schemes[scheme] {
		findings = append(findings, Finding{Rule: "VL-005", Path: d.RelPath, Message: fmt.Sprintf("story %q uses scheme %q, which verdi.yaml does not configure a provider for", d.Spec.Story, scheme)})
	}
	return findings
}

// checkFeatureEpicRef enforces the feature class's lighter, post-rescope
// check (02 §Lint rules, amended VL-005 row): the optional story: epic ref,
// when present, is validated against the same configured schemes — no
// "exactly one" or links[] mirror-agreement requirement, since the scalar
// is no longer the canonical per-story tracker binding on this class
// (R4-I-2).
func (vl005) checkFeatureEpicRef(d *Document, schemes map[string]bool) []Finding {
	if d.Spec.Story == "" {
		return nil
	}
	scheme, _, _ := strings.Cut(d.Spec.Story, ":")
	if !schemes[scheme] {
		return []Finding{{Rule: "VL-005", Path: d.RelPath, Message: fmt.Sprintf("story epic ref %q uses scheme %q, which verdi.yaml does not configure a provider for", d.Spec.Story, scheme)}}
	}
	return nil
}
