package contextcompile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// digestRe matches the shared "sha256:" + 64 lowercase hex digest form
// (authority design §9; mirrors internal/journey's own digestRe).
var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// gitHashRe matches a short-or-full lowercase hex git object hash, the
// same 7-40 hex grammar internal/artifact's own commitRe uses for a spec
// ref's pinned commit — accepted_spec.blob/.commit and expected.head are
// the same kind of value.
var gitHashRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

func validateDigest(field, value string) error {
	if !digestRe.MatchString(value) {
		return fmt.Errorf("contextcompile: %s: %q is not a valid sha256:<64 lowercase hex> digest", field, value)
	}
	return nil
}

func validateOptionalDigest(field string, value *string) error {
	if value == nil {
		return nil
	}
	if *value == "" {
		return fmt.Errorf("contextcompile: %s: present but empty (omit rather than send an empty string)", field)
	}
	return validateDigest(field, *value)
}

func validateGitHash(field, value string) error {
	if !gitHashRe.MatchString(value) {
		return fmt.Errorf("contextcompile: %s: %q is not a valid short-or-full lowercase hex git hash", field, value)
	}
	return nil
}

func validateNonEmpty(field, value string) error {
	if value == "" {
		return fmt.Errorf("contextcompile: %s: must be non-empty", field)
	}
	return nil
}

func validateArtifactRef(field, ref string) error {
	if _, err := artifact.ParseRef(ref); err != nil {
		return fmt.Errorf("contextcompile: %s: invalid artifact ref: %w", field, err)
	}
	return nil
}

func validatePolicyArtifactRef(field, ref string) error {
	kind, name, ok := strings.Cut(ref, "/")
	if !ok || name == "" || strings.Contains(name, "/") {
		return fmt.Errorf("contextcompile: %s: invalid policy artifact ref %q", field, ref)
	}

	var rel string
	switch kind {
	case policyartifact.KindPolicy:
		rel = policyartifact.DirPolicies + "/" + name + ".md"
	case policyartifact.KindOverlay:
		rel = policyartifact.DirOverlays + "/" + name + ".md"
	case policyartifact.KindExemption:
		rel = policyartifact.DirExemptions + "/" + name + ".md"
	case policyartifact.KindConstitution:
		if name != policyartifact.ConstitutionName {
			return fmt.Errorf("contextcompile: %s: invalid policy artifact ref %q", field, ref)
		}
		rel = policyartifact.ConstitutionName + ".md"
	case policyartifact.KindProfileStorage:
		rel = policyartifact.DirProfiles + "/" + name + ".md"
	default:
		return fmt.Errorf("contextcompile: %s: invalid policy artifact ref %q: unknown policy kind %q", field, ref, kind)
	}
	parsedKind, parsedName, err := policyartifact.ClassifyPolicyPath(rel)
	if err != nil {
		return fmt.Errorf("contextcompile: %s: invalid policy artifact ref %q: %w", field, ref, err)
	}
	if parsedKind != kind || parsedName != name {
		return fmt.Errorf("contextcompile: %s: invalid policy artifact ref %q", field, ref)
	}
	return nil
}

// validateSpecWholeRef checks that ref is an exact, unpinned, non-fragment
// "spec/<name>" whole ref (authority design §3: "spec is one unpinned
// whole spec/<name> ref").
func validateSpecWholeRef(field, ref string) error {
	parsed, err := artifact.ParseRef(ref)
	if err != nil {
		return fmt.Errorf("contextcompile: %s: %w", field, err)
	}
	if parsed.Kind != artifact.KindSpec {
		return fmt.Errorf("contextcompile: %s: %q must be a spec/<name> ref, got kind %q", field, ref, parsed.Kind)
	}
	if parsed.Pinned() {
		return fmt.Errorf("contextcompile: %s: %q must be unpinned (no @commit)", field, ref)
	}
	if parsed.Fragment() {
		return fmt.Errorf("contextcompile: %s: %q must name the whole spec (no #object-id)", field, ref)
	}
	return nil
}

// validateDecisionFragmentRef checks that ref is a "spec/<name>#dc-<slug>"
// fragment ref naming a decision object.
func validateDecisionFragmentRef(field, ref string) error {
	parsed, err := artifact.ParseRef(ref)
	if err != nil {
		return fmt.Errorf("contextcompile: %s: %w", field, err)
	}
	if parsed.Kind != artifact.KindSpec {
		return fmt.Errorf("contextcompile: %s: %q must be a spec/<name>#<object-id> ref, got kind %q", field, ref, parsed.Kind)
	}
	if !parsed.Fragment() {
		return fmt.Errorf("contextcompile: %s: %q must carry a #<decision-id> fragment", field, ref)
	}
	return nil
}

