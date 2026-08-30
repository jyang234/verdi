package sealedreview

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/countersign"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
)

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

var packetExclusions = []string{
	"ambient-context",
	"builder-conversation",
	"global-memory",
	"personal-memory",
	"prior-reviewer-conversation",
}

// EncodePacket validates, self-digests, and canonically encodes a packet.
func EncodePacket(packet Packet) ([]byte, error) {
	if err := validatePacket(packet, false); err != nil {
		return nil, err
	}
	want, err := packetDigest(packet)
	if err != nil {
		return nil, err
	}
	if packet.Digest != "" && packet.Digest != want {
		return nil, fmt.Errorf("sealedreview: packet digest does not match canonical packet")
	}
	packet.Digest = want
	return canonjson.Marshal(packet)
}

// DecodePacket strictly decodes canonical packet bytes.
func DecodePacket(reader io.Reader) (Packet, error) {
	var packet Packet
	raw, err := readExact(reader, &packet, "packet")
	if err != nil {
		return Packet{}, err
	}
	if err := validatePacket(packet, true); err != nil {
		return Packet{}, err
	}
	canonical, err := canonjson.Marshal(packet)
	if err != nil {
		return Packet{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Packet{}, fmt.Errorf("sealedreview: packet is not byte-canonical")
	}
	return clonePacket(packet), nil
}

// EncodeDiff validates, self-digests, and canonically encodes an exact diff.
func EncodeDiff(diff Diff) ([]byte, error) {
	if err := validateDiff(diff, false); err != nil {
		return nil, err
	}
	want, err := diffDigest(diff)
	if err != nil {
		return nil, err
	}
	if diff.Digest != "" && diff.Digest != want {
		return nil, fmt.Errorf("sealedreview: diff digest does not match canonical diff")
	}
	diff.Digest = want
	return canonjson.Marshal(diff)
}

// DecodeDiff strictly decodes canonical diff bytes.
func DecodeDiff(reader io.Reader) (Diff, error) {
	var diff Diff
	raw, err := readExact(reader, &diff, "diff")
	if err != nil {
		return Diff{}, err
	}
	if err := validateDiff(diff, true); err != nil {
		return Diff{}, err
	}
	canonical, err := canonjson.Marshal(diff)
	if err != nil {
		return Diff{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Diff{}, fmt.Errorf("sealedreview: diff is not byte-canonical")
	}
	return cloneDiff(diff), nil
}

// EncodeEvidenceResult validates and self-digests one redacted result.
func EncodeEvidenceResult(result EvidenceResult) ([]byte, error) {
	if err := validateEvidenceResult(result, false); err != nil {
		return nil, err
	}
	want, err := evidenceResultDigest(result)
	if err != nil {
		return nil, err
	}
	if result.Digest != "" && result.Digest != want {
		return nil, fmt.Errorf("sealedreview: evidence result digest does not match canonical result")
	}
	result.Digest = want
	return canonjson.Marshal(result)
}

// DecodeEvidenceResult strictly decodes a canonical redacted result.
func DecodeEvidenceResult(reader io.Reader) (EvidenceResult, error) {
	var result EvidenceResult
	raw, err := readExact(reader, &result, "evidence result")
	if err != nil {
		return EvidenceResult{}, err
	}
	if err := validateEvidenceResult(result, true); err != nil {
		return EvidenceResult{}, err
	}
	canonical, err := canonjson.Marshal(result)
	if err != nil {
		return EvidenceResult{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return EvidenceResult{}, fmt.Errorf("sealedreview: evidence result is not byte-canonical")
	}
	return cloneEvidenceResult(result), nil
}

// VerifyEvidenceProof strict-decodes the component-owned result, including
// its self-digest and exact output digest, and returns only contextreceipt's
// closed cycle-free projection.
func VerifyEvidenceProof(raw []byte) (contextreceipt.EvidenceProofProjection, error) {
	result, err := DecodeEvidenceResult(bytes.NewReader(raw))
	if err != nil {
		return contextreceipt.EvidenceProofProjection{}, err
	}
	return contextreceipt.EvidenceProofProjection{
		CommandID: result.CommandID, Argv: append([]string(nil), result.Argv...), ExitCode: result.ExitCode,
		Verdict: result.Verdict, OutputDigest: result.OutputDigest,
	}, nil
}

// EncodeEvidenceBundle validates and self-digests one evidence wrapper.
func EncodeEvidenceBundle(bundle EvidenceBundle) ([]byte, error) {
	if err := validateEvidenceBundle(bundle, false); err != nil {
		return nil, err
	}
	want, err := evidenceBundleDigest(bundle)
	if err != nil {
		return nil, err
	}
	if bundle.Digest != "" && bundle.Digest != want {
		return nil, fmt.Errorf("sealedreview: evidence bundle digest does not match canonical bundle")
	}
	bundle.Digest = want
	return canonjson.Marshal(bundle)
}

// DecodeEvidenceBundle strictly decodes canonical evidence wrapper bytes.
func DecodeEvidenceBundle(reader io.Reader) (EvidenceBundle, error) {
	var bundle EvidenceBundle
	raw, err := readExact(reader, &bundle, "evidence bundle")
	if err != nil {
		return EvidenceBundle{}, err
	}
	if err := validateEvidenceBundle(bundle, true); err != nil {
		return EvidenceBundle{}, err
	}
	canonical, err := canonjson.Marshal(bundle)
	if err != nil {
		return EvidenceBundle{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return EvidenceBundle{}, fmt.Errorf("sealedreview: evidence bundle is not byte-canonical")
	}
	return cloneEvidenceBundle(bundle), nil
}

// EncodeAdjudication validates and self-digests an acknowledged adjudication.
func EncodeAdjudication(adjudication Adjudication) ([]byte, error) {
	if err := validateAdjudication(adjudication, false); err != nil {
		return nil, err
	}
	want, err := adjudicationDigest(adjudication)
	if err != nil {
		return nil, err
	}
	if adjudication.Digest != "" && adjudication.Digest != want {
		return nil, fmt.Errorf("sealedreview: adjudication digest does not match canonical adjudication")
	}
	adjudication.Digest = want
	return canonjson.Marshal(adjudication)
}

// DecodeAdjudication strictly decodes canonical adjudication wrapper bytes.
func DecodeAdjudication(reader io.Reader) (Adjudication, error) {
	var adjudication Adjudication
	raw, err := readExact(reader, &adjudication, "adjudication")
	if err != nil {
		return Adjudication{}, err
	}
	if err := validateAdjudication(adjudication, true); err != nil {
		return Adjudication{}, err
	}
	canonical, err := canonjson.Marshal(adjudication)
	if err != nil {
		return Adjudication{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Adjudication{}, fmt.Errorf("sealedreview: adjudication is not byte-canonical")
	}
	return cloneAdjudication(adjudication), nil
}

// EncodeContextBinding validates, self-digests, and canonically encodes the
// packet binding embedded in the review instruction projection.
func EncodeContextBinding(binding ContextBinding) ([]byte, error) {
	if err := validateContextBinding(binding, false); err != nil {
		return nil, err
	}
	want, err := contextBindingDigest(binding)
	if err != nil {
		return nil, err
	}
	if binding.Digest != "" && binding.Digest != want {
		return nil, fmt.Errorf("sealedreview: context binding digest does not match canonical binding")
	}
	binding.Digest = want
	return canonjson.Marshal(binding)
}

// DecodeContextBinding strictly decodes canonical packet-binding bytes.
func DecodeContextBinding(reader io.Reader) (ContextBinding, error) {
	var binding ContextBinding
	raw, err := readExact(reader, &binding, "context binding")
	if err != nil {
		return ContextBinding{}, err
	}
	if err := validateContextBinding(binding, true); err != nil {
		return ContextBinding{}, err
	}
	canonical, err := canonjson.Marshal(binding)
	if err != nil {
		return ContextBinding{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return ContextBinding{}, fmt.Errorf("sealedreview: context binding is not byte-canonical")
	}
	return cloneContextBinding(binding), nil
}

func readExact(reader io.Reader, out any, name string) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("sealedreview: read %s: nil reader", name)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("sealedreview: read %s: %w", name, err)
	}
	if err := artifact.DecodeExactJSON(raw, out); err != nil {
		return nil, fmt.Errorf("sealedreview: decode %s: %w", name, err)
	}
	return raw, nil
}

func packetDigest(packet Packet) (string, error) {
	packet.Digest = ""
	return canonjson.Digest(packet)
}

func diffDigest(diff Diff) (string, error) {
	diff.Digest = ""
	return canonjson.Digest(diff)
}

func evidenceResultDigest(result EvidenceResult) (string, error) {
	result.Digest = ""
	return canonjson.Digest(result)
}

func evidenceBundleDigest(bundle EvidenceBundle) (string, error) {
	bundle.Digest = ""
	return canonjson.Digest(bundle)
}

func adjudicationDigest(adjudication Adjudication) (string, error) {
	adjudication.Digest = ""
	return canonjson.Digest(adjudication)
}

func contextBindingDigest(binding ContextBinding) (string, error) {
	binding.Digest = ""
	return canonjson.Digest(binding)
}

func validatePacket(packet Packet, requireSelfDigest bool) error {
	if packet.Schema != PacketSchemaID {
		return fmt.Errorf("sealedreview: packet schema must be %q", PacketSchemaID)
	}
	if err := validateRound(packet.Round); err != nil {
		return err
	}
	if err := validateCandidate(packet.Candidate); err != nil {
		return err
	}
	if err := validateReviewer(packet.Reviewer); err != nil {
		return err
	}
	if err := requireDigest("builder_receipt_digest", packet.BuilderReceiptDigest); err != nil {
		return err
	}
	if packet.Items == nil {
		return fmt.Errorf("sealedreview: packet items must be non-null")
	}
	wantKinds := itemKinds(packet.Round)
	if len(packet.Items) != len(wantKinds) {
		return fmt.Errorf("sealedreview: %s packet must contain exactly %d items", packet.Round, len(wantKinds))
	}
	for i := range packet.Items {
		if packet.Items[i].Kind != wantKinds[i] {
			return fmt.Errorf("sealedreview: packet item[%d] kind must be %q", i, wantKinds[i])
		}
		if err := validateItem(packet, i); err != nil {
			return err
		}
	}
	if err := validateBuilderEvidenceReceiptBinding(packet.Items[2].Content, packet.Items[3].Content); err != nil {
		return err
	}
	if !equalStrings(packet.Exclusions, packetExclusions) {
		return fmt.Errorf("sealedreview: packet exclusions do not match the fixed exclusion set")
	}
	if packet.Round == RoundR2 && packet.Items[2].ContentDigest == packet.Items[6].ContentDigest {
		return fmt.Errorf("sealedreview: R2 current-candidate evidence must differ from builder evidence")
	}
	if requireSelfDigest || packet.Digest != "" {
		if err := requireDigest("packet digest", packet.Digest); err != nil {
			return err
		}
		want, err := packetDigest(packet)
		if err != nil {
			return err
		}
		if packet.Digest != want {
			return fmt.Errorf("sealedreview: packet digest does not match canonical packet")
		}
	}
	return nil
}

func validateBuilderEvidenceReceiptBinding(bundleBytes, receiptBytes []byte) error {
	bundle, err := DecodeEvidenceBundle(bytes.NewReader(bundleBytes))
	if err != nil {
		return fmt.Errorf("sealedreview: builder evidence binding: %w", err)
	}
	receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		return fmt.Errorf("sealedreview: builder evidence receipt binding: %w", err)
	}
	if len(bundle.Rows) != len(receipt.Evidence) {
		return fmt.Errorf("sealedreview: builder evidence count does not match embedded builder receipt")
	}
	for i, row := range bundle.Rows {
		projection, err := VerifyEvidenceProof(row.ResultBytes)
		if err != nil {
			return fmt.Errorf("sealedreview: builder evidence row[%d]: %w", i, err)
		}
		want := receipt.Evidence[i]
		if projection.CommandID != want.CommandID || !equalStrings(projection.Argv, want.Argv) ||
			projection.ExitCode != want.ExitCode || projection.Verdict != want.Verdict || projection.OutputDigest != want.OutputDigest {
			return fmt.Errorf("sealedreview: builder evidence row[%d] does not match embedded builder receipt", i)
		}
	}
	return nil
}

func validateItem(packet Packet, index int) error {
	item := packet.Items[index]
	prefix := fmt.Sprintf("packet item[%d]", index)
	if err := requireText(prefix+" id", item.ID); err != nil {
		return err
	}
	if len(item.Content) == 0 {
		return fmt.Errorf("sealedreview: %s content must be nonempty", prefix)
	}
	if err := requireDigest(prefix+" content_digest", item.ContentDigest); err != nil {
		return err
	}
	if item.ContentDigest != rawDigest(item.Content) {
		return fmt.Errorf("sealedreview: %s content_digest does not match exact content bytes", prefix)
	}
	wantMedia := "application/json"
	var wantID string
	switch item.Kind {
	case ItemAcceptedSpec:
		wantMedia = "text/markdown; charset=utf-8"
		id, err := decodeSpecID(item.Content)
		if err != nil {
			return fmt.Errorf("sealedreview: accepted spec: %w", err)
		}
		wantID = id
	case ItemCurrentDiff:
		diff, err := DecodeDiff(bytes.NewReader(item.Content))
		if err != nil {
			return fmt.Errorf("sealedreview: current diff: %w", err)
		}
		if diff.BaseCommit != packet.Candidate.BaseCommit || diff.BaseTree != packet.Candidate.BaseTree || diff.HeadCommit != packet.Candidate.HeadCommit || diff.HeadTree != packet.Candidate.HeadTree {
			return fmt.Errorf("sealedreview: current diff candidate does not match packet")
		}
		wantID = packet.Candidate.BaseCommit + ".." + packet.Candidate.HeadCommit
	case ItemEvidenceBundle:
		bundle, err := DecodeEvidenceBundle(bytes.NewReader(item.Content))
		if err != nil {
			return fmt.Errorf("sealedreview: builder evidence: %w", err)
		}
		if bundle.Scope != EvidenceScopeBuilder || bundle.Candidate != packet.Candidate {
			return fmt.Errorf("sealedreview: builder evidence scope or candidate mismatch")
		}
		wantID = packet.Candidate.HeadCommit
	case ItemBuilderReceipt:
		receipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(item.Content))
		if err != nil {
			return fmt.Errorf("sealedreview: builder receipt: %w", err)
		}
		if receipt.Role != contextreceipt.RoleBuilder || receipt.Digest != packet.BuilderReceiptDigest {
			return fmt.Errorf("sealedreview: builder receipt identity mismatch")
		}
		if receipt.InputCommit != packet.Candidate.BaseCommit || receipt.InputTree != packet.Candidate.BaseTree ||
			receipt.OutputCommit != packet.Candidate.HeadCommit || receipt.OutputTree != packet.Candidate.HeadTree || !receipt.Clean {
			return fmt.Errorf("sealedreview: builder receipt candidate mismatch")
		}
		wantID = receipt.Digest
	case ItemReviewPolicy:
		wantMedia = "text/markdown; charset=utf-8"
		policy, err := policyartifact.DecodePolicy(item.Content)
		if err != nil {
			return fmt.Errorf("sealedreview: review policy: %w", err)
		}
		wantID = policy.ID
	case ItemAdjudication:
		adjudication, err := DecodeAdjudication(bytes.NewReader(item.Content))
		if err != nil {
			return fmt.Errorf("sealedreview: adjudication: %w", err)
		}
		wantID = adjudication.R0ReceiptDigest
	case ItemCurrentCandidateEvidence:
		bundle, err := DecodeEvidenceBundle(bytes.NewReader(item.Content))
		if err != nil {
			return fmt.Errorf("sealedreview: current candidate evidence: %w", err)
		}
		if bundle.Scope != EvidenceScopeCurrentCandidate || bundle.Candidate != packet.Candidate {
			return fmt.Errorf("sealedreview: current candidate evidence scope or candidate mismatch")
		}
		wantID = packet.Candidate.HeadCommit
	default:
		return fmt.Errorf("sealedreview: unknown packet item kind %q", item.Kind)
	}
	if item.ID != wantID {
		return fmt.Errorf("sealedreview: %s id must be %q", prefix, wantID)
	}
	if item.MediaType != wantMedia {
		return fmt.Errorf("sealedreview: %s media_type must be %q", prefix, wantMedia)
	}
	return nil
}

func validateDiff(diff Diff, requireSelfDigest bool) error {
	if diff.Schema != DiffSchemaID {
		return fmt.Errorf("sealedreview: diff schema must be %q", DiffSchemaID)
	}
	candidate := contextreceipt.Candidate{BaseCommit: diff.BaseCommit, BaseTree: diff.BaseTree, HeadCommit: diff.HeadCommit, HeadTree: diff.HeadTree}
	if err := validateCandidate(candidate); err != nil {
		return err
	}
	if diff.Entries == nil {
		return fmt.Errorf("sealedreview: diff entries must be non-null")
	}
	for i, entry := range diff.Entries {
		if err := validateDiffEntry(entry); err != nil {
			return fmt.Errorf("sealedreview: diff entries[%d]: %w", i, err)
		}
		if i > 0 && bytes.Compare(diff.Entries[i-1].Path, entry.Path) >= 0 {
			return fmt.Errorf("sealedreview: diff entries must be sorted and deduplicated by path")
		}
	}
	if requireSelfDigest || diff.Digest != "" {
		if err := requireDigest("diff digest", diff.Digest); err != nil {
			return err
		}
		want, err := diffDigest(diff)
		if err != nil {
			return err
		}
		if diff.Digest != want {
			return fmt.Errorf("sealedreview: diff digest does not match canonical diff")
		}
	}
	return nil
}

func validateDiffEntry(entry DiffEntry) error {
	if err := validateRawPath(entry.Path); err != nil {
		return err
	}
	beforePresent := entry.BeforeMode != "" || entry.BeforeBlob != "" || len(entry.BeforeBytes) != 0
	afterPresent := entry.AfterMode != "" || entry.AfterBlob != "" || len(entry.AfterBytes) != 0
	if entry.BeforeBytes == nil || entry.AfterBytes == nil {
		return fmt.Errorf("before_bytes and after_bytes must be non-null")
	}
	switch entry.State {
	case DiffAdded:
		if beforePresent || !afterPresent {
			return fmt.Errorf("added entry must carry only its after side")
		}
	case DiffModified:
		if !beforePresent || !afterPresent {
			return fmt.Errorf("modified entry must carry both sides")
		}
	case DiffDeleted:
		if !beforePresent || afterPresent {
			return fmt.Errorf("deleted entry must carry only its before side")
		}
	default:
		return fmt.Errorf("unknown diff state %q", entry.State)
	}
	if beforePresent {
		if err := validateBlobSide("before", entry.BeforeMode, entry.BeforeBlob, entry.BeforeBytes); err != nil {
			return err
		}
	}
	if afterPresent {
		if err := validateBlobSide("after", entry.AfterMode, entry.AfterBlob, entry.AfterBytes); err != nil {
			return err
		}
	}
	if entry.State == DiffModified && entry.BeforeMode == entry.AfterMode && entry.BeforeBlob == entry.AfterBlob {
		return fmt.Errorf("modified entry has identical sides")
	}
	return nil
}

func validateEvidenceResult(result EvidenceResult, requireSelfDigest bool) error {
	if result.Schema != EvidenceResultSchemaID {
		return fmt.Errorf("sealedreview: evidence result schema must be %q", EvidenceResultSchemaID)
	}
	if err := requireText("evidence command_id", result.CommandID); err != nil {
		return err
	}
	if len(result.Argv) == 0 {
		return fmt.Errorf("sealedreview: evidence argv must be non-null and nonempty")
	}
	for i, arg := range result.Argv {
		if err := requireText(fmt.Sprintf("evidence argv[%d]", i), arg); err != nil {
			return err
		}
	}
	if result.Output == nil {
		return fmt.Errorf("sealedreview: evidence output must be non-null")
	}
	switch result.Verdict {
	case countersign.VerdictProven, countersign.VerdictViolated, countersign.VerdictUnproven:
	default:
		return fmt.Errorf("sealedreview: unknown evidence verdict %q", result.Verdict)
	}
	if err := requireDigest("evidence output_digest", result.OutputDigest); err != nil {
		return err
	}
	if result.OutputDigest != rawDigest(result.Output) {
		return fmt.Errorf("sealedreview: evidence output_digest does not match exact output bytes")
	}
	if requireSelfDigest || result.Digest != "" {
		if err := requireDigest("evidence result digest", result.Digest); err != nil {
			return err
		}
		want, err := evidenceResultDigest(result)
		if err != nil {
			return err
		}
		if result.Digest != want {
			return fmt.Errorf("sealedreview: evidence result digest does not match canonical result")
		}
	}
	return nil
}

func validateEvidenceBundle(bundle EvidenceBundle, requireSelfDigest bool) error {
	if bundle.Schema != EvidenceBundleSchemaID {
		return fmt.Errorf("sealedreview: evidence bundle schema must be %q", EvidenceBundleSchemaID)
	}
	switch bundle.Scope {
	case EvidenceScopeBuilder, EvidenceScopeCurrentCandidate:
	default:
		return fmt.Errorf("sealedreview: unknown evidence scope %q", bundle.Scope)
	}
	if err := validateCandidate(bundle.Candidate); err != nil {
		return err
	}
	if bundle.Rows == nil {
		return fmt.Errorf("sealedreview: evidence rows must be non-null")
	}
	for i, row := range bundle.Rows {
		if err := requireText(fmt.Sprintf("evidence rows[%d] command_id", i), row.CommandID); err != nil {
			return err
		}
		if len(row.ResultBytes) == 0 {
			return fmt.Errorf("sealedreview: evidence rows[%d] result_bytes must be nonempty", i)
		}
		if err := requireDigest(fmt.Sprintf("evidence rows[%d] result_digest", i), row.ResultDigest); err != nil {
			return err
		}
		if row.ResultDigest != rawDigest(row.ResultBytes) {
			return fmt.Errorf("sealedreview: evidence rows[%d] result_digest does not match exact bytes", i)
		}
		result, err := DecodeEvidenceResult(bytes.NewReader(row.ResultBytes))
		if err != nil {
			return fmt.Errorf("sealedreview: evidence rows[%d]: %w", i, err)
		}
		if result.CommandID != row.CommandID {
			return fmt.Errorf("sealedreview: evidence rows[%d] command identity mismatch", i)
		}
		if i > 0 && bundle.Rows[i-1].CommandID >= row.CommandID {
			return fmt.Errorf("sealedreview: evidence rows must be sorted and deduplicated by command_id")
		}
	}
	if requireSelfDigest || bundle.Digest != "" {
		if err := requireDigest("evidence bundle digest", bundle.Digest); err != nil {
			return err
		}
		want, err := evidenceBundleDigest(bundle)
		if err != nil {
			return err
		}
		if bundle.Digest != want {
			return fmt.Errorf("sealedreview: evidence bundle digest does not match canonical bundle")
		}
	}
	return nil
}

func validateAdjudication(adjudication Adjudication, requireSelfDigest bool) error {
	if adjudication.Schema != AdjudicationSchemaID {
		return fmt.Errorf("sealedreview: adjudication schema must be %q", AdjudicationSchemaID)
	}
	if err := requireDigest("adjudication r0_receipt_digest", adjudication.R0ReceiptDigest); err != nil {
		return err
	}
	if adjudication.Rows == nil {
		return fmt.Errorf("sealedreview: adjudication rows must be non-null")
	}
	if len(adjudication.Rows) == 0 {
		return fmt.Errorf("sealedreview: adjudication rows must contain at least one accepted decision")
	}
	priorID, priorDigest := "", ""
	for i, row := range adjudication.Rows {
		if err := requireText(fmt.Sprintf("adjudication rows[%d] finding_or_deviation_id", i), row.FindingOrDeviationID); err != nil {
			return err
		}
		if len(row.EventBytes) == 0 {
			return fmt.Errorf("sealedreview: adjudication rows[%d] event_bytes must be nonempty", i)
		}
		event, err := contextevent.DecodeEvent(bytes.NewReader(row.EventBytes))
		if err != nil {
			return fmt.Errorf("sealedreview: adjudication rows[%d] event: %w", i, err)
		}
		payload, ok := event.Payload.(*contextevent.AdjudicationPayload)
		if !ok || event.Kind != contextevent.KindAdjudication || payload.FindingOrDeviationID != row.FindingOrDeviationID {
			return fmt.Errorf("sealedreview: adjudication rows[%d] event identity mismatch", i)
		}
		if payload.Decision != "accept" {
			return fmt.Errorf("sealedreview: adjudication rows[%d] decision must be accept", i)
		}
		if payload.PrincipalResolution.State != gp.ResolutionAuthenticated {
			return fmt.Errorf("sealedreview: adjudication rows[%d] principal must be authenticated", i)
		}
		if _, err := contextevent.EncodeEventAck(row.Ack); err != nil {
			return fmt.Errorf("sealedreview: adjudication rows[%d] acknowledgment: %w", i, err)
		}
		if !ackMatchesEvent(row.Ack, event) {
			return fmt.Errorf("sealedreview: adjudication rows[%d] acknowledgment does not bind exact event", i)
		}
		if i > 0 && (priorID > row.FindingOrDeviationID || (priorID == row.FindingOrDeviationID && priorDigest >= event.EventDigest)) {
			return fmt.Errorf("sealedreview: adjudication rows must be sorted and deduplicated")
		}
		priorID, priorDigest = row.FindingOrDeviationID, event.EventDigest
	}
	if requireSelfDigest || adjudication.Digest != "" {
		if err := requireDigest("adjudication digest", adjudication.Digest); err != nil {
			return err
		}
		want, err := adjudicationDigest(adjudication)
		if err != nil {
			return err
		}
		if adjudication.Digest != want {
			return fmt.Errorf("sealedreview: adjudication digest does not match canonical adjudication")
		}
	}
	return nil
}

func validateContextBinding(binding ContextBinding, requireSelfDigest bool) error {
	if binding.Schema != ReviewBindingSchemaID {
		return fmt.Errorf("sealedreview: context binding schema must be %q", ReviewBindingSchemaID)
	}
	for field, value := range map[string]string{
		"packet_digest":          binding.PacketDigest,
		"accepted_spec_digest":   binding.AcceptedSpecDigest,
		"review_policy_digest":   binding.ReviewPolicyDigest,
		"builder_receipt_digest": binding.BuilderReceiptDigest,
	} {
		if err := requireDigest("context binding "+field, value); err != nil {
			return err
		}
	}
	if err := requireGitOID("context binding head_commit", binding.HeadCommit); err != nil {
		return err
	}
	if err := requireGitOID("context binding head_tree", binding.HeadTree); err != nil {
		return err
	}
	if binding.ItemProjection == nil {
		return fmt.Errorf("sealedreview: context binding item_projection must be non-null")
	}
	if len(binding.ItemProjection) != 5 && len(binding.ItemProjection) != 7 {
		return fmt.Errorf("sealedreview: context binding item_projection must contain an exact R0 or R2 inventory")
	}
	wantKinds := []string{"accepted-spec", "builder-receipt", "current-diff", "evidence-bundle", "review-policy"}
	if len(binding.ItemProjection) == 7 {
		wantKinds = []string{"accepted-spec", "adjudication", "builder-receipt", "current-candidate-evidence", "current-diff", "evidence-bundle", "review-policy"}
	}
	for i, row := range binding.ItemProjection {
		if row.Kind != wantKinds[i] {
			return fmt.Errorf("sealedreview: context binding item_projection[%d] kind must be %q", i, wantKinds[i])
		}
		if err := requireDigest(fmt.Sprintf("context binding item_projection[%d] content_digest", i), row.ContentDigest); err != nil {
			return err
		}
	}
	if requireSelfDigest || binding.Digest != "" {
		if err := requireDigest("context binding digest", binding.Digest); err != nil {
			return err
		}
		want, err := contextBindingDigest(binding)
		if err != nil {
			return err
		}
		if binding.Digest != want {
			return fmt.Errorf("sealedreview: context binding digest does not match canonical binding")
		}
	}
	return nil
}

func itemKinds(round Round) []ItemKind {
	kinds := []ItemKind{ItemAcceptedSpec, ItemCurrentDiff, ItemEvidenceBundle, ItemBuilderReceipt, ItemReviewPolicy}
	if round == RoundR2 {
		kinds = append(kinds, ItemAdjudication, ItemCurrentCandidateEvidence)
	}
	return kinds
}

func validateRound(round Round) error {
	if round != RoundR0 && round != RoundR2 {
		return fmt.Errorf("sealedreview: unknown review round %q", round)
	}
	return nil
}

func validateCandidate(candidate contextreceipt.Candidate) error {
	for field, value := range map[string]string{
		"base_commit": candidate.BaseCommit,
		"base_tree":   candidate.BaseTree,
		"head_commit": candidate.HeadCommit,
		"head_tree":   candidate.HeadTree,
	} {
		if err := requireGitOID(field, value); err != nil {
			return err
		}
	}
	return nil
}

func validateReviewer(reviewer Reviewer) error {
	for field, value := range map[string]string{
		"reviewer lane":            reviewer.Lane,
		"reviewer adapter_version": reviewer.AdapterVersion,
		"reviewer model":           reviewer.Model,
		"reviewer profile_id":      reviewer.ProfileID,
	} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	if err := reviewer.Adapter.Validate(); err != nil {
		return fmt.Errorf("sealedreview: reviewer: %w", err)
	}
	return requireDigest("reviewer profile_digest", reviewer.ProfileDigest)
}

func validateBlobSide(name, mode, oid string, content []byte) error {
	switch mode {
	case "100644", "100755", "120000":
	default:
		return fmt.Errorf("%s_mode %q is not a supported blob mode", name, mode)
	}
	if err := requireGitOID(name+"_blob", oid); err != nil {
		return err
	}
	if gitObjectOID("blob", content) != oid {
		return fmt.Errorf("%s_blob does not match exact bytes", name)
	}
	return nil
}

func validateSelectorPath(value string) error {
	if err := requireText("diff path", value); err != nil {
		return err
	}
	if strings.HasPrefix(value, "/") || strings.Contains(value, "\\") || path.Clean(value) != value || value == "." || strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("diff path %q is not canonical repository-relative path", value)
	}
	return nil
}

func validateRawPath(value []byte) error {
	if len(value) == 0 {
		return fmt.Errorf("diff path must be nonempty")
	}
	if value[0] == '/' || value[len(value)-1] == '/' || bytes.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("diff path %q is not a repository-relative Git path", value)
	}
	for _, component := range bytes.Split(value, []byte{'/'}) {
		if len(component) == 0 || bytes.Equal(component, []byte(".")) || bytes.Equal(component, []byte("..")) {
			return fmt.Errorf("diff path %q contains an invalid component", value)
		}
	}
	return nil
}

