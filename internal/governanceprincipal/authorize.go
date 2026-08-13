package governanceprincipal

import (
	"fmt"
	"reflect"
	"sort"
)

// AuthorityPosture is the closed authority-posture vocabulary.
type AuthorityPosture string

// The two postures. Unknown postures fail closed.
const (
	PostureAuthoritative AuthorityPosture = "authoritative"
	PostureAdvisory      AuthorityPosture = "advisory"
)

// Validate fails closed on any posture outside the vocabulary.
func (p AuthorityPosture) Validate() error {
	switch p {
	case PostureAuthoritative, PostureAdvisory:
		return nil
	}
	return fmt.Errorf("governanceprincipal: unknown authority posture %q", string(p))
}

// AuthorizationState is the closed three-valued authorization state.
type AuthorizationState string

// The three authorization states. Unknown states fail closed.
const (
	AuthorizationAuthorized AuthorizationState = "authorized"
	AuthorizationViolated   AuthorizationState = "violated-with-witness"
	AuthorizationUnproven   AuthorizationState = "unproven"
)

// RuleFactKind is the closed rule-fact kind vocabulary.
type RuleFactKind string

// The two rule-fact kinds. Unknown kinds fail closed.
const (
	RuleFactSignature RuleFactKind = "signature"
	RuleFactOwnership RuleFactKind = "ownership"
)

// Validate fails closed on any kind outside the vocabulary.
func (k RuleFactKind) Validate() error {
	switch k {
	case RuleFactSignature, RuleFactOwnership:
		return nil
	}
	return fmt.Errorf("governanceprincipal: unknown rule-fact kind %q", string(k))
}

// RuleFactState is the closed three-valued rule-fact state.
type RuleFactState string

// The three rule-fact states. Unknown states fail closed.
const (
	RuleFactProven   RuleFactState = "proven"
	RuleFactViolated RuleFactState = "violated-with-witness"
	RuleFactUnproven RuleFactState = "unproven"
)

// Validate fails closed on any state outside the vocabulary.
func (s RuleFactState) Validate() error {
	switch s {
	case RuleFactProven, RuleFactViolated, RuleFactUnproven:
		return nil
	}
	return fmt.Errorf("governanceprincipal: unknown rule-fact state %q", string(s))
}

// RuleFact is one adapter-evaluated signature or ownership observation
// about one principal against one profile rule source. Signature facts
// are keyed by the signed-commit trust-source ID; ownership facts by the
// ownership-source ID. Like a TrustFact it is evidence, never a verdict.
type RuleFact struct {
	Kind        RuleFactKind  `json:"kind"`
	SourceID    string        `json:"source_id"`
	PrincipalID PrincipalID   `json:"principal_id"`
	State       RuleFactState `json:"state"`
	// EvidenceDigest is the canonical sha256 digest of the evaluated
	// evidence; required when the fact is proven.
	EvidenceDigest string `json:"evidence_digest,omitempty"`
	// Reason is the stable explanation, required for violated and
	// unproven facts.
	Reason string `json:"reason,omitempty"`
}

// ApprovalRecord asserts that one principal acts in one role for the
// requested transition. Only records whose principal has an authenticated
// resolution in the same request can fill the role.
type ApprovalRecord struct {
	Role        string      `json:"role"`
	PrincipalID PrincipalID `json:"principal_id"`
}

// AuthorizationRequest is one governed-transition authorization question.
// It carries kernel principal resolutions, never attribution records:
// attribution is advisory testimony and can never satisfy Authorize.
type AuthorizationRequest struct {
	Transition        string                `json:"transition"`
	Posture           AuthorityPosture      `json:"posture"`
	Resolutions       []PrincipalResolution `json:"resolutions"`
	Approvals         []ApprovalRecord      `json:"approvals"`
	RuleFacts         []RuleFact            `json:"rule_facts"`
	EvidenceSources   []string              `json:"evidence_sources"`
	EscalationMetrics map[string]int        `json:"escalation_metrics"`
}

