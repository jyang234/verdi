package contextreceipt

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/countersign"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

var receiptDigestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// EncodeReceipt validates, self-digests, and canonically encodes receipt. A
// blank digest is populated in the encoded value; a nonblank mismatch fails.
func EncodeReceipt(receipt Receipt) ([]byte, error) {
	if err := validateReceipt(receipt, false); err != nil {
		return nil, err
	}
	want, err := receiptDigest(receipt)
	if err != nil {
		return nil, err
	}
	if receipt.Digest != "" && receipt.Digest != want {
		return nil, fmt.Errorf("contextreceipt: digest does not match canonical receipt")
	}
	receipt.Digest = want
	return canonjson.Marshal(receipt)
}

// DecodeReceipt strictly decodes, validates, digest-checks, and requires the
// input receipt bytes to already be canonical.
func DecodeReceipt(reader io.Reader) (Receipt, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return Receipt{}, fmt.Errorf("contextreceipt: read receipt: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var receipt Receipt
	if err := decoder.Decode(&receipt); err != nil {
		return Receipt{}, fmt.Errorf("contextreceipt: decode receipt: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Receipt{}, fmt.Errorf("contextreceipt: trailing data after receipt")
		}
		return Receipt{}, fmt.Errorf("contextreceipt: trailing data after receipt: %w", err)
	}
	if err := validateReceipt(receipt, true); err != nil {
		return Receipt{}, err
	}
	canonical, err := canonjson.Marshal(receipt)
	if err != nil {
		return Receipt{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Receipt{}, fmt.Errorf("contextreceipt: receipt is not byte-canonical")
	}
	return receipt, nil
}

// EncodeVerifyRequest validates, self-digests, and canonically encodes a
// public receipt-verification request.
func EncodeVerifyRequest(request VerifyRequest) ([]byte, error) {
	if err := validateVerifyRequest(request, false); err != nil {
		return nil, err
	}
	want, err := verifyRequestDigest(request)
	if err != nil {
		return nil, err
	}
	if request.Digest != "" && request.Digest != want {
		return nil, fmt.Errorf("contextreceipt: verify request digest does not match canonical request")
	}
	request.Digest = want
	return canonjson.Marshal(request)
}

// DecodeVerifyRequest strictly decodes, validates, digest-checks, and
// requires an already-canonical request document.
func DecodeVerifyRequest(reader io.Reader) (VerifyRequest, error) {
	var request VerifyRequest
	raw, err := readExactJSON(reader, &request, "verify request")
	if err != nil {
		return VerifyRequest{}, err
	}
	if err := validateVerifyRequest(request, true); err != nil {
		return VerifyRequest{}, err
	}
	canonical, err := canonjson.Marshal(request)
	if err != nil {
		return VerifyRequest{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return VerifyRequest{}, fmt.Errorf("contextreceipt: verify request is not byte-canonical")
	}
	return cloneVerifyRequest(request), nil
}

// EncodeRepositoryProof validates, self-digests, and canonically encodes an
// offline SHA-1 repository proof.
func EncodeRepositoryProof(proof RepositoryProof) ([]byte, error) {
	if err := validateRepositoryProofShape(proof, false); err != nil {
		return nil, err
	}
	want, err := repositoryProofDigest(proof)
	if err != nil {
		return nil, err
	}
	if proof.Digest != "" && proof.Digest != want {
		return nil, fmt.Errorf("contextreceipt: repository proof digest does not match canonical proof")
	}
	proof.Digest = want
	return canonjson.Marshal(proof)
}

// DecodeRepositoryProof strictly decodes a canonical repository proof.
func DecodeRepositoryProof(reader io.Reader) (RepositoryProof, error) {
	var proof RepositoryProof
	raw, err := readExactJSON(reader, &proof, "repository proof")
	if err != nil {
		return RepositoryProof{}, err
	}
	if err := validateRepositoryProofShape(proof, true); err != nil {
		return RepositoryProof{}, err
	}
	canonical, err := canonjson.Marshal(proof)
	if err != nil {
		return RepositoryProof{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return RepositoryProof{}, fmt.Errorf("contextreceipt: repository proof is not byte-canonical")
	}
	proof.Objects = cloneRepositoryObjects(proof.Objects)
	return proof, nil
}

// EncodeVerdict validates, self-digests, and canonically encodes one verdict.
func EncodeVerdict(verdict Verdict) ([]byte, error) {
	if err := validateVerdict(verdict, false); err != nil {
		return nil, err
	}
	want, err := verdictDigest(verdict)
	if err != nil {
		return nil, err
	}
	if verdict.Digest != "" && verdict.Digest != want {
		return nil, fmt.Errorf("contextreceipt: verdict digest does not match canonical verdict")
	}
	verdict.Digest = want
	return canonjson.Marshal(verdict)
}

// DecodeVerdict strictly decodes a canonical self-digested verdict.
func DecodeVerdict(reader io.Reader) (Verdict, error) {
	var verdict Verdict
	raw, err := readExactJSON(reader, &verdict, "verdict")
	if err != nil {
		return Verdict{}, err
	}
	if err := validateVerdict(verdict, true); err != nil {
		return Verdict{}, err
	}
	canonical, err := canonjson.Marshal(verdict)
	if err != nil {
		return Verdict{}, err
	}
	if !bytes.Equal(raw, canonical) {
		return Verdict{}, fmt.Errorf("contextreceipt: verdict is not byte-canonical")
	}
	return cloneVerdict(verdict), nil
}

func readExactJSON(reader io.Reader, out any, name string) ([]byte, error) {
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("contextreceipt: read %s: %w", name, err)
	}
	if err := artifact.DecodeExactJSON(raw, out); err != nil {
		return nil, fmt.Errorf("contextreceipt: decode %s: %w", name, err)
	}
	return raw, nil
}

func verifyRequestDigest(request VerifyRequest) (string, error) {
	request.Digest = ""
	return canonjson.Digest(request)
}

func repositoryProofDigest(proof RepositoryProof) (string, error) {
	proof.Digest = ""
	return canonjson.Digest(proof)
}

func verdictDigest(verdict Verdict) (string, error) {
	verdict.Digest = ""
	return canonjson.Digest(verdict)
}

func validateVerifyRequest(request VerifyRequest, requireDigest bool) error {
	if request.Schema != VerifyRequestSchemaID {
		return fmt.Errorf("contextreceipt: verify request schema must be %q", VerifyRequestSchemaID)
	}
	receiptBytes, err := EncodeReceipt(request.Receipt)
	if err != nil {
		return fmt.Errorf("contextreceipt: verify request receipt: %w", err)
	}
	if request.Receipt.Digest == "" {
		return fmt.Errorf("contextreceipt: verify request receipt must carry its digest")
	}
	if _, err := DecodeReceipt(bytes.NewReader(receiptBytes)); err != nil {
		return fmt.Errorf("contextreceipt: verify request receipt: %w", err)
	}
	if _, err := contextevent.EncodeReceiptEventAck(request.ReceiptEventAck); err != nil {
		return fmt.Errorf("contextreceipt: verify request receipt_event_ack: %w", err)
	}
	if err := validateCandidate(request.Candidate); err != nil {
		return err
	}
	proofs := request.Proofs
	for field, value := range map[string][]byte{
		"execution_request_bytes": proofs.ExecutionRequestBytes,
		"repository_proof_bytes":  proofs.RepositoryProofBytes,
		"receipt_event_bytes":     proofs.ReceiptEventBytes,
	} {
		if len(value) == 0 {
			return fmt.Errorf("contextreceipt: verify request proofs.%s must be nonempty", field)
		}
	}
	for field, value := range map[string][][]byte{
		"execution_event_bytes": proofs.ExecutionEventBytes,
		"expansion_data_bytes":  proofs.ExpansionDataBytes,
		"obligation_bytes":      proofs.ObligationBytes,
		"evidence_result_bytes": proofs.EvidenceResultBytes,
	} {
		if value == nil {
			return fmt.Errorf("contextreceipt: verify request proofs.%s must be non-null", field)
		}
		for i, document := range value {
			if len(document) == 0 {
				return fmt.Errorf("contextreceipt: verify request proofs.%s[%d] must be nonempty", field, i)
			}
		}
	}
	if len(proofs.ExecutionEventBytes) == 0 {
		return fmt.Errorf("contextreceipt: verify request proofs.execution_event_bytes must be nonempty")
	}
	if proofs.ReviewPacketBytes == nil {
		return fmt.Errorf("contextreceipt: verify request proofs.review_packet_bytes must be non-null")
	}
	if request.Receipt.Role == RoleBuilder && len(proofs.ReviewPacketBytes) != 0 {
		return fmt.Errorf("contextreceipt: builder verify request forbids review_packet_bytes")
	}
	if request.Receipt.Role == RoleReviewer && len(proofs.ReviewPacketBytes) == 0 {
		return fmt.Errorf("contextreceipt: reviewer verify request requires review_packet_bytes")
	}
	if requireDigest || request.Digest != "" {
		if err := validateReceiptDigest("verify request digest", request.Digest); err != nil {
			return err
		}
		want, err := verifyRequestDigest(request)
		if err != nil {
			return err
		}
		if request.Digest != want {
			return fmt.Errorf("contextreceipt: verify request digest does not match canonical request")
		}
	}
	return nil
}

func validateCandidate(candidate Candidate) error {
	for field, value := range map[string]string{
		"candidate.base_commit": candidate.BaseCommit, "candidate.base_tree": candidate.BaseTree,
		"candidate.head_commit": candidate.HeadCommit, "candidate.head_tree": candidate.HeadTree,
	} {
		if len(value) != 40 || !isLowerHex(value) {
			return fmt.Errorf("contextreceipt: %s must be a SHA-1 Git object", field)
		}
	}
	return nil
}

func isLowerHex(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}

func validateRepositoryProofShape(proof RepositoryProof, requireDigest bool) error {
	if proof.Schema != RepositoryProofSchemaID {
		return fmt.Errorf("contextreceipt: repository proof schema must be %q", RepositoryProofSchemaID)
	}
	if proof.ObjectFormat != "sha1" {
		return fmt.Errorf("contextreceipt: repository proof object_format must be %q", "sha1")
	}
	if err := validateCandidate(proof.Candidate); err != nil {
		return err
	}
	if proof.Objects == nil || len(proof.Objects) == 0 {
		return fmt.Errorf("contextreceipt: repository proof objects must be non-null and nonempty")
	}
	for i, object := range proof.Objects {
		if len(object.OID) != 40 || !isLowerHex(object.OID) {
			return fmt.Errorf("contextreceipt: repository proof objects[%d].oid must be SHA-1", i)
		}
		if object.Type != "commit" && object.Type != "tree" {
			return fmt.Errorf("contextreceipt: repository proof objects[%d].type is unknown", i)
		}
		if object.Content == nil {
			return fmt.Errorf("contextreceipt: repository proof objects[%d].content must be non-null", i)
		}
		if i > 0 && proof.Objects[i-1].OID >= object.OID {
			return fmt.Errorf("contextreceipt: repository proof objects must be sorted and deduplicated by oid")
		}
	}
	observation := proof.ExecutionObservation
	if err := requireReceiptText("repository proof execution_observation.workspace_id", observation.WorkspaceID); err != nil {
		return err
	}
	for field, value := range map[string]string{"commit": observation.Commit, "tree": observation.Tree} {
		if len(value) != 40 || !isLowerHex(value) {
			return fmt.Errorf("contextreceipt: repository proof execution_observation.%s must be SHA-1", field)
		}
	}
	if err := validateReceiptDigest("repository proof execution_observation.event_digest", observation.EventDigest); err != nil {
		return err
	}
	if requireDigest || proof.Digest != "" {
		if err := validateReceiptDigest("repository proof digest", proof.Digest); err != nil {
			return err
		}
		want, err := repositoryProofDigest(proof)
		if err != nil {
			return err
		}
		if proof.Digest != want {
			return fmt.Errorf("contextreceipt: repository proof digest does not match canonical proof")
		}
	}
	return nil
}

func validateVerdict(verdict Verdict, requireDigest bool) error {
	if verdict.Schema != VerdictSchemaID {
		return fmt.Errorf("contextreceipt: verdict schema must be %q", VerdictSchemaID)
	}
	for field, value := range map[string]string{"request_digest": verdict.RequestDigest, "receipt_digest": verdict.ReceiptDigest} {
		if err := validateReceiptDigest("verdict "+field, value); err != nil {
			return err
		}
	}
	if err := validateRole(verdict.ReceiptRole); err != nil {
		return err
	}
	if err := validateAuthority(verdict.ReceiptAuthority); err != nil {
		return err
	}
	if err := validateState(verdict.State); err != nil {
		return err
	}
	if verdict.Operands == nil || len(verdict.Operands) != len(operandKinds) {
		return fmt.Errorf("contextreceipt: verdict operands must contain exactly nineteen rows")
	}
	adverse := make(map[OperandKind]State)
	for i, operand := range verdict.Operands {
		if operand.Kind != operandKinds[i] || operand.ID != string(operandKinds[i]) {
			return fmt.Errorf("contextreceipt: verdict operands[%d] has wrong singleton identity", i)
		}
		if err := validateState(operand.State); err != nil {
			return err
		}
		for _, digest := range []string{operand.ExpectedDigest, operand.ObservedDigest} {
			if digest != "" {
				if err := validateReceiptDigest("verdict operand digest", digest); err != nil {
					return err
				}
			}
		}
		if operand.ExpectedDigest == "" || (operand.State == StateProven && operand.ObservedDigest == "") {
			return fmt.Errorf("contextreceipt: verdict operands[%d] omits an available digest", i)
		}
		if err := validateWitnesses(operand.Witnesses, operand.State != StateProven); err != nil {
			return fmt.Errorf("contextreceipt: verdict operands[%d]: %w", i, err)
		}
		if operand.State != StateProven {
			adverse[operand.Kind] = operand.State
		}
	}
	if verdict.Findings == nil || verdict.Witnesses == nil {
		return fmt.Errorf("contextreceipt: verdict findings and witnesses must be non-null")
	}
	if len(verdict.Findings) != len(adverse) {
		return fmt.Errorf("contextreceipt: verdict requires exactly one finding per non-proven operand")
	}
	lastIndex := -1
	findingCodes := make(map[OperandKind]string, len(verdict.Findings))
	for _, finding := range verdict.Findings {
		index := operandKindIndex(finding.OperandKind)
		if index <= lastIndex || finding.OperandID != string(finding.OperandKind) || adverse[finding.OperandKind] != finding.State || !validFindingForKind(finding.Code, finding.OperandKind) {
			return fmt.Errorf("contextreceipt: verdict finding is invalid or out of order")
		}
		findingCodes[finding.OperandKind] = finding.Code
		lastIndex = index
	}
	if err := validateWitnesses(verdict.Witnesses, len(adverse) != 0); err != nil {
		return err
	}
	union := make([]Witness, 0)
	for i, operand := range verdict.Operands {
		if operand.State == StateProven {
			continue
		}
		code := findingCodes[operand.Kind]
		if operand.Kind != "runner" || code == "runner-role-refused" || code == "runner-unavailable" {
			if len(operand.Witnesses) != 1 {
				return fmt.Errorf("contextreceipt: verdict operands[%d] requires exactly one fixed witness", i)
			}
			witness := operand.Witnesses[0]
			if witness.Code != code || witness.SourceID != "verdi.context-receipt-verify/"+string(operand.Kind) || witness.EvidenceDigest != operand.ObservedDigest || witness.Detail != stateDetail(operand.State) {
				return fmt.Errorf("contextreceipt: verdict operands[%d] witness does not match its finding", i)
			}
		}
		union = append(union, operand.Witnesses...)
	}
	union = sortDeduplicateWitnesses(union)
	if !reflect.DeepEqual(verdict.Witnesses, union) {
		return fmt.Errorf("contextreceipt: verdict witnesses are not the exact operand-witness union")
	}
	if verdict.ReceiptAuthority == AuthorityAdvisory && (verdict.Operands[0].State != StateUnproven || findingCodes["receipt"] != "advisory-receipt") {
		return fmt.Errorf("contextreceipt: advisory receipt must remain visibly unproven")
	}
	if verdict.State != reducedState(verdict.Operands) {
		return fmt.Errorf("contextreceipt: verdict state does not match operand reduction")
	}
	if requireDigest || verdict.Digest != "" {
		if err := validateReceiptDigest("verdict digest", verdict.Digest); err != nil {
			return err
		}
		want, err := verdictDigest(verdict)
		if err != nil {
			return err
		}
		if verdict.Digest != want {
			return fmt.Errorf("contextreceipt: verdict digest does not match canonical verdict")
		}
	}
	return nil
}

func validateState(state State) error {
	switch state {
	case StateProven, StateViolated, StateUnproven:
		return nil
	default:
		return fmt.Errorf("contextreceipt: unknown verification state %q", state)
	}
}

func validateWitnesses(witnesses []Witness, requireNonempty bool) error {
	if witnesses == nil || (requireNonempty && len(witnesses) == 0) {
		return fmt.Errorf("contextreceipt: witnesses must be non-null%s", map[bool]string{true: " and nonempty"}[requireNonempty])
	}
	if !requireNonempty && len(witnesses) != 0 {
		return fmt.Errorf("contextreceipt: proven fact carries witnesses")
	}
	for i, witness := range witnesses {
		if err := requireReceiptText("witness.code", witness.Code); err != nil {
			return err
		}
		if err := requireReceiptText("witness.source_id", witness.SourceID); err != nil {
			return err
		}
		if witness.EvidenceDigest != "" {
			if err := validateReceiptDigest("witness.evidence_digest", witness.EvidenceDigest); err != nil {
				return err
			}
		}
		if i > 0 && !principalWitnessLess(witnesses[i-1], witness) {
			return fmt.Errorf("contextreceipt: witnesses must be sorted and deduplicated")
		}
	}
	return nil
}

func reducedState(operands []Operand) State {
	state := StateProven
	for _, operand := range operands {
		if operand.State == StateViolated {
			return StateViolated
		}
		if operand.State == StateUnproven {
			state = StateUnproven
		}
	}
	return state
}

func operandKindIndex(kind OperandKind) int {
	for i, candidate := range operandKinds {
		if candidate == kind {
			return i
		}
	}
	return -1
}

func validFindingCode(code string) bool {
	switch code {
	case "advisory-receipt", "receipt-mismatch", "candidate-stale", "execution-request-mismatch",
		"repository-mismatch", "manifest-mismatch", "dispatch-mismatch", "event-mismatch",
		"event-chain-mismatch", "expansion-incomplete", "obligation-mismatch", "evidence-mismatch",
		"receipt-event-mismatch", "receipt-ack-mismatch", "profile-mismatch", "runner-untrusted",
		"runner-role-refused", "runner-unavailable", "isolation-violated", "isolation-unavailable",
		"review-packet-mismatch", "review-link-mismatch", "review-stale", "authority-unavailable":
		return true
	default:
		return false
	}
}

func validFindingForKind(code string, kind OperandKind) bool {
	if !validFindingCode(code) {
		return false
	}
	switch kind {
	case "receipt":
		return code == "receipt-mismatch" || code == "advisory-receipt"
	case "governance-profile":
		return code == "profile-mismatch" || code == "authority-unavailable"
	case "runner":
		return code == "runner-untrusted" || code == "runner-role-refused" || code == "runner-unavailable"
	case "isolation":
		return code == "isolation-violated" || code == "isolation-unavailable"
	default:
		return code == defaultFindingCode(kind)
	}
}

func cloneVerifyRequest(request VerifyRequest) VerifyRequest {
	request.Proofs.ExecutionRequestBytes = append([]byte{}, request.Proofs.ExecutionRequestBytes...)
	request.Proofs.RepositoryProofBytes = append([]byte{}, request.Proofs.RepositoryProofBytes...)
	request.Proofs.ExecutionEventBytes = cloneByteDocuments(request.Proofs.ExecutionEventBytes)
	request.Proofs.ReceiptEventBytes = append([]byte{}, request.Proofs.ReceiptEventBytes...)
	request.Proofs.ExpansionDataBytes = cloneByteDocuments(request.Proofs.ExpansionDataBytes)
	request.Proofs.ObligationBytes = cloneByteDocuments(request.Proofs.ObligationBytes)
	request.Proofs.EvidenceResultBytes = cloneByteDocuments(request.Proofs.EvidenceResultBytes)
	request.Proofs.ReviewPacketBytes = append([]byte{}, request.Proofs.ReviewPacketBytes...)
	return request
}

func cloneByteDocuments(documents [][]byte) [][]byte {
	cloned := make([][]byte, len(documents))
	for i := range documents {
		cloned[i] = append([]byte(nil), documents[i]...)
	}
	return cloned
}

func cloneRepositoryObjects(objects []RepositoryObject) []RepositoryObject {
	cloned := append([]RepositoryObject{}, objects...)
	for i := range cloned {
		cloned[i].Content = append([]byte{}, cloned[i].Content...)
	}
	return cloned
}

func cloneVerdict(verdict Verdict) Verdict {
	verdict.Operands = append([]Operand{}, verdict.Operands...)
	for i := range verdict.Operands {
		verdict.Operands[i].Witnesses = append([]Witness{}, verdict.Operands[i].Witnesses...)
	}
	verdict.Findings = append([]Finding{}, verdict.Findings...)
	verdict.Witnesses = append([]Witness{}, verdict.Witnesses...)
	return verdict
}

func receiptDigest(receipt Receipt) (string, error) {
	receipt.Digest = ""
	return canonjson.Digest(receipt)
}

func validateReceipt(receipt Receipt, requireDigest bool) error {
	if receipt.Schema != SchemaID {
		return fmt.Errorf("contextreceipt: schema must be %q", SchemaID)
	}
	if err := validateRole(receipt.Role); err != nil {
		return err
	}
	if err := validateAuthority(receipt.Authority); err != nil {
		return err
	}
	for field, value := range map[string]string{
		"atc_runway": receipt.ATCRunway, "execution_workspace_id": receipt.ExecutionWorkspaceID,
		"adapter_version": receipt.AdapterVersion,
	} {
		if err := requireReceiptText(field, value); err != nil {
			return err
		}
	}
	for field, value := range map[string]string{
		"manifest_digest": receipt.ManifestDigest, "dispatch_digest": receipt.DispatchDigest,
		"execution_workspace_request_digest": receipt.ExecutionWorkspaceRequestDigest,
		"event_chain_root":                   receipt.EventChainRoot,
	} {
		if err := validateReceiptDigest(field, value); err != nil {
			return err
		}
	}
	for field, value := range map[string]string{
		"input_commit": receipt.InputCommit, "input_tree": receipt.InputTree,
		"output_commit": receipt.OutputCommit, "output_tree": receipt.OutputTree,
	} {
		if err := validateReceiptSHA(field, value); err != nil {
			return err
		}
	}
	if err := receipt.Adapter.Validate(); err != nil {
		return fmt.Errorf("contextreceipt: %w", err)
	}
	if receipt.RevisionSegments == nil {
		return fmt.Errorf("contextreceipt: revision_segments must be non-null")
	}
	root, err := contextevent.EventChainRoot(receipt.RevisionSegments)
	if err != nil {
		return fmt.Errorf("contextreceipt: revision_segments: %w", err)
	}
	if receipt.EventChainRoot != root {
		return fmt.Errorf("contextreceipt: event_chain_root does not match revision_segments")
	}
	terminal := receipt.RevisionSegments[len(receipt.RevisionSegments)-1]
	if receipt.TerminalManifestRevision != terminal.ManifestRevision || receipt.TerminalSourceSequence != terminal.TerminalSourceSequence || receipt.TerminalGlobalSequence != terminal.TerminalGlobalSequence {
		return fmt.Errorf("contextreceipt: terminal identity does not match final revision segment")
	}
	if receipt.ManifestDigest != terminal.ManifestDigest {
		return fmt.Errorf("contextreceipt: manifest_digest does not match terminal revision")
	}
	if err := validateExpansions(receipt.Expansions, receipt.RevisionSegments); err != nil {
		return err
	}
	if err := validateObligations(receipt.Obligations); err != nil {
		return err
	}
	if err := validateEvidence(receipt.Evidence); err != nil {
		return err
	}
	if err := validatePrincipalResolution(receipt.RunnerPrincipalResolution); err != nil {
		return err
	}
	if receipt.Authority == AuthorityAuthoritative && receipt.RunnerPrincipalResolution.State != gp.ResolutionAuthenticated {
		return fmt.Errorf("contextreceipt: authoritative receipt requires authenticated runner resolution")
	}
	if err := validateReviewInputs(receipt.ReviewInputs); err != nil {
		return err
	}
	switch receipt.Role {
	case RoleBuilder:
		if receipt.ReviewOf != nil {
			return fmt.Errorf("contextreceipt: builder receipt forbids review_of")
		}
	case RoleReviewer:
		if len(receipt.ReviewOf) != 1 {
			return fmt.Errorf("contextreceipt: reviewer receipt requires exactly one review_of digest")
		}
		if err := validateReceiptDigest("review_of[0]", receipt.ReviewOf[0]); err != nil {
			return err
		}
	}
	if requireDigest || receipt.Digest != "" {
		if err := validateReceiptDigest("digest", receipt.Digest); err != nil {
			return err
		}
		want, err := receiptDigest(receipt)
		if err != nil {
			return err
		}
		if receipt.Digest != want {
			return fmt.Errorf("contextreceipt: digest does not match canonical receipt")
		}
	}
	return nil
}

func validateExpansions(expansions []Expansion, revisions []contextevent.Revision) error {
	if expansions == nil {
		return fmt.Errorf("contextreceipt: expansions must be non-null")
	}
	if len(expansions) != len(revisions)-1 {
		return fmt.Errorf("contextreceipt: expansions must cover every revision transition")
	}
	byParent := make(map[uint64]Expansion, len(expansions))
	for i, expansion := range expansions {
		prefix := fmt.Sprintf("expansions[%d]", i)
		if err := requireReceiptText(prefix+".request_id", expansion.RequestID); err != nil {
			return err
		}
		if expansion.ChildRevision != expansion.ParentRevision+1 {
			return fmt.Errorf("contextreceipt: %s child_revision must immediately follow parent_revision", prefix)
		}
		for field, value := range map[string]string{
			"parent_manifest_digest": expansion.ParentManifestDigest,
			"child_manifest_digest":  expansion.ChildManifestDigest,
			"expansion_digest":       expansion.ExpansionDigest,
		} {
			if err := validateReceiptDigest(prefix+"."+field, value); err != nil {
				return err
			}
		}
		if i > 0 && !expansionLess(expansions[i-1], expansion) {
			return fmt.Errorf("contextreceipt: expansions must be sorted and deduplicated")
		}
		byParent[expansion.ParentRevision] = expansion
	}
	for i := 0; i < len(revisions)-1; i++ {
		parent, child := revisions[i], revisions[i+1]
		expansion, ok := byParent[parent.ManifestRevision]
		if !ok || expansion.ParentManifestDigest != parent.ManifestDigest || expansion.ChildRevision != child.ManifestRevision || expansion.ChildManifestDigest != child.ManifestDigest {
			return fmt.Errorf("contextreceipt: expansion rows contradict revision transition %d", i)
		}
	}
	return nil
}

func expansionLess(a, b Expansion) bool {
	if a.RequestID != b.RequestID {
		return a.RequestID < b.RequestID
	}
	if a.ParentRevision != b.ParentRevision {
		return a.ParentRevision < b.ParentRevision
	}
	if a.ParentManifestDigest != b.ParentManifestDigest {
		return a.ParentManifestDigest < b.ParentManifestDigest
	}
	if a.ChildRevision != b.ChildRevision {
		return a.ChildRevision < b.ChildRevision
	}
	if a.ChildManifestDigest != b.ChildManifestDigest {
		return a.ChildManifestDigest < b.ChildManifestDigest
	}
	return a.ExpansionDigest < b.ExpansionDigest
}

func validateObligations(obligations []Obligation) error {
	if obligations == nil {
		return fmt.Errorf("contextreceipt: obligations must be non-null")
	}
	for i, obligation := range obligations {
		prefix := fmt.Sprintf("obligations[%d]", i)
		for field, value := range map[string]string{"ref": obligation.Ref, "path": obligation.Path, "ac": obligation.AC, "producer": obligation.Producer} {
			if err := requireReceiptText(prefix+"."+field, value); err != nil {
				return err
			}
		}
		switch obligation.Kind {
		case artifact.EvidenceStatic, artifact.EvidenceBehavioral, artifact.EvidenceRuntime, artifact.EvidenceAttestation:
		default:
			return fmt.Errorf("contextreceipt: %s has unknown evidence kind %q", prefix, obligation.Kind)
		}
		if err := validateReceiptDigest(prefix+".content_digest", obligation.ContentDigest); err != nil {
			return err
		}
		if i > 0 && !obligationLess(obligations[i-1], obligation) {
			return fmt.Errorf("contextreceipt: obligations must be sorted and deduplicated")
		}
	}
	return nil
}

func obligationLess(a, b Obligation) bool {
	return compareStrings(
		[]string{a.Ref, a.Path, a.AC, string(a.Kind), a.ContentDigest, a.Producer},
		[]string{b.Ref, b.Path, b.AC, string(b.Kind), b.ContentDigest, b.Producer},
	) < 0
}

func validateEvidence(rows []Evidence) error {
	if rows == nil {
		return fmt.Errorf("contextreceipt: evidence must be non-null")
	}
	for i, row := range rows {
		prefix := fmt.Sprintf("evidence[%d]", i)
		if err := requireReceiptText(prefix+".command_id", row.CommandID); err != nil {
			return err
		}
		if len(row.Argv) == 0 {
			return fmt.Errorf("contextreceipt: %s.argv must be non-null and nonempty", prefix)
		}
		for j, arg := range row.Argv {
			if err := requireReceiptText(fmt.Sprintf("%s.argv[%d]", prefix, j), arg); err != nil {
				return err
			}
		}
		if err := validateReceiptVerdict(row.Verdict); err != nil {
			return err
		}
		if err := validateReceiptDigest(prefix+".output_digest", row.OutputDigest); err != nil {
			return err
		}
		if i > 0 && !evidenceLess(rows[i-1], row) {
			return fmt.Errorf("contextreceipt: evidence must be sorted and deduplicated")
		}
	}
	return nil
}

func evidenceLess(a, b Evidence) bool {
	if a.CommandID != b.CommandID {
		return a.CommandID < b.CommandID
	}
	if compared := compareStringSlices(a.Argv, b.Argv); compared != 0 {
		return compared < 0
	}
	if a.ExitCode != b.ExitCode {
		return a.ExitCode < b.ExitCode
	}
	if a.Verdict != b.Verdict {
		return a.Verdict < b.Verdict
	}
	return a.OutputDigest < b.OutputDigest
}

func validateReviewInputs(inputs []ReviewInput) error {
	if inputs == nil {
		return fmt.Errorf("contextreceipt: review_inputs must be non-null")
	}
	for i, input := range inputs {
		prefix := fmt.Sprintf("review_inputs[%d]", i)
		if err := requireReceiptText(prefix+".kind", input.Kind); err != nil {
			return err
		}
		if err := validateReceiptDigest(prefix+".content_digest", input.ContentDigest); err != nil {
			return err
		}
		if i > 0 && !reviewInputLess(inputs[i-1], input) {
			return fmt.Errorf("contextreceipt: review_inputs must be sorted and deduplicated")
		}
	}
	return nil
}

func reviewInputLess(a, b ReviewInput) bool {
	return compareStrings([]string{a.Kind, a.ContentDigest}, []string{b.Kind, b.ContentDigest}) < 0
}

func validatePrincipalResolution(resolution gp.PrincipalResolution) error {
	if err := resolution.State.Validate(); err != nil {
		return fmt.Errorf("contextreceipt: runner principal resolution: %w", err)
	}
	if err := resolution.Claim.Validate(); err != nil {
		return fmt.Errorf("contextreceipt: runner principal resolution: %w", err)
	}
	derived, err := gp.CanonicalPrincipalID(resolution.Claim.TrustSource, resolution.Claim.Subject)
	if err != nil {
		return err
	}
	if resolution.State == gp.ResolutionAuthenticated {
		if resolution.PrincipalID != derived {
			return fmt.Errorf("contextreceipt: authenticated principal id does not match claim")
		}
	} else if resolution.PrincipalID != "" {
		return fmt.Errorf("contextreceipt: non-authenticated resolution carries principal id")
	}
	if len(resolution.Witnesses) == 0 {
		return fmt.Errorf("contextreceipt: runner principal witnesses must be non-null and nonempty")
	}
	for i, witness := range resolution.Witnesses {
		if err := requireReceiptText(fmt.Sprintf("runner_principal_resolution.witnesses[%d].code", i), witness.Code); err != nil {
			return err
		}
		if err := requireReceiptText(fmt.Sprintf("runner_principal_resolution.witnesses[%d].source_id", i), witness.SourceID); err != nil {
			return err
		}
		if witness.EvidenceDigest != "" {
			if err := validateReceiptDigest("runner principal witness evidence_digest", witness.EvidenceDigest); err != nil {
				return err
			}
		}
		if i > 0 && !principalWitnessLess(resolution.Witnesses[i-1], witness) {
			return fmt.Errorf("contextreceipt: runner principal witnesses must be strictly ordered")
		}
	}
	return nil
}

func principalWitnessLess(a, b gp.Witness) bool {
	return compareStrings([]string{a.Code, a.SourceID, a.EvidenceDigest, a.Detail}, []string{b.Code, b.SourceID, b.EvidenceDigest, b.Detail}) < 0
}

func compareStrings(a, b []string) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func compareStringSlices(a, b []string) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

func validateRole(role Role) error {
	switch role {
	case RoleBuilder, RoleReviewer:
		return nil
	default:
		return fmt.Errorf("contextreceipt: unknown role %q", role)
	}
}

func validateAuthority(authority Authority) error {
	switch authority {
	case AuthorityAuthoritative, AuthorityAdvisory:
		return nil
	default:
		return fmt.Errorf("contextreceipt: unknown authority %q", authority)
	}
}

func validateReceiptVerdict(verdict countersign.Verdict) error {
	switch verdict {
	case countersign.VerdictProven, countersign.VerdictViolated, countersign.VerdictUnproven:
		return nil
	default:
		return fmt.Errorf("contextreceipt: unknown evidence verdict %q", verdict)
	}
}

func requireReceiptText(field, value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return fmt.Errorf("contextreceipt: %s must be nonempty without surrounding whitespace", field)
	}
	return nil
}

func validateReceiptDigest(field, value string) error {
	if !receiptDigestRE.MatchString(value) {
		return fmt.Errorf("contextreceipt: %s must be a canonical sha256 digest", field)
	}
	return nil
}

func validateReceiptSHA(field, value string) error {
	if len(value) != 40 && len(value) != 64 {
		return fmt.Errorf("contextreceipt: %s must be a full 40- or 64-character SHA", field)
	}
	if value != strings.ToLower(value) {
		return fmt.Errorf("contextreceipt: %s must be lowercase hexadecimal", field)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("contextreceipt: %s must be hexadecimal: %w", field, err)
	}
	return nil
}
