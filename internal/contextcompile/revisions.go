package contextcompile

import (
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// authorityRevisionDecision is one decision operand admitted into the §9
// authority-revision preimage: a decision's identity and its already-
// computed content digest. This package does not define how a decision's
// own digest is computed (that is the caller's concern, e.g. the eventual
// manifest.DecisionRef.ContentDigest); authorityRevision only binds the
// (ref, digest) pair it is handed.
type authorityRevisionDecision struct {
	Ref    string
	Digest string
}

// authorityRevisionObligation is one obligation operand admitted into the
// §9 authority-revision preimage: an obligation's identity and its
// already-computed exact-byte content digest.
type authorityRevisionObligation struct {
	Ref    string
	Digest string
}

// authorityRevisionInput is the complete, closed set of operands the §9
// authority revision binds (authority design §9: "contains only ..."). Its
// fields are an exhaustive transcription of that list:
//
//  1. EffectivePolicyDigest — the effective-policy digest;
//  2. AcceptedSpec — the accepted spec's ref, merge-signaled baseline
//     identity (path, blob, commit), and exact-byte content digest;
//  3. ParentFragments — every parent feature fragment, bound by identity
//     plus the raw exact-byte digest of its EncodeFeatureFragment output;
//  4. Decisions — every decision, bound by identity and digest;
//  5. Obligations — every obligation, bound by identity and exact-byte
//     digest.
//
// Exclusion by construction: §9 also says repository state, grants, actor
// posture, payload classification, projection files and opaque disclosures
// are bound by the manifest self digest, never folded into authority
// identity. This type gives none of those a field to be supplied through —
// there is no repository, grants, actors, classification, projection, or
// disclosure field here, on any nested type, at all. A caller cannot leak
// one of those operands into the authority revision by mistake; the type
// itself makes it unrepresentable.
type authorityRevisionInput struct {
	EffectivePolicyDigest string
	AcceptedSpec          AcceptedSpec
	ParentFragments       []FeatureFragment
	Decisions             []authorityRevisionDecision
	Obligations           []authorityRevisionObligation
}

// authorityRevisionFragmentOperand, authorityRevisionDecisionOperand and
// authorityRevisionObligationOperand are the canonical wire shapes for the
// three sorted lists inside authorityRevisionPreimage. They carry json tags
// because they are digested via canonjson, never decoded.
type authorityRevisionFragmentOperand struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type authorityRevisionDecisionOperand struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

type authorityRevisionObligationOperand struct {
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

// authorityRevisionPreimage is the one private canonical preimage authority
// design §9 describes. authorityRevision's digest is canonjson.Digest of
// exactly this value — nothing more, nothing less.
type authorityRevisionPreimage struct {
	EffectivePolicyDigest string                               `json:"effective_policy_digest"`
	AcceptedSpec          AcceptedSpec                         `json:"accepted_spec"`
	ParentFragments       []authorityRevisionFragmentOperand   `json:"parent_fragments"`
	Decisions             []authorityRevisionDecisionOperand   `json:"decisions"`
	Obligations           []authorityRevisionObligationOperand `json:"obligations"`
}

// authorityRevision computes the §9 authority revision digest from in. It
// validates every required digest and identity, sorts the three set-like
// lists on copies (in's slices are never mutated), rejects duplicate
// identities within each list, and is otherwise a pure function of in: the
// same in — in any input order — always yields the same digest.
func authorityRevision(in authorityRevisionInput) (string, error) {
	if err := validateDigest("authority revision.effective_policy_digest", in.EffectivePolicyDigest); err != nil {
		return "", err
	}
	if err := in.AcceptedSpec.validate(); err != nil {
		return "", err
	}
	if in.ParentFragments == nil {
		return "", fmt.Errorf("contextcompile: authority revision.parent_fragments: must be non-nil (an explicitly empty set is [])")
	}
	if in.Decisions == nil {
		return "", fmt.Errorf("contextcompile: authority revision.decisions: must be non-nil (an explicitly empty set is [])")
	}
	if in.Obligations == nil {
		return "", fmt.Errorf("contextcompile: authority revision.obligations: must be non-nil (an explicitly empty set is [])")
	}

	fragmentOperands := make([]authorityRevisionFragmentOperand, len(in.ParentFragments))
	for i, fragment := range in.ParentFragments {
		encoded, err := EncodeFeatureFragment(fragment)
		if err != nil {
			return "", fmt.Errorf("contextcompile: authority revision.parent_fragments[%d]: %w", i, err)
		}
		fragmentOperands[i] = authorityRevisionFragmentOperand{
			Ref:    fragment.Feature.Ref,
			Digest: rawContentDigest(encoded),
		}
	}
	sortedFragments, err := sortedUniqueByRef(fragmentOperands, func(o authorityRevisionFragmentOperand) string { return o.Ref },
		"authority revision.parent_fragments")
	if err != nil {
		return "", err
	}

	decisionOperands := make([]authorityRevisionDecisionOperand, len(in.Decisions))
	for i, d := range in.Decisions {
		if err := validateNonEmpty(fmt.Sprintf("authority revision.decisions[%d].ref", i), d.Ref); err != nil {
			return "", err
		}
		if err := validateDigest(fmt.Sprintf("authority revision.decisions[%d].digest", i), d.Digest); err != nil {
			return "", err
		}
		decisionOperands[i] = authorityRevisionDecisionOperand{Ref: d.Ref, Digest: d.Digest}
	}
	sortedDecisions, err := sortedUniqueByRef(decisionOperands, func(o authorityRevisionDecisionOperand) string { return o.Ref },
		"authority revision.decisions")
	if err != nil {
		return "", err
	}

	obligationOperands := make([]authorityRevisionObligationOperand, len(in.Obligations))
	for i, o := range in.Obligations {
		if err := validateNonEmpty(fmt.Sprintf("authority revision.obligations[%d].ref", i), o.Ref); err != nil {
			return "", err
		}
		if err := validateDigest(fmt.Sprintf("authority revision.obligations[%d].digest", i), o.Digest); err != nil {
			return "", err
		}
		obligationOperands[i] = authorityRevisionObligationOperand{Ref: o.Ref, Digest: o.Digest}
	}
	sortedObligations, err := sortedUniqueByRef(obligationOperands, func(o authorityRevisionObligationOperand) string { return o.Ref },
		"authority revision.obligations")
	if err != nil {
		return "", err
	}

	preimage := authorityRevisionPreimage{
		EffectivePolicyDigest: in.EffectivePolicyDigest,
		AcceptedSpec:          in.AcceptedSpec,
		ParentFragments:       sortedFragments,
		Decisions:             sortedDecisions,
		Obligations:           sortedObligations,
	}
	digest, err := canonjson.Digest(preimage)
	if err != nil {
		return "", fmt.Errorf("contextcompile: authority revision: digest preimage: %w", err)
	}
	return digest, nil
}

// sortedUniqueByRef returns a freshly sorted copy of items (ascending by
// ref) with no duplicate ref, or an error naming field and the first
// duplicate. It never mutates items: the returned slice has its own
// backing array.
func sortedUniqueByRef[T any](items []T, ref func(T) string, field string) ([]T, error) {
	// Non-nil dst: an explicit empty operand set (items non-nil, len 0 —
	// authorityRevision's own callers already require non-nil here) must
	// stay a non-nil "[]" through canonjson.Marshal, never collapse to
	// "null" the way append([]T(nil), items...) would when items is empty.
	out := append([]T{}, items...)
	sort.Slice(out, func(i, j int) bool { return ref(out[i]) < ref(out[j]) })
	for i := 1; i < len(out); i++ {
		if ref(out[i]) == ref(out[i-1]) {
			return nil, fmt.Errorf("contextcompile: %s: duplicate identity %q", field, ref(out[i]))
		}
	}
	return out, nil
}

// contextRevisions returns the manifest's `revisions` section for a freshly
// computed compile. authority must already be a valid "sha256:"+64-lowercase-
// hex digest (authorityRevision's return value is always exactly that
// shape); contextRevisions still validates it so a malformed caller value
// fails closed here rather than silently reaching the manifest. Context is
// always 1 and Revisions has no field for a parent: v1's root context
// revision is always 1 with no parent (authority design §9), so — as
// Revisions' own doc comment says — the domain type simply carries no
// field a parent could occupy.
func contextRevisions(authority string) (Revisions, error) {
	if err := validateDigest("authority revision", authority); err != nil {
		return Revisions{}, err
	}
	return Revisions{Authority: authority, Context: 1}, nil
}

// collectManifestDecisions returns the manifest's `decisions` rows (authority
// design §8.2: "every governing decision already carried in accepted-spec
// or parent-fragment payloads"): the accepted target's own Decisions plus
// each governing parent-feature fragment's Decisions, sorted by ref with
// duplicate refs rejected. Each row's ref is "<owning-spec-ref>#<decision-
// id>" and its content digest is over the complete normalized
// {id,text,anchor,links} decision object — the identical canonical shape
// EncodeFeatureFragment already uses for a fragment's own decisions
// (fragmentDecisionDoc/fragmentLinkDoc, defined in fragments.go), reused
// here rather than a second decision encoder so the two encodings can
// never drift apart.
func collectManifestDecisions(targetRef string, targetDecisions []artifact.Decision, fragments []FeatureFragment) ([]DecisionRef, error) {
	type decisionRow struct {
		ref      string
		decision artifact.Decision
	}
	rows := make([]decisionRow, 0, len(targetDecisions))
	for _, d := range targetDecisions {
		rows = append(rows, decisionRow{ref: targetRef + "#" + d.ID, decision: d})
	}
	for _, f := range fragments {
		for _, d := range f.Decisions {
			rows = append(rows, decisionRow{ref: f.Feature.Ref + "#" + d.ID, decision: d})
		}
	}

	seen := make(map[string]bool, len(rows))
	out := make([]DecisionRef, 0, len(rows))
	for _, r := range rows {
		if seen[r.ref] {
			return nil, fmt.Errorf("contextcompile: manifest decisions: duplicate decision ref %q", r.ref)
		}
		seen[r.ref] = true
		digest, err := canonicalDecisionDigest(r.decision)
		if err != nil {
			return nil, fmt.Errorf("contextcompile: manifest decisions: %s: %w", r.ref, err)
		}
		out = append(out, DecisionRef{Ref: r.ref, ContentDigest: digest})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out, nil
}

// canonicalDecisionDigest is the complete normalized {id,text,anchor,links}
// decision object's canonical-JSON digest, computed through the same
// private fragmentDecisionDoc/fragmentLinkDoc wire shapes
// EncodeFeatureFragment already uses (fragments.go).
func canonicalDecisionDigest(d artifact.Decision) (string, error) {
	doc := fragmentDecisionDoc{ID: d.ID, Text: d.Text, Anchor: d.Anchor}
	if len(d.Links) > 0 {
		doc.Links = make([]fragmentLinkDoc, len(d.Links))
		for i, l := range d.Links {
			doc.Links[i] = fragmentLinkDoc{Type: l.Type, Ref: l.Ref, Note: l.Note}
		}
	}
	digest, err := canonjson.Digest(doc)
	if err != nil {
		return "", fmt.Errorf("contextcompile: canonical decision digest: %w", err)
	}
	return digest, nil
}
