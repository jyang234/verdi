package sealedexec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyconflict"
)

var canonicalDigestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

type executionTuple struct {
	Flight  string `json:"flight"`
	Lane    string `json:"lane"`
	Epoch   string `json:"epoch"`
	Session string `json:"session"`
}

// NewExecutionWorkspaceRequest derives U4's exact-SHA execution-workspace
// request identity from the complete dispatch tuple and input commit.
func NewExecutionWorkspaceRequest(flight, lane, epoch, session, inputCommit string) (execworkspace.Identity, error) {
	tuple := executionTuple{Flight: flight, Lane: lane, Epoch: epoch, Session: session}
	for field, value := range map[string]string{"flight": flight, "lane": lane, "epoch": epoch, "session": session} {
		if err := requireText(field, value); err != nil {
			return execworkspace.Identity{}, err
		}
	}
	encoded, err := canonjson.Marshal(tuple)
	if err != nil {
		return execworkspace.Identity{}, fmt.Errorf("sealedexec: derive execution workspace run id: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return execworkspace.NewExactIdentity("vatc-"+hex.EncodeToString(sum[:]), inputCommit)
}

// ExecutionWorkspaceRequestDigest returns the digest of the component-owned
// exact canonical request-sidecar bytes.
func ExecutionWorkspaceRequestDigest(identity execworkspace.Identity) (string, error) {
	if identity.Shape != execworkspace.ExactSHA {
		return "", fmt.Errorf("sealedexec: execution workspace request must use exact-sha shape")
	}
	encoded, err := execworkspace.EncodeSidecar(identity)
	if err != nil {
		return "", fmt.Errorf("sealedexec: encode execution workspace request: %w", err)
	}
	return digestBytes(encoded), nil
}

type instructionFileDoc struct {
	Path          *string `json:"path"`
	ContentDigest *string `json:"content_digest"`
	Content       *string `json:"content"`
}

type instructionProjectionDoc struct {
	Schema *string              `json:"schema"`
	Files  []instructionFileDoc `json:"files"`
	Digest *string              `json:"digest"`
}

func projectionDocFor(projection InstructionProjection, digest string) instructionProjectionDoc {
	files := make([]instructionFileDoc, len(projection.Files))
	for i, file := range projection.Files {
		filePath, contentDigest, content := file.Path, file.ContentDigest, file.Content
		files[i] = instructionFileDoc{Path: &filePath, ContentDigest: &contentDigest, Content: &content}
	}
	schema, dig := projection.Schema, digest
	return instructionProjectionDoc{Schema: &schema, Files: files, Digest: &dig}
}

func (doc instructionProjectionDoc) toDomain() (InstructionProjection, error) {
	if doc.Schema == nil || doc.Files == nil || doc.Digest == nil {
		return InstructionProjection{}, fmt.Errorf("sealedexec: instruction projection has an absent or null mandatory field")
	}
	files := make([]InstructionFile, len(doc.Files))
	for i, file := range doc.Files {
		if file.Path == nil || file.ContentDigest == nil || file.Content == nil {
			return InstructionProjection{}, fmt.Errorf("sealedexec: instruction projection files[%d] has an absent or null mandatory field", i)
		}
		files[i] = InstructionFile{Path: *file.Path, ContentDigest: *file.ContentDigest, Content: *file.Content}
	}
	return InstructionProjection{Schema: *doc.Schema, Files: files, Digest: *doc.Digest}, nil
}

// EncodeInstructionProjection validates, self-digests, and canonically
// encodes the sole project-authority channel.
func EncodeInstructionProjection(projection InstructionProjection) ([]byte, error) {
	if err := validateInstructionProjection(projection, false); err != nil {
		return nil, err
	}
	want, err := canonjson.Digest(projectionDocFor(projection, ""))
	if err != nil {
		return nil, fmt.Errorf("sealedexec: digest instruction projection: %w", err)
	}
	if projection.Digest != "" && projection.Digest != want {
		return nil, fmt.Errorf("sealedexec: instruction projection digest does not match canonical projection")
	}
	return canonjson.Marshal(projectionDocFor(projection, want))
}

// DecodeInstructionProjection strict-decodes a canonical self-digested
// instruction projection.
func DecodeInstructionProjection(reader io.Reader) (InstructionProjection, error) {
	var doc instructionProjectionDoc
	raw, err := decodeStrict(reader, &doc)
	if err != nil {
		return InstructionProjection{}, fmt.Errorf("sealedexec: decode instruction projection: %w", err)
	}
	projection, err := doc.toDomain()
	if err != nil {
		return InstructionProjection{}, err
	}
	if err := validateInstructionProjection(projection, true); err != nil {
		return InstructionProjection{}, err
	}
	canonical, err := canonjson.Marshal(doc)
	if err != nil {
		return InstructionProjection{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return InstructionProjection{}, fmt.Errorf("sealedexec: instruction projection is not byte-canonical")
	}
	return projection, nil
}

func validateInstructionProjection(projection InstructionProjection, requireDigest bool) error {
	if projection.Schema != InstructionProjectionSchemaID {
		return fmt.Errorf("sealedexec: instruction projection schema must be %q", InstructionProjectionSchemaID)
	}
	if len(projection.Files) == 0 {
		return fmt.Errorf("sealedexec: instruction projection files must be non-null and nonempty")
	}
	for i, file := range projection.Files {
		if err := validateProjectionPath(file.Path); err != nil {
			return fmt.Errorf("sealedexec: instruction projection files[%d]: %w", i, err)
		}
		if !utf8.ValidString(file.Content) {
			return fmt.Errorf("sealedexec: instruction projection files[%d].content must be UTF-8", i)
		}
		if file.ContentDigest != digestBytes([]byte(file.Content)) {
			return fmt.Errorf("sealedexec: instruction projection files[%d].content_digest does not match content bytes", i)
		}
		if i > 0 && projection.Files[i-1].Path >= file.Path {
			return fmt.Errorf("sealedexec: instruction projection files must be sorted and deduplicated by path")
		}
	}
	if requireDigest || projection.Digest != "" {
		if err := validateDigest("instruction projection digest", projection.Digest); err != nil {
			return err
		}
		want, err := canonjson.Digest(projectionDocFor(projection, ""))
		if err != nil {
			return err
		}
		if projection.Digest != want {
			return fmt.Errorf("sealedexec: instruction projection digest does not match canonical projection")
		}
	}
	return nil
}

type executionRequestDoc struct {
	Schema                    *string         `json:"schema"`
	Action                    *string         `json:"action"`
	Flight                    *string         `json:"flight"`
	Lane                      *string         `json:"lane"`
	Epoch                     *string         `json:"epoch"`
	ManifestRevision          *uint64         `json:"manifest_revision"`
	Session                   *string         `json:"session"`
	ATCRunway                 *string         `json:"atc_runway"`
	InputCommit               *string         `json:"input_commit"`
	InputTree                 *string         `json:"input_tree"`
	Manifest                  json.RawMessage `json:"manifest"`
	ManifestDigest            *string         `json:"manifest_digest"`
	InstructionProjection     json.RawMessage `json:"instruction_projection"`
	ProjectionDigest          *string         `json:"projection_digest"`
	ExecutionWorkspaceRequest json.RawMessage `json:"execution_workspace_request"`
	Adapter                   *string         `json:"adapter"`
	AdapterVersion            *string         `json:"adapter_version"`
	Profile                   *LogicalRef     `json:"profile"`
	Grants                    json.RawMessage `json:"grants"`
	AuthorityVerdict          json.RawMessage `json:"authority_verdict"`
	RecorderEndpoint          *LogicalRef     `json:"recorder_endpoint"`
	Start                     *StartArm       `json:"start,omitempty"`
	Resume                    json.RawMessage `json:"resume,omitempty"`
}

type resumeArmDoc struct {
	Continuity       json.RawMessage `json:"continuity"`
	ContinuityDigest *string         `json:"continuity_digest"`
}

func requestDocFor(request ExecutionRequest) (executionRequestDoc, error) {
	manifest, err := contextcompile.EncodeManifest(request.Manifest)
	if err != nil {
		return executionRequestDoc{}, fmt.Errorf("sealedexec: encode request manifest: %w", err)
	}
	projection, err := EncodeInstructionProjection(request.InstructionProjection)
	if err != nil {
		return executionRequestDoc{}, err
	}
	workspace, err := execworkspace.EncodeSidecar(request.ExecutionWorkspaceRequest)
	if err != nil {
		return executionRequestDoc{}, fmt.Errorf("sealedexec: encode request workspace identity: %w", err)
	}
	grants, err := execworkspace.EncodeGrantSet(request.Grants)
	if err != nil {
		return executionRequestDoc{}, fmt.Errorf("sealedexec: encode request grants: %w", err)
	}
	authority, err := policyconflict.EncodeReport(request.AuthorityVerdict)
	if err != nil {
		return executionRequestDoc{}, fmt.Errorf("sealedexec: encode request authority verdict: %w", err)
	}
	schema, action := request.Schema, string(request.Action)
	flight, lane, epoch := request.Flight, request.Lane, request.Epoch
	revision, session, runway := request.ManifestRevision, request.Session, request.ATCRunway
	inputCommit, inputTree := request.InputCommit, request.InputTree
	manifestDigest, projectionDigest := request.ManifestDigest, request.ProjectionDigest
	adapter, adapterVersion := string(request.Adapter), request.AdapterVersion
	doc := executionRequestDoc{
		Schema: &schema, Action: &action, Flight: &flight, Lane: &lane, Epoch: &epoch,
		ManifestRevision: &revision, Session: &session, ATCRunway: &runway,
		InputCommit: &inputCommit, InputTree: &inputTree, Manifest: manifest,
		ManifestDigest: &manifestDigest, InstructionProjection: projection,
		ProjectionDigest: &projectionDigest, ExecutionWorkspaceRequest: workspace,
		Adapter: &adapter, AdapterVersion: &adapterVersion, Profile: &request.Profile,
		Grants: grants, AuthorityVerdict: authority, RecorderEndpoint: &request.RecorderEndpoint,
		Start: request.Start,
	}
	if request.Resume != nil {
		continuity, err := EncodeExecutionContinuity(request.Resume.Continuity)
		if err != nil {
			return executionRequestDoc{}, fmt.Errorf("sealedexec: encode request resume continuity: %w", err)
		}
		digest := request.Resume.ContinuityDigest
		resume, err := canonjson.Marshal(resumeArmDoc{Continuity: continuity, ContinuityDigest: &digest})
		if err != nil {
			return executionRequestDoc{}, err
		}
		doc.Resume = resume
	}
	return doc, nil
}

func (doc executionRequestDoc) toDomain() (ExecutionRequest, error) {
	if doc.Schema == nil || doc.Action == nil || doc.Flight == nil || doc.Lane == nil || doc.Epoch == nil ||
		doc.ManifestRevision == nil || doc.Session == nil || doc.ATCRunway == nil || doc.InputCommit == nil ||
		doc.InputTree == nil || rawMissing(doc.Manifest) || doc.ManifestDigest == nil || rawMissing(doc.InstructionProjection) ||
		doc.ProjectionDigest == nil || rawMissing(doc.ExecutionWorkspaceRequest) || doc.Adapter == nil ||
		doc.AdapterVersion == nil || doc.Profile == nil || rawMissing(doc.Grants) || rawMissing(doc.AuthorityVerdict) ||
		doc.RecorderEndpoint == nil {
		return ExecutionRequest{}, fmt.Errorf("sealedexec: execution request has an absent or null mandatory field")
	}
	manifestBytes, err := canonicalNested(doc.Manifest)
	if err != nil {
		return ExecutionRequest{}, err
	}
	manifest, err := contextcompile.DecodeManifest(manifestBytes)
	if err != nil {
		return ExecutionRequest{}, fmt.Errorf("sealedexec: decode request manifest: %w", err)
	}
	projectionBytes, err := canonicalNested(doc.InstructionProjection)
	if err != nil {
		return ExecutionRequest{}, err
	}
	projection, err := DecodeInstructionProjection(bytes.NewReader(projectionBytes))
	if err != nil {
		return ExecutionRequest{}, err
	}
	workspaceBytes, err := canonicalNested(doc.ExecutionWorkspaceRequest)
	if err != nil {
		return ExecutionRequest{}, err
	}
	workspace, err := execworkspace.DecodeSidecar(workspaceBytes)
	if err != nil {
		return ExecutionRequest{}, fmt.Errorf("sealedexec: decode request workspace identity: %w", err)
	}
	grantBytes, err := canonicalNested(doc.Grants)
	if err != nil {
		return ExecutionRequest{}, err
	}
	grants, err := execworkspace.DecodeGrantSet(grantBytes)
	if err != nil {
		return ExecutionRequest{}, fmt.Errorf("sealedexec: decode request grants: %w", err)
	}
	authorityBytes, err := canonicalNested(doc.AuthorityVerdict)
	if err != nil {
		return ExecutionRequest{}, err
	}
	authority, err := policyconflict.DecodeReport(authorityBytes)
	if err != nil {
		return ExecutionRequest{}, fmt.Errorf("sealedexec: decode request authority verdict: %w", err)
	}
	request := ExecutionRequest{
		Schema: *doc.Schema, Action: Action(*doc.Action), Flight: *doc.Flight, Lane: *doc.Lane,
		Epoch: *doc.Epoch, ManifestRevision: *doc.ManifestRevision, Session: *doc.Session,
		ATCRunway: *doc.ATCRunway, InputCommit: *doc.InputCommit, InputTree: *doc.InputTree,
		Manifest: manifest, ManifestDigest: *doc.ManifestDigest, InstructionProjection: projection,
		ProjectionDigest: *doc.ProjectionDigest, ExecutionWorkspaceRequest: workspace,
		Adapter: contextevent.Adapter(*doc.Adapter), AdapterVersion: *doc.AdapterVersion,
		Profile: *doc.Profile, Grants: grants, AuthorityVerdict: authority,
		RecorderEndpoint: *doc.RecorderEndpoint, Start: doc.Start,
	}
	if !rawMissing(doc.Resume) {
		resumeBytes, err := canonicalNested(doc.Resume)
		if err != nil {
			return ExecutionRequest{}, err
		}
		var resumeDoc resumeArmDoc
		if _, err := decodeStrict(bytes.NewReader(resumeBytes), &resumeDoc); err != nil {
			return ExecutionRequest{}, fmt.Errorf("sealedexec: decode resume arm: %w", err)
		}
		if rawMissing(resumeDoc.Continuity) || resumeDoc.ContinuityDigest == nil {
			return ExecutionRequest{}, fmt.Errorf("sealedexec: resume arm has an absent or null mandatory field")
		}
		continuityBytes, err := canonicalNested(resumeDoc.Continuity)
		if err != nil {
			return ExecutionRequest{}, err
		}
		continuity, err := DecodeExecutionContinuity(bytes.NewReader(continuityBytes))
		if err != nil {
			return ExecutionRequest{}, err
		}
		request.Resume = &ResumeArm{Continuity: continuity, ContinuityDigest: *resumeDoc.ContinuityDigest}
	}
	return request, nil
}

// EncodeExecutionRequest validates and canonically encodes a sealed request.
func EncodeExecutionRequest(request ExecutionRequest) ([]byte, error) {
	if err := validateExecutionRequest(request); err != nil {
		return nil, err
	}
	doc, err := requestDocFor(request)
	if err != nil {
		return nil, err
	}
	return canonjson.Marshal(doc)
}

// DecodeExecutionRequest strict-decodes a canonical sealed request.
func DecodeExecutionRequest(reader io.Reader) (ExecutionRequest, error) {
	var doc executionRequestDoc
	raw, err := decodeStrict(reader, &doc)
	if err != nil {
		return ExecutionRequest{}, fmt.Errorf("sealedexec: decode execution request: %w", err)
	}
	request, err := doc.toDomain()
	if err != nil {
		return ExecutionRequest{}, err
	}
	if err := validateExecutionRequest(request); err != nil {
		return ExecutionRequest{}, err
	}
	canonical, err := EncodeExecutionRequest(request)
	if err != nil {
		return ExecutionRequest{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return ExecutionRequest{}, fmt.Errorf("sealedexec: execution request is not byte-canonical")
	}
	return request, nil
}

func validateExecutionRequest(request ExecutionRequest) error {
	if request.Schema != ExecutionRequestSchemaID {
		return fmt.Errorf("sealedexec: execution request schema must be %q", ExecutionRequestSchemaID)
	}
	for field, value := range map[string]string{"flight": request.Flight, "lane": request.Lane, "epoch": request.Epoch, "session": request.Session, "atc_runway": request.ATCRunway, "adapter_version": request.AdapterVersion} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	if err := validateGitOID("input_commit", request.InputCommit, true); err != nil {
		return err
	}
	if err := validateGitOID("input_tree", request.InputTree, false); err != nil {
		return err
	}
	if err := request.Adapter.Validate(); err != nil {
		return fmt.Errorf("sealedexec: %w", err)
	}
	manifestBytes, err := contextcompile.EncodeManifest(request.Manifest)
	if err != nil {
		return fmt.Errorf("sealedexec: request manifest: %w", err)
	}
	manifest, err := contextcompile.DecodeManifest(manifestBytes)
	if err != nil {
		return err
	}
	if request.Manifest.Digest != manifest.Digest || request.ManifestDigest != manifest.Digest {
		return fmt.Errorf("sealedexec: request manifest_digest does not match canonical manifest")
	}
	projectionBytes, err := EncodeInstructionProjection(request.InstructionProjection)
	if err != nil {
		return err
	}
	projection, err := DecodeInstructionProjection(bytes.NewReader(projectionBytes))
	if err != nil {
		return err
	}
	if request.InstructionProjection.Digest != projection.Digest || request.ProjectionDigest != projection.Digest {
		return fmt.Errorf("sealedexec: request projection_digest does not match canonical instruction projection")
	}
	if err := matchProjectionInventory(manifest.ProjectionFiles, projection.Files); err != nil {
		return err
	}
	if manifest.Adapter.ID != string(request.Adapter) || manifest.Adapter.Version != request.AdapterVersion {
		return fmt.Errorf("sealedexec: request adapter contradicts manifest adapter")
	}
	if request.ExecutionWorkspaceRequest.Shape != execworkspace.ExactSHA || request.ExecutionWorkspaceRequest.CommitSHA != request.InputCommit {
		return fmt.Errorf("sealedexec: request execution_workspace_request must be exact-sha at input_commit")
	}
	wantWorkspace, err := NewExecutionWorkspaceRequest(request.Flight, request.Lane, request.Epoch, request.Session, request.InputCommit)
	if err != nil {
		return err
	}
	if !request.ExecutionWorkspaceRequest.Equal(wantWorkspace) {
		return fmt.Errorf("sealedexec: request execution_workspace_request run identity contradicts dispatch tuple")
	}
	if err := validateLogicalRef("profile", request.Profile, ProjectProfileRefSchemaID); err != nil {
		return err
	}
	grantBytes, err := execworkspace.EncodeGrantSet(request.Grants)
	if err != nil {
		return fmt.Errorf("sealedexec: request grants: %w", err)
	}
	manifestGrantBytes, err := execworkspace.EncodeGrantSet(manifest.Capabilities)
	if err != nil {
		return err
	}
	if !bytes.Equal(grantBytes, manifestGrantBytes) {
		return fmt.Errorf("sealedexec: request grants contradict manifest capabilities")
	}
	authorityBytes, err := policyconflict.EncodeReport(request.AuthorityVerdict)
	if err != nil {
		return fmt.Errorf("sealedexec: request authority verdict: %w", err)
	}
	authority, err := policyconflict.DecodeReport(authorityBytes)
	if err != nil {
		return err
	}
	if request.AuthorityVerdict.Digest != authority.Digest {
		return fmt.Errorf("sealedexec: request authority verdict digest is stale")
	}
	if authority.Input.Target.Kind != policyconflict.TargetAcceptedContext || authority.Input.Target.Accepted == nil || authority.Input.Target.Accepted.ManifestDigest != manifest.Digest {
		return fmt.Errorf("sealedexec: request authority verdict does not bind manifest")
	}
	if err := validateLogicalRef("recorder_endpoint", request.RecorderEndpoint, RecorderEndpointRefSchemaID); err != nil {
		return err
	}
	switch request.Action {
	case ActionStart:
		if request.Start == nil || request.Resume != nil || request.Start.ExpectedSourceSequence != 1 {
			return fmt.Errorf("sealedexec: start action requires only start.expected_source_sequence=1")
		}
	case ActionResume:
		if request.Start != nil || request.Resume == nil {
			return fmt.Errorf("sealedexec: resume action requires only the resume arm")
		}
		if request.Resume.ContinuityDigest != request.Resume.Continuity.Digest {
			return fmt.Errorf("sealedexec: resume continuity_digest does not match continuity")
		}
		if err := matchRequestContinuity(request, request.Resume.Continuity, grantBytes, authority.Digest); err != nil {
			return err
		}
	default:
		return fmt.Errorf("sealedexec: unknown execution action %q", request.Action)
	}
	return nil
}

func matchRequestContinuity(request ExecutionRequest, continuity ExecutionContinuity, grantBytes []byte, authorityDigest string) error {
	workspaceDigest, err := ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		return err
	}
	pairs := []struct{ field, left, right string }{
		{"flight", request.Flight, continuity.Flight}, {"lane", request.Lane, continuity.Lane},
		{"epoch", request.Epoch, continuity.Epoch}, {"session", request.Session, continuity.Session},
		{"atc_runway", request.ATCRunway, continuity.ATCRunway}, {"input_commit", request.InputCommit, continuity.InputCommit},
		{"input_tree", request.InputTree, continuity.InputTree}, {"adapter_version", request.AdapterVersion, continuity.AdapterVersion},
		{"manifest_digest", request.ManifestDigest, continuity.CurrentManifestDigest},
		{"projection_digest", request.ProjectionDigest, continuity.ProjectionDigest},
		{"workspace_request_digest", workspaceDigest, continuity.ExecutionWorkspaceRequestDigest},
		{"profile_digest", request.Profile.Digest, continuity.ProfileDigest},
		{"grant_digest", digestBytes(grantBytes), continuity.GrantDigest},
		{"authority_verdict_digest", authorityDigest, continuity.AuthorityVerdictDigest},
	}
	for _, pair := range pairs {
		if pair.left != pair.right {
			return fmt.Errorf("sealedexec: resume continuity %s contradicts request", pair.field)
		}
	}
	if request.Adapter != continuity.Adapter || request.ManifestRevision != continuity.CurrentManifestRevision {
		return fmt.Errorf("sealedexec: resume continuity typed identity contradicts request")
	}
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		return err
	}
	if workspaceID != continuity.ExecutionWorkspaceID {
		return fmt.Errorf("sealedexec: resume continuity workspace id contradicts request")
	}
	return nil
}

// EncodeExecutionContinuity validates, self-digests, and canonically encodes
// one resume checkpoint.
func EncodeExecutionContinuity(continuity ExecutionContinuity) ([]byte, error) {
	if err := validateExecutionContinuity(continuity, false); err != nil {
		return nil, err
	}
	want, err := continuityDigest(continuity)
	if err != nil {
		return nil, err
	}
	if continuity.Digest != "" && continuity.Digest != want {
		return nil, fmt.Errorf("sealedexec: continuity digest does not match canonical record")
	}
	continuity.Digest = want
	return canonjson.Marshal(continuity)
}

// DecodeExecutionContinuity strict-decodes a canonical continuity record.
func DecodeExecutionContinuity(reader io.Reader) (ExecutionContinuity, error) {
	var continuity ExecutionContinuity
	raw, err := decodeStrict(reader, &continuity)
	if err != nil {
		return ExecutionContinuity{}, fmt.Errorf("sealedexec: decode continuity: %w", err)
	}
	if err := requireFields(raw, "schema", "flight", "lane", "epoch", "session", "adapter", "adapter_version", "atc_runway", "input_commit", "input_tree", "current_commit", "current_tree", "execution_workspace_id", "execution_workspace_request_digest", "profile_digest", "grant_digest", "authority_verdict_digest", "current_manifest_revision", "current_manifest_digest", "projection_digest", "revision_segments", "event_chain_root", "expansion_ledger_root", "terminal_source_sequence", "terminal_global_sequence", "recorder_checkpoint_digest", "adapter_session_ref", "digest"); err != nil {
		return ExecutionContinuity{}, err
	}
	if err := validateExecutionContinuity(continuity, true); err != nil {
		return ExecutionContinuity{}, err
	}
	canonical, err := canonjson.Marshal(continuity)
	if err != nil {
		return ExecutionContinuity{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return ExecutionContinuity{}, fmt.Errorf("sealedexec: continuity is not byte-canonical")
	}
	return continuity, nil
}

func continuityDigest(continuity ExecutionContinuity) (string, error) {
	continuity.Digest = ""
	return canonjson.Digest(continuity)
}

func validateExecutionContinuity(continuity ExecutionContinuity, requireSelfDigest bool) error {
	if continuity.Schema != ExecutionContinuitySchemaID {
		return fmt.Errorf("sealedexec: continuity schema must be %q", ExecutionContinuitySchemaID)
	}
	for field, value := range map[string]string{"flight": continuity.Flight, "lane": continuity.Lane, "epoch": continuity.Epoch, "session": continuity.Session, "adapter_version": continuity.AdapterVersion, "atc_runway": continuity.ATCRunway, "execution_workspace_id": continuity.ExecutionWorkspaceID, "adapter_session_ref": continuity.AdapterSessionRef} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	if err := continuity.Adapter.Validate(); err != nil {
		return fmt.Errorf("sealedexec: %w", err)
	}
	for field, value := range map[string]string{"input_commit": continuity.InputCommit, "input_tree": continuity.InputTree, "current_commit": continuity.CurrentCommit, "current_tree": continuity.CurrentTree} {
		if err := validateGitOID(field, value, field == "input_commit"); err != nil {
			return err
		}
	}
	for field, value := range map[string]string{
		"execution_workspace_request_digest": continuity.ExecutionWorkspaceRequestDigest,
		"profile_digest":                     continuity.ProfileDigest, "grant_digest": continuity.GrantDigest,
		"authority_verdict_digest": continuity.AuthorityVerdictDigest,
		"current_manifest_digest":  continuity.CurrentManifestDigest,
		"projection_digest":        continuity.ProjectionDigest, "event_chain_root": continuity.EventChainRoot,
		"expansion_ledger_root":      continuity.ExpansionLedgerRoot,
		"recorder_checkpoint_digest": continuity.RecorderCheckpointDigest,
	} {
		if err := validateDigest(field, value); err != nil {
			return err
		}
	}
	root, err := contextevent.EventChainRoot(continuity.RevisionSegments)
	if err != nil {
		return fmt.Errorf("sealedexec: continuity revision_segments: %w", err)
	}
	if root != continuity.EventChainRoot {
		return fmt.Errorf("sealedexec: continuity event_chain_root does not match revision_segments")
	}
	terminal := continuity.RevisionSegments[len(continuity.RevisionSegments)-1]
	if terminal.ManifestRevision != continuity.CurrentManifestRevision || terminal.ManifestDigest != continuity.CurrentManifestDigest || terminal.TerminalSourceSequence != continuity.TerminalSourceSequence || terminal.TerminalGlobalSequence != continuity.TerminalGlobalSequence {
		return fmt.Errorf("sealedexec: continuity terminal identity does not match final revision")
	}
	if requireSelfDigest || continuity.Digest != "" {
		if err := validateDigest("continuity digest", continuity.Digest); err != nil {
			return err
		}
		want, err := continuityDigest(continuity)
		if err != nil {
			return err
		}
		if continuity.Digest != want {
			return fmt.Errorf("sealedexec: continuity digest does not match canonical record")
		}
	}
	return nil
}

type executionResultDoc struct {
	Schema                   *string         `json:"schema"`
	Verdict                  *string         `json:"verdict"`
	Authority                *string         `json:"authority"`
	Witnesses                []string        `json:"witnesses"`
	Flight                   *string         `json:"flight"`
	Lane                     *string         `json:"lane"`
	Epoch                    *string         `json:"epoch"`
	Session                  *string         `json:"session"`
	ATCRunway                *string         `json:"atc_runway"`
	ExecutionWorkspaceID     *string         `json:"execution_workspace_id"`
	Adapter                  *string         `json:"adapter"`
	AdapterVersion           *string         `json:"adapter_version"`
	InputCommit              *string         `json:"input_commit"`
	InputTree                *string         `json:"input_tree"`
	OutputCommit             *string         `json:"output_commit"`
	OutputTree               *string         `json:"output_tree"`
	Clean                    *bool           `json:"clean"`
	TerminalManifestDigest   *string         `json:"terminal_manifest_digest"`
	TerminalManifestRevision *uint64         `json:"terminal_manifest_revision"`
	TerminalSourceSequence   *uint64         `json:"terminal_source_sequence"`
	TerminalGlobalSequence   *uint64         `json:"terminal_global_sequence"`
	EventChainRoot           *string         `json:"event_chain_root"`
	Receipt                  json.RawMessage `json:"receipt"`
	ReceiptEventAck          json.RawMessage `json:"receipt_event_ack"`
}

func resultDocFor(result ExecutionResult) (executionResultDoc, error) {
	receipt, err := contextreceipt.EncodeReceipt(result.Receipt)
	if err != nil {
		return executionResultDoc{}, fmt.Errorf("sealedexec: encode result receipt: %w", err)
	}
	ack, err := contextevent.EncodeReceiptEventAck(result.ReceiptEventAck)
	if err != nil {
		return executionResultDoc{}, fmt.Errorf("sealedexec: encode result receipt_event_ack: %w", err)
	}
	schema, verdict, authority := result.Schema, string(result.Verdict), string(result.Authority)
	flight, lane, epoch, session := result.Flight, result.Lane, result.Epoch, result.Session
	runway, workspace := result.ATCRunway, result.ExecutionWorkspaceID
	adapter, adapterVersion := string(result.Adapter), result.AdapterVersion
	inputCommit, inputTree, outputCommit, outputTree := result.InputCommit, result.InputTree, result.OutputCommit, result.OutputTree
	clean, terminalDigest, terminalRevision := result.Clean, result.TerminalManifestDigest, result.TerminalManifestRevision
	terminalSource, terminalGlobal, root := result.TerminalSourceSequence, result.TerminalGlobalSequence, result.EventChainRoot
	return executionResultDoc{
		Schema: &schema, Verdict: &verdict, Authority: &authority, Witnesses: nonNilStrings(result.Witnesses),
		Flight: &flight, Lane: &lane, Epoch: &epoch, Session: &session, ATCRunway: &runway,
		ExecutionWorkspaceID: &workspace, Adapter: &adapter, AdapterVersion: &adapterVersion,
		InputCommit: &inputCommit, InputTree: &inputTree, OutputCommit: &outputCommit, OutputTree: &outputTree,
		Clean: &clean, TerminalManifestDigest: &terminalDigest, TerminalManifestRevision: &terminalRevision,
		TerminalSourceSequence: &terminalSource, TerminalGlobalSequence: &terminalGlobal,
		EventChainRoot: &root, Receipt: receipt, ReceiptEventAck: ack,
	}, nil
}

func (doc executionResultDoc) toDomain() (ExecutionResult, error) {
	if doc.Schema == nil || doc.Verdict == nil || doc.Authority == nil || doc.Witnesses == nil || doc.Flight == nil ||
		doc.Lane == nil || doc.Epoch == nil || doc.Session == nil || doc.ATCRunway == nil || doc.ExecutionWorkspaceID == nil ||
		doc.Adapter == nil || doc.AdapterVersion == nil || doc.InputCommit == nil || doc.InputTree == nil || doc.OutputCommit == nil ||
		doc.OutputTree == nil || doc.Clean == nil || doc.TerminalManifestDigest == nil || doc.TerminalManifestRevision == nil ||
		doc.TerminalSourceSequence == nil || doc.TerminalGlobalSequence == nil || doc.EventChainRoot == nil || rawMissing(doc.Receipt) || rawMissing(doc.ReceiptEventAck) {
		return ExecutionResult{}, fmt.Errorf("sealedexec: execution result has an absent or null mandatory field")
	}
	receiptBytes, err := canonicalNested(doc.Receipt)
	if err != nil {
		return ExecutionResult{}, err
	}
	receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("sealedexec: decode result receipt: %w", err)
	}
	ackBytes, err := canonicalNested(doc.ReceiptEventAck)
	if err != nil {
		return ExecutionResult{}, err
	}
	ack, err := contextevent.DecodeReceiptEventAck(bytes.NewReader(ackBytes))
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("sealedexec: decode result receipt_event_ack: %w", err)
	}
	return ExecutionResult{
		Schema: *doc.Schema, Verdict: contextcompile.Resolution(*doc.Verdict), Authority: contextevent.Authority(*doc.Authority),
		Witnesses: doc.Witnesses, Flight: *doc.Flight, Lane: *doc.Lane, Epoch: *doc.Epoch, Session: *doc.Session,
		ATCRunway: *doc.ATCRunway, ExecutionWorkspaceID: *doc.ExecutionWorkspaceID,
		Adapter: contextevent.Adapter(*doc.Adapter), AdapterVersion: *doc.AdapterVersion,
		InputCommit: *doc.InputCommit, InputTree: *doc.InputTree, OutputCommit: *doc.OutputCommit, OutputTree: *doc.OutputTree,
		Clean: *doc.Clean, TerminalManifestDigest: *doc.TerminalManifestDigest,
		TerminalManifestRevision: *doc.TerminalManifestRevision, TerminalSourceSequence: *doc.TerminalSourceSequence,
		TerminalGlobalSequence: *doc.TerminalGlobalSequence, EventChainRoot: *doc.EventChainRoot,
		Receipt: receipt, ReceiptEventAck: ack,
	}, nil
}

// EncodeExecutionResult validates and canonically encodes a finalized result.
func EncodeExecutionResult(result ExecutionResult) ([]byte, error) {
	if err := validateExecutionResult(result); err != nil {
		return nil, err
	}
	doc, err := resultDocFor(result)
	if err != nil {
		return nil, err
	}
	return canonjson.Marshal(doc)
}

// DecodeExecutionResult strict-decodes a canonical finalized result.
func DecodeExecutionResult(reader io.Reader) (ExecutionResult, error) {
	var doc executionResultDoc
	raw, err := decodeStrict(reader, &doc)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("sealedexec: decode execution result: %w", err)
	}
	result, err := doc.toDomain()
	if err != nil {
		return ExecutionResult{}, err
	}
	if err := validateExecutionResult(result); err != nil {
		return ExecutionResult{}, err
	}
	canonical, err := EncodeExecutionResult(result)
	if err != nil {
		return ExecutionResult{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return ExecutionResult{}, fmt.Errorf("sealedexec: execution result is not byte-canonical")
	}
	return result, nil
}

func validateExecutionResult(result ExecutionResult) error {
	if result.Schema != ExecutionResultSchemaID {
		return fmt.Errorf("sealedexec: execution result schema must be %q", ExecutionResultSchemaID)
	}
	if err := result.Verdict.Validate(); err != nil {
		return fmt.Errorf("sealedexec: execution result verdict: %w", err)
	}
	if result.Authority != contextevent.AuthorityAuthoritative && result.Authority != contextevent.AuthorityAdvisory {
		return fmt.Errorf("sealedexec: execution result has unknown authority %q", result.Authority)
	}
	for field, value := range map[string]string{"flight": result.Flight, "lane": result.Lane, "epoch": result.Epoch, "session": result.Session, "atc_runway": result.ATCRunway, "execution_workspace_id": result.ExecutionWorkspaceID, "adapter_version": result.AdapterVersion} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	if err := result.Adapter.Validate(); err != nil {
		return fmt.Errorf("sealedexec: %w", err)
	}
	for field, value := range map[string]string{"input_commit": result.InputCommit, "input_tree": result.InputTree, "output_commit": result.OutputCommit, "output_tree": result.OutputTree} {
		if err := validateGitOID(field, value, false); err != nil {
			return err
		}
	}
	if err := validateDigest("terminal_manifest_digest", result.TerminalManifestDigest); err != nil {
		return err
	}
	if err := validateDigest("event_chain_root", result.EventChainRoot); err != nil {
		return err
	}
	if result.TerminalSourceSequence == 0 || result.TerminalGlobalSequence == 0 {
		return fmt.Errorf("sealedexec: execution result terminal sequences must be positive")
	}
	if err := validateWitnesses(result.Witnesses); err != nil {
		return err
	}
	receiptBytes, err := contextreceipt.EncodeReceipt(result.Receipt)
	if err != nil {
		return fmt.Errorf("sealedexec: execution result receipt: %w", err)
	}
	receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		return err
	}
	if result.Receipt.Digest != receipt.Digest {
		return fmt.Errorf("sealedexec: execution result receipt digest is stale")
	}
	if receipt.Role != contextreceipt.RoleBuilder {
		return fmt.Errorf("sealedexec: execution result requires a builder receipt")
	}
	if receipt.Authority != result.Authority {
		return fmt.Errorf("sealedexec: execution result authority contradicts receipt")
	}
	pairs := []struct{ field, left, right string }{
		{"atc_runway", result.ATCRunway, receipt.ATCRunway}, {"execution_workspace_id", result.ExecutionWorkspaceID, receipt.ExecutionWorkspaceID},
		{"adapter_version", result.AdapterVersion, receipt.AdapterVersion}, {"input_commit", result.InputCommit, receipt.InputCommit},
		{"input_tree", result.InputTree, receipt.InputTree}, {"output_commit", result.OutputCommit, receipt.OutputCommit},
		{"output_tree", result.OutputTree, receipt.OutputTree}, {"terminal_manifest_digest", result.TerminalManifestDigest, receipt.ManifestDigest},
		{"event_chain_root", result.EventChainRoot, receipt.EventChainRoot},
	}
	for _, pair := range pairs {
		if pair.left != pair.right {
			return fmt.Errorf("sealedexec: execution result %s contradicts receipt", pair.field)
		}
	}
	if result.Adapter != receipt.Adapter || result.Clean != receipt.Clean || result.TerminalManifestRevision != receipt.TerminalManifestRevision || result.TerminalSourceSequence != receipt.TerminalSourceSequence || result.TerminalGlobalSequence != receipt.TerminalGlobalSequence {
		return fmt.Errorf("sealedexec: execution result typed terminal or repository fact contradicts receipt")
	}
	ackBytes, err := contextevent.EncodeReceiptEventAck(result.ReceiptEventAck)
	if err != nil {
		return fmt.Errorf("sealedexec: execution result receipt_event_ack: %w", err)
	}
	ack, err := contextevent.DecodeReceiptEventAck(bytes.NewReader(ackBytes))
	if err != nil {
		return err
	}
	if ack.Flight != result.Flight || ack.Lane != result.Lane || ack.Epoch != result.Epoch || ack.Session != result.Session || ack.ManifestRevision != result.TerminalManifestRevision || ack.SourceSequence != result.TerminalSourceSequence+1 || ack.GlobalSequence <= result.TerminalGlobalSequence || ack.ReceiptDigest != receipt.Digest {
		return fmt.Errorf("sealedexec: execution result receipt_event_ack identity does not match result and receipt")
	}
	switch result.Authority {
	case contextevent.AuthorityAuthoritative:
		if result.Verdict != contextcompile.ResolutionProven || !result.Clean || len(result.Witnesses) != 0 {
			return fmt.Errorf("sealedexec: authoritative result requires proven verdict, clean output, and no adverse witnesses")
		}
	case contextevent.AuthorityAdvisory:
		if result.Verdict == contextcompile.ResolutionProven || len(result.Witnesses) == 0 {
			return fmt.Errorf("sealedexec: advisory result requires non-proven verdict and explicit witnesses")
		}
	}
	return nil
}

