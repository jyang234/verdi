package contextcompile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/specstate"
)

// FragmentFeature identifies the exact accepted feature source from which a
// parent fragment was projected.
type FragmentFeature struct {
	Ref          string
	Path         string
	SourceDigest string
}

// FragmentTarget is one legal story-to-feature object edge. Evidence is
// declaration-ordered and nonempty for ac-* targets and nil for oq-* targets.
type FragmentTarget struct {
	ID       string
	Text     string
	Evidence []artifact.EvidenceKind
	Anchor   string
}

// FeatureFragment is the complete compiler-local semantic projection for one
// governing feature. It deliberately excludes feature prose, stubs and every
// untargeted acceptance criterion or open question.
type FeatureFragment struct {
	Feature     FragmentFeature
	Problem     artifact.Attribute
	Outcome     artifact.Attribute
	Targets     []FragmentTarget
	Constraints []artifact.Constraint
	Decisions   []artifact.Decision
}

type fragmentFeatureDoc struct {
	Ref          string `json:"ref"`
	Path         string `json:"path"`
	SourceDigest string `json:"source_digest"`
}

type fragmentAttributeDoc struct {
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
}

type fragmentTargetDoc struct {
	ID       string                  `json:"id"`
	Text     string                  `json:"text"`
	Evidence []artifact.EvidenceKind `json:"evidence,omitempty"`
	Anchor   string                  `json:"anchor"`
}

type fragmentConstraintDoc struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Anchor string `json:"anchor"`
}

type fragmentLinkDoc struct {
	Type artifact.LinkType `json:"type"`
	Ref  string            `json:"ref"`
	Note string            `json:"note,omitempty"`
}

type fragmentDecisionDoc struct {
	ID     string            `json:"id"`
	Text   string            `json:"text"`
	Anchor string            `json:"anchor"`
	Links  []fragmentLinkDoc `json:"links,omitempty"`
}

type fragmentEncodeDoc struct {
	Feature     fragmentFeatureDoc      `json:"feature"`
	Problem     fragmentAttributeDoc    `json:"problem"`
	Outcome     fragmentAttributeDoc    `json:"outcome"`
	Targets     []fragmentTargetDoc     `json:"targets"`
	Constraints []fragmentConstraintDoc `json:"constraints"`
	Decisions   []fragmentDecisionDoc   `json:"decisions"`
}

type fragmentFeatureDecodeDoc struct {
	Ref          *string `json:"ref"`
	Path         *string `json:"path"`
	SourceDigest *string `json:"source_digest"`
}

type fragmentAttributeDecodeDoc struct {
	Text   *string `json:"text"`
	Anchor *string `json:"anchor"`
}

type fragmentTargetDecodeDoc struct {
	ID       *string         `json:"id"`
	Text     *string         `json:"text"`
	Evidence json.RawMessage `json:"evidence"`
	Anchor   *string         `json:"anchor"`
}

type fragmentConstraintDecodeDoc struct {
	ID     *string `json:"id"`
	Text   *string `json:"text"`
	Anchor *string `json:"anchor"`
}

type fragmentDecisionDecodeDoc struct {
	ID     *string         `json:"id"`
	Text   *string         `json:"text"`
	Anchor *string         `json:"anchor"`
	Links  json.RawMessage `json:"links"`
}

type fragmentDecodeDoc struct {
	Feature     *fragmentFeatureDecodeDoc      `json:"feature"`
	Problem     *fragmentAttributeDecodeDoc    `json:"problem"`
	Outcome     *fragmentAttributeDecodeDoc    `json:"outcome"`
	Targets     *[]fragmentTargetDecodeDoc     `json:"targets"`
	Constraints *[]fragmentConstraintDecodeDoc `json:"constraints"`
	Decisions   *[]fragmentDecisionDecodeDoc   `json:"decisions"`
}

