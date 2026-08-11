package draftmutation

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifact/splice"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designprovenance"
)

const (
	RequestSchema = "verdi.draftmutation/v1"
	ResultSchema  = "verdi.draftmutation-result/v1"
	RefusalSchema = "verdi.draftmutation-refusal/v1"

	MaxRequestBytes = 1 << 20
)

var fullSHARe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ExpectedIdentity is the caller's stale-safe assertion, never an authority
// source. The service compares every byte to its independently resolved
// canonical identity.
type ExpectedIdentity struct {
	Checkout string `json:"checkout"`
	Branch   string `json:"branch"`
	Head     string `json:"head"`
}

// Identity is constructed exactly once by the service after request decode
// and reused by success and all typed refusals.
type Identity struct {
	Checkout string `json:"checkout"`
	Branch   string `json:"branch"`
	Head     string `json:"head"`
	Spec     string `json:"spec"`
}

func (i Identity) Validate() error {
	if i.Checkout == "" || !filepath.IsAbs(i.Checkout) || filepath.ToSlash(i.Checkout) != i.Checkout || filepath.Clean(i.Checkout) != i.Checkout {
		return fmt.Errorf("draftmutation: identity checkout %q is not a clean absolute POSIX path", i.Checkout)
	}
	if i.Branch == "" {
		return fmt.Errorf("draftmutation: identity branch is empty")
	}
	if !fullSHARe.MatchString(i.Head) {
		return fmt.Errorf("draftmutation: identity HEAD %q is not a full lowercase SHA", i.Head)
	}
	ref, err := artifact.ParseRef(i.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return fmt.Errorf("draftmutation: identity spec %q is not an unpinned whole spec ref", i.Spec)
	}
	return nil
}

// ExcerptRequest omits target_digest; Apply computes it from the resulting
// semantic object.
type ExcerptRequest struct {
	Target         string                                 `json:"target"`
	Classification designprovenance.ExcerptClassification `json:"classification"`
	Representation designprovenance.ExcerptRepresentation `json:"representation"`
	Text           string                                 `json:"text"`
}

func (e ExcerptRequest) validate() error {
	if !designprovenance.ValidExcerptTarget(e.Target) || e.Text == "" || !utf8.ValidString(e.Text) {
		return fmt.Errorf("draftmutation: excerpt target must be problem, outcome, or an object ID, and text must be nonempty valid UTF-8")
	}
	if utf8.RuneCountInString(e.Text) > designprovenance.MaxExcerptScalars {
		return fmt.Errorf("draftmutation: excerpt text exceeds 600 Unicode scalars")
	}
	switch e.Classification {
	case designprovenance.ClassificationHumanStated, designprovenance.ClassificationAISynthesized, designprovenance.ClassificationAIInferred, designprovenance.ClassificationUnresolved:
	default:
		return fmt.Errorf("draftmutation: unknown excerpt classification %q", e.Classification)
	}
	switch e.Representation {
	case designprovenance.RepresentationVerbatim, designprovenance.RepresentationParaphrase:
	default:
		return fmt.Errorf("draftmutation: unknown excerpt representation %q", e.Representation)
	}
	return nil
}

// Request is the strict structured mutation input. BaseSpec is decoded exact
// prior bytes and is never serialized or persisted in provenance.
type Request struct {
	Schema      string           `json:"schema"`
	Spec        string           `json:"spec"`
	BaseDigest  string           `json:"base_digest"`
	BaseSpecB64 string           `json:"base_spec_b64"`
	Expected    ExpectedIdentity `json:"expected"`
	Operations  []Operation      `json:"operations"`
	Excerpts    []ExcerptRequest `json:"excerpts,omitempty"`
	BaseSpec    []byte           `json:"-"`
}

// DecodeRequest enforces the exact v1 grammar and resource ceiling before any
// checkout identity is claimed.
func DecodeRequest(data []byte) (Request, error) {
	if len(data) > MaxRequestBytes {
		return Request{}, fmt.Errorf("draftmutation: request exceeds 1 MiB")
	}
	var request Request
	if err := artifact.DecodeExactJSON(data, &request); err != nil {
		return Request{}, fmt.Errorf("draftmutation: decoding request: %w", err)
	}
	if err := request.validate(); err != nil {
		return Request{}, err
	}
	canonical, err := canonjson.Marshal(request)
	if err != nil {
		return Request{}, fmt.Errorf("draftmutation: canonicalizing request: %w", err)
	}
	if !bytes.Equal(data, canonical) {
		return Request{}, fmt.Errorf("draftmutation: request is not canonical JSON")
	}
	return request, nil
}

func (r *Request) validate() error {
	if r.Schema != RequestSchema {
		return fmt.Errorf("draftmutation: unknown request schema %q", r.Schema)
	}
	ref, err := artifact.ParseRef(r.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return fmt.Errorf("draftmutation: spec %q must be an unpinned whole spec ref", r.Spec)
	}
	if !artifact.ValidDigest(r.BaseDigest) {
		return fmt.Errorf("draftmutation: base_digest %q is invalid", r.BaseDigest)
	}
	decoded, err := base64.StdEncoding.DecodeString(r.BaseSpecB64)
	if err != nil || base64.StdEncoding.EncodeToString(decoded) != r.BaseSpecB64 {
		return fmt.Errorf("draftmutation: base_spec_b64 must be canonical standard padded base64")
	}
	if DigestBytes(decoded) != r.BaseDigest {
		return fmt.Errorf("draftmutation: base_digest does not match base_spec_b64 exact bytes")
	}
	if err := splice.Validate(decoded); err != nil {
		return fmt.Errorf("draftmutation: base_spec_b64 is not a valid spec: %w", err)
	}
	frontmatter, _, err := artifact.SplitFrontmatter(decoded)
	if err != nil {
		return fmt.Errorf("draftmutation: base_spec_b64 frontmatter: %w", err)
	}
	baseSpec, err := artifact.DecodeSpec(frontmatter)
	if err != nil {
		return fmt.Errorf("draftmutation: base_spec_b64 decode: %w", err)
	}
	if baseSpec.ID != r.Spec {
		return fmt.Errorf("draftmutation: base spec id %q does not match request spec %q", baseSpec.ID, r.Spec)
	}
	r.BaseSpec = decoded
	if r.Expected.Checkout == "" || r.Expected.Branch == "" || !fullSHARe.MatchString(r.Expected.Head) {
		return fmt.Errorf("draftmutation: expected checkout, branch, and full lowercase HEAD are required")
	}
	if !filepath.IsAbs(r.Expected.Checkout) || filepath.ToSlash(r.Expected.Checkout) != r.Expected.Checkout || strings.Contains(r.Expected.Checkout, "//") {
		return fmt.Errorf("draftmutation: expected checkout must be an absolute POSIX path")
	}
	if len(r.Operations) == 0 {
		return fmt.Errorf("draftmutation: operations must be nonempty")
	}
	for i, operation := range r.Operations {
		if err := operation.Validate(); err != nil {
			return fmt.Errorf("draftmutation: operations[%d]: %w", i, err)
		}
	}
	counts := map[string]int{}
	for i, excerpt := range r.Excerpts {
		if err := excerpt.validate(); err != nil {
			return fmt.Errorf("draftmutation: excerpts[%d]: %w", i, err)
		}
		counts[excerpt.Target]++
		if counts[excerpt.Target] > designprovenance.MaxExcerptsPerTarget {
			return fmt.Errorf("draftmutation: excerpt target %q has more than three excerpts", excerpt.Target)
		}
	}
	return nil
}