// Validate proves the provider input's structural authority/data separation.
func (input ProviderInput) Validate() error {
	if err := validateInstructionProjection(input.Instructions.Projection, true); err != nil {
		return fmt.Errorf("sealedexec: provider instruction authority: %w", err)
	}
	if input.Data == nil {
		return fmt.Errorf("sealedexec: provider data must be a non-null DataItem array")
	}
	for i, item := range input.Data {
		if item.Kind == contextcompile.IncludedInstructionProjection {
			return fmt.Errorf("sealedexec: provider data[%d] attempts to carry instruction authority", i)
		}
		if _, err := contextcompile.EncodeDataItem(item); err != nil {
			return fmt.Errorf("sealedexec: provider data[%d]: %w", i, err)
		}
	}
	return nil
}

func matchProjectionInventory(manifest []contextcompile.ProjectionFileRef, files []InstructionFile) error {
	if len(manifest) != len(files) {
		return fmt.Errorf("sealedexec: instruction projection does not exactly cover manifest projection_files")
	}
	for i := range manifest {
		if manifest[i].Path != files[i].Path || manifest[i].Digest != files[i].ContentDigest {
			return fmt.Errorf("sealedexec: instruction projection contradicts manifest projection_files[%d]", i)
		}
	}
	return nil
}

