package contextcompile

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// --- shared grant-seam splice helpers ---------------------------------------

// nullLiteral is the raw JSON literal for "explicitly present as null" —
// used to detect an explicit null on a bare json.RawMessage field, which
// (unlike a plain slice or a pointer field) captures null's own four bytes
// rather than collapsing to Go nil.
var nullLiteral = []byte("null")

func rawMessageMissing(raw json.RawMessage) bool {
	return raw == nil || bytes.Equal(raw, nullLiteral)
}

// decodeNestedGrants re-encodes nested's exact bytes canonically (whatever
// their original formatting/key order) and decodes the result through
// execworkspace.DecodeGrantSet — the only grant seam (plan Task 1 Step 2:
// "decode the nested grants by re-encoding that exact nested document
// canonically and calling execworkspace.DecodeGrantSet"). This package
// never reimplements grant kind/payload validation.
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

// encodeNestedGrants renders set through execworkspace.EncodeGrantSet — the
// only grant seam — and returns the resulting canonical bytes as a raw
// message ready to splice into an enclosing wire document.
func encodeNestedGrants(set execworkspace.GrantSet) (json.RawMessage, error) {
	data, err := execworkspace.EncodeGrantSet(set)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// --- Request (authority design §3) ------------------------------------------

// requestDoc is Request's strict decode target. Every top-level field is a
// pointer (or, for grants, a bare json.RawMessage — see rawMessageMissing)
// so an omitted or explicitly null field is distinguishable from an
// explicitly present one; Expected alone is genuinely optional.
type requestDoc struct {
	Schema   *string               `json:"schema"`
	Adapter  *AdapterRef           `json:"adapter"`
	Expected *Expected             `json:"expected,omitempty"`
	Grants   json.RawMessage       `json:"grants"`
	Phase    *string               `json:"phase"`
	Scope    *policyartifact.Scope `json:"scope"`
	Spec     *string               `json:"spec"`
}

func (d requestDoc) toDomain() (Request, error) {
	missing := func(field string) error {
		return fmt.Errorf("contextcompile: request.%s is missing (absent or explicitly null)", field)
	}
	switch {
	case d.Schema == nil:
		return Request{}, missing("schema")
	case d.Adapter == nil:
		return Request{}, missing("adapter")
	case rawMessageMissing(d.Grants):
		return Request{}, missing("grants")
	case d.Phase == nil:
		return Request{}, missing("phase")
	case d.Scope == nil:
		return Request{}, missing("scope")
	case d.Spec == nil:
		return Request{}, missing("spec")
	}

	grants, err := decodeNestedGrants(d.Grants)
	if err != nil {
		return Request{}, fmt.Errorf("contextcompile: request.grants: %w", err)
	}

	return Request{
		Schema:   *d.Schema,
		Adapter:  *d.Adapter,
		Expected: d.Expected,
		Grants:   grants,
		Phase:    Phase(*d.Phase),
		Scope:    *d.Scope,
		Spec:     *d.Spec,
	}, nil
}

func requestDocFor(r Request) (requestDoc, error) {
	grantsRaw, err := encodeNestedGrants(r.Grants)
	if err != nil {
		return requestDoc{}, fmt.Errorf("contextcompile: encoding request.grants: %w", err)
	}
	schema, phase, spec := r.Schema, string(r.Phase), r.Spec
	adapter, scope := r.Adapter, r.Scope
	return requestDoc{
		Schema:   &schema,
		Adapter:  &adapter,
		Expected: r.Expected,
		Grants:   grantsRaw,
		Phase:    &phase,
		Scope:    &scope,
		Spec:     &spec,
	}, nil
}

// DecodeRequest strictly decodes data as a `verdi.context-compile-request/v1`
// document: exact JSON (unknown fields, duplicate keys, and trailing data
// all fail closed; internal/artifact.DecodeExactJSON), every mandatory
// field present and non-null, the nested grants document decoded through
// the one execworkspace grant seam, full domain validation, and byte
// equality between data and this package's own canonical re-encoding of
// the decoded value. A canonical, schema-valid request whose phase falls
// outside a nonempty scope.phases is returned as *PhaseScopeRefusal, not a
// generic error — see PhaseScopeRefusal.
func DecodeRequest(data []byte) (Request, error) {
	var doc requestDoc
	if err := artifact.DecodeExactJSON(data, &doc); err != nil {
		return Request{}, fmt.Errorf("contextcompile: decoding request: %w", err)
	}
	request, err := doc.toDomain()
	if err != nil {
		return Request{}, fmt.Errorf("contextcompile: decoding request: %w", err)
	}
	canonical, err := EncodeRequest(request)
	if err != nil {
		return Request{}, fmt.Errorf("contextcompile: decoding request: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return Request{}, fmt.Errorf("contextcompile: decoding request: input bytes are not the canonical encoding of the document they decode to")
	}
	return request, nil
}

// EncodeRequest validates request and returns its canonical JSON encoding
// (sorted keys, no HTML escaping, one trailing newline —
// internal/canonjson.Marshal). The nested grants document is rendered
// through execworkspace.EncodeGrantSet, the only grant seam.
func EncodeRequest(request Request) ([]byte, error) {
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("contextcompile: encoding request: %w", err)
	}
	doc, err := requestDocFor(request)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: encoding request: %w", err)
	}
	out, err := canonjson.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: encoding request: %w", err)
	}
	return out, nil
}