// Finding is one applicable authorization finding: a stable reason code,
// the finding's three-valued contribution (violated or unproven — an
// authorized decision has no findings), and its witnesses.
//
// Roles is the finding's exact rule identity where a rule is defined over a
// role PAIR: every distinctness finding carries its rule's two roles,
// normalized lexically, and findings from every other rule family carry no
// pair at all. A consumer scoped to one relation therefore attributes a
// finding by structured identity instead of parsing Detail prose (GLG
// DC-17/DC-22; ledger SI-106).
type Finding struct {
	Code           string             `json:"code"`
	State          AuthorizationState `json:"state"`
	Role           string             `json:"role,omitempty"`
	Roles          []string           `json:"roles,omitempty"`
	SourceID       string             `json:"source_id,omitempty"`
	PrincipalID    PrincipalID        `json:"principal_id,omitempty"`
	EvidenceDigest string             `json:"evidence_digest,omitempty"`
	Detail         string             `json:"detail,omitempty"`
}

// Disclosure is one stable non-blocking disclosure, such as a solo
// profile's role collapse.
type Disclosure struct {
	Code        string      `json:"code"`
	PrincipalID PrincipalID `json:"principal_id,omitempty"`
	Roles       []string    `json:"roles,omitempty"`
	Detail      string      `json:"detail,omitempty"`
}

// AuthorizationDecision is the deterministic interpretation result:
// effective posture, three-valued state, and all applicable findings and
// disclosures sorted deterministically.
type AuthorizationDecision struct {
	Transition  string             `json:"transition"`
	Posture     AuthorityPosture   `json:"posture"`
	State       AuthorizationState `json:"state"`
	Findings    []Finding          `json:"findings"`
	Disclosures []Disclosure       `json:"disclosures"`
}

// Authorize is the single authorization interpretation (GLG DC-23): pure,
// no I/O, deterministic over its inputs. Malformed requests and
// internally inconsistent kernel records are operational errors; every
// governed shortfall is a finding. Explicit contradiction outranks
// unproven, and unproven is never reinterpreted as authorized.
func Authorize(profile Profile, request AuthorizationRequest) (AuthorizationDecision, error) {
	if err := profile.checkSeal(); err != nil {
		return AuthorizationDecision{}, err
	}
	if err := ValidateID(request.Transition); err != nil {
		return AuthorizationDecision{}, fmt.Errorf("governanceprincipal: request transition: %w", err)
	}
	if err := request.Posture.Validate(); err != nil {
		return AuthorizationDecision{}, err
	}
	resolutions, err := indexResolutions(request.Resolutions)
	if err != nil {
		return AuthorizationDecision{}, err
	}
	approvals, err := normalizeApprovals(request.Approvals)
	if err != nil {
		return AuthorizationDecision{}, err
	}
	facts, err := indexRuleFacts(request.RuleFacts)
	if err != nil {
		return AuthorizationDecision{}, err
	}
	evidence, err := normalizeEvidenceSources(request.EvidenceSources)
	if err != nil {
		return AuthorizationDecision{}, err
	}
	if err := validateEscalationMetrics(request.EscalationMetrics); err != nil {
		return AuthorizationDecision{}, err
	}

	d := AuthorizationDecision{Transition: request.Transition, Posture: request.Posture}
	var findings []Finding

	// An experimental profile can never produce an authoritative
	// authorization: the effective posture is always advisory, and a
	// request for authoritative posture is violated.
	if profile.Class == ClassExperimental {
		d.Posture = PostureAdvisory
		if request.Posture == PostureAuthoritative {
			findings = append(findings, Finding{
				Code:   ReasonExperimentalAuthorityForbidden,
				State:  AuthorizationViolated,
				Detail: fmt.Sprintf("experimental profile %q cannot produce an authoritative authorization", profile.ID),
			})
		}
	}

	if !contains(profile.ApplicableTransitions, request.Transition) {
		findings = append(findings, Finding{
			Code:   ReasonTransitionNotApplicable,
			State:  AuthorizationViolated,
			Detail: fmt.Sprintf("transition %q is not among profile %q applicable transitions", request.Transition, profile.ID),
		})
		return finishDecision(d, findings, nil), nil
	}

	fillers, principalFindings := fillRoles(profile, approvals, resolutions)
	findings = append(findings, principalFindings...)
	findings = append(findings, evaluateRequiredApprovers(profile, request.Transition, fillers)...)
	findings = append(findings, evaluateEscalation(profile, request.Transition, request.EscalationMetrics, fillers)...)
	findings = append(findings, evaluateDistinctness(profile, request.Transition, fillers)...)
	findings = append(findings, evaluateSignatures(profile, request.Transition, fillers, facts)...)
	findings = append(findings, evaluateOwnership(profile, request.Transition, fillers, facts)...)
	findings = append(findings, evaluateEvidenceSources(profile, request.Transition, evidence)...)

	var disclosures []Disclosure
	if profile.Class == ClassSolo {
		disclosures = soloCollapseDisclosures(fillers)
	}
	return finishDecision(d, findings, disclosures), nil
}