func validateLogicalRef(field string, ref LogicalRef, schema string) error {
	if ref.Schema != schema {
		return fmt.Errorf("sealedexec: %s.schema must be %q", field, schema)
	}
	if err := requireText(field+".id", ref.ID); err != nil {
		return err
	}
	return validateDigest(field+".digest", ref.Digest)
}

func validateWitnesses(witnesses []string) error {
	if witnesses == nil {
		return fmt.Errorf("sealedexec: witnesses must be non-null")
	}
	for i, witness := range witnesses {
		if err := requireText(fmt.Sprintf("witnesses[%d]", i), witness); err != nil {
			return err
		}
		if i > 0 && witnesses[i-1] >= witness {
			return fmt.Errorf("sealedexec: witnesses must be sorted and deduplicated")
		}
	}
	return nil
}

func validateProjectionPath(value string) error {
	if err := requireText("path", value); err != nil {
		return err
	}
	if strings.Contains(value, "\\") || strings.HasPrefix(value, "/") || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") {
		return fmt.Errorf("path %q must be a clean relative slash path", value)
	}
	return nil
}

func requireText(field, value string) error {
	if value == "" || !utf8.ValidString(value) || strings.TrimSpace(value) != value {
		return fmt.Errorf("sealedexec: %s must be nonempty UTF-8 without surrounding whitespace", field)
	}
	return nil
}