// --- Manifest (authority design §8.2) ---------------------------------------

// revisionsWireDoc is Revisions's strict decode target. Parent is a bare
// json.RawMessage (not a pointer): absent leaves it Go-nil, but ANY
// present value — including an explicit null — makes RawMessage's own
// UnmarshalJSON copy those exact bytes, so it becomes non-nil. That is
// exactly the check v1 needs: revision 1 never carries a parent at all
// (authority design §9), so presence in any form is the failure, not a
// particular value.
type revisionsWireDoc struct {
	Authority *string         `json:"authority"`
	Context   *int            `json:"context"`
	Parent    json.RawMessage `json:"parent,omitempty"`
}

func (d revisionsWireDoc) toDomain() (Revisions, error) {
	if d.Authority == nil {
		return Revisions{}, fmt.Errorf("contextcompile: manifest.revisions.authority is missing (absent or explicitly null)")
	}
	if d.Context == nil {
		return Revisions{}, fmt.Errorf("contextcompile: manifest.revisions.context is missing (absent or explicitly null)")
	}
	if d.Parent != nil {
		return Revisions{}, fmt.Errorf("contextcompile: manifest.revisions.parent: must be omitted (v1's root context revision 1 has no parent)")
	}
	return Revisions{Authority: *d.Authority, Context: *d.Context}, nil
}

// manifestDoc is Manifest's strict decode target. Dispositions and
// Expansions are bare []json.RawMessage: v1 requires them present as an
// exactly-empty array, so both an absent/null key (nil slice) and a
// nonempty array fail closed, and only "[]" (a non-nil, zero-length
// slice) decodes cleanly.
type manifestDoc struct {
	Schema            *string               `json:"schema"`
	Phase             *string               `json:"phase"`
	Adapter           *AdapterRef           `json:"adapter"`
	Revisions         *revisionsWireDoc     `json:"revisions"`
	AcceptedSpec      *AcceptedSpec         `json:"accepted_spec"`
	ParentFeatures    []ParentFeature       `json:"parent_features"`
	Decisions         []DecisionRef         `json:"decisions"`
	Obligations       []Obligation          `json:"obligations"`
	Repository        *RepositoryFacts      `json:"repository"`
	Policy            *PolicySection        `json:"policy"`
	Dispositions      []json.RawMessage     `json:"dispositions"`
	Owners            []string              `json:"owners"`
	Scope             *policyartifact.Scope `json:"scope"`
	GovernanceProfile *GovernanceProfileRef `json:"governance_profile"`
	Actors            *ActorsSection        `json:"actors"`
	Included          []IncludedEntry       `json:"included"`
	Excluded          []ExcludedEntry       `json:"excluded"`
	Opaque            []OpaqueEntry         `json:"opaque"`
	Capabilities      json.RawMessage       `json:"capabilities"`
	ProjectionFiles   []ProjectionFileRef   `json:"projection_files"`
	RequiredInputs    []RequiredInput       `json:"required_inputs"`
	Evidence          *EvidenceSection      `json:"evidence"`
	Disclosures       []DisclosureCode      `json:"disclosures"`
	Expansions        []json.RawMessage     `json:"expansions"`
	Digest            *string               `json:"digest"`
}