// validateObligationRef checks that ref is an "obligation/<story-slug>--<ac-id>--<for-kind>" ref.
func validateObligationRef(field, ref string) error {
	parsed, err := artifact.ParseRef(ref)
	if err != nil {
		return fmt.Errorf("contextcompile: %s: %w", field, err)
	}
	if parsed.Kind != artifact.KindObligation {
		return fmt.Errorf("contextcompile: %s: %q must be an obligation/<...> ref, got kind %q", field, ref, parsed.Kind)
	}
	return nil
}

var acIDRe = regexp.MustCompile(`^ac-[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validateACID(field, id string) error {
	if !acIDRe.MatchString(id) {
		return fmt.Errorf("contextcompile: %s: %q must look like ac-<slug>", field, id)
	}
	return nil
}

func validateEvidenceKind(field string, k artifact.EvidenceKind) error {
	switch k {
	case artifact.EvidenceStatic, artifact.EvidenceBehavioral, artifact.EvidenceRuntime, artifact.EvidenceAttestation:
		return nil
	}
	return fmt.Errorf("contextcompile: %s: unknown evidence kind %q", field, string(k))
}

// requireSortedUnique validates that items is already in strict ascending
// order by key with no duplicate keys (CLAUDE.md/plan: "Set-like slices
// sorted with duplicates rejected"). It reports the first violation.
func requireSortedUnique[T any](field string, items []T, key func(T) string) error {
	for i := 1; i < len(items); i++ {
		prev, cur := key(items[i-1]), key(items[i])
		switch {
		case cur == prev:
			return fmt.Errorf("contextcompile: %s: duplicate identity %q", field, cur)
		case cur < prev:
			return fmt.Errorf("contextcompile: %s: must be sorted ascending (found %q after %q)", field, cur, prev)
		}
	}
	return nil
}

func requireSortedUniqueStrings(field string, ss []string) error {
	return requireSortedUnique(field, ss, func(s string) string { return s })
}

func requireSortedUniqueDisclosures(field string, ds []DisclosureCode) error {
	return requireSortedUnique(field, ds, func(d DisclosureCode) string { return string(d) })
}

// --- enum closure checks ----------------------------------------------------

func (p Phase) Validate() error {
	switch p {
	case PhaseDesign, PhaseBuild, PhaseReview:
		return nil
	}
	return fmt.Errorf("contextcompile: unknown phase %q", string(p))
}

func (s Source) Validate() error {
	switch s {
	case SourceHeadTree, SourceWorktreeOverlay, SourceStoreAuthority, SourceDeclaredContext, SourceProjection, SourceOpaque:
		return nil
	}
	return fmt.Errorf("contextcompile: unknown source %q", string(s))
}

func (k IncludedKind) Validate() error {
	switch k {
	case IncludedAcceptedSpec, IncludedParentFeatureFragment, IncludedObligation, IncludedPolicyArtifact,
		IncludedRepositoryFile, IncludedDeclaredContextRef, IncludedInstructionProjection:
		return nil
	}
	return fmt.Errorf("contextcompile: unknown included kind %q", string(k))
}

func (r ExclusionReason) Validate() error {
	switch r {
	case ExclusionDesignProvenanceSidecar, ExclusionDataZoneDisposable, ExclusionUncommittedContent,
		ExclusionOutOfDeclaredScope, ExclusionPhaseInapplicable, ExclusionSupersededSpec,
		ExclusionArchivedRecord, ExclusionGeneratedProjectionOutput, ExclusionNonTextData, ExclusionNonRegularFile:
		return nil
	}
	return fmt.Errorf("contextcompile: unknown exclusion reason %q", string(r))
}

func (a Applicability) Validate() error {
	switch a {
	case ApplicabilityApplicable, ApplicabilityInapplicable, ApplicabilityUnknown:
		return nil
	}
	return fmt.Errorf("contextcompile: unknown applicability %q", string(a))
}

func (r Resolution) Validate() error {
	switch r {
	case ResolutionProven, ResolutionViolatedWithWitness, ResolutionUnproven:
		return nil
	}
	return fmt.Errorf("contextcompile: unknown resolution %q", string(r))
}

func (c PayloadChannel) Validate() error {
	switch c {
	case ChannelData, ChannelAuthority:
		return nil
	}
	return fmt.Errorf("contextcompile: unknown payload channel %q", string(c))
}

func (d DisclosureCode) Validate() error {
	switch d {
	case DisclosureActorResolutionUnproven, DisclosureRepositoryRemoteUnknown, DisclosureRepositoryBranchUnknown,
		DisclosureRepositoryHeadUnknown, DisclosureDefaultBranchUnknown, DisclosureDefaultRelationshipUnknown,
		DisclosureDirtyStateUnknown, DisclosureStagedStateUnknown, DisclosureFreshnessUnknown,
		DisclosureApplicabilityUnknown, DisclosureReviewResultDiffUnproven, DisclosureReviewEvidenceBundleUnproven,
		DisclosureReviewBuilderReceiptUnproven, DisclosureOpaqueHarnessVendorBase:
		return nil
	}
	return fmt.Errorf("contextcompile: unknown disclosure code %q", string(d))
}

func validateDisclosures(field string, ds []DisclosureCode) error {
	if ds == nil {
		return fmt.Errorf("contextcompile: %s: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, d := range ds {
		if err := d.Validate(); err != nil {
			return fmt.Errorf("contextcompile: %s[%d]: %w", field, i, err)
		}
	}
	return requireSortedUniqueDisclosures(field, ds)
}

var validPolicyEntryKinds = map[string]bool{PolicyEntryPolicy: true, PolicyEntryOverlay: true, PolicyEntryExemption: true}

func validatePolicyEntryKind(field, kind string) error {
	if !validPolicyEntryKinds[kind] {
		return fmt.Errorf("contextcompile: %s: unknown policy entry kind %q", field, kind)
	}
	return nil
}

var validRequiredInputKinds = map[string]bool{
	RequiredInputAcceptedSpec: true, RequiredInputResultDiff: true, RequiredInputEvidenceBundle: true,
	RequiredInputBuilderReceipt: true, RequiredInputReviewPolicy: true,
}

func validateRequiredInputKind(field, kind string) error {
	if !validRequiredInputKinds[kind] {
		return fmt.Errorf("contextcompile: %s: unknown required-input kind %q", field, kind)
	}
	return nil
}

var validEvidenceFreshness = map[string]bool{EvidenceFreshnessFresh: true, EvidenceFreshnessStale: true, EvidenceFreshnessUnknown: true}

var validRelationships = map[string]bool{
	RelationshipEqual: true, RelationshipAhead: true, RelationshipBehind: true, RelationshipDiverged: true, RelationshipUnknown: true,
}

var validRepoSources = map[string]bool{
	RepoSourceHead: true, RepoSourceWorkingTree: true, RepoSourceRemoteRef: true, RepoSourceReceiptBound: true,
}

// --- PhaseScopeRefusal -------------------------------------------------------

// PhaseScopeRefusal is the typed exit-1 refusal for a canonical,
// schema-valid request whose phase does not occur in a nonempty
// scope.phases (authority design §3, §10). It is not a malformed-input
// error: callers map it to exit 1 via errors.As, never message matching.
type PhaseScopeRefusal struct {
	Phase       Phase
	ScopePhases []string
}

func (e *PhaseScopeRefusal) Error() string {
	return fmt.Sprintf("contextcompile: phase %q does not occur in declared scope.phases %v", e.Phase, e.ScopePhases)
}

// --- Request -----------------------------------------------------------------

// Validate checks r's complete grammar: enum closure, ref/digest/hash
// grammar, sorted-unique set-like fields, the grants seam, and the
// phase/scope relationship. A phase outside a nonempty scope.phases
// returns *PhaseScopeRefusal, not an ordinary error.
func (r Request) Validate() error {
	if r.Schema != RequestSchema {
		return fmt.Errorf("contextcompile: request: schema %q, want %q", r.Schema, RequestSchema)
	}
	if err := r.Adapter.validate("request.adapter"); err != nil {
		return err
	}
	if r.Expected != nil {
		if err := r.Expected.validate("request.expected"); err != nil {
			return err
		}
	}
	if err := r.Grants.Validate(); err != nil {
		return fmt.Errorf("contextcompile: request.grants: %w", err)
	}
	if err := r.Phase.Validate(); err != nil {
		return fmt.Errorf("contextcompile: request.phase: %w", err)
	}
	if err := validateScope("request.scope", r.Scope); err != nil {
		return err
	}
	if err := validateSpecWholeRef("request.spec", r.Spec); err != nil {
		return err
	}

	if len(r.Scope.Phases) > 0 {
		found := false
		for _, p := range r.Scope.Phases {
			if p == string(r.Phase) {
				found = true
				break
			}
		}
		if !found {
			return &PhaseScopeRefusal{Phase: r.Phase, ScopePhases: append([]string(nil), r.Scope.Phases...)}
		}
	}
	return nil
}

func (a AdapterRef) validate(field string) error {
	if err := validateNonEmpty(field+".id", a.ID); err != nil {
		return err
	}
	if err := validateNonEmpty(field+".version", a.Version); err != nil {
		return err
	}
	return nil
}

func (e Expected) validate(field string) error {
	if err := validateGitHash(field+".head", e.Head); err != nil {
		return err
	}
	return validateNonEmpty(field+".branch", e.Branch)
}

// validateScope delegates grammar/uniqueness to policyartifact.Scope.Validate
// and additionally requires each dimension to already be sorted ascending —
// Scope.Validate alone only rejects duplicates, not out-of-order input, but
// this package's exact-byte contract requires canonical (sorted) set-like
// lists throughout. field names the enclosing document's scope field
// ("request.scope" or "manifest.scope") for error messages.
func validateScope(field string, s policyartifact.Scope) error {
	if err := s.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s: %w", field, err)
	}
	if err := requireSortedUniqueStrings(field+".phases", s.Phases); err != nil {
		return err
	}
	if err := requireSortedUniqueStrings(field+".environments", s.Environments); err != nil {
		return err
	}
	if err := requireSortedUniqueStrings(field+".paths", s.Paths); err != nil {
		return err
	}
	return requireSortedUniqueStrings(field+".refs", s.Refs)
}

// --- Manifest ------------------------------------------------------------

// Validate checks m's complete grammar: enum closure, ref/digest/hash
// grammar, and sorted-unique set-like fields across every §8.2 section.
func (m Manifest) Validate() error {
	if m.Schema != ManifestSchema {
		return fmt.Errorf("contextcompile: manifest: schema %q, want %q", m.Schema, ManifestSchema)
	}
	if err := m.Phase.Validate(); err != nil {
		return fmt.Errorf("contextcompile: manifest.phase: %w", err)
	}
	if err := m.Adapter.validate("manifest.adapter"); err != nil {
		return err
	}
	if err := m.Revisions.validate(); err != nil {
		return err
	}
	if err := m.AcceptedSpec.validate(); err != nil {
		return err
	}
	if m.ParentFeatures == nil {
		return fmt.Errorf("contextcompile: manifest.parent_features: must be non-nil (an explicitly empty set is [])")
	}
	if err := requireSortedUnique("manifest.parent_features", m.ParentFeatures, func(p ParentFeature) string { return p.Ref }); err != nil {
		return err
	}
	for i, p := range m.ParentFeatures {
		if err := p.validate(i); err != nil {
			return err
		}
	}
	if m.Decisions == nil {
		return fmt.Errorf("contextcompile: manifest.decisions: must be non-nil (an explicitly empty set is [])")
	}
	if err := requireSortedUnique("manifest.decisions", m.Decisions, func(d DecisionRef) string { return d.Ref }); err != nil {
		return err
	}
	for i, d := range m.Decisions {
		if err := d.validate(i); err != nil {
			return err
		}
	}
	if m.Obligations == nil {
		return fmt.Errorf("contextcompile: manifest.obligations: must be non-nil (an explicitly empty set is [])")
	}
	if err := requireSortedUnique("manifest.obligations", m.Obligations, func(o Obligation) string { return o.Ref }); err != nil {
		return err
	}
	for i, o := range m.Obligations {
		if err := o.validate(i); err != nil {
			return err
		}
	}
	if err := m.Repository.validate(); err != nil {
		return err
	}
	if err := m.Policy.validate(); err != nil {
		return err
	}
	if m.Owners == nil {
		return fmt.Errorf("contextcompile: manifest.owners: must be non-nil (an explicitly empty set is [])")
	}
	for i, o := range m.Owners {
		if err := validateNonEmpty(fmt.Sprintf("manifest.owners[%d]", i), o); err != nil {
			return err
		}
	}
	if err := requireSortedUniqueStrings("manifest.owners", m.Owners); err != nil {
		return err
	}
	if err := validateScope("manifest.scope", m.Scope); err != nil {
		return err
	}
	if err := m.GovernanceProfile.validate(); err != nil {
		return err
	}
	if err := m.Actors.validate(); err != nil {
		return err
	}
	if m.Included == nil {
		return fmt.Errorf("contextcompile: manifest.included: must be non-nil (an explicitly empty set is [])")
	}
	if err := requireSortedUnique("manifest.included", m.Included, func(e IncludedEntry) string { return string(e.Source) + "\x00" + e.ID }); err != nil {
		return err
	}
	for i, e := range m.Included {
		if err := e.validate(i); err != nil {
			return err
		}
	}
	if m.Excluded == nil {
		return fmt.Errorf("contextcompile: manifest.excluded: must be non-nil (an explicitly empty set is [])")
	}
	if err := requireSortedUnique("manifest.excluded", m.Excluded, func(e ExcludedEntry) string { return string(e.Source) + "\x00" + e.ID }); err != nil {
		return err
	}
	for i, e := range m.Excluded {
		if err := e.validate(i); err != nil {
			return err
		}
	}
	if m.Opaque == nil {
		return fmt.Errorf("contextcompile: manifest.opaque: must be non-nil (an explicitly empty set is [])")
	}
	if err := requireSortedUnique("manifest.opaque", m.Opaque, func(e OpaqueEntry) string { return e.ID }); err != nil {
		return err
	}
	for i, e := range m.Opaque {
		if err := e.validate(i); err != nil {
			return err
		}
	}
	if err := m.Capabilities.Validate(); err != nil {
		return fmt.Errorf("contextcompile: manifest.capabilities: %w", err)
	}
	if m.ProjectionFiles == nil {
		return fmt.Errorf("contextcompile: manifest.projection_files: must be non-nil (an explicitly empty set is [])")
	}
	if err := requireSortedUnique("manifest.projection_files", m.ProjectionFiles, func(p ProjectionFileRef) string { return p.Path }); err != nil {
		return err
	}
	for i, p := range m.ProjectionFiles {
		if err := p.validate(i); err != nil {
			return err
		}
	}
	if m.RequiredInputs == nil {
		return fmt.Errorf("contextcompile: manifest.required_inputs: must be non-nil (an explicitly empty set is [])")
	}
	if err := requireSortedUnique("manifest.required_inputs", m.RequiredInputs, func(r RequiredInput) string { return r.Kind }); err != nil {
		return err
	}
	for i, r := range m.RequiredInputs {
		if err := r.validate(i); err != nil {
			return err
		}
	}
	if err := m.Evidence.validate(); err != nil {
		return err
	}
	if err := validateDisclosures("manifest.disclosures", m.Disclosures); err != nil {
		return err
	}
	if m.Digest != "" {
		// Digest is checked against a fresh recomputation by DecodeManifest;
		// here we only check its grammar so a caller-constructed value with
		// a garbage digest fails fast rather than silently encoding it (it
		// is discarded and recomputed by EncodeManifest regardless).
		if err := validateDigest("manifest.digest", m.Digest); err != nil {
			return err
		}
	}
	return nil
}

func (rv Revisions) validate() error {
	if err := validateDigest("manifest.revisions.authority", rv.Authority); err != nil {
		return err
	}
	if rv.Context != 1 {
		return fmt.Errorf("contextcompile: manifest.revisions.context: must be 1 in v1, got %d", rv.Context)
	}
	return nil
}

func (a AcceptedSpec) validate() error {
	if err := validateSpecWholeRef("manifest.accepted_spec.ref", a.Ref); err != nil {
		return err
	}
	if err := validateNonEmpty("manifest.accepted_spec.path", a.Path); err != nil {
		return err
	}
	if err := validateGitHash("manifest.accepted_spec.blob", a.Blob); err != nil {
		return err
	}
	if err := validateGitHash("manifest.accepted_spec.commit", a.Commit); err != nil {
		return err
	}
	return validateDigest("manifest.accepted_spec.content_digest", a.ContentDigest)
}

func (p ParentFeature) validate(i int) error {
	field := fmt.Sprintf("manifest.parent_features[%d]", i)
	if err := validateSpecWholeRef(field+".ref", p.Ref); err != nil {
		return err
	}
	if err := validateNonEmpty(field+".path", p.Path); err != nil {
		return err
	}
	if err := validateDigest(field+".source_digest", p.SourceDigest); err != nil {
		return err
	}
	if err := validateDigest(field+".fragment_digest", p.FragmentDigest); err != nil {
		return err
	}
	return validateDigest(field+".payload_digest", p.PayloadDigest)
}

func (d DecisionRef) validate(i int) error {
	field := fmt.Sprintf("manifest.decisions[%d]", i)
	if err := validateDecisionFragmentRef(field+".ref", d.Ref); err != nil {
		return err
	}
	return validateDigest(field+".content_digest", d.ContentDigest)
}

func (o Obligation) validate(i int) error {
	field := fmt.Sprintf("manifest.obligations[%d]", i)
	if err := validateObligationRef(field+".ref", o.Ref); err != nil {
		return err
	}
	if err := validateNonEmpty(field+".path", o.Path); err != nil {
		return err
	}
	if err := validateACID(field+".ac", o.AC); err != nil {
		return err
	}
	if err := validateEvidenceKind(field+".kind", o.Kind); err != nil {
		return err
	}
	return validateDigest(field+".content_digest", o.ContentDigest)
}

func (f StringFact) validate(field string) error {
	if f.Known && f.Value == "" {
		return fmt.Errorf("contextcompile: %s: known is true but value is empty", field)
	}
	if !f.Known && f.Value != "" {
		return fmt.Errorf("contextcompile: %s: known is false but value %q is non-empty", field, f.Value)
	}
	return nil
}

func (f BoolFact) validate(field string) error {
	if !f.Known && f.Value {
		return fmt.Errorf("contextcompile: %s: known is false but value is true", field)
	}
	return nil
}

func (f DefaultBranchFact) validate(field string) error {
	if f.Known && (f.Name == "" || f.Ref == "" || f.Head == "") {
		return fmt.Errorf("contextcompile: %s: known is true but name/ref/head are not all non-empty", field)
	}
	if !f.Known && (f.Name != "" || f.Ref != "" || f.Head != "") {
		return fmt.Errorf("contextcompile: %s: known is false but name/ref/head are non-empty", field)
	}
	return nil
}

func (f WorktreeFact) validate(field string) error {
	if f.Managed && f.Name == "" {
		return fmt.Errorf("contextcompile: %s: managed is true but name is empty", field)
	}
	if !f.Managed && f.Name != "" {
		return fmt.Errorf("contextcompile: %s: managed is false but name %q is non-empty", field, f.Name)
	}
	return nil
}

func (rf RepositoryFacts) validate() error {
	const field = "manifest.repository"
	if err := rf.RemoteOrigin.validate(field + ".remote_origin"); err != nil {
		return err
	}
	if err := rf.Branch.validate(field + ".branch"); err != nil {
		return err
	}
	if err := rf.Head.validate(field + ".head"); err != nil {
		return err
	}
	if err := rf.DefaultBranch.validate(field + ".default_branch"); err != nil {
		return err
	}
	if !validRelationships[rf.Relationship] {
		return fmt.Errorf("contextcompile: %s.relationship: unknown value %q", field, rf.Relationship)
	}
	if err := rf.Dirty.validate(field + ".dirty"); err != nil {
		return err
	}
	if err := rf.Staged.validate(field + ".staged"); err != nil {
		return err
	}
	if err := rf.Worktree.validate(field + ".worktree"); err != nil {
		return err
	}
	if !validRepoSources[rf.Source] {
		return fmt.Errorf("contextcompile: %s.source: unknown value %q", field, rf.Source)
	}
	return validateDisclosures(field+".disclosures", rf.Disclosures)
}

func (pe PolicyEntry) validate(i int) error {
	field := fmt.Sprintf("manifest.policy.entries[%d]", i)
	if err := validatePolicyEntryKind(field+".kind", pe.Kind); err != nil {
		return err
	}
	if err := validateNonEmpty(field+".id", pe.ID); err != nil {
		return err
	}
	if err := validateDigest(field+".digest", pe.Digest); err != nil {
		return err
	}
	return pe.Applicability.Validate()
}

func (p PolicySection) validate() error {
	const field = "manifest.policy"
	if err := validateDigest(field+".effective_digest", p.EffectiveDigest); err != nil {
		return err
	}
	if err := validateDigest(field+".constitution_digest", p.ConstitutionDigest); err != nil {
		return err
	}
	if err := governanceprincipal.ValidateID(p.ProfileID); err != nil {
		return fmt.Errorf("contextcompile: %s.profile_id: %w", field, err)
	}
	if err := validateDigest(field+".profile_digest", p.ProfileDigest); err != nil {
		return err
	}
	if err := requireSortedUnique(field+".entries", p.Entries, func(e PolicyEntry) string { return e.Kind + "\x00" + e.ID }); err != nil {
		return err
	}
	for i, e := range p.Entries {
		if err := e.validate(i); err != nil {
			return err
		}
	}
	return nil
}

func (g GovernanceProfileRef) validate() error {
	const field = "manifest.governance_profile"
	if err := governanceprincipal.ValidateID(g.ID); err != nil {
		return fmt.Errorf("contextcompile: %s.id: %w", field, err)
	}
	if err := g.Class.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.class: %w", field, err)
	}
	return validateDigest(field+".digest", g.Digest)
}

func (a ActorsSection) validate() error {
	const field = "manifest.actors"
	if err := a.Posture.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.posture: %w", field, err)
	}
	if a.Resolutions == nil {
		return fmt.Errorf("contextcompile: %s.resolutions: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, r := range a.Resolutions {
		if err := validatePrincipalResolution(fmt.Sprintf("%s.resolutions[%d]", field, i), r); err != nil {
			return err
		}
	}
	if err := requireSortedUnique(field+".resolutions", a.Resolutions, func(r governanceprincipal.PrincipalResolution) string {
		return r.Claim.TrustSource + "\x00" + r.Claim.Subject
	}); err != nil {
		return err
	}
	return validateDisclosures(field+".disclosures", a.Disclosures)
}

// validatePrincipalResolution checks a manifest-carried
// governanceprincipal.PrincipalResolution's grammar only — never its
// kernel seal, which never round-trips through this wire schema (authority
// design §4: "a canonical projection ... it does not turn that projection
// back into kernel authority").
func validatePrincipalResolution(field string, r governanceprincipal.PrincipalResolution) error {
	if err := r.Claim.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.claim: %w", field, err)
	}
	if err := r.State.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.state: %w", field, err)
	}
	if r.State == governanceprincipal.ResolutionAuthenticated {
		if err := r.PrincipalID.Validate(); err != nil {
			return fmt.Errorf("contextcompile: %s.principal_id: %w", field, err)
		}
	} else if r.PrincipalID != "" {
		return fmt.Errorf("contextcompile: %s.principal_id: must be empty unless state is authenticated", field)
	}
	if r.Witnesses == nil {
		return fmt.Errorf("contextcompile: %s.witnesses: must be non-nil (an explicitly empty set is [])", field)
	}
	for i, w := range r.Witnesses {
		if w.Code == "" {
			return fmt.Errorf("contextcompile: %s.witnesses[%d].code: must be non-empty", field, i)
		}
		if w.SourceID == "" {
			return fmt.Errorf("contextcompile: %s.witnesses[%d].source_id: must be non-empty", field, i)
		}
	}
	return nil
}

func (e IncludedEntry) validate(i int) error {
	field := fmt.Sprintf("manifest.included[%d]", i)
	if err := validateNonEmpty(field+".id", e.ID); err != nil {
		return err
	}
	if err := e.Source.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.source: %w", field, err)
	}
	if err := e.Kind.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.kind: %w", field, err)
	}
	if err := e.Applicability.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.applicability: %w", field, err)
	}
	if err := e.PayloadChannel.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.payload_channel: %w", field, err)
	}
	if e.Path != nil && *e.Path == "" {
		return fmt.Errorf("contextcompile: %s.path: present but empty", field)
	}
	if e.Ref != nil && *e.Ref == "" {
		return fmt.Errorf("contextcompile: %s.ref: present but empty", field)
	}
	if err := validateDigest(field+".content_digest", e.ContentDigest); err != nil {
		return err
	}
	if err := validateDigest(field+".payload_digest", e.PayloadDigest); err != nil {
		return err
	}
	return validateDisclosures(field+".disclosures", e.Disclosures)
}

func (e ExcludedEntry) validate(i int) error {
	field := fmt.Sprintf("manifest.excluded[%d]", i)
	if err := validateNonEmpty(field+".id", e.ID); err != nil {
		return err
	}
	if err := e.Source.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.source: %w", field, err)
	}
	if err := e.Reason.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.reason: %w", field, err)
	}
	if err := e.Applicability.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.applicability: %w", field, err)
	}
	if e.Path != nil && *e.Path == "" {
		return fmt.Errorf("contextcompile: %s.path: present but empty", field)
	}
	if e.Ref != nil && *e.Ref == "" {
		return fmt.Errorf("contextcompile: %s.ref: present but empty", field)
	}
	return validateDisclosures(field+".disclosures", e.Disclosures)
}

func (e OpaqueEntry) validate(i int) error {
	field := fmt.Sprintf("manifest.opaque[%d]", i)
	if err := validateNonEmpty(field+".id", e.ID); err != nil {
		return err
	}
	if e.Kind != OpaqueKindHarnessVendorBase {
		return fmt.Errorf("contextcompile: %s.kind: must be %q, got %q", field, OpaqueKindHarnessVendorBase, e.Kind)
	}
	if err := e.Adapter.validate(field + ".adapter"); err != nil {
		return err
	}
	return validateDisclosures(field+".disclosures", e.Disclosures)
}

func (p ProjectionFileRef) validate(i int) error {
	field := fmt.Sprintf("manifest.projection_files[%d]", i)
	if err := validateNonEmpty(field+".path", p.Path); err != nil {
		return err
	}
	return validateDigest(field+".digest", p.Digest)
}

func (r RequiredInput) validate(i int) error {
	field := fmt.Sprintf("manifest.required_inputs[%d]", i)
	if err := validateRequiredInputKind(field+".kind", r.Kind); err != nil {
		return err
	}
	if err := r.Resolution.Validate(); err != nil {
		return fmt.Errorf("contextcompile: %s.resolution: %w", field, err)
	}
	if err := validateOptionalDigest(field+".digest", r.Digest); err != nil {
		return err
	}
	if r.Witnesses == nil {
		return fmt.Errorf("contextcompile: %s.witnesses: must be non-nil (an explicitly empty set is [])", field)
	}
	if (r.Resolution == ResolutionUnproven || r.Resolution == ResolutionViolatedWithWitness) && len(r.Witnesses) == 0 && r.Digest == nil {
		return fmt.Errorf("contextcompile: %s: an unproven or violated row must name a disclosure or witness (CO-1)", field)
	}
	return requireSortedUniqueStrings(field+".witnesses", r.Witnesses)
}

func (e EvidenceSection) validate() error {
	const field = "manifest.evidence"
	if e.Authority != EvidenceAuthorityAdvisory {
		return fmt.Errorf("contextcompile: %s.authority: must be %q in v1, got %q", field, EvidenceAuthorityAdvisory, e.Authority)
	}
	if !validEvidenceFreshness[e.Freshness] {
		return fmt.Errorf("contextcompile: %s.freshness: unknown value %q", field, e.Freshness)
	}
	if e.ConsumedReports == nil {
		return fmt.Errorf("contextcompile: %s.consumed_reports: must be non-nil (an explicitly empty set is [])", field)
	}
	if len(e.ConsumedReports) != 0 {
		return fmt.Errorf("contextcompile: %s.consumed_reports: must be [] in v1, got %d entries", field, len(e.ConsumedReports))
	}
	return validateDisclosures(field+".disclosures", e.Disclosures)
}

// --- DataItem ------------------------------------------------------------

// Validate checks item's complete grammar: enum closure, digest format and
// binding, and the classification/kind restrictions specific to a data
// item (authority design §8.1).
func (item DataItem) Validate() error {
	if item.Schema != DataItemSchema {
		return fmt.Errorf("contextcompile: data item: schema %q, want %q", item.Schema, DataItemSchema)
	}
	if err := validateNonEmpty("data item.id", item.ID); err != nil {
		return err
	}
	if err := item.Source.Validate(); err != nil {
		return fmt.Errorf("contextcompile: data item.source: %w", err)
	}
	if err := item.Kind.Validate(); err != nil {
		return fmt.Errorf("contextcompile: data item.kind: %w", err)
	}
	if item.Kind == IncludedInstructionProjection {
		return fmt.Errorf("contextcompile: data item.kind: instruction-projection never receives a data-item wrapper (authority design §8.1)")
	}
	if item.Path != nil && *item.Path == "" {
		return fmt.Errorf("contextcompile: data item.path: present but empty")
	}
	if item.Ref != nil && *item.Ref == "" {
		return fmt.Errorf("contextcompile: data item.ref: present but empty")
	}
	if item.Ref != nil {
		validateRef := validateArtifactRef
		if item.Kind == IncludedPolicyArtifact {
			validateRef = validatePolicyArtifactRef
		}
		if err := validateRef("data item.ref", *item.Ref); err != nil {
			return err
		}
	}
	if item.Classification != DataItemClassification {
		return fmt.Errorf("contextcompile: data item.classification: must be %q, got %q", DataItemClassification, item.Classification)
	}
	if nonTextContent([]byte(item.Content)) {
		return fmt.Errorf("contextcompile: data item.content: not text (invalid UTF-8 or contains NUL)")
	}
	if err := validateDigest("data item.content_digest", item.ContentDigest); err != nil {
		return err
	}
	if want := rawContentDigest([]byte(item.Content)); want != item.ContentDigest {
		return fmt.Errorf("contextcompile: data item.content_digest: %q does not match the exact bytes carried in content (want %q)", item.ContentDigest, want)
	}
	if item.Digest != "" {
		if err := validateDigest("data item.digest", item.Digest); err != nil {
			return err
		}
	}
	return nil
}

// rawContentDigest returns data's content address in the shared
// "sha256:"+hex form, computed over data's exact bytes (never a canonical-
// JSON re-encoding). Package-local per repo convention (mirrors
// internal/instructionprojection's own contentDigest helper): a data
// item's content_digest is defined over the raw bytes carried in content,
// a distinct notion from canonjson.Digest's canonical-value digest.
func rawContentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