func validateDigest(field, value string) error {
	if !canonicalDigestRE.MatchString(value) {
		return fmt.Errorf("sealedexec: %s must be a canonical sha256 digest", field)
	}
	return nil
}

func validateGitOID(field, value string, commit bool) error {
	wantLengths := []int{40, 64}
	if commit {
		wantLengths = []int{40}
	}
	validLength := false
	for _, length := range wantLengths {
		validLength = validLength || len(value) == length
	}
	if !validLength || value != strings.ToLower(value) {
		return fmt.Errorf("sealedexec: %s must be a full lowercase hexadecimal Git object id", field)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("sealedexec: %s must be hexadecimal: %w", field, err)
	}
	return nil
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func decodeStrict(reader io.Reader, target any) ([]byte, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("trailing data")
		}
		return nil, fmt.Errorf("trailing data: %w", err)
	}
	return raw, nil
}

func requireFields(raw []byte, fields ...string) error {
	var object map[string]json.RawMessage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&object); err != nil {
		return err
	}
	for _, field := range fields {
		value, ok := object[field]
		if !ok || rawMissing(value) {
			return fmt.Errorf("sealedexec: %s is absent or null", field)
		}
	}
	return nil
}

func rawMissing(raw json.RawMessage) bool {
	return raw == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func canonicalNested(raw json.RawMessage) ([]byte, error) {
	if rawMissing(raw) {
		return nil, fmt.Errorf("sealedexec: nested document is absent or null")
	}
	canonical, err := canonjson.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("sealedexec: canonicalize nested document: %w", err)
	}
	return canonical, nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