// indexResolutions validates every supplied resolution as an internally
// consistent kernel record and indexes it by the principal ID its claim
// derives.
func indexResolutions(resolutions []PrincipalResolution) (map[PrincipalID]PrincipalResolution, error) {
	byID := make(map[PrincipalID]PrincipalResolution, len(resolutions))
	for i, res := range resolutions {
		field := fmt.Sprintf("resolutions[%d]", i)
		if err := res.State.Validate(); err != nil {
			return nil, fmt.Errorf("governanceprincipal: %s: %w", field, err)
		}
		if err := res.Claim.Validate(); err != nil {
			return nil, fmt.Errorf("governanceprincipal: %s: %w", field, err)
		}
		derived, err := CanonicalPrincipalID(res.Claim.TrustSource, res.Claim.Subject)
		if err != nil {
			return nil, fmt.Errorf("governanceprincipal: %s: %w", field, err)
		}
		if res.State == ResolutionAuthenticated {
			if res.PrincipalID == "" {
				return nil, fmt.Errorf("governanceprincipal: %s: authenticated resolution must carry its derived principal id", field)
			}
			if res.PrincipalID != derived {
				return nil, fmt.Errorf("governanceprincipal: %s: principal id %q does not match the id its claim derives (%q)", field, res.PrincipalID, derived)
			}
		} else if res.PrincipalID != "" {
			return nil, fmt.Errorf("governanceprincipal: %s: %s resolution must not carry a principal id", field, res.State)
		}
		if err := res.checkSeal(); err != nil {
			return nil, fmt.Errorf("governanceprincipal: %s: %w", field, err)
		}
		if prev, ok := byID[derived]; ok {
			if !reflect.DeepEqual(prev, res) {
				return nil, fmt.Errorf("governanceprincipal: conflicting duplicate resolutions for principal %q", derived)
			}
			continue
		}
		byID[derived] = res
	}
	return byID, nil
}

// normalizeApprovals validates, dedupes, and deterministically orders the
// approval records.
func normalizeApprovals(approvals []ApprovalRecord) ([]ApprovalRecord, error) {
	seen := make(map[ApprovalRecord]bool, len(approvals))
	out := make([]ApprovalRecord, 0, len(approvals))
	for i, ap := range approvals {
		field := fmt.Sprintf("approvals[%d]", i)
		if err := ValidateID(ap.Role); err != nil {
			return nil, fmt.Errorf("governanceprincipal: %s role: %w", field, err)
		}
		if err := ap.PrincipalID.Validate(); err != nil {
			return nil, fmt.Errorf("governanceprincipal: %s: %w", field, err)
		}
		if seen[ap] {
			continue
		}
		seen[ap] = true
		out = append(out, ap)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Role != out[j].Role {
			return out[i].Role < out[j].Role
		}
		return out[i].PrincipalID < out[j].PrincipalID
	})
	return out, nil
}

// factKey identifies one rule-fact subject.
type factKey struct {
	Kind        RuleFactKind
	SourceID    string
	PrincipalID PrincipalID
}