// EncodeFeatureFragment returns SI-88's private lowercase canonical JSON
// projection. Shared artifact structs are projected field-by-field rather
// than marshaled directly.
func EncodeFeatureFragment(fragment FeatureFragment) ([]byte, error) {
	if err := fragment.validate(); err != nil {
		return nil, err
	}
	doc := fragmentEncodeDoc{
		Feature: fragmentFeatureDoc{
			Ref: fragment.Feature.Ref, Path: fragment.Feature.Path, SourceDigest: fragment.Feature.SourceDigest,
		},
		Problem:     fragmentAttributeDoc{Text: fragment.Problem.Text, Anchor: fragment.Problem.Anchor},
		Outcome:     fragmentAttributeDoc{Text: fragment.Outcome.Text, Anchor: fragment.Outcome.Anchor},
		Targets:     make([]fragmentTargetDoc, len(fragment.Targets)),
		Constraints: make([]fragmentConstraintDoc, len(fragment.Constraints)),
		Decisions:   make([]fragmentDecisionDoc, len(fragment.Decisions)),
	}
	for i, target := range fragment.Targets {
		doc.Targets[i] = fragmentTargetDoc{
			ID: target.ID, Text: target.Text, Evidence: append([]artifact.EvidenceKind(nil), target.Evidence...), Anchor: target.Anchor,
		}
	}
	for i, constraint := range fragment.Constraints {
		doc.Constraints[i] = fragmentConstraintDoc{ID: constraint.ID, Text: constraint.Text, Anchor: constraint.Anchor}
	}
	for i, decision := range fragment.Decisions {
		doc.Decisions[i] = fragmentDecisionDoc{ID: decision.ID, Text: decision.Text, Anchor: decision.Anchor}
		if len(decision.Links) > 0 {
			doc.Decisions[i].Links = make([]fragmentLinkDoc, len(decision.Links))
			for j, link := range decision.Links {
				doc.Decisions[i].Links[j] = fragmentLinkDoc{Type: link.Type, Ref: link.Ref, Note: link.Note}
			}
		}
	}
	data, err := canonjson.Marshal(doc)
	if err != nil {
		// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
		return nil, fmt.Errorf("contextcompile: encode feature fragment: %w", err)
	}
	return data, nil
}

// DecodeFeatureFragment strictly decodes and validates the one canonical
// lowercase fragment projection. Null, unknown, duplicate and noncanonical
// spellings fail closed.
func DecodeFeatureFragment(data []byte) (FeatureFragment, error) {
	var doc fragmentDecodeDoc
	if err := artifact.DecodeExactJSON(data, &doc); err != nil {
		// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
		return FeatureFragment{}, fmt.Errorf("contextcompile: decode feature fragment: %w", err)
	}
	fragment, err := doc.fragment()
	if err != nil {
		return FeatureFragment{}, err
	}
	canonical, err := EncodeFeatureFragment(fragment)
	if err != nil {
		return FeatureFragment{}, err
	}
	if !bytes.Equal(data, canonical) {
		// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
		return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment is not in canonical JSON form")
	}
	return fragment, nil
}

