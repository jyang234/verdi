package policyconflict

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/repositoryfacts"
)

// --- shared nested-splice helpers -------------------------------------------
//
// These mirror internal/contextcompile/codec.go's own nested-grant-splice
// pattern exactly ("decode the nested document by re-encoding that exact
// nested document canonically and calling the owning package's Decode"):
// this package never reimplements grant or nested-request grammar.

var nullLiteral = []byte("null")

func rawMessageMissing(raw json.RawMessage) bool {
	return raw == nil || bytes.Equal(raw, nullLiteral)
}

func decodeNestedGrants(nested json.RawMessage) (execworkspace.GrantSet, error) {
	canonical, err := canonjson.Marshal(nested)
	if err != nil {
		return execworkspace.GrantSet{}, fmt.Errorf("re-encoding nested grants document: %w", err)
	}
	set, err := execworkspace.DecodeGrantSet(canonical)
	if err != nil {
		return execworkspace.GrantSet{}, err
	}
	return set, nil
}

func encodeNestedGrants(set execworkspace.GrantSet) (json.RawMessage, error) {
	data, err := execworkspace.EncodeGrantSet(set)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func decodeNestedAcceptedContext(nested json.RawMessage) (contextcompile.Request, error) {
	canonical, err := canonjson.Marshal(nested)
	if err != nil {
		return contextcompile.Request{}, fmt.Errorf("re-encoding nested accepted_context document: %w", err)
	}
	req, err := contextcompile.DecodeRequest(canonical)
	if err != nil {
		return contextcompile.Request{}, err
	}
	return req, nil
}

func encodeNestedAcceptedContext(r contextcompile.Request) (json.RawMessage, error) {
	data, err := contextcompile.EncodeRequest(r)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// --- Request (authority design §2) ------------------------------------------

type acceptanceCandidateDoc struct {
	Adapter  *contextcompile.AdapterRef `json:"adapter"`
	Expected *contextcompile.Expected   `json:"expected"`
	Grants   json.RawMessage            `json:"grants"`
	Scope    *policyartifact.Scope      `json:"scope"`
	Spec     *string                    `json:"spec"`
}

func (d acceptanceCandidateDoc) toDomain() (AcceptanceCandidate, error) {
	missing := func(field string) error {
		return fmt.Errorf("policyconflict: acceptance_candidate.%s is missing (absent or explicitly null)", field)
	}
	switch {
	case d.Adapter == nil:
		return AcceptanceCandidate{}, missing("adapter")
	case d.Expected == nil:
		return AcceptanceCandidate{}, missing("expected")
	case rawMessageMissing(d.Grants):
		return AcceptanceCandidate{}, missing("grants")
	case d.Scope == nil:
		return AcceptanceCandidate{}, missing("scope")
	case d.Spec == nil:
		return AcceptanceCandidate{}, missing("spec")
	}
	grants, err := decodeNestedGrants(d.Grants)
	if err != nil {
		return AcceptanceCandidate{}, fmt.Errorf("policyconflict: acceptance_candidate.grants: %w", err)
	}
	return AcceptanceCandidate{
		Adapter:  *d.Adapter,
		Expected: *d.Expected,
		Grants:   grants,
		Scope:    *d.Scope,
		Spec:     *d.Spec,
	}, nil
}

func acceptanceCandidateDocFor(c AcceptanceCandidate) (acceptanceCandidateDoc, error) {
	grantsRaw, err := encodeNestedGrants(c.Grants)
	if err != nil {
		return acceptanceCandidateDoc{}, fmt.Errorf("encoding acceptance_candidate.grants: %w", err)
	}
	adapter, expected, scope, spec := c.Adapter, c.Expected, c.Scope, c.Spec
	return acceptanceCandidateDoc{
		Adapter:  &adapter,
		Expected: &expected,
		Grants:   grantsRaw,
		Scope:    &scope,
		Spec:     &spec,
	}, nil
}

// targetDoc is Target's strict decode target: Kind is mandatory, and
// exactly the matching arm's raw bytes may be present (non-null); the
// other key must be entirely absent (design §2: "the other key is
// omitted").
type targetDoc struct {
	Kind                *string         `json:"kind"`
	AcceptedContext     json.RawMessage `json:"accepted_context,omitempty"`
	AcceptanceCandidate json.RawMessage `json:"acceptance_candidate,omitempty"`
}

func (d targetDoc) toDomain() (Target, error) {
	if d.Kind == nil {
		return Target{}, fmt.Errorf("policyconflict: target.kind is missing (absent or explicitly null)")
	}
	kind := TargetKind(*d.Kind)

	acceptedPresent := d.AcceptedContext != nil
	candidatePresent := d.AcceptanceCandidate != nil
	if acceptedPresent && candidatePresent {
		return Target{}, fmt.Errorf("policyconflict: target: both accepted_context and acceptance_candidate are present; exactly one arm is legal")
	}
	if !acceptedPresent && !candidatePresent {
		return Target{}, fmt.Errorf("policyconflict: target: neither accepted_context nor acceptance_candidate is present")
	}

	switch kind {
	case TargetAcceptedContext:
		if !acceptedPresent {
			return Target{}, fmt.Errorf("policyconflict: target.kind is %q but accepted_context is absent; acceptance_candidate must be entirely omitted for this kind", kind)
		}
		if rawMessageMissing(d.AcceptedContext) {
			return Target{}, fmt.Errorf("policyconflict: target.accepted_context is explicitly null")
		}
		req, err := decodeNestedAcceptedContext(d.AcceptedContext)
		if err != nil {
			return Target{}, fmt.Errorf("policyconflict: target.accepted_context: %w", err)
		}
		return Target{Kind: kind, AcceptedContext: &req}, nil
	case TargetAcceptanceCandidate:
		if !candidatePresent {
			return Target{}, fmt.Errorf("policyconflict: target.kind is %q but acceptance_candidate is absent; accepted_context must be entirely omitted for this kind", kind)
		}
		if rawMessageMissing(d.AcceptanceCandidate) {
			return Target{}, fmt.Errorf("policyconflict: target.acceptance_candidate is explicitly null")
		}
		var cd acceptanceCandidateDoc
		if err := artifact.DecodeExactJSON(d.AcceptanceCandidate, &cd); err != nil {
			return Target{}, fmt.Errorf("policyconflict: target.acceptance_candidate: %w", err)
		}
		candidate, err := cd.toDomain()
		if err != nil {
			return Target{}, fmt.Errorf("policyconflict: target.acceptance_candidate: %w", err)
		}
		return Target{Kind: kind, AcceptanceCandidate: &candidate}, nil
	default:
		return Target{}, fmt.Errorf("policyconflict: target.kind: unknown value %q", kind)
	}
}

func targetDocFor(t Target) (targetDoc, error) {
	kind := string(t.Kind)
	switch t.Kind {
	case TargetAcceptedContext:
		if t.AcceptedContext == nil {
			return targetDoc{}, fmt.Errorf("policyconflict: target: kind is accepted-context but accepted_context is nil")
		}
		raw, err := encodeNestedAcceptedContext(*t.AcceptedContext)
		if err != nil {
			return targetDoc{}, fmt.Errorf("encoding target.accepted_context: %w", err)
		}
		return targetDoc{Kind: &kind, AcceptedContext: raw}, nil
	case TargetAcceptanceCandidate:
		if t.AcceptanceCandidate == nil {
			return targetDoc{}, fmt.Errorf("policyconflict: target: kind is acceptance-candidate but acceptance_candidate is nil")
		}
		cd, err := acceptanceCandidateDocFor(*t.AcceptanceCandidate)
		if err != nil {
			return targetDoc{}, err
		}
		raw, err := canonjson.Marshal(cd)
		if err != nil {
			return targetDoc{}, fmt.Errorf("encoding target.acceptance_candidate: %w", err)
		}
		return targetDoc{Kind: &kind, AcceptanceCandidate: raw}, nil
	default:
		return targetDoc{}, fmt.Errorf("policyconflict: target: unknown kind %q", t.Kind)
	}
}

type requestDoc struct {
	Schema *string    `json:"schema"`
	Target *targetDoc `json:"target"`
}

func (d requestDoc) toDomain() (Request, error) {
	if d.Schema == nil {
		return Request{}, fmt.Errorf("policyconflict: request.schema is missing (absent or explicitly null)")
	}
	if d.Target == nil {
		return Request{}, fmt.Errorf("policyconflict: request.target is missing (absent or explicitly null)")
	}
	target, err := d.Target.toDomain()
	if err != nil {
		return Request{}, err
	}
	return Request{Schema: *d.Schema, Target: target}, nil
}

// DecodeRequest strictly decodes data as a `verdi.policy-conflict-request/v1`
// document: exact JSON (unknown fields, duplicate keys, trailing data, and
// invalid UTF-8 all fail closed), every mandatory field present and
// non-null, the strict accepted-context/acceptance-candidate union, full
// domain validation, and byte equality between data and this package's own
// canonical re-encoding of the decoded value.
func DecodeRequest(data []byte) (Request, error) {
	var doc requestDoc
	if err := artifact.DecodeExactJSON(data, &doc); err != nil {
		return Request{}, fmt.Errorf("policyconflict: decoding request: %w", err)
	}
	request, err := doc.toDomain()
	if err != nil {
		return Request{}, fmt.Errorf("policyconflict: decoding request: %w", err)
	}
	if err := request.Validate(); err != nil {
		return Request{}, fmt.Errorf("policyconflict: decoding request: %w", err)
	}
	canonical, err := EncodeRequest(request)
	if err != nil {
		return Request{}, fmt.Errorf("policyconflict: decoding request: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return Request{}, fmt.Errorf("policyconflict: decoding request: input bytes are not the canonical encoding of the document they decode to")
	}
	return request, nil
}

// EncodeRequest validates request and returns its canonical JSON encoding.
func EncodeRequest(request Request) ([]byte, error) {
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("policyconflict: encoding request: %w", err)
	}
	schema := request.Schema
	targetDoc, err := targetDocFor(request.Target)
	if err != nil {
		return nil, fmt.Errorf("policyconflict: encoding request: %w", err)
	}
	doc := requestDoc{Schema: &schema, Target: &targetDoc}
	out, err := canonjson.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("policyconflict: encoding request: %w", err)
	}
	return out, nil
}

// --- JudgeResult (authority design §6) --------------------------------------

type claimWitnessDoc struct {
	ID       *string `json:"id"`
	Digest   *string `json:"digest"`
	Category *string `json:"category"`
}

func (d claimWitnessDoc) toDomain(field string) (ClaimWitness, error) {
	if d.ID == nil || d.Digest == nil || d.Category == nil {
		return ClaimWitness{}, fmt.Errorf("policyconflict: %s: id, digest, and category are all required", field)
	}
	return ClaimWitness{ID: *d.ID, Digest: *d.Digest, Category: *d.Category}, nil
}

func claimWitnessDocFor(w ClaimWitness) claimWitnessDoc {
	id, digest, category := w.ID, w.Digest, w.Category
	return claimWitnessDoc{ID: &id, Digest: &digest, Category: &category}
}

type judgeFindingDoc struct {
	Claims      []claimWitnessDoc `json:"claims"`
	Categories  []string          `json:"categories"`
	Explanation *string           `json:"explanation"`
}

func (d judgeFindingDoc) toDomain(i int) (JudgeFinding, error) {
	if d.Claims == nil {
		return JudgeFinding{}, fmt.Errorf("policyconflict: findings[%d].claims is missing (absent or explicitly null)", i)
	}
	if d.Categories == nil {
		return JudgeFinding{}, fmt.Errorf("policyconflict: findings[%d].categories is missing (absent or explicitly null)", i)
	}
	if d.Explanation == nil {
		return JudgeFinding{}, fmt.Errorf("policyconflict: findings[%d].explanation is missing (absent or explicitly null)", i)
	}
	claims := make([]ClaimWitness, 0, len(d.Claims))
	for j, cd := range d.Claims {
		w, err := cd.toDomain(fmt.Sprintf("findings[%d].claims[%d]", i, j))
		if err != nil {
			return JudgeFinding{}, err
		}
		claims = append(claims, w)
	}
	return JudgeFinding{Claims: claims, Categories: d.Categories, Explanation: *d.Explanation}, nil
}

func judgeFindingDocFor(f JudgeFinding) judgeFindingDoc {
	claims := make([]claimWitnessDoc, len(f.Claims))
	for i, c := range f.Claims {
		claims[i] = claimWitnessDocFor(c)
	}
	explanation := f.Explanation
	categories := f.Categories
	if categories == nil {
		categories = []string{}
	}
	return judgeFindingDoc{Claims: claims, Categories: categories, Explanation: &explanation}
}

type judgeResultDoc struct {
	Schema         *string           `json:"schema"`
	Recommendation *string           `json:"recommendation"`
	Findings       []judgeFindingDoc `json:"findings"`
}

func (d judgeResultDoc) toDomain() (JudgeResult, error) {
	if d.Schema == nil {
		return JudgeResult{}, fmt.Errorf("policyconflict: judge result schema is missing (absent or explicitly null)")
	}
	if d.Recommendation == nil {
		return JudgeResult{}, fmt.Errorf("policyconflict: judge result recommendation is missing (absent or explicitly null)")
	}
	if d.Findings == nil {
		return JudgeResult{}, fmt.Errorf("policyconflict: judge result findings is missing (absent or explicitly null)")
	}
	findings := make([]JudgeFinding, 0, len(d.Findings))
	for i, fd := range d.Findings {
		f, err := fd.toDomain(i)
		if err != nil {
			return JudgeResult{}, err
		}
		findings = append(findings, f)
	}
	return JudgeResult{Schema: *d.Schema, Recommendation: Recommendation(*d.Recommendation), Findings: findings}, nil
}

func judgeResultDocFor(r JudgeResult) judgeResultDoc {
	schema := r.Schema
	recommendation := string(r.Recommendation)
	findings := make([]judgeFindingDoc, len(r.Findings))
	for i, f := range r.Findings {
		findings[i] = judgeFindingDocFor(f)
	}
	if findings == nil {
		findings = []judgeFindingDoc{}
	}
	return judgeResultDoc{Schema: &schema, Recommendation: &recommendation, Findings: findings}
}

// DecodeJudgeResult strictly decodes data as a
// `verdi.policy-conflict-judge-result/v1` document.
func DecodeJudgeResult(data []byte) (JudgeResult, error) {
	var doc judgeResultDoc
	if err := artifact.DecodeExactJSON(data, &doc); err != nil {
		return JudgeResult{}, fmt.Errorf("policyconflict: decoding judge result: %w", err)
	}
	result, err := doc.toDomain()
	if err != nil {
		return JudgeResult{}, fmt.Errorf("policyconflict: decoding judge result: %w", err)
	}
	if err := result.Validate(); err != nil {
		return JudgeResult{}, fmt.Errorf("policyconflict: decoding judge result: %w", err)
	}
	canonical, err := EncodeJudgeResult(result)
	if err != nil {
		return JudgeResult{}, fmt.Errorf("policyconflict: decoding judge result: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return JudgeResult{}, fmt.Errorf("policyconflict: decoding judge result: input bytes are not the canonical encoding of the document they decode to")
	}
	return result, nil
}

// EncodeJudgeResult validates result and returns its canonical JSON
// encoding.
func EncodeJudgeResult(result JudgeResult) ([]byte, error) {
	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("policyconflict: encoding judge result: %w", err)
	}
	out, err := canonjson.Marshal(judgeResultDocFor(result))
	if err != nil {
		return nil, fmt.Errorf("policyconflict: encoding judge result: %w", err)
	}
	return out, nil
}

// --- Judgment (authority design §7) -----------------------------------------

type judgmentExchangeDoc struct {
	Role          *string                    `json:"role"`
	Adapter       *contextcompile.AdapterRef `json:"adapter"`
	Model         *string                    `json:"model"`
	CommandDigest *string                    `json:"command_digest"`
	PromptDigest  *string                    `json:"prompt_digest"`
	InputDigest   *string                    `json:"input_digest"`
	RawResult     *string                    `json:"raw_result"`
	RawDigest     *string                    `json:"raw_digest"`
	Result        *judgeResultDoc            `json:"result"`
}

func (d judgmentExchangeDoc) toDomain() (JudgmentExchange, error) {
	missing := func(field string) error {
		return fmt.Errorf("policyconflict: judgment.exchange.%s is missing (absent or explicitly null)", field)
	}
	switch {
	case d.Role == nil:
		return JudgmentExchange{}, missing("role")
	case d.Adapter == nil:
		return JudgmentExchange{}, missing("adapter")
	case d.Model == nil:
		return JudgmentExchange{}, missing("model")
	case d.CommandDigest == nil:
		return JudgmentExchange{}, missing("command_digest")
	case d.PromptDigest == nil:
		return JudgmentExchange{}, missing("prompt_digest")
	case d.InputDigest == nil:
		return JudgmentExchange{}, missing("input_digest")
	case d.RawResult == nil:
		return JudgmentExchange{}, missing("raw_result")
	case d.RawDigest == nil:
		return JudgmentExchange{}, missing("raw_digest")
	case d.Result == nil:
		return JudgmentExchange{}, missing("result")
	}
	result, err := d.Result.toDomain()
	if err != nil {
		return JudgmentExchange{}, fmt.Errorf("policyconflict: judgment.exchange.result: %w", err)
	}
	return JudgmentExchange{
		Role:          JudgeRole(*d.Role),
		Adapter:       *d.Adapter,
		Model:         *d.Model,
		CommandDigest: *d.CommandDigest,
		PromptDigest:  *d.PromptDigest,
		InputDigest:   *d.InputDigest,
		RawResult:     *d.RawResult,
		RawDigest:     *d.RawDigest,
		Result:        result,
	}, nil
}

func judgmentExchangeDocFor(e JudgmentExchange) judgmentExchangeDoc {
	role := string(e.Role)
	adapter := e.Adapter
	model, cmd, prompt, input := e.Model, e.CommandDigest, e.PromptDigest, e.InputDigest
	raw, rawDigest := e.RawResult, e.RawDigest
	resultDoc := judgeResultDocFor(e.Result)
	return judgmentExchangeDoc{
		Role: &role, Adapter: &adapter, Model: &model,
		CommandDigest: &cmd, PromptDigest: &prompt, InputDigest: &input,
		RawResult: &raw, RawDigest: &rawDigest, Result: &resultDoc,
	}
}

// judgmentDoc is Judgment's strict decode target. Digest is a plain
// pointer (never omitted); it is checked against a fresh digestless
// recomputation by DecodeJudgment, mirroring
// internal/contextcompile.manifestDoc's own self-digest pattern.
type judgmentDoc struct {
	Schema      *string              `json:"schema"`
	TreeHash    *string              `json:"tree_hash"`
	InputDigest *string              `json:"input_digest"`
	Exchange    *judgmentExchangeDoc `json:"exchange"`
	Digest      *string              `json:"digest"`
}

func (d judgmentDoc) toDomain() (Judgment, error) {
	missing := func(field string) error {
		return fmt.Errorf("policyconflict: judgment.%s is missing (absent or explicitly null)", field)
	}
	switch {
	case d.Schema == nil:
		return Judgment{}, missing("schema")
	case d.TreeHash == nil:
		return Judgment{}, missing("tree_hash")
	case d.InputDigest == nil:
		return Judgment{}, missing("input_digest")
	case d.Exchange == nil:
		return Judgment{}, missing("exchange")
	case d.Digest == nil:
		return Judgment{}, missing("digest")
	}
	exchange, err := d.Exchange.toDomain()
	if err != nil {
		return Judgment{}, err
	}
	return Judgment{
		Schema:      *d.Schema,
		TreeHash:    *d.TreeHash,
		InputDigest: *d.InputDigest,
		Exchange:    exchange,
		Digest:      *d.Digest,
	}, nil
}

// judgmentDocFor builds the wire document for j with digest spliced in.
// Called twice by EncodeJudgment: once with digest == "" to compute the
// digestless canonical form the self digest is taken over, and once with
// the freshly computed digest for the final encoding — mirrors
// internal/contextcompile.manifestDocFor exactly.
func judgmentDocFor(j Judgment, digest string) judgmentDoc {
	schema, treeHash, inputDigest, dig := j.Schema, j.TreeHash, j.InputDigest, digest
	exchange := judgmentExchangeDocFor(j.Exchange)
	return judgmentDoc{
		Schema:      &schema,
		TreeHash:    &treeHash,
		InputDigest: &inputDigest,
		Exchange:    &exchange,
		Digest:      &dig,
	}
}

// DecodeJudgment strictly decodes data as a
// `verdi.policy-conflict-judgment/v1` document: exact JSON, every
// mandatory field present and non-null, full domain validation, the
// carried self digest checked against a fresh digestless recomputation,
// and byte equality between data and this package's own canonical
// re-encoding.
func DecodeJudgment(data []byte) (Judgment, error) {
	var doc judgmentDoc
	if err := artifact.DecodeExactJSON(data, &doc); err != nil {
		return Judgment{}, fmt.Errorf("policyconflict: decoding judgment: %w", err)
	}
	judgment, err := doc.toDomain()
	if err != nil {
		return Judgment{}, fmt.Errorf("policyconflict: decoding judgment: %w", err)
	}
	canonical, err := EncodeJudgment(judgment)
	if err != nil {
		return Judgment{}, fmt.Errorf("policyconflict: decoding judgment: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return Judgment{}, fmt.Errorf("policyconflict: decoding judgment: input bytes are not the canonical encoding of the document they decode to")
	}
	return judgment, nil
}

// EncodeJudgment validates judgment, recomputes its self digest from a
// digestless copy (any digest judgment already carries is discarded, never
// trusted), and returns the canonical JSON encoding carrying that fresh
// digest.
func EncodeJudgment(judgment Judgment) ([]byte, error) {
	digestless := judgment
	digestless.Digest = ""
	if err := digestless.Validate(); err != nil {
		return nil, fmt.Errorf("policyconflict: encoding judgment: %w", err)
	}
	digestlessDoc := judgmentDocFor(digestless, "")
	digest, err := canonjson.Digest(digestlessDoc)
	if err != nil {
		return nil, fmt.Errorf("policyconflict: encoding judgment: computing digest: %w", err)
	}
	out, err := canonjson.Marshal(judgmentDocFor(digestless, digest))
	if err != nil {
		return nil, fmt.Errorf("policyconflict: encoding judgment: %w", err)
	}
	return out, nil
}

// --- Report (authority design §10) ------------------------------------------
//
// Every nested type below carries `json` tags and decodes directly (no
// separate per-field wire mirror), matching internal/contextcompile's own
// established idiom for a schema whose nested fields need no absent/null
// distinction beyond "empty is invalid" — Report itself is the only type
// exempt, decoded through a private pointer-backed wire document instead.

type dimensionProofDoc struct {
	Dimension    string   `json:"dimension"`
	State        string   `json:"state"`
	Left         []string `json:"left"`
	Right        []string `json:"right"`
	Intersection []string `json:"intersection"`
	Witnesses    []string `json:"witnesses"`
}

type scopeProofDoc struct {
	State      string              `json:"state"`
	Dimensions []dimensionProofDoc `json:"dimensions"`
}

type solverProofDoc struct {
	State      string   `json:"state"`
	Domain     string   `json:"domain"`
	Values     []string `json:"values"`
	Required   []string `json:"required"`
	Forbidden  []string `json:"forbidden"`
	Minimum    *int     `json:"minimum,omitempty"`
	Maximum    *int     `json:"maximum,omitempty"`
	OpenDomain bool     `json:"open_domain"`
	Witnesses  []string `json:"witnesses"`
}

type typedClaimRecordDoc struct {
	PolicyID     string               `json:"policy_id"`
	PolicyDigest string               `json:"policy_digest"`
	ClaimDigest  string               `json:"claim_digest"`
	Claim        policyartifact.Claim `json:"claim"`
}

type authorityResolutionDoc struct {
	Match         string `json:"match"`
	Freshness     string `json:"freshness"`
	Scope         string `json:"scope"`
	Bound         string `json:"bound"`
	Authorization string `json:"authorization"`
}

type exemptionResolutionDoc struct {
	ID            string                 `json:"id"`
	Digest        string                 `json:"digest"`
	Resolution    authorityResolutionDoc `json:"resolution"`
	RemovedClaims []claimWitnessPlainDoc `json:"removed_claims"`
}

// claimWitnessPlainDoc is ClaimWitness's plain (non-pointer) nested-report
// wire shape — mandatory-non-null presence for these leaf fields is
// already implied by the enclosing mandatory array, matching
// internal/contextcompile's convention of using plain structs for nested
// report-only rows.
type claimWitnessPlainDoc struct {
	ID       string `json:"id"`
	Digest   string `json:"digest"`
	Category string `json:"category"`
}

type mechanicalEvaluationDoc struct {
	ID         string                   `json:"id"`
	Family     string                   `json:"family"`
	Subject    string                   `json:"subject"`
	Claims     []typedClaimRecordDoc    `json:"claims"`
	Scope      scopeProofDoc            `json:"scope"`
	Domain     string                   `json:"domain"`
	Before     solverProofDoc           `json:"before"`
	Exemptions []exemptionResolutionDoc `json:"exemptions"`
	After      solverProofDoc           `json:"after"`
	State      string                   `json:"state"`
	Reasons    []string                 `json:"reasons"`
}

type dispositionResolutionDoc struct {
	ID         string                 `json:"id"`
	Digest     string                 `json:"digest"`
	Conclusion string                 `json:"conclusion"`
	Resolution authorityResolutionDoc `json:"resolution"`
}

type semanticEvaluationDoc struct {
	ID            string                                `json:"id"`
	InputID       string                                `json:"input_id"`
	Claims        []policyartifact.SemanticClaimWitness `json:"claims"`
	UnknownScopes []scopeProofDoc                       `json:"unknown_scopes"`
	Primary       *judgmentExchangeDoc                  `json:"primary,omitempty"`
	Challenger    *judgmentExchangeDoc                  `json:"challenger,omitempty"`
	Dispositions  []dispositionResolutionDoc            `json:"dispositions"`
	State         string                                `json:"state"`
	Reasons       []string                              `json:"reasons"`
}

type acceptedIdentityDoc struct {
	ManifestDigest string `json:"manifest_digest"`
}

type candidateIdentityDoc struct {
	Ref           string                    `json:"ref"`
	Path          string                    `json:"path"`
	Branch        string                    `json:"branch"`
	Head          string                    `json:"head"`
	Blob          string                    `json:"blob"`
	ContentDigest string                    `json:"content_digest"`
	Scope         policyartifact.Scope      `json:"scope"`
	Adapter       contextcompile.AdapterRef `json:"adapter"`
	GrantDigest   string                    `json:"grant_digest"`
}

type targetIdentityDoc struct {
	Kind      string                `json:"kind"`
	Accepted  *acceptedIdentityDoc  `json:"accepted,omitempty"`
	Candidate *candidateIdentityDoc `json:"candidate,omitempty"`
}

type policyEntryIdentityDoc struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

type profileIdentityDoc struct {
	ID     string `json:"id"`
	Class  string `json:"class"`
	Digest string `json:"digest"`
}

type inputIdentityDoc struct {
	Target                targetIdentityDoc        `json:"target"`
	Repository            json.RawMessage          `json:"repository"`
	ConstitutionDigest    string                   `json:"constitution_digest"`
	EffectivePolicyDigest string                   `json:"effective_policy_digest"`
	PolicyEntries         []policyEntryIdentityDoc `json:"policy_entries"`
	Profile               profileIdentityDoc       `json:"profile"`
	EvaluatedOn           string                   `json:"evaluated_on"`
}

type disclosureDoc struct {
	Code      string   `json:"code"`
	Witnesses []string `json:"witnesses"`
}

// reportDoc is Report's strict decode target: every mandatory top-level
// field is a pointer (or, for input, a nested pointer struct) so an
// omitted or explicitly null field is distinguishable from an explicitly
// present one, mirroring internal/contextcompile.manifestDoc exactly.
type reportDoc struct {
	Schema      *string                   `json:"schema"`
	Input       *inputIdentityDoc         `json:"input"`
	Mechanical  []mechanicalEvaluationDoc `json:"mechanical"`
	Semantic    []semanticEvaluationDoc   `json:"semantic"`
	Disclosures []disclosureDoc           `json:"disclosures"`
	Verdict     *string                   `json:"verdict"`
	Digest      *string                   `json:"digest"`
}

func dimensionProofFromDoc(d dimensionProofDoc) DimensionProof {
	return DimensionProof{
		Dimension: d.Dimension, State: ScopeState(d.State),
		Left: d.Left, Right: d.Right, Intersection: d.Intersection, Witnesses: d.Witnesses,
	}
}

func dimensionProofDocFor(d DimensionProof) dimensionProofDoc {
	return dimensionProofDoc{
		Dimension: d.Dimension, State: string(d.State),
		Left: nonNilSlice(d.Left), Right: nonNilSlice(d.Right),
		Intersection: nonNilSlice(d.Intersection), Witnesses: nonNilSlice(d.Witnesses),
	}
}

func scopeProofFromDoc(d scopeProofDoc) ScopeProof {
	dims := make([]DimensionProof, len(d.Dimensions))
	for i, dd := range d.Dimensions {
		dims[i] = dimensionProofFromDoc(dd)
	}
	return ScopeProof{State: ScopeState(d.State), Dimensions: dims}
}

func scopeProofDocFor(p ScopeProof) scopeProofDoc {
	dims := make([]dimensionProofDoc, len(p.Dimensions))
	for i, d := range p.Dimensions {
		dims[i] = dimensionProofDocFor(d)
	}
	return scopeProofDoc{State: string(p.State), Dimensions: nonNilDimSlice(dims)}
}

func nonNilDimSlice(s []dimensionProofDoc) []dimensionProofDoc {
	if s != nil {
		return s
	}
	return []dimensionProofDoc{}
}

func solverProofFromDoc(d solverProofDoc) SolverProof {
	return SolverProof{
		State: SolverState(d.State), Domain: d.Domain,
		Values: d.Values, Required: d.Required, Forbidden: d.Forbidden,
		Minimum: d.Minimum, Maximum: d.Maximum, OpenDomain: d.OpenDomain, Witnesses: d.Witnesses,
	}
}

func solverProofDocFor(p SolverProof) solverProofDoc {
	return solverProofDoc{
		State: string(p.State), Domain: p.Domain,
		Values: nonNilSlice(p.Values), Required: nonNilSlice(p.Required), Forbidden: nonNilSlice(p.Forbidden),
		Minimum: p.Minimum, Maximum: p.Maximum, OpenDomain: p.OpenDomain, Witnesses: nonNilSlice(p.Witnesses),
	}
}

func typedClaimRecordFromDoc(d typedClaimRecordDoc) TypedClaimRecord {
	return TypedClaimRecord{PolicyID: d.PolicyID, PolicyDigest: d.PolicyDigest, ClaimDigest: d.ClaimDigest, Claim: d.Claim}
}

func typedClaimRecordDocFor(r TypedClaimRecord) typedClaimRecordDoc {
	return typedClaimRecordDoc{PolicyID: r.PolicyID, PolicyDigest: r.PolicyDigest, ClaimDigest: r.ClaimDigest, Claim: r.Claim}
}

func authorityResolutionFromDoc(d authorityResolutionDoc) AuthorityResolution {
	return AuthorityResolution{
		Match: ProofState(d.Match), Freshness: ProofState(d.Freshness), Scope: ProofState(d.Scope),
		Bound: ProofState(d.Bound), Authorization: ProofState(d.Authorization),
	}
}

func authorityResolutionDocFor(r AuthorityResolution) authorityResolutionDoc {
	return authorityResolutionDoc{
		Match: string(r.Match), Freshness: string(r.Freshness), Scope: string(r.Scope),
		Bound: string(r.Bound), Authorization: string(r.Authorization),
	}
}

func claimWitnessFromPlainDoc(d claimWitnessPlainDoc) ClaimWitness {
	return ClaimWitness{ID: d.ID, Digest: d.Digest, Category: d.Category}
}

func claimWitnessPlainDocFor(w ClaimWitness) claimWitnessPlainDoc {
	return claimWitnessPlainDoc{ID: w.ID, Digest: w.Digest, Category: w.Category}
}

func exemptionResolutionFromDoc(d exemptionResolutionDoc) ExemptionResolution {
	removed := make([]ClaimWitness, len(d.RemovedClaims))
	for i, rc := range d.RemovedClaims {
		removed[i] = claimWitnessFromPlainDoc(rc)
	}
	return ExemptionResolution{ID: d.ID, Digest: d.Digest, Resolution: authorityResolutionFromDoc(d.Resolution), RemovedClaims: removed}
}

func exemptionResolutionDocFor(e ExemptionResolution) exemptionResolutionDoc {
	removed := make([]claimWitnessPlainDoc, len(e.RemovedClaims))
	for i, rc := range e.RemovedClaims {
		removed[i] = claimWitnessPlainDocFor(rc)
	}
	return exemptionResolutionDoc{ID: e.ID, Digest: e.Digest, Resolution: authorityResolutionDocFor(e.Resolution), RemovedClaims: nonNilRemovedSlice(removed)}
}

func nonNilRemovedSlice(s []claimWitnessPlainDoc) []claimWitnessPlainDoc {
	if s != nil {
		return s
	}
	return []claimWitnessPlainDoc{}
}

func mechanicalEvaluationFromDoc(d mechanicalEvaluationDoc) MechanicalEvaluation {
	claims := make([]TypedClaimRecord, len(d.Claims))
	for i, c := range d.Claims {
		claims[i] = typedClaimRecordFromDoc(c)
	}
	exemptions := make([]ExemptionResolution, len(d.Exemptions))
	for i, e := range d.Exemptions {
		exemptions[i] = exemptionResolutionFromDoc(e)
	}
	reasons := make([]ReasonCode, len(d.Reasons))
	for i, r := range d.Reasons {
		reasons[i] = ReasonCode(r)
	}
	return MechanicalEvaluation{
		ID: d.ID, Family: policyartifact.Family(d.Family), Subject: d.Subject,
		Claims: claims, Scope: scopeProofFromDoc(d.Scope), Domain: d.Domain,
		Before: solverProofFromDoc(d.Before), Exemptions: exemptions, After: solverProofFromDoc(d.After),
		State: ProofState(d.State), Reasons: reasons,
	}
}

func mechanicalEvaluationDocFor(m MechanicalEvaluation) mechanicalEvaluationDoc {
	claims := make([]typedClaimRecordDoc, len(m.Claims))
	for i, c := range m.Claims {
		claims[i] = typedClaimRecordDocFor(c)
	}
	exemptions := make([]exemptionResolutionDoc, len(m.Exemptions))
	for i, e := range m.Exemptions {
		exemptions[i] = exemptionResolutionDocFor(e)
	}
	reasons := make([]string, len(m.Reasons))
	for i, r := range m.Reasons {
		reasons[i] = string(r)
	}
	return mechanicalEvaluationDoc{
		ID: m.ID, Family: string(m.Family), Subject: m.Subject,
		Claims: nonNilClaimSlice(claims), Scope: scopeProofDocFor(m.Scope), Domain: m.Domain,
		Before: solverProofDocFor(m.Before), Exemptions: nonNilExemptionSlice(exemptions), After: solverProofDocFor(m.After),
		State: string(m.State), Reasons: nonNilSlice(reasons),
	}
}

func nonNilClaimSlice(s []typedClaimRecordDoc) []typedClaimRecordDoc {
	if s != nil {
		return s
	}
	return []typedClaimRecordDoc{}
}

func nonNilExemptionSlice(s []exemptionResolutionDoc) []exemptionResolutionDoc {
	if s != nil {
		return s
	}
	return []exemptionResolutionDoc{}
}

func dispositionResolutionFromDoc(d dispositionResolutionDoc) DispositionResolution {
	return DispositionResolution{
		ID: d.ID, Digest: d.Digest,
		Conclusion: policyartifact.DispositionConclusion(d.Conclusion),
		Resolution: authorityResolutionFromDoc(d.Resolution),
	}
}

func dispositionResolutionDocFor(d DispositionResolution) dispositionResolutionDoc {
	return dispositionResolutionDoc{
		ID: d.ID, Digest: d.Digest,
		Conclusion: string(d.Conclusion),
		Resolution: authorityResolutionDocFor(d.Resolution),
	}
}

func semanticEvaluationFromDoc(d semanticEvaluationDoc) SemanticEvaluation {
	unknownScopes := make([]ScopeProof, len(d.UnknownScopes))
	for i, s := range d.UnknownScopes {
		unknownScopes[i] = scopeProofFromDoc(s)
	}
	dispositions := make([]DispositionResolution, len(d.Dispositions))
	for i, disp := range d.Dispositions {
		dispositions[i] = dispositionResolutionFromDoc(disp)
	}
	reasons := make([]ReasonCode, len(d.Reasons))
	for i, r := range d.Reasons {
		reasons[i] = ReasonCode(r)
	}
	var primary, challenger *JudgmentExchange
	if d.Primary != nil {
		p, err := d.Primary.toDomain()
		if err == nil {
			primary = &p
		}
	}
	if d.Challenger != nil {
		c, err := d.Challenger.toDomain()
		if err == nil {
			challenger = &c
		}
	}
	return SemanticEvaluation{
		ID: d.ID, InputID: d.InputID, Claims: d.Claims, UnknownScopes: unknownScopes,
		Primary: primary, Challenger: challenger, Dispositions: dispositions,
		State: ProofState(d.State), Reasons: reasons,
	}
}

func semanticEvaluationDocFor(s SemanticEvaluation) semanticEvaluationDoc {
	unknownScopes := make([]scopeProofDoc, len(s.UnknownScopes))
	for i, u := range s.UnknownScopes {
		unknownScopes[i] = scopeProofDocFor(u)
	}
	dispositions := make([]dispositionResolutionDoc, len(s.Dispositions))
	for i, d := range s.Dispositions {
		dispositions[i] = dispositionResolutionDocFor(d)
	}
	reasons := make([]string, len(s.Reasons))
	for i, r := range s.Reasons {
		reasons[i] = string(r)
	}
	var primary, challenger *judgmentExchangeDoc
	if s.Primary != nil {
		pd := judgmentExchangeDocFor(*s.Primary)
		primary = &pd
	}
	if s.Challenger != nil {
		cd := judgmentExchangeDocFor(*s.Challenger)
		challenger = &cd
	}
	claims := s.Claims
	if claims == nil {
		claims = []policyartifact.SemanticClaimWitness{}
	}
	return semanticEvaluationDoc{
		ID: s.ID, InputID: s.InputID, Claims: claims, UnknownScopes: nonNilUnknownScopeSlice(unknownScopes),
		Primary: primary, Challenger: challenger, Dispositions: nonNilDispositionSlice(dispositions),
		State: string(s.State), Reasons: nonNilSlice(reasons),
	}
}

func nonNilUnknownScopeSlice(s []scopeProofDoc) []scopeProofDoc {
	if s != nil {
		return s
	}
	return []scopeProofDoc{}
}

func nonNilDispositionSlice(s []dispositionResolutionDoc) []dispositionResolutionDoc {
	if s != nil {
		return s
	}
	return []dispositionResolutionDoc{}
}

func targetIdentityFromDoc(d targetIdentityDoc) TargetIdentity {
	var accepted *AcceptedIdentity
	var candidate *CandidateIdentity
	if d.Accepted != nil {
		accepted = &AcceptedIdentity{ManifestDigest: d.Accepted.ManifestDigest}
	}
	if d.Candidate != nil {
		candidate = &CandidateIdentity{
			Ref: d.Candidate.Ref, Path: d.Candidate.Path, Branch: d.Candidate.Branch,
			Head: d.Candidate.Head, Blob: d.Candidate.Blob, ContentDigest: d.Candidate.ContentDigest,
			Scope: d.Candidate.Scope, Adapter: d.Candidate.Adapter, GrantDigest: d.Candidate.GrantDigest,
		}
	}
	return TargetIdentity{Kind: TargetKind(d.Kind), Accepted: accepted, Candidate: candidate}
}

func targetIdentityDocFor(t TargetIdentity) targetIdentityDoc {
	var accepted *acceptedIdentityDoc
	var candidate *candidateIdentityDoc
	if t.Accepted != nil {
		accepted = &acceptedIdentityDoc{ManifestDigest: t.Accepted.ManifestDigest}
	}
	if t.Candidate != nil {
		candidate = &candidateIdentityDoc{
			Ref: t.Candidate.Ref, Path: t.Candidate.Path, Branch: t.Candidate.Branch,
			Head: t.Candidate.Head, Blob: t.Candidate.Blob, ContentDigest: t.Candidate.ContentDigest,
			Scope: t.Candidate.Scope, Adapter: t.Candidate.Adapter, GrantDigest: t.Candidate.GrantDigest,
		}
	}
	return targetIdentityDoc{Kind: string(t.Kind), Accepted: accepted, Candidate: candidate}
}

func inputIdentityFromDoc(d inputIdentityDoc) (InputIdentity, error) {
	canonicalRepo, err := canonjson.Marshal(d.Repository)
	if err != nil {
		return InputIdentity{}, fmt.Errorf("re-encoding input.repository: %w", err)
	}
	var repo repositoryfacts.Facts
	if err := artifact.DecodeExactJSON(canonicalRepo, &repo); err != nil {
		return InputIdentity{}, fmt.Errorf("input.repository: %w", err)
	}
	entries := make([]PolicyEntryIdentity, len(d.PolicyEntries))
	for i, e := range d.PolicyEntries {
		entries[i] = PolicyEntryIdentity{Kind: e.Kind, ID: e.ID, Digest: e.Digest}
	}
	return InputIdentity{
		Target:                targetIdentityFromDoc(d.Target),
		Repository:            repo,
		ConstitutionDigest:    d.ConstitutionDigest,
		EffectivePolicyDigest: d.EffectivePolicyDigest,
		PolicyEntries:         entries,
		Profile:               ProfileIdentity{ID: d.Profile.ID, Class: d.Profile.Class, Digest: d.Profile.Digest},
		EvaluatedOn:           d.EvaluatedOn,
	}, nil
}

func inputIdentityDocFor(i InputIdentity) (inputIdentityDoc, error) {
	repoRaw, err := canonjson.Marshal(i.Repository)
	if err != nil {
		return inputIdentityDoc{}, fmt.Errorf("encoding input.repository: %w", err)
	}
	entries := make([]policyEntryIdentityDoc, len(i.PolicyEntries))
	for idx, e := range i.PolicyEntries {
		entries[idx] = policyEntryIdentityDoc{Kind: e.Kind, ID: e.ID, Digest: e.Digest}
	}
	return inputIdentityDoc{
		Target:                targetIdentityDocFor(i.Target),
		Repository:            repoRaw,
		ConstitutionDigest:    i.ConstitutionDigest,
		EffectivePolicyDigest: i.EffectivePolicyDigest,
		PolicyEntries:         nonNilPolicyEntrySlice(entries),
		Profile:               profileIdentityDoc{ID: i.Profile.ID, Class: i.Profile.Class, Digest: i.Profile.Digest},
		EvaluatedOn:           i.EvaluatedOn,
	}, nil
}

func nonNilPolicyEntrySlice(s []policyEntryIdentityDoc) []policyEntryIdentityDoc {
	if s != nil {
		return s
	}
	return []policyEntryIdentityDoc{}
}

func nonNilSlice[T any](s []T) []T {
	if s != nil {
		return s
	}
	return []T{}
}

// DecodeReport strictly decodes data as a `verdi.policy-conflict-report/v1`
// document.
func DecodeReport(data []byte) (Report, error) {
	var doc reportDoc
	if err := artifact.DecodeExactJSON(data, &doc); err != nil {
		return Report{}, fmt.Errorf("policyconflict: decoding report: %w", err)
	}
	report, err := reportFromDoc(doc)
	if err != nil {
		return Report{}, fmt.Errorf("policyconflict: decoding report: %w", err)
	}
	canonical, err := EncodeReport(report)
	if err != nil {
		return Report{}, fmt.Errorf("policyconflict: decoding report: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return Report{}, fmt.Errorf("policyconflict: decoding report: input bytes are not the canonical encoding of the document they decode to")
	}
	return report, nil
}

func reportFromDoc(d reportDoc) (Report, error) {
	missing := func(field string) error {
		return fmt.Errorf("report.%s is missing (absent or explicitly null)", field)
	}
	switch {
	case d.Schema == nil:
		return Report{}, missing("schema")
	case d.Input == nil:
		return Report{}, missing("input")
	case d.Mechanical == nil:
		return Report{}, missing("mechanical")
	case d.Semantic == nil:
		return Report{}, missing("semantic")
	case d.Disclosures == nil:
		return Report{}, missing("disclosures")
	case d.Verdict == nil:
		return Report{}, missing("verdict")
	case d.Digest == nil:
		return Report{}, missing("digest")
	}
	input, err := inputIdentityFromDoc(*d.Input)
	if err != nil {
		return Report{}, fmt.Errorf("report.input: %w", err)
	}
	mechanical := make([]MechanicalEvaluation, len(d.Mechanical))
	for i, m := range d.Mechanical {
		mechanical[i] = mechanicalEvaluationFromDoc(m)
	}
	semantic := make([]SemanticEvaluation, len(d.Semantic))
	for i, s := range d.Semantic {
		semantic[i] = semanticEvaluationFromDoc(s)
	}
	disclosures := make([]Disclosure, len(d.Disclosures))
	for i, disc := range d.Disclosures {
		disclosures[i] = Disclosure{Code: DisclosureCode(disc.Code), Witnesses: disc.Witnesses}
	}
	return Report{
		Schema: *d.Schema, Input: input, Mechanical: mechanical, Semantic: semantic,
		Disclosures: disclosures, Verdict: Verdict(*d.Verdict), Digest: *d.Digest,
	}, nil
}

func reportDocFor(r Report, digest string) (reportDoc, error) {
	inputDoc, err := inputIdentityDocFor(r.Input)
	if err != nil {
		return reportDoc{}, err
	}
	mechanical := make([]mechanicalEvaluationDoc, len(r.Mechanical))
	for i, m := range r.Mechanical {
		mechanical[i] = mechanicalEvaluationDocFor(m)
	}
	semantic := make([]semanticEvaluationDoc, len(r.Semantic))
	for i, s := range r.Semantic {
		semantic[i] = semanticEvaluationDocFor(s)
	}
	disclosures := make([]disclosureDoc, len(r.Disclosures))
	for i, disc := range r.Disclosures {
		disclosures[i] = disclosureDoc{Code: string(disc.Code), Witnesses: nonNilSlice(disc.Witnesses)}
	}
	schema, verdict, dig := r.Schema, string(r.Verdict), digest
	return reportDoc{
		Schema: &schema, Input: &inputDoc,
		Mechanical: nonNilMechSlice(mechanical), Semantic: nonNilSemSlice(semantic),
		Disclosures: nonNilDisclosureSlice(disclosures), Verdict: &verdict, Digest: &dig,
	}, nil
}

func nonNilMechSlice(s []mechanicalEvaluationDoc) []mechanicalEvaluationDoc {
	if s != nil {
		return s
	}
	return []mechanicalEvaluationDoc{}
}

func nonNilSemSlice(s []semanticEvaluationDoc) []semanticEvaluationDoc {
	if s != nil {
		return s
	}
	return []semanticEvaluationDoc{}
}

func nonNilDisclosureSlice(s []disclosureDoc) []disclosureDoc {
	if s != nil {
		return s
	}
	return []disclosureDoc{}
}

// EncodeReport validates report, recomputes its self digest from a
// digestless copy, and returns the canonical JSON encoding carrying that
// fresh digest — mirrors internal/contextcompile.EncodeManifest exactly.
func EncodeReport(report Report) ([]byte, error) {
	digestless := report
	digestless.Digest = ""
	if err := digestless.Validate(); err != nil {
		return nil, fmt.Errorf("policyconflict: encoding report: %w", err)
	}
	digestlessDoc, err := reportDocFor(digestless, "")
	if err != nil {
		return nil, fmt.Errorf("policyconflict: encoding report: %w", err)
	}
	digest, err := canonjson.Digest(digestlessDoc)
	if err != nil {
		return nil, fmt.Errorf("policyconflict: encoding report: computing digest: %w", err)
	}
	finalDoc, err := reportDocFor(digestless, digest)
	if err != nil {
		return nil, fmt.Errorf("policyconflict: encoding report: %w", err)
	}
	out, err := canonjson.Marshal(finalDoc)
	if err != nil {
		return nil, fmt.Errorf("policyconflict: encoding report: %w", err)
	}
	return out, nil
}
