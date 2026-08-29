package experiment

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

const (
	// ProvenanceSchema is the strict CSE human/agent mutation record schema.
	ProvenanceSchema = "verdi.experiment-mutation-provenance/v1"
	// ProvenanceFile is the append-only JSONL sidecar within an experiment.
	ProvenanceFile = "mutation-provenance.jsonl"
)

// MutationOperation is the closed successful human/agent mutation vocabulary.
type MutationOperation string

const (
	MutationDraftDefinition     MutationOperation = "draft-definition"
	MutationCaptureCandidate    MutationOperation = "capture-candidate"
	MutationReconcileDirect     MutationOperation = "reconcile-direct-draft"
	MutationProposeRegistration MutationOperation = "propose-registration"
	MutationProposeRatification MutationOperation = "propose-ratification"
)

func (o MutationOperation) validate() error {
	switch o {
	case MutationDraftDefinition, MutationCaptureCandidate, MutationReconcileDirect, MutationProposeRegistration, MutationProposeRatification:
		return nil
	default:
		return fmt.Errorf("experiment: unknown mutation provenance operation %q", o)
	}
}

// ProvenancePolicyState is the closed successful policy posture. Refusals are
// returned to callers and never appended merely to create an audit side effect.
type ProvenancePolicyState string

const PolicyAllowed ProvenancePolicyState = "allowed"

// ProvenancePolicyReason is deliberately an empty successful vocabulary in
// v1. The authority names no successful reason IDs, so accepting one would
// invent policy meaning.
type ProvenancePolicyReason string

// ProvenancePolicyDecision records the exact successful policy posture.
type ProvenancePolicyDecision struct {
	State   ProvenancePolicyState    `json:"state"`
	Reasons []ProvenancePolicyReason `json:"reasons"`
}

func (d ProvenancePolicyDecision) validate() error {
	if d.State != PolicyAllowed {
		return fmt.Errorf("experiment: unknown provenance policy decision state %q", d.State)
	}
	if d.Reasons == nil {
		return fmt.Errorf("experiment: provenance policy decision reasons must be present as an array")
	}
	if len(d.Reasons) != 0 {
		return fmt.Errorf("experiment: provenance policy decision v1 has no registered successful reason ids")
	}
	return nil
}

// ProvenanceExperiment binds a record to one spike child experiment.
type ProvenanceExperiment struct {
	Spike string `json:"spike"`
	ID    string `json:"id"`
}

// ProvenanceFileSnapshot preserves one exact file preimage for a mutation
// that overwrites bytes which the accepted result tree cannot reconstruct.
// Present distinguishes an absent path from a present empty file.
type ProvenanceFileSnapshot struct {
	Path             string `json:"path"`
	Present          bool   `json:"present"`
	ContentBase64URL string `json:"content_base64url"`
}

// NewProvenanceFileSnapshot creates one canonical exact-byte preimage row.
func NewProvenanceFileSnapshot(path string, data []byte, present bool) (ProvenanceFileSnapshot, error) {
	if !present && len(data) != 0 {
		return ProvenanceFileSnapshot{}, fmt.Errorf("experiment: absent provenance previous file %q cannot carry content", path)
	}
	snapshot := ProvenanceFileSnapshot{Path: path, Present: present}
	if present {
		snapshot.ContentBase64URL = base64.RawURLEncoding.EncodeToString(data)
	}
	if err := snapshot.validate(); err != nil {
		return ProvenanceFileSnapshot{}, err
	}
	return snapshot, nil
}

// Bytes returns a clone of the preserved bytes and their exact presence bit.
func (f ProvenanceFileSnapshot) Bytes() ([]byte, bool, error) {
	if err := f.validate(); err != nil {
		return nil, false, err
	}
	if !f.Present {
		return nil, false, nil
	}
	data, err := decodeCanonicalBase64URL(f.ContentBase64URL)
	if err != nil {
		return nil, false, fmt.Errorf("experiment: provenance previous file %q content: %w", f.Path, err)
	}
	return append([]byte(nil), data...), true, nil
}

func (f ProvenanceFileSnapshot) validate() error {
	if err := ValidateRepoRelativePath(f.Path); err != nil {
		return fmt.Errorf("experiment: provenance previous file path: %w", err)
	}
	if !f.Present {
		if f.ContentBase64URL != "" {
			return fmt.Errorf("experiment: absent provenance previous file %q must carry empty content_base64url", f.Path)
		}
		return nil
	}
	if _, err := decodeCanonicalBase64URL(f.ContentBase64URL); err != nil {
		return fmt.Errorf("experiment: provenance previous file %q content: %w", f.Path, err)
	}
	return nil
}