func (doc fragmentDecodeDoc) fragment() (FeatureFragment, error) {
	if doc.Feature == nil || doc.Problem == nil || doc.Outcome == nil || doc.Targets == nil || doc.Constraints == nil || doc.Decisions == nil {
		// vocab:identity — SI-88 schema diagnostic naming fixed feature-fragment fields
		return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment requires non-null feature, problem, outcome, targets, constraints and decisions")
	}
	if doc.Feature.Ref == nil || doc.Feature.Path == nil || doc.Feature.SourceDigest == nil {
		// vocab:identity — SI-88 schema diagnostic naming fixed feature-fragment fields
		return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment feature fields must be present and non-null")
	}
	if doc.Problem.Text == nil || doc.Problem.Anchor == nil || doc.Outcome.Text == nil || doc.Outcome.Anchor == nil {
		// vocab:identity — SI-88 schema diagnostic naming fixed feature-fragment fields
		return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment problem/outcome fields must be present and non-null")
	}
	fragment := FeatureFragment{
		Feature:     FragmentFeature{Ref: *doc.Feature.Ref, Path: *doc.Feature.Path, SourceDigest: *doc.Feature.SourceDigest},
		Problem:     artifact.Attribute{Text: *doc.Problem.Text, Anchor: *doc.Problem.Anchor},
		Outcome:     artifact.Attribute{Text: *doc.Outcome.Text, Anchor: *doc.Outcome.Anchor},
		Targets:     make([]FragmentTarget, len(*doc.Targets)),
		Constraints: make([]artifact.Constraint, len(*doc.Constraints)),
		Decisions:   make([]artifact.Decision, len(*doc.Decisions)),
	}
	for i, target := range *doc.Targets {
		if target.ID == nil || target.Text == nil || target.Anchor == nil {
			// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
			return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment targets[%d] requires non-null id, text and anchor", i)
		}
		var evidence []artifact.EvidenceKind
		if len(target.Evidence) > 0 {
			if bytes.Equal(bytes.TrimSpace(target.Evidence), []byte("null")) {
				// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
				return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment targets[%d].evidence must not be null", i)
			}
			if err := artifact.DecodeExactJSON(target.Evidence, &evidence); err != nil {
				// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
				return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment targets[%d].evidence: %w", i, err)
			}
		}
		fragment.Targets[i] = FragmentTarget{ID: *target.ID, Text: *target.Text, Evidence: evidence, Anchor: *target.Anchor}
	}
	for i, constraint := range *doc.Constraints {
		if constraint.ID == nil || constraint.Text == nil || constraint.Anchor == nil {
			// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
			return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment constraints[%d] fields must be present and non-null", i)
		}
		fragment.Constraints[i] = artifact.Constraint{ID: *constraint.ID, Text: *constraint.Text, Anchor: *constraint.Anchor}
	}
	for i, decision := range *doc.Decisions {
		if decision.ID == nil || decision.Text == nil || decision.Anchor == nil {
			// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
			return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment decisions[%d] fields must be present and non-null", i)
		}
		out := artifact.Decision{ID: *decision.ID, Text: *decision.Text, Anchor: *decision.Anchor}
		if len(decision.Links) > 0 {
			if bytes.Equal(bytes.TrimSpace(decision.Links), []byte("null")) {
				// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
				return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment decisions[%d].links must not be null", i)
			}
			var links []fragmentLinkDoc
			if err := artifact.DecodeExactJSON(decision.Links, &links); err != nil {
				// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
				return FeatureFragment{}, fmt.Errorf("contextcompile: feature fragment decisions[%d].links: %w", i, err)
			}
			out.Links = make([]artifact.Link, len(links))
			for j, link := range links {
				out.Links[j] = artifact.Link{Type: link.Type, Ref: link.Ref, Note: link.Note}
			}
		}
		fragment.Decisions[i] = out
	}
	return fragment, nil
}