func (d manifestDoc) toDomain() (Manifest, error) {
	missing := func(field string) error {
		return fmt.Errorf("contextcompile: manifest.%s is missing (absent or explicitly null)", field)
	}
	switch {
	case d.Schema == nil:
		return Manifest{}, missing("schema")
	case d.Phase == nil:
		return Manifest{}, missing("phase")
	case d.Adapter == nil:
		return Manifest{}, missing("adapter")
	case d.Revisions == nil:
		return Manifest{}, missing("revisions")
	case d.AcceptedSpec == nil:
		return Manifest{}, missing("accepted_spec")
	case d.ParentFeatures == nil:
		return Manifest{}, missing("parent_features")
	case d.Decisions == nil:
		return Manifest{}, missing("decisions")
	case d.Obligations == nil:
		return Manifest{}, missing("obligations")
	case d.Repository == nil:
		return Manifest{}, missing("repository")
	case d.Policy == nil:
		return Manifest{}, missing("policy")
	case d.Dispositions == nil:
		return Manifest{}, missing("dispositions")
	case d.Owners == nil:
		return Manifest{}, missing("owners")
	case d.Scope == nil:
		return Manifest{}, missing("scope")
	case d.GovernanceProfile == nil:
		return Manifest{}, missing("governance_profile")
	case d.Actors == nil:
		return Manifest{}, missing("actors")
	case d.Included == nil:
		return Manifest{}, missing("included")
	case d.Excluded == nil:
		return Manifest{}, missing("excluded")
	case d.Opaque == nil:
		return Manifest{}, missing("opaque")
	case rawMessageMissing(d.Capabilities):
		return Manifest{}, missing("capabilities")
	case d.ProjectionFiles == nil:
		return Manifest{}, missing("projection_files")
	case d.RequiredInputs == nil:
		return Manifest{}, missing("required_inputs")
	case d.Evidence == nil:
		return Manifest{}, missing("evidence")
	case d.Disclosures == nil:
		return Manifest{}, missing("disclosures")
	case d.Expansions == nil:
		return Manifest{}, missing("expansions")
	case d.Digest == nil:
		return Manifest{}, missing("digest")
	}
	if len(d.Dispositions) != 0 {
		return Manifest{}, fmt.Errorf("contextcompile: manifest.dispositions: must be [] in v1, got %d entries", len(d.Dispositions))
	}
	if len(d.Expansions) != 0 {
		return Manifest{}, fmt.Errorf("contextcompile: manifest.expansions: must be [] in v1, got %d entries", len(d.Expansions))
	}

	revisions, err := d.Revisions.toDomain()
	if err != nil {
		return Manifest{}, err
	}
	capabilities, err := decodeNestedGrants(d.Capabilities)
	if err != nil {
		return Manifest{}, fmt.Errorf("contextcompile: manifest.capabilities: %w", err)
	}

	return Manifest{
		Schema:            *d.Schema,
		Phase:             Phase(*d.Phase),
		Adapter:           *d.Adapter,
		Revisions:         revisions,
		AcceptedSpec:      *d.AcceptedSpec,
		ParentFeatures:    d.ParentFeatures,
		Decisions:         d.Decisions,
		Obligations:       d.Obligations,
		Repository:        *d.Repository,
		Policy:            *d.Policy,
		Owners:            d.Owners,
		Scope:             *d.Scope,
		GovernanceProfile: *d.GovernanceProfile,
		Actors:            *d.Actors,
		Included:          d.Included,
		Excluded:          d.Excluded,
		Opaque:            d.Opaque,
		Capabilities:      capabilities,
		ProjectionFiles:   d.ProjectionFiles,
		RequiredInputs:    d.RequiredInputs,
		Evidence:          *d.Evidence,
		Disclosures:       d.Disclosures,
		Digest:            *d.Digest,
	}, nil
}

// nonNilSlice returns s unchanged if it is already non-nil, else a
// zero-length non-nil slice of the same type — so a mandatory collection
// always serializes as "[]", never "null" (plan: "mandatory empty
// collections serialize as explicit []").
func nonNilSlice[T any](s []T) []T {
	if s != nil {
		return s
	}
	return []T{}
}