var provenanceSpikePattern = regexp.MustCompile(`^spec/[a-z0-9]+(-[a-z0-9]+)*$`)

func (e ProvenanceExperiment) validate() error {
	if !provenanceSpikePattern.MatchString(e.Spike) {
		// vocab:identity — "spike" names the provenance record's fixed spec/<id> artifact-ref field grammar, not renameable display vocabulary.
		return fmt.Errorf("experiment: provenance spike %q does not match ^spec/<id>$", e.Spike)
	}
	if err := ValidateID(e.ID); err != nil {
		return fmt.Errorf("experiment: provenance experiment id: %w", err)
	}
	return nil
}

// ProvenanceRecord is one canonical append-only human/agent mutation record.
type ProvenanceRecord struct {
	Schema         string                          `json:"schema"`
	Experiment     ProvenanceExperiment            `json:"experiment"`
	Operation      MutationOperation               `json:"operation"`
	PreviousDigest string                          `json:"previous_digest"`
	ResultDigest   string                          `json:"result_digest"`
	PolicyDigest   string                          `json:"policy_digest"`
	PolicyDecision ProvenancePolicyDecision        `json:"policy_decision"`
	Attribution    governanceprincipal.Attribution `json:"attribution"`
	Harness        string                          `json:"harness,omitempty"`
	Session        string                          `json:"session,omitempty"`
	Paths          []string                        `json:"paths"`
	PreviousFiles  []ProvenanceFileSnapshot        `json:"previous_files,omitempty"`
	Digest         string                          `json:"digest"`
}

type provenanceDigestProjection struct {
	Schema         string                          `json:"schema"`
	Experiment     ProvenanceExperiment            `json:"experiment"`
	Operation      MutationOperation               `json:"operation"`
	PreviousDigest string                          `json:"previous_digest"`
	ResultDigest   string                          `json:"result_digest"`
	PolicyDigest   string                          `json:"policy_digest"`
	PolicyDecision ProvenancePolicyDecision        `json:"policy_decision"`
	Attribution    governanceprincipal.Attribution `json:"attribution"`
	Harness        string                          `json:"harness,omitempty"`
	Session        string                          `json:"session,omitempty"`
	Paths          []string                        `json:"paths"`
	PreviousFiles  []ProvenanceFileSnapshot        `json:"previous_files,omitempty"`
}

func (r ProvenanceRecord) digestProjection() provenanceDigestProjection {
	return provenanceDigestProjection{
		Schema: r.Schema, Experiment: r.Experiment, Operation: r.Operation,
		PreviousDigest: r.PreviousDigest, ResultDigest: r.ResultDigest,
		PolicyDigest: r.PolicyDigest, PolicyDecision: r.PolicyDecision,
		Attribution: r.Attribution, Harness: r.Harness, Session: r.Session,
		Paths: slices.Clone(r.Paths), PreviousFiles: slices.Clone(r.PreviousFiles),
	}
}

// Seal validates r and sets its canonical self-digest.
func (r *ProvenanceRecord) Seal() error {
	if r == nil {
		return fmt.Errorf("experiment: mutation provenance record is nil")
	}
	r.Digest = ""
	if err := r.validate(false); err != nil {
		return err
	}
	digest, err := canonjson.Digest(r.digestProjection())
	if err != nil {
		return fmt.Errorf("experiment: digesting mutation provenance: %w", err)
	}
	r.Digest = digest
	return nil
}

// Validate checks the strict grammar and verifies the self-digest.
func (r ProvenanceRecord) Validate() error { return r.validate(true) }