func (fragment FeatureFragment) validate() error {
	if err := validateSpecWholeRef("feature.ref", fragment.Feature.Ref); err != nil {
		return err
	}
	parsed, _ := artifact.ParseRef(fragment.Feature.Ref)
	wantActive := ".verdi/specs/active/" + parsed.Name + "/spec.md"
	wantArchive := ".verdi/specs/archive/" + parsed.Name + "/spec.md"
	if fragment.Feature.Path != wantActive && fragment.Feature.Path != wantArchive {
		// vocab:identity — SI-88 schema diagnostic naming fixed feature fields and refs
		return fmt.Errorf("contextcompile: feature.path %q does not match feature ref %q", fragment.Feature.Path, fragment.Feature.Ref)
	}
	if err := validateDigest("feature.source_digest", fragment.Feature.SourceDigest); err != nil {
		return err
	}
	if err := fragment.Problem.Validate(); err != nil {
		return fmt.Errorf("contextcompile: fragment problem: %w", err)
	}
	if err := fragment.Outcome.Validate(); err != nil {
		return fmt.Errorf("contextcompile: fragment outcome: %w", err)
	}
	if len(fragment.Targets) == 0 {
		// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
		return fmt.Errorf("contextcompile: feature fragment targets must be a nonempty array")
	}
	if fragment.Constraints == nil || fragment.Decisions == nil {
		// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
		return fmt.Errorf("contextcompile: feature fragment constraints and decisions must be explicit arrays")
	}
	seenObjects := make(map[string]bool, len(fragment.Targets)+len(fragment.Constraints)+len(fragment.Decisions))
	for i, target := range fragment.Targets {
		if target.Text == "" || target.Anchor == "" {
			return fmt.Errorf("contextcompile: targets[%d] requires text and anchor", i)
		}
		switch {
		case strings.HasPrefix(target.ID, "ac-"):
			if err := validateACID(fmt.Sprintf("targets[%d].id", i), target.ID); err != nil {
				return err
			}
			if len(target.Evidence) == 0 {
				return fmt.Errorf("contextcompile: targets[%d] AC evidence must be nonempty", i)
			}
			for j, kind := range target.Evidence {
				if err := validateEvidenceKind(fmt.Sprintf("targets[%d].evidence[%d]", i, j), kind); err != nil {
					return err
				}
			}
		case strings.HasPrefix(target.ID, "oq-"):
			q := artifact.OpenQuestion{ID: target.ID, Text: target.Text, Anchor: target.Anchor}
			if err := q.Validate(); err != nil {
				return fmt.Errorf("contextcompile: targets[%d]: %w", i, err)
			}
			if target.Evidence != nil {
				return fmt.Errorf("contextcompile: targets[%d] OQ evidence must be omitted", i)
			}
		default:
			return fmt.Errorf("contextcompile: targets[%d].id %q must be ac-* or oq-*", i, target.ID)
		}
		if seenObjects[target.ID] {
			return fmt.Errorf("contextcompile: duplicate fragment object %q", target.ID)
		}
		seenObjects[target.ID] = true
	}
	// vocab:identity — SI-88 schema diagnostic naming the fixed feature-fragment wire identity
	if err := requireSortedUnique("feature fragment targets", fragment.Targets, func(t FragmentTarget) string {
		return fragment.Feature.Ref + "#" + t.ID
	}); err != nil {
		return err
	}
	for i, constraint := range fragment.Constraints {
		if err := constraint.Validate(); err != nil {
			return fmt.Errorf("contextcompile: constraints[%d]: %w", i, err)
		}
		if seenObjects[constraint.ID] {
			return fmt.Errorf("contextcompile: duplicate fragment object %q", constraint.ID)
		}
		seenObjects[constraint.ID] = true
	}
	for i, decision := range fragment.Decisions {
		if err := decision.Validate(); err != nil {
			return fmt.Errorf("contextcompile: decisions[%d]: %w", i, err)
		}
		if seenObjects[decision.ID] {
			return fmt.Errorf("contextcompile: duplicate fragment object %q", decision.ID)
		}
		seenObjects[decision.ID] = true
	}
	return nil
}

