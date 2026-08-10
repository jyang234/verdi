package artifact

import (
	"fmt"
	"strings"
)

// ObligationFrontmatter is the frontmatter schema for kind "obligation"
// (spec/obligation-artifact DC-1): a first-class evidence-obligation
// artifact, decoded through internal/artifact exactly like an attestation —
// markdown frontmatter + prose body, no `schema:` line (that is for JSON
// artifacts), frozen unconditionally ("existence is the record", mirroring
// AttestationFrontmatter's own posture — see attestation.go). ForKind is
// the one EvidenceKind this obligation states what that evidence must
// specifically show for.
//
// The obligation's single `verifies` edge (carried in the embedded Base's
// Links, per DC-1: "reuse the existing link types" rather than inventing a
// new frontmatter key) names the WHOLE STORY SPEC it backs — a bare
// spec/<story-name> ref with no object fragment — exactly as an
// attestation's own `verifies` edge does (attestation.go; e.g.
// `ref: spec/stale-decline`). The acceptance criterion itself is NOT carried
// on that edge: like an attestation, the obligation encodes its AC in its
// own id and on-disk path (obligation/<story-slug>--<ac-id>--<for-kind>, at
// .verdi/obligations/<story-ref-slug>/<ac-id>--<for-kind>.md), so a fragment
// on the verifies ref would be redundant — and is rejected, since 02 §Link
// taxonomy's closed spec-object edge vocabulary already forbids a `verifies`
// edge from targeting a fragment (the base Link.Validate, common.go).
//
// Whether that whole-spec target is actually a STORY (as opposed to a
// feature) that genuinely declares the id's own <ac-id> as one of its
// acceptance criteria needs the corpus/index to resolve, which a bare
// frontmatter decode cannot see. VL-019 (internal/lint) owns that
// classification (spec/obligation-artifact AC-2/DC-3). This type's own
// Validate is deliberately narrower: it confirms exactly one verifies link
// is present and its ref is well-formed (a whole-spec ref, per the base
// link vocabulary), nothing about what class of thing it resolves to.
type ObligationFrontmatter struct {
	Base    `yaml:",inline"`
	ForKind EvidenceKind `yaml:"for_kind"`
	// Quality is optional only for compatibility with obligations frozen
	// before the obligation-quality adoption commit. A present block is a
	// strict unresolved/elaborated union; absence is legacy-unelaborated and
	// is interpreted prospectively by internal/evidence.
	Quality *ObligationQuality `yaml:"quality,omitempty"`
}

// ObligationQualityState is the closed discriminator for an obligation's
// quality declaration. Elaboration makes a declaration eligible for exact
// evidence matching; it never proves the declaration by shape alone.
type ObligationQualityState string

const (
	ObligationQualityUnresolved ObligationQualityState = "unresolved-design-debt"
	ObligationQualityElaborated ObligationQualityState = "elaborated"
)

// ObligationProducerKind identifies the kind of producer an elaborated
// obligation requires.
type ObligationProducerKind string

const (
	ObligationProducerTest               ObligationProducerKind = "test"
	ObligationProducerChecker            ObligationProducerKind = "checker"
	ObligationProducerAuthenticatedHuman ObligationProducerKind = "authenticated-human"
)

// ObligationSourceKind identifies the authoritative source required by an
// elaborated obligation.
type ObligationSourceKind string

const (
	ObligationSourceCIJob               ObligationSourceKind = "ci-job"
	ObligationSourceGovernedAttestation ObligationSourceKind = "governed-attestation"
)

// ObligationInvalidator is a freshness dimension whose change invalidates an
// elaborated obligation's evidence.
type ObligationInvalidator string

const (
	ObligationInvalidatorSpec        ObligationInvalidator = "spec"
	ObligationInvalidatorCode        ObligationInvalidator = "code"
	ObligationInvalidatorDependency  ObligationInvalidator = "dependency"
	ObligationInvalidatorEnvironment ObligationInvalidator = "environment"
	ObligationInvalidatorPolicy      ObligationInvalidator = "policy"
)

var validObligationProducerKinds = map[ObligationProducerKind]bool{
	ObligationProducerTest: true, ObligationProducerChecker: true,
	ObligationProducerAuthenticatedHuman: true,
}

var validObligationSourceKinds = map[ObligationSourceKind]bool{
	ObligationSourceCIJob: true, ObligationSourceGovernedAttestation: true,
}