// manifestDocFor builds the wire document for m with digest spliced in.
// Called twice by EncodeManifest: once with digest == "" to compute the
// digestless canonical form the self digest is taken over, and once with
// the freshly computed digest for the final encoding.
func manifestDocFor(m Manifest, digest string) (manifestDoc, error) {
	capabilitiesRaw, err := encodeNestedGrants(m.Capabilities)
	if err != nil {
		return manifestDoc{}, fmt.Errorf("encoding manifest.capabilities: %w", err)
	}
	schema, phase, dig := m.Schema, string(m.Phase), digest
	adapter, acceptedSpec := m.Adapter, m.AcceptedSpec
	repository, policy := m.Repository, m.Policy
	scope, governanceProfile, actors, evidence := m.Scope, m.GovernanceProfile, m.Actors, m.Evidence
	revisions := revisionsWireDoc{Authority: &m.Revisions.Authority, Context: &m.Revisions.Context}

	return manifestDoc{
		Schema:            &schema,
		Phase:             &phase,
		Adapter:           &adapter,
		Revisions:         &revisions,
		AcceptedSpec:      &acceptedSpec,
		ParentFeatures:    nonNilSlice(m.ParentFeatures),
		Decisions:         nonNilSlice(m.Decisions),
		Obligations:       nonNilSlice(m.Obligations),
		Repository:        &repository,
		Policy:            &policy,
		Dispositions:      []json.RawMessage{},
		Owners:            nonNilSlice(m.Owners),
		Scope:             &scope,
		GovernanceProfile: &governanceProfile,
		Actors:            &actors,
		Included:          nonNilSlice(m.Included),
		Excluded:          nonNilSlice(m.Excluded),
		Opaque:            nonNilSlice(m.Opaque),
		Capabilities:      capabilitiesRaw,
		ProjectionFiles:   nonNilSlice(m.ProjectionFiles),
		RequiredInputs:    nonNilSlice(m.RequiredInputs),
		Evidence:          &evidence,
		Disclosures:       nonNilSlice(m.Disclosures),
		Expansions:        []json.RawMessage{},
		Digest:            &dig,
	}, nil
}

// DecodeManifest strictly decodes data as a `verdi.context-manifest/v1`
// document: exact JSON, every mandatory section present and non-null,
// dispositions/expansions/revisions.parent rejected unless exactly empty/
// absent, the nested capabilities document decoded through the one
// execworkspace grant seam, full domain validation, the carried self
// digest checked against a fresh digestless recomputation, and byte
// equality between data and this package's own canonical re-encoding.
func DecodeManifest(data []byte) (Manifest, error) {
	var doc manifestDoc
	if err := artifact.DecodeExactJSON(data, &doc); err != nil {
		return Manifest{}, fmt.Errorf("contextcompile: decoding manifest: %w", err)
	}
	manifest, err := doc.toDomain()
	if err != nil {
		return Manifest{}, fmt.Errorf("contextcompile: decoding manifest: %w", err)
	}
	canonical, err := EncodeManifest(manifest)
	if err != nil {
		return Manifest{}, fmt.Errorf("contextcompile: decoding manifest: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return Manifest{}, fmt.Errorf("contextcompile: decoding manifest: input bytes are not the canonical encoding of the document they decode to")
	}
	return manifest, nil
}

// EncodeManifest validates manifest, recomputes its self digest from a
// digestless copy (any digest manifest already carries is discarded, never
// trusted — no constructor accepts a caller-supplied self digest), and
// returns the canonical JSON encoding carrying that fresh digest.
func EncodeManifest(manifest Manifest) ([]byte, error) {
	digestless := manifest
	digestless.Digest = ""
	if err := digestless.Validate(); err != nil {
		return nil, fmt.Errorf("contextcompile: encoding manifest: %w", err)
	}
	digestlessDoc, err := manifestDocFor(digestless, "")
	if err != nil {
		return nil, fmt.Errorf("contextcompile: encoding manifest: %w", err)
	}
	digest, err := canonjson.Digest(digestlessDoc)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: encoding manifest: computing digest: %w", err)
	}

	finalDoc, err := manifestDocFor(digestless, digest)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: encoding manifest: %w", err)
	}
	out, err := canonjson.Marshal(finalDoc)
	if err != nil {
		return nil, fmt.Errorf("contextcompile: encoding manifest: %w", err)
	}
	return out, nil
}