// indexRuleFacts validates every rule fact and indexes them by subject.
// Identical duplicates collapse; differing facts about the same subject
// are contradictory kernel records and an operational error.
func indexRuleFacts(facts []RuleFact) (map[factKey]RuleFact, error) {
	byKey := make(map[factKey]RuleFact, len(facts))
	for i, f := range facts {
		field := fmt.Sprintf("rule_facts[%d]", i)
		if err := f.Kind.Validate(); err != nil {
			return nil, fmt.Errorf("governanceprincipal: %s: %w", field, err)
		}
		if err := f.State.Validate(); err != nil {
			return nil, fmt.Errorf("governanceprincipal: %s: %w", field, err)
		}
		if err := ValidateID(f.SourceID); err != nil {
			return nil, fmt.Errorf("governanceprincipal: %s source: %w", field, err)
		}
		if err := f.PrincipalID.Validate(); err != nil {
			return nil, fmt.Errorf("governanceprincipal: %s: %w", field, err)
		}
		if f.State == RuleFactProven && f.EvidenceDigest == "" {
			return nil, fmt.Errorf("governanceprincipal: %s: proven rule fact must carry an evidence digest", field)
		}
		if f.EvidenceDigest != "" && !validEvidenceDigest(f.EvidenceDigest) {
			return nil, fmt.Errorf("governanceprincipal: %s: malformed evidence digest %q", field, f.EvidenceDigest)
		}
		if f.State != RuleFactProven && f.Reason == "" {
			return nil, fmt.Errorf("governanceprincipal: %s: %s rule fact must carry a reason", field, f.State)
		}
		key := factKey{Kind: f.Kind, SourceID: f.SourceID, PrincipalID: f.PrincipalID}
		if prev, ok := byKey[key]; ok {
			if prev != f {
				return nil, fmt.Errorf("governanceprincipal: conflicting rule facts for %s source %q principal %q", f.Kind, f.SourceID, f.PrincipalID)
			}
			continue
		}
		byKey[key] = f
	}
	return byKey, nil
}