func decodeSpecID(content []byte) (string, error) {
	frontmatter := content
	if bytes.HasPrefix(content, []byte("---\n")) {
		decoded, _, err := artifact.SplitFrontmatter(content)
		if err != nil {
			return "", err
		}
		frontmatter = decoded
	}
	spec, err := artifact.DecodeSpec(frontmatter)
	if err != nil {
		return "", err
	}
	return spec.ID, nil
}

func ackMatchesEvent(ack contextevent.EventAck, event contextevent.Event) bool {
	return ack.Flight == event.Flight && ack.Lane == event.Lane && ack.Epoch == event.Epoch &&
		ack.Session == event.Session && ack.ManifestRevision == event.ManifestRevision &&
		ack.Kind == event.Kind && ack.SourceSequence == event.SourceSequence &&
		ack.EventDigest == event.EventDigest
}

func requireText(field, value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return fmt.Errorf("sealedreview: %s must be nonempty without surrounding whitespace", field)
	}
	return nil
}

func requireDigest(field, value string) error {
	if !digestPattern.MatchString(value) {
		return fmt.Errorf("sealedreview: %s must be a canonical sha256 digest", field)
	}
	return nil
}

func requireGitOID(field, value string) error {
	if len(value) != 40 {
		return fmt.Errorf("sealedreview: %s must be a full I-90 SHA-1 Git object id", field)
	}
	if value != strings.ToLower(value) {
		return fmt.Errorf("sealedreview: %s must be lowercase hexadecimal", field)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("sealedreview: %s must be hexadecimal: %w", field, err)
	}
	return nil
}

func rawDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func gitObjectOID(kind string, content []byte) string {
	preimage := append([]byte(fmt.Sprintf("%s %d\x00", kind, len(content))), content...)
	sum := sha1.Sum(preimage)
	return hex.EncodeToString(sum[:])
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func clonePacket(packet Packet) Packet {
	packet.Items = cloneItems(packet.Items)
	if packet.Exclusions != nil {
		packet.Exclusions = append([]string{}, packet.Exclusions...)
	}
	return packet
}

func cloneItems(items []Item) []Item {
	if items == nil {
		return nil
	}
	cloned := append([]Item{}, items...)
	for i := range cloned {
		cloned[i].Content = append([]byte(nil), cloned[i].Content...)
	}
	return cloned
}

func cloneDiff(diff Diff) Diff {
	if diff.Entries != nil {
		diff.Entries = append([]DiffEntry{}, diff.Entries...)
	}
	for i := range diff.Entries {
		diff.Entries[i].Path = cloneBytes(diff.Entries[i].Path)
		diff.Entries[i].BeforeBytes = cloneBytes(diff.Entries[i].BeforeBytes)
		diff.Entries[i].AfterBytes = cloneBytes(diff.Entries[i].AfterBytes)
	}
	return diff
}

func cloneEvidenceResult(result EvidenceResult) EvidenceResult {
	result.Argv = append([]string(nil), result.Argv...)
	result.Output = cloneBytes(result.Output)
	return result
}

func cloneEvidenceBundle(bundle EvidenceBundle) EvidenceBundle {
	if bundle.Rows != nil {
		bundle.Rows = append([]EvidenceRow{}, bundle.Rows...)
	}
	for i := range bundle.Rows {
		bundle.Rows[i].ResultBytes = append([]byte(nil), bundle.Rows[i].ResultBytes...)
	}
	return bundle
}

func cloneAdjudication(adjudication Adjudication) Adjudication {
	if adjudication.Rows != nil {
		adjudication.Rows = append([]AdjudicationRow{}, adjudication.Rows...)
	}
	for i := range adjudication.Rows {
		adjudication.Rows[i].EventBytes = append([]byte(nil), adjudication.Rows[i].EventBytes...)
	}
	return adjudication
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte{}, value...)
}