// --- DataItem (authority design §8.1) ---------------------------------------

// dataItemDoc is DataItem's strict decode target.
type dataItemDoc struct {
	Schema         *string `json:"schema"`
	ID             *string `json:"id"`
	Source         *string `json:"source"`
	Kind           *string `json:"kind"`
	Path           *string `json:"path,omitempty"`
	Ref            *string `json:"ref,omitempty"`
	Classification *string `json:"classification"`
	ContentDigest  *string `json:"content_digest"`
	Content        *string `json:"content"`
	Digest         *string `json:"digest"`
}

func (d dataItemDoc) toDomain() (DataItem, error) {
	missing := func(field string) error {
		return fmt.Errorf("contextcompile: data item.%s is missing (absent or explicitly null)", field)
	}
	switch {
	case d.Schema == nil:
		return DataItem{}, missing("schema")
	case d.ID == nil:
		return DataItem{}, missing("id")
	case d.Source == nil:
		return DataItem{}, missing("source")
	case d.Kind == nil:
		return DataItem{}, missing("kind")
	case d.Classification == nil:
		return DataItem{}, missing("classification")
	case d.ContentDigest == nil:
		return DataItem{}, missing("content_digest")
	case d.Content == nil:
		return DataItem{}, missing("content")
	case d.Digest == nil:
		return DataItem{}, missing("digest")
	}
	return DataItem{
		Schema:         *d.Schema,
		ID:             *d.ID,
		Source:         Source(*d.Source),
		Kind:           IncludedKind(*d.Kind),
		Path:           d.Path,
		Ref:            d.Ref,
		Classification: *d.Classification,
		ContentDigest:  *d.ContentDigest,
		Content:        *d.Content,
		Digest:         *d.Digest,
	}, nil
}

func dataItemDocFor(item DataItem, digest string) dataItemDoc {
	schema, id := item.Schema, item.ID
	source, kind := string(item.Source), string(item.Kind)
	classification, contentDigest, content, dig := item.Classification, item.ContentDigest, item.Content, digest
	return dataItemDoc{
		Schema:         &schema,
		ID:             &id,
		Source:         &source,
		Kind:           &kind,
		Path:           item.Path,
		Ref:            item.Ref,
		Classification: &classification,
		ContentDigest:  &contentDigest,
		Content:        &content,
		Digest:         &dig,
	}
}

// DecodeDataItem strictly decodes data as a `verdi.context-data-item/v1`
// document: exact JSON, every mandatory field present and non-null, full
// domain validation (including that content_digest matches content's exact
// bytes), the carried self digest checked against a fresh digestless
// recomputation, and byte equality between data and this package's own
// canonical re-encoding.
func DecodeDataItem(data []byte) (DataItem, error) {
	var doc dataItemDoc
	if err := artifact.DecodeExactJSON(data, &doc); err != nil {
		return DataItem{}, fmt.Errorf("contextcompile: decoding data item: %w", err)
	}
	item, err := doc.toDomain()
	if err != nil {
		return DataItem{}, fmt.Errorf("contextcompile: decoding data item: %w", err)
	}
	canonical, err := EncodeDataItem(item)
	if err != nil {
		return DataItem{}, fmt.Errorf("contextcompile: decoding data item: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return DataItem{}, fmt.Errorf("contextcompile: decoding data item: input bytes are not the canonical encoding of the document they decode to")
	}
	return item, nil
}

// EncodeDataItem validates item, recomputes its self digest from a
// digestless copy (any digest item already carries is discarded, never
// trusted — no constructor accepts a caller-supplied self digest), and
// returns the canonical JSON encoding carrying that fresh digest.
func EncodeDataItem(item DataItem) ([]byte, error) {
	digestless := item
	digestless.Digest = ""
	if err := digestless.Validate(); err != nil {
		return nil, fmt.Errorf("contextcompile: encoding data item: %w", err)
	}
	digest, err := canonjson.Digest(dataItemDocFor(digestless, ""))
	if err != nil {
		return nil, fmt.Errorf("contextcompile: encoding data item: computing digest: %w", err)
	}
	out, err := canonjson.Marshal(dataItemDocFor(digestless, digest))
	if err != nil {
		return nil, fmt.Errorf("contextcompile: encoding data item: %w", err)
	}
	return out, nil
}