func (r ProvenanceRecord) validate(checkDigest bool) error {
	if r.Schema != ProvenanceSchema {
		return fmt.Errorf("experiment: unknown mutation provenance schema %q", r.Schema)
	}
	if err := r.Experiment.validate(); err != nil {
		return err
	}
	if err := r.Operation.validate(); err != nil {
		return err
	}
	for name, digest := range map[string]string{
		"previous_digest": r.PreviousDigest,
		"result_digest":   r.ResultDigest,
		"policy_digest":   r.PolicyDigest,
	} {
		if err := ValidateDigest(digest); err != nil {
			return fmt.Errorf("experiment: provenance %s: %w", name, err)
		}
	}
	if err := r.PolicyDecision.validate(); err != nil {
		return err
	}
	if err := r.Attribution.Validate(); err != nil {
		return err
	}
	if r.Attribution.Unauthenticated {
		if r.Harness != "" && (!utf8.ValidString(r.Harness) || strings.TrimSpace(r.Harness) == "") {
			return fmt.Errorf("experiment: provenance harness must be nonblank valid UTF-8 when present")
		}
		if r.Session != "" && r.Harness == "" {
			return fmt.Errorf("experiment: provenance session requires harness attribution")
		}
		if r.Session != "" && (!utf8.ValidString(r.Session) || strings.TrimSpace(r.Session) == "") {
			return fmt.Errorf("experiment: provenance session must be nonblank valid UTF-8 when present")
		}
	} else if r.Harness != "" || r.Session != "" {
		return fmt.Errorf("experiment: principal provenance attribution must omit harness and session")
	}
	if len(r.Paths) == 0 {
		return fmt.Errorf("experiment: provenance paths must be nonempty")
	}
	for i, path := range r.Paths {
		if err := ValidateRepoRelativePath(path); err != nil {
			return fmt.Errorf("experiment: provenance paths[%d]: %w", i, err)
		}
		if i > 0 && r.Paths[i-1] >= path {
			return fmt.Errorf("experiment: provenance paths must be sorted and unique")
		}
	}
	if r.PreviousFiles != nil {
		if len(r.PreviousFiles) == 0 {
			return fmt.Errorf("experiment: provenance previous_files must be omitted or nonempty")
		}
		pathSet := make(map[string]bool, len(r.Paths))
		for _, changedPath := range r.Paths {
			pathSet[changedPath] = true
		}
		for i, previous := range r.PreviousFiles {
			if err := previous.validate(); err != nil {
				return fmt.Errorf("experiment: provenance previous_files[%d]: %w", i, err)
			}
			if !pathSet[previous.Path] {
				return fmt.Errorf("experiment: provenance previous file %q is absent from paths", previous.Path)
			}
			if i > 0 && r.PreviousFiles[i-1].Path >= previous.Path {
				return fmt.Errorf("experiment: provenance previous_files must be sorted and unique")
			}
		}
	}
	if checkDigest {
		want, err := canonjson.Digest(r.digestProjection())
		if err != nil {
			return err
		}
		if r.Digest != want {
			return fmt.Errorf("experiment: provenance digest %q does not match %q", r.Digest, want)
		}
	}
	return nil
}

// EncodeProvenanceRecord returns one canonical JSONL record including newline.
func EncodeProvenanceRecord(record ProvenanceRecord) ([]byte, error) {
	if err := record.Validate(); err != nil {
		return nil, err
	}
	return canonjson.Marshal(record)
}

// DecodeProvenanceRecord strict-decodes one canonical JSONL line without its
// line terminator.
func DecodeProvenanceRecord(data []byte) (ProvenanceRecord, error) {
	var record ProvenanceRecord
	if err := decodeStrictJSON(data, &record); err != nil {
		return ProvenanceRecord{}, fmt.Errorf("experiment: decoding mutation provenance: %w", err)
	}
	if err := record.Validate(); err != nil {
		return ProvenanceRecord{}, err
	}
	canonical, err := EncodeProvenanceRecord(record)
	if err != nil {
		return ProvenanceRecord{}, err
	}
	if !bytes.Equal(data, bytes.TrimSuffix(canonical, []byte("\n"))) {
		return ProvenanceRecord{}, fmt.Errorf("experiment: mutation provenance record is not canonical JSON")
	}
	return record, nil
}

// DecodeProvenanceLog decodes a canonical append-only JSONL sequence and
// verifies one continuous artifact-digest chain for one experiment.
func DecodeProvenanceLog(data []byte) ([]ProvenanceRecord, error) {
	if len(data) == 0 {
		return []ProvenanceRecord{}, nil
	}
	if !bytes.HasSuffix(data, []byte("\n")) {
		return nil, fmt.Errorf("experiment: mutation provenance JSONL must end with a newline")
	}
	lines := bytes.Split(data[:len(data)-1], []byte("\n"))
	records := make([]ProvenanceRecord, 0, len(lines))
	seen := make(map[string]bool, len(lines))
	for i, line := range lines {
		if len(line) == 0 {
			return nil, fmt.Errorf("experiment: mutation provenance JSONL line %d is blank", i+1)
		}
		record, err := DecodeProvenanceRecord(line)
		if err != nil {
			return nil, fmt.Errorf("experiment: mutation provenance JSONL line %d: %w", i+1, err)
		}
		if seen[record.Digest] {
			return nil, fmt.Errorf("experiment: duplicate mutation provenance digest %q", record.Digest)
		}
		seen[record.Digest] = true
		if i > 0 {
			previous := records[i-1]
			if record.Experiment != previous.Experiment {
				return nil, fmt.Errorf("experiment: mutation provenance experiment identity changed")
			}
			if record.PreviousDigest != previous.ResultDigest {
				return nil, fmt.Errorf("experiment: mutation provenance chain breaks from %q to %q", previous.ResultDigest, record.PreviousDigest)
			}
		}
		records = append(records, record)
	}
	return records, nil
}