// ResolveFeatureFragments resolves every legal story parent edge through the
// exact accepted-spec seam and returns one sorted fragment per parent feature.
func ResolveFeatureFragments(ctx context.Context, git GitReader, states StateResolver, root, head string, target ResolvedSpec) ([]FeatureFragment, error) {
	if target.Spec == nil {
		// vocab:identity — SI-84 compiler diagnostic naming the fixed feature-fragment identity
		return nil, fmt.Errorf("contextcompile: feature fragment target has no decoded spec")
	}
	if target.State != specstate.AcceptedPendingBuild && target.State != specstate.Closed {
		return nil, &AcceptedSpecRefusal{Ref: target.Ref, State: target.State, Relation: specstate.RelationUnproven}
	}
	if err := target.Spec.Validate(); err != nil {
		// vocab:identity — SI-84 compiler diagnostic naming the fixed feature-fragment identity
		return nil, fmt.Errorf("contextcompile: feature fragment target %s: %w", target.Ref, err)
	}
	if target.Spec.ID != target.Ref || target.Spec.Class != artifact.ClassStory {
		// vocab:identity — SI-84 compiler diagnostic naming fixed feature-fragment and story-class identities
		return nil, fmt.Errorf("contextcompile: feature fragments require a matching story target, got %q class %q", target.Ref, target.Spec.Class)
	}
	expectedType := artifact.LinkImplements
	expectedPrefix := "ac-"
	if target.Spec.Spike {
		expectedType = artifact.LinkResolves
		expectedPrefix = "oq-"
	}

	objectsByFeature := make(map[string][]string)
	seen := make(map[string]bool)
	for _, link := range target.Spec.Links {
		if link.Type != expectedType {
			continue
		}
		parsed, err := wholeSpecRef(link.Ref)
		if err != nil {
			return nil, fmt.Errorf("contextcompile: parent link %q: %w", link.Ref, err)
		}
		if !strings.HasPrefix(parsed.Object, expectedPrefix) {
			return nil, fmt.Errorf("contextcompile: %s edge %q targets the wrong object class", expectedType, link.Ref)
		}
		if seen[link.Ref] {
			return nil, fmt.Errorf("contextcompile: duplicate parent fragment %q", link.Ref)
		}
		seen[link.Ref] = true
		featureRef := artifact.Ref{Kind: artifact.KindSpec, Name: parsed.Name}.String()
		objectsByFeature[featureRef] = append(objectsByFeature[featureRef], parsed.Object)
	}
	if len(objectsByFeature) == 0 {
		// vocab:identity — SI-84 compiler diagnostic naming fixed story and parent-feature identities
		return nil, fmt.Errorf("contextcompile: story %s has no legal parent-feature edge", target.Ref)
	}

	featureRefs := make([]string, 0, len(objectsByFeature))
	for ref := range objectsByFeature {
		featureRefs = append(featureRefs, ref)
	}
	sort.Strings(featureRefs)
	fragments := make([]FeatureFragment, 0, len(featureRefs))
	for _, featureRef := range featureRefs {
		parent, err := ResolveAcceptedSpec(ctx, git, states, root, head, featureRef)
		if err != nil {
			return nil, err
		}
		if parent.Spec == nil || parent.Spec.Class != artifact.ClassFeature || parent.Spec.ID != featureRef {
			// vocab:identity — SI-84 compiler diagnostic naming the fixed feature-class identity
			return nil, fmt.Errorf("contextcompile: parent %s is not a matching feature spec", featureRef)
		}
		if parent.Spec.Problem == nil || parent.Spec.Outcome == nil {
			// vocab:identity — SI-84 compiler diagnostic naming the fixed feature-class identity
			return nil, fmt.Errorf("contextcompile: parent feature %s lacks problem/outcome authority", featureRef)
		}
		objectIDs := append([]string(nil), objectsByFeature[featureRef]...)
		sort.Strings(objectIDs)
		fragment := FeatureFragment{
			Feature:     FragmentFeature{Ref: parent.Ref, Path: parent.Path, SourceDigest: parent.ContentDigest},
			Problem:     *parent.Spec.Problem,
			Outcome:     *parent.Spec.Outcome,
			Targets:     make([]FragmentTarget, 0, len(objectIDs)),
			Constraints: cloneConstraints(parent.Spec.Constraints),
			Decisions:   cloneDecisions(parent.Spec.Decisions),
		}
		for _, objectID := range objectIDs {
			projected, ok := projectFeatureTarget(parent.Spec, objectID, target.Spec.Spike)
			if !ok {
				return nil, fmt.Errorf("contextcompile: parent fragment %s#%s is not declared", featureRef, objectID)
			}
			fragment.Targets = append(fragment.Targets, projected)
		}
		if err := fragment.validate(); err != nil {
			return nil, fmt.Errorf("contextcompile: parent fragment %s: %w", featureRef, err)
		}
		fragments = append(fragments, fragment)
	}
	return fragments, nil
}

func projectFeatureTarget(spec *artifact.SpecFrontmatter, objectID string, spike bool) (FragmentTarget, bool) {
	if spike {
		for _, question := range spec.OpenQuestions {
			if question.ID == objectID {
				return FragmentTarget{ID: question.ID, Text: question.Text, Anchor: question.Anchor}, true
			}
		}
		return FragmentTarget{}, false
	}
	for _, criterion := range spec.AcceptanceCriteria {
		if criterion.ID == objectID {
			return FragmentTarget{
				ID: criterion.ID, Text: criterion.Text,
				Evidence: append([]artifact.EvidenceKind(nil), criterion.Evidence...), Anchor: criterion.Anchor,
			}, true
		}
	}
	return FragmentTarget{}, false
}

func cloneConstraints(in []artifact.Constraint) []artifact.Constraint {
	return append([]artifact.Constraint{}, in...)
}

func cloneDecisions(in []artifact.Decision) []artifact.Decision {
	out := make([]artifact.Decision, len(in))
	for i, decision := range in {
		out[i] = decision
		out[i].Links = append([]artifact.Link(nil), decision.Links...)
	}
	return out
}