var validObligationInvalidators = map[ObligationInvalidator]bool{
	ObligationInvalidatorSpec: true, ObligationInvalidatorCode: true,
	ObligationInvalidatorDependency: true, ObligationInvalidatorEnvironment: true,
	ObligationInvalidatorPolicy: true,
}

// ObligationProducer is the exact producer identity required by an elaborated
// obligation.
type ObligationProducer struct {
	Kind ObligationProducerKind `yaml:"kind"`
	Ref  string                 `yaml:"ref"`
}

// ObligationAuthoritativeSource is the exact authoritative source identity
// required by an elaborated obligation.
type ObligationAuthoritativeSource struct {
	Kind ObligationSourceKind `yaml:"kind"`
	Ref  string               `yaml:"ref"`
}

// ObligationFreshness declares the closed freshness dimensions and their
// human-readable rule. InvalidatedBy preserves declaration order.
type ObligationFreshness struct {
	InvalidatedBy []ObligationInvalidator `yaml:"invalidated_by"`
	Rule          string                  `yaml:"rule"`
}

// ObligationQuality is a strict unresolved/elaborated union.
type ObligationQuality struct {
	State               ObligationQualityState        `yaml:"state"`
	Claim               string                        `yaml:"claim,omitempty"`
	Falsifier           string                        `yaml:"falsifier,omitempty"`
	Scope               string                        `yaml:"scope,omitempty"`
	Producer            ObligationProducer            `yaml:"producer,omitempty"`
	AuthoritativeSource ObligationAuthoritativeSource `yaml:"authoritative_source,omitempty"`
	Freshness           ObligationFreshness           `yaml:"freshness,omitempty"`
}

// DecodeObligation strict-decodes and validates obligation frontmatter.
func DecodeObligation(data []byte) (*ObligationFrontmatter, error) {
	var fm ObligationFrontmatter
	if err := DecodeStrict(data, &fm); err != nil {
		return nil, err
	}
	if err := fm.Validate(); err != nil {
		return nil, err
	}
	return &fm, nil
}

// Validate checks the common fields (including, via validateBase, that the
// id parses as an obligation ref whose compound name matches
// obligationNameRe's <story-slug>--<ac-id>--<for-kind> shape — a malformed
// id fails here, and every link's ref shape is checked by the base
// Link.Validate(), so a `verifies` edge carrying an object fragment is
// rejected there — obligations verify a WHOLE story spec, never a
// fragment), that ForKind is a known evidence kind and agrees with the id's
// own <for-kind> segment (DC-2's id/for_kind agreement), and that exactly
// one `verifies` link is present (a missing verifies link, or one of any
// other type, is rejected). Frozen is required unconditionally (DC-1:
// "existence is the record", attestation's own posture) — see requireFrozen.
//
// Path/id agreement (DC-2's other half: on-disk home
// .verdi/obligations/<story-ref-slug>/<ac-id>--<for-kind>.md) is NOT
// checked here: DecodeObligation, like DecodeAttestation, takes only raw
// frontmatter bytes and has no path to compare against. That half is
// internal/lint's VL-011 (extended for the obligation kind alongside its
// existing attestation/waiver/reaffirmation coverage), which walks the
// committed zone and does have both the id and the file's real path.
//
// Similarly NOT checked here: whether the whole-spec verifies target is a
// STORY (vs a feature or an unresolvable spec) that genuinely declares the
// id's own <ac-id> as one of its acceptance criteria — that classification
// needs the corpus/index and is VL-019's job (spec/obligation-artifact
// AC-2).
func (fm ObligationFrontmatter) Validate() error {
	if err := fm.validateBase(KindObligation); err != nil {
		return err
	}

	if !validEvidenceKinds[fm.ForKind] {
		return fmt.Errorf("artifact: obligation for_kind %q is not a known evidence kind", fm.ForKind)
	}

	ref, err := ParseRef(fm.ID)
	if err != nil {
		// Unreachable in practice: validateBase already parsed and
		// validated fm.ID above. Fail closed rather than panic below if
		// that invariant is ever broken by a future refactor.
		return fmt.Errorf("artifact: id: %w", err)
	}
	_, _, idForKind, ok := SplitObligationName(ref.Name)
	if !ok {
		// Unreachable in practice: validateBase's Ref.Validate call already
		// enforced obligationNameRe's three-segment shape for KindObligation
		// above. Guarded rather than assumed for the same reason.
		return fmt.Errorf("artifact: obligation id %q does not split into <story-slug>--<ac-id>--<for-kind>", fm.ID)
	}
	if idForKind != string(fm.ForKind) {
		return fmt.Errorf("artifact: obligation id %q names for_kind segment %q, but frontmatter for_kind is %q (spec/obligation-artifact DC-2: id/for_kind agreement)", fm.ID, idForKind, fm.ForKind)
	}

	if len(fm.Links) != 1 || fm.Links[0].Type != LinkVerifies {
		return fmt.Errorf("artifact: obligation must carry exactly one links entry of type verifies, got %d", len(fm.Links))
	}
	if fm.Quality != nil {
		if err := fm.Quality.validate(fm.ForKind); err != nil {
			return err
		}
	}

	return requireFrozen(fm.Frozen, true, "obligation", "")
}