// normalizeEvidenceSources validates, dedupes, and sorts the presented
// evidence-source IDs.
func normalizeEvidenceSources(sources []string) ([]string, error) {
	seen := make(map[string]bool, len(sources))
	out := make([]string, 0, len(sources))
	for _, s := range sources {
		if err := ValidateID(s); err != nil {
			return nil, fmt.Errorf("governanceprincipal: evidence source: %w", err)
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out, nil
}

// validateEscalationMetrics validates metric keys and values.
func validateEscalationMetrics(metrics map[string]int) error {
	keys := make([]string, 0, len(metrics))
	for k := range metrics {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := ValidateID(k); err != nil {
			return fmt.Errorf("governanceprincipal: escalation metric: %w", err)
		}
		if metrics[k] < 0 {
			return fmt.Errorf("governanceprincipal: escalation metric %q must be nonnegative, got %d", k, metrics[k])
		}
	}
	return nil
}

// fillRoles maps approvals onto role fillers. Only authenticated
// resolutions whose profile role mapping matches the claimed trust source
// and subject fill a role; everything else is a finding.
func fillRoles(profile Profile, approvals []ApprovalRecord, resolutions map[PrincipalID]PrincipalResolution) (map[string][]PrincipalID, []Finding) {
	fillers := make(map[string][]PrincipalID)
	var findings []Finding
	for _, ap := range approvals {
		res, ok := resolutions[ap.PrincipalID]
		if !ok {
			findings = append(findings, Finding{
				Code:        ReasonPrincipalUnproven,
				State:       AuthorizationUnproven,
				Role:        ap.Role,
				PrincipalID: ap.PrincipalID,
				Detail:      "no principal resolution supplied for this approval",
			})
			continue
		}
		switch res.State {
		case ResolutionViolated:
			findings = append(findings, Finding{
				Code:        ReasonPrincipalViolated,
				State:       AuthorizationViolated,
				Role:        ap.Role,
				PrincipalID: ap.PrincipalID,
				Detail:      "principal resolution is violated-with-witness",
			})
		case ResolutionUnproven:
			findings = append(findings, Finding{
				Code:        ReasonPrincipalUnproven,
				State:       AuthorizationUnproven,
				Role:        ap.Role,
				PrincipalID: ap.PrincipalID,
				Detail:      "principal resolution is unproven",
			})
		case ResolutionAuthenticated:
			if !holdsRole(profile, res.Claim, ap.Role) {
				findings = append(findings, Finding{
					Code:        ReasonRoleNotAuthorized,
					State:       AuthorizationViolated,
					Role:        ap.Role,
					PrincipalID: ap.PrincipalID,
					Detail:      fmt.Sprintf("no role mapping grants role %q to this principal", ap.Role),
				})
				continue
			}
			if !containsPrincipal(fillers[ap.Role], ap.PrincipalID) {
				fillers[ap.Role] = append(fillers[ap.Role], ap.PrincipalID)
			}
		}
	}
	return fillers, findings
}

// HoldsRole reports whether profile's role mappings grant role to claim's
// already-authenticated trust source and subject. It is the ONE exported
// role-membership query (ledger SI-106): a consumer that must know which
// principals are even eligible to fill a role asks the kernel instead of
// reimplementing role-mapping semantics against Profile.RoleMappings.
//
// It answers a membership question only — never an authorization one. A
// true result is not approval, not distinctness, and not a verdict; only
// Authorize interprets those. Malformed operands (an unsealed profile, a
// malformed claim, or a role outside the local ID grammar) are operational
// errors, never a false answer.
func HoldsRole(profile Profile, claim PrincipalClaim, role string) (bool, error) {
	if err := profile.checkSeal(); err != nil {
		return false, err
	}
	if err := claim.Validate(); err != nil {
		return false, err
	}
	if err := ValidateID(role); err != nil {
		return false, fmt.Errorf("governanceprincipal: role: %w", err)
	}
	return holdsRole(profile, claim, role), nil
}

// holdsRole reports whether a role mapping grants role to the claim's
// trust source and subject. It is the single inner predicate both the
// kernel's own rule evaluation and the exported HoldsRole query run, so
// role-mapping semantics exist exactly once.
func holdsRole(profile Profile, claim PrincipalClaim, role string) bool {
	for _, m := range profile.RoleMappings {
		if m.Role == role && m.TrustSource == claim.TrustSource && contains(m.Subjects, claim.Subject) {
			return true
		}
	}
	return false
}

func containsPrincipal(set []PrincipalID, p PrincipalID) bool {
	for _, e := range set {
		if e == p {
			return true
		}
	}
	return false
}

// evaluateRequiredApprovers counts distinct authenticated principals per
// applicable approver rule.
func evaluateRequiredApprovers(profile Profile, transition string, fillers map[string][]PrincipalID) []Finding {
	var findings []Finding
	for _, rule := range profile.RequiredApprovers {
		if !contains(rule.Transitions, transition) {
			continue
		}
		distinct := make(map[PrincipalID]bool)
		for _, role := range rule.Roles {
			for _, p := range fillers[role] {
				distinct[p] = true
			}
		}
		if len(distinct) < rule.Minimum {
			findings = append(findings, Finding{
				Code:   ReasonRequiredApproverMissing,
				State:  AuthorizationUnproven,
				Detail: fmt.Sprintf("transition %q roles %v: %d of %d required distinct authenticated approvers", transition, rule.Roles, len(distinct), rule.Minimum),
			})
		}
	}
	return findings
}

// evaluateEscalation makes every required escalation role a required
// authenticated approver once its metric reaches the threshold. A missing
// metric value leaves the threshold unproven.
func evaluateEscalation(profile Profile, transition string, metrics map[string]int, fillers map[string][]PrincipalID) []Finding {
	var findings []Finding
	for _, e := range profile.EscalationThresholds {
		if !contains(e.Transitions, transition) {
			continue
		}
		value, ok := metrics[e.Metric]
		if !ok {
			findings = append(findings, Finding{
				Code:   ReasonEscalationMetricUnavailable,
				State:  AuthorizationUnproven,
				Detail: fmt.Sprintf("no value supplied for escalation metric %q", e.Metric),
			})
			continue
		}
		if value < e.AtLeast {
			continue
		}
		for _, role := range e.RequiredRoles {
			if len(fillers[role]) == 0 {
				findings = append(findings, Finding{
					Code:   ReasonEscalationRoleMissing,
					State:  AuthorizationUnproven,
					Role:   role,
					Detail: fmt.Sprintf("metric %q is %d (threshold %d): role %q requires an authenticated approver", e.Metric, value, e.AtLeast, role),
				})
			}
		}
	}
	return findings
}

// sortedRolePair is a distinctness rule's canonical identity: the two roles
// are a semantic set, so they normalize lexically and a reversed rule
// spelling identifies identically. A fresh slice is returned per call, so no
// two findings ever share backing storage.
func sortedRolePair(a, b string) []string {
	if a > b {
		a, b = b, a
	}
	return []string{a, b}
}

// evaluateDistinctness applies same-principal and different-principal
// rules centrally over the role fillers. An applicable rule with an
// unfilled side is unproven, never vacuously satisfied. Every finding this
// family emits carries its rule's exact canonical role pair (SI-106), so a
// relation-scoped consumer can tell one rule's evidence from another's.
func evaluateDistinctness(profile Profile, transition string, fillers map[string][]PrincipalID) []Finding {
	var findings []Finding
	for _, rule := range profile.DistinctnessRules {
		if !contains(rule.Transitions, transition) {
			continue
		}
		left, right := fillers[rule.LeftRole], fillers[rule.RightRole]
		if len(left) == 0 || len(right) == 0 {
			for _, role := range []string{rule.LeftRole, rule.RightRole} {
				if len(fillers[role]) == 0 {
					findings = append(findings, Finding{
						Code:   ReasonDistinctnessUnproven,
						State:  AuthorizationUnproven,
						Role:   role,
						Roles:  sortedRolePair(rule.LeftRole, rule.RightRole),
						Detail: fmt.Sprintf("%s rule between %q and %q: role %q has no authenticated filler", rule.Relation, rule.LeftRole, rule.RightRole, role),
					})
				}
			}
			continue
		}
		switch rule.Relation {
		case RelationDifferentPrincipal:
			for _, p := range left {
				if containsPrincipal(right, p) {
					findings = append(findings, Finding{
						Code:        ReasonDistinctnessViolated,
						State:       AuthorizationViolated,
						Roles:       sortedRolePair(rule.LeftRole, rule.RightRole),
						PrincipalID: p,
						Detail:      fmt.Sprintf("roles %q and %q require different principals", rule.LeftRole, rule.RightRole),
					})
				}
			}
		case RelationSamePrincipal:
			distinct := make(map[PrincipalID]bool)
			for _, p := range left {
				distinct[p] = true
			}
			for _, p := range right {
				distinct[p] = true
			}
			if len(distinct) > 1 {
				findings = append(findings, Finding{
					Code:   ReasonDistinctnessViolated,
					State:  AuthorizationViolated,
					Roles:  sortedRolePair(rule.LeftRole, rule.RightRole),
					Detail: fmt.Sprintf("roles %q and %q require the same principal", rule.LeftRole, rule.RightRole),
				})
			}
		}
	}
	return findings
}

// evaluateSignatures applies role-specific signature requirements. The
// rule's trust sources are alternatives: one proven signature fact from
// any listed source satisfies the principal; otherwise an explicitly
// violated fact outranks an unproven or missing one.
func evaluateSignatures(profile Profile, transition string, fillers map[string][]PrincipalID, facts map[factKey]RuleFact) []Finding {
	var findings []Finding
	for _, rule := range profile.SignatureRequirements {
		if !contains(rule.Transitions, transition) {
			continue
		}
		for _, role := range rule.Roles {
			if len(fillers[role]) == 0 {
				findings = append(findings, Finding{
					Code:   ReasonSignatureUnproven,
					State:  AuthorizationUnproven,
					Role:   role,
					Detail: fmt.Sprintf("signature rule applies to role %q but no authenticated principal fills it", role),
				})
				continue
			}
			for _, p := range fillers[role] {
				var violated, unproven *RuleFact
				proven := false
				for _, src := range rule.TrustSources {
					f, ok := facts[factKey{Kind: RuleFactSignature, SourceID: src, PrincipalID: p}]
					if !ok {
						continue
					}
					switch f.State {
					case RuleFactProven:
						proven = true
					case RuleFactViolated:
						if violated == nil {
							fc := f
							violated = &fc
						}
					case RuleFactUnproven:
						if unproven == nil {
							fc := f
							unproven = &fc
						}
					}
				}
				switch {
				case proven:
				case violated != nil:
					findings = append(findings, Finding{
						Code:           ReasonSignatureViolated,
						State:          AuthorizationViolated,
						Role:           role,
						SourceID:       violated.SourceID,
						PrincipalID:    p,
						EvidenceDigest: violated.EvidenceDigest,
						Detail:         violated.Reason,
					})
				case unproven != nil:
					findings = append(findings, Finding{
						Code:        ReasonSignatureUnproven,
						State:       AuthorizationUnproven,
						Role:        role,
						SourceID:    unproven.SourceID,
						PrincipalID: p,
						Detail:      unproven.Reason,
					})
				default:
					findings = append(findings, Finding{
						Code:        ReasonSignatureUnproven,
						State:       AuthorizationUnproven,
						Role:        role,
						PrincipalID: p,
						Detail:      fmt.Sprintf("no signature fact supplied from sources %v", rule.TrustSources),
					})
				}
			}
		}
	}
	return findings
}

// evaluateOwnership applies role-specific ownership rules. Ownership
// sources are conjunctive: every source covering the transition must be
// proven for every filler of its roles.
func evaluateOwnership(profile Profile, transition string, fillers map[string][]PrincipalID, facts map[factKey]RuleFact) []Finding {
	var findings []Finding
	for _, src := range profile.OwnershipSources {
		if !contains(src.Transitions, transition) {
			continue
		}
		for _, role := range src.Roles {
			if len(fillers[role]) == 0 {
				findings = append(findings, Finding{
					Code:     ReasonOwnershipUnproven,
					State:    AuthorizationUnproven,
					Role:     role,
					SourceID: src.ID,
					Detail:   fmt.Sprintf("ownership source %q applies to role %q but no authenticated principal fills it", src.ID, role),
				})
				continue
			}
			for _, p := range fillers[role] {
				f, ok := facts[factKey{Kind: RuleFactOwnership, SourceID: src.ID, PrincipalID: p}]
				switch {
				case ok && f.State == RuleFactProven:
				case ok && f.State == RuleFactViolated:
					findings = append(findings, Finding{
						Code:           ReasonOwnershipViolated,
						State:          AuthorizationViolated,
						Role:           role,
						SourceID:       src.ID,
						PrincipalID:    p,
						EvidenceDigest: f.EvidenceDigest,
						Detail:         f.Reason,
					})
				case ok:
					findings = append(findings, Finding{
						Code:        ReasonOwnershipUnproven,
						State:       AuthorizationUnproven,
						Role:        role,
						SourceID:    src.ID,
						PrincipalID: p,
						Detail:      f.Reason,
					})
				default:
					findings = append(findings, Finding{
						Code:        ReasonOwnershipUnproven,
						State:       AuthorizationUnproven,
						Role:        role,
						SourceID:    src.ID,
						PrincipalID: p,
						Detail:      fmt.Sprintf("no ownership fact supplied from source %q", src.ID),
					})
				}
			}
		}
	}
	return findings
}

// evaluateEvidenceSources applies the transition's evidence restriction:
// every presented source must be allowed, and a required restriction with
// no presented source is unproven.
func evaluateEvidenceSources(profile Profile, transition string, presented []string) []Finding {
	var findings []Finding
	for _, r := range profile.EvidenceSourceRestrictions {
		if !contains(r.Transitions, transition) {
			continue
		}
		if len(presented) == 0 {
			findings = append(findings, Finding{
				Code:   ReasonEvidenceSourceUnproven,
				State:  AuthorizationUnproven,
				Detail: fmt.Sprintf("transition %q restricts evidence sources but none was presented", transition),
			})
			continue
		}
		for _, s := range presented {
			if !contains(r.AllowedSources, s) {
				findings = append(findings, Finding{
					Code:     ReasonEvidenceSourceForbidden,
					State:    AuthorizationViolated,
					SourceID: s,
					Detail:   fmt.Sprintf("evidence source %q is not allowed for transition %q", s, transition),
				})
			}
		}
	}
	return findings
}

// soloCollapseDisclosures reports every principal actually filling more
// than one role under a solo profile.
func soloCollapseDisclosures(fillers map[string][]PrincipalID) []Disclosure {
	roles := make([]string, 0, len(fillers))
	for role := range fillers {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	byPrincipal := make(map[PrincipalID][]string)
	for _, role := range roles {
		for _, p := range fillers[role] {
			byPrincipal[p] = append(byPrincipal[p], role)
		}
	}
	principals := make([]PrincipalID, 0, len(byPrincipal))
	for p, held := range byPrincipal {
		if len(held) > 1 {
			principals = append(principals, p)
		}
	}
	sort.Slice(principals, func(i, j int) bool { return principals[i] < principals[j] })
	disclosures := make([]Disclosure, 0, len(principals))
	for _, p := range principals {
		disclosures = append(disclosures, Disclosure{
			Code:        ReasonSoloRoleCollapse,
			PrincipalID: p,
			Roles:       byPrincipal[p],
			Detail:      "one principal fills multiple roles under the solo profile",
		})
	}
	return disclosures
}

// finishDecision sorts and dedupes findings and disclosures and derives
// the decision state: explicit contradiction outranks unproven, and
// unproven outranks authorized.
func finishDecision(d AuthorizationDecision, findings []Finding, disclosures []Disclosure) AuthorizationDecision {
	sort.Slice(findings, func(i, j int) bool { return findingLess(findings[i], findings[j]) })
	// Finding carries a role pair, so it is no longer a comparable struct:
	// adjacent-duplicate collapse compares complete field content instead of
	// using ==. The order above is total over that same content, so equal
	// findings are always adjacent.
	deduped := make([]Finding, 0, len(findings))
	for i, f := range findings {
		if i == 0 || !reflect.DeepEqual(f, findings[i-1]) {
			deduped = append(deduped, f)
		}
	}
	d.Findings = deduped

	sort.Slice(disclosures, func(i, j int) bool {
		if disclosures[i].Code != disclosures[j].Code {
			return disclosures[i].Code < disclosures[j].Code
		}
		return disclosures[i].PrincipalID < disclosures[j].PrincipalID
	})
	d.Disclosures = disclosures

	d.State = AuthorizationAuthorized
	for _, f := range d.Findings {
		if f.State == AuthorizationViolated {
			d.State = AuthorizationViolated
			break
		}
		d.State = AuthorizationUnproven
	}
	return d
}

// findingLess is the deterministic finding order: complete field content.
func findingLess(a, b Finding) bool {
	if a.Code != b.Code {
		return a.Code < b.Code
	}
	if a.State != b.State {
		return a.State < b.State
	}
	if a.Role != b.Role {
		return a.Role < b.Role
	}
	if c := compareStrings(a.Roles, b.Roles); c != 0 {
		return c < 0
	}
	if a.SourceID != b.SourceID {
		return a.SourceID < b.SourceID
	}
	if a.PrincipalID != b.PrincipalID {
		return a.PrincipalID < b.PrincipalID
	}
	if a.EvidenceDigest != b.EvidenceDigest {
		return a.EvidenceDigest < b.EvidenceDigest
	}
	return a.Detail < b.Detail
}

// compareStrings is the lexicographic order over two string sequences:
// negative, zero, or positive as a sorts before, equal to, or after b. A
// shorter sequence that is a prefix of a longer one sorts first.
func compareStrings(a, b []string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	}
	return 0
}