func (q ObligationQuality) validate(forKind EvidenceKind) error {
	switch q.State {
	case ObligationQualityUnresolved:
		if q.Claim != "" || q.Falsifier != "" || q.Scope != "" ||
			q.Producer != (ObligationProducer{}) ||
			q.AuthoritativeSource != (ObligationAuthoritativeSource{}) ||
			len(q.Freshness.InvalidatedBy) != 0 || q.Freshness.Rule != "" {
			return fmt.Errorf("artifact: unresolved obligation quality permits only state")
		}
		return nil
	case ObligationQualityElaborated:
		// Continue below.
	default:
		return fmt.Errorf("artifact: obligation quality state %q is not known", q.State)
	}

	for name, value := range map[string]string{
		"claim": q.Claim, "falsifier": q.Falsifier, "scope": q.Scope,
		"producer.ref":             q.Producer.Ref,
		"authoritative_source.ref": q.AuthoritativeSource.Ref,
		"freshness.rule":           q.Freshness.Rule,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("artifact: elaborated obligation quality %s must be nonblank", name)
		}
		if strings.TrimSpace(value) != value {
			return fmt.Errorf("artifact: elaborated obligation quality %s must be normalized", name)
		}
	}
	if !validObligationProducerKinds[q.Producer.Kind] {
		return fmt.Errorf("artifact: obligation quality producer kind %q is not known", q.Producer.Kind)
	}
	if !validObligationSourceKinds[q.AuthoritativeSource.Kind] {
		return fmt.Errorf("artifact: obligation quality authoritative source kind %q is not known", q.AuthoritativeSource.Kind)
	}
	if len(q.Freshness.InvalidatedBy) == 0 {
		return fmt.Errorf("artifact: elaborated obligation quality freshness.invalidated_by must not be empty")
	}
	seen := make(map[ObligationInvalidator]bool, len(q.Freshness.InvalidatedBy))
	for _, invalidator := range q.Freshness.InvalidatedBy {
		if !validObligationInvalidators[invalidator] {
			return fmt.Errorf("artifact: obligation quality freshness invalidator %q is not known", invalidator)
		}
		if seen[invalidator] {
			return fmt.Errorf("artifact: obligation quality freshness invalidator %q is duplicated", invalidator)
		}
		seen[invalidator] = true
	}

	if forKind == EvidenceAttestation {
		if q.Producer.Kind != ObligationProducerAuthenticatedHuman || q.AuthoritativeSource.Kind != ObligationSourceGovernedAttestation {
			return fmt.Errorf("artifact: attestation obligation quality requires authenticated-human producer and governed-attestation source")
		}
		return nil
	}
	if q.Producer.Kind != ObligationProducerTest && q.Producer.Kind != ObligationProducerChecker {
		return fmt.Errorf("artifact: %s obligation quality requires test or checker producer", forKind)
	}
	if q.AuthoritativeSource.Kind != ObligationSourceCIJob {
		return fmt.Errorf("artifact: %s obligation quality requires ci-job source", forKind)
	}
	return nil
}

// SplitObligationName splits an obligation ref's compound name
// "<story-slug>--<ac-id>--<for-kind>" into its three parts
// (spec/obligation-artifact DC-2: id and on-disk path are two views of the
// same (story, ac, for-kind) triple). ok is false when name does not have
// exactly three "--"-joined segments. Exported so internal/lint's VL-011
// (path/id agreement) shares this exact parse rather than re-deriving it —
// callers that already know name comes from a successfully-Validate()'d Ref
// (obligationNameRe already enforced the shape at decode time) use ok only
// as a defensive guard, mirroring VL-011's own "shape already enforced at
// decode" posture for attestation/waiver/reaffirmation's two-segment split.
func SplitObligationName(name string) (storySlug, acID, forKind string, ok bool) {
	parts := strings.Split(name, "--")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}
