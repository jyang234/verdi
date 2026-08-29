package contextreceipt

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

const receiptVerificationTransition = "context-receipt-verify"
const receiptVerificationRole = "managed-runner"

// AuthorityResolver is the sole controller-owned verification authority port.
type AuthorityResolver interface {
	ResolveReceiptVerificationAuthority(context.Context, AuthorityQuery) (AuthorityFacts, error)
}

// Verifier recomputes declared content proofs and consults optional inherited
// authority. It never reads ambient repository, identity, or provider state.
type Verifier struct {
	authority AuthorityResolver
	execution ExecutionProofDecoder
	expansion ExpansionProofVerifier
	review    ReviewProofVerifier
}

// NewVerifier constructs a verifier without an execution-wire adapter. It is
// useful for malformed-proof checks; complete verification requires the
// component adapter supplied by NewVerifierWithExecutionProof.
func NewVerifier(authority AuthorityResolver) *Verifier { return &Verifier{authority: authority} }

// NewVerifierWithExecutionProof constructs a complete verifier using the
// component-owned execution request decoder.
func NewVerifierWithExecutionProof(authority AuthorityResolver, execution ExecutionProofDecoder) *Verifier {
	verifier := &Verifier{authority: authority, execution: execution}
	if expansion, ok := execution.(ExpansionProofVerifier); ok {
		verifier.expansion = expansion
	}
	if review, ok := execution.(ReviewProofVerifier); ok {
		verifier.review = review
	}
	return verifier
}

// NewVerifierWithProofs assembles the cycle-free component proof ports.
func NewVerifierWithProofs(authority AuthorityResolver, execution ExecutionProofDecoder, expansion ExpansionProofVerifier, review ReviewProofVerifier) *Verifier {
	return &Verifier{authority: authority, execution: execution, expansion: expansion, review: review}
}

// Verify returns one deterministic nineteen-operand verdict for a valid
// request. Malformed proofs or broken ports return an operational error.
func (v *Verifier) Verify(ctx context.Context, request VerifyRequest) (Verdict, error) {
	if ctx == nil {
		return Verdict{}, fmt.Errorf("contextreceipt: verify: nil context")
	}
	encoded, err := EncodeVerifyRequest(request)
	if err != nil {
		return Verdict{}, fmt.Errorf("contextreceipt: verify request: %w", err)
	}
	request, err = DecodeVerifyRequest(bytes.NewReader(encoded))
	if err != nil {
		return Verdict{}, err
	}

	repository, err := DecodeRepositoryProof(bytes.NewReader(request.Proofs.RepositoryProofBytes))
	if err != nil {
		return Verdict{}, fmt.Errorf("contextreceipt: repository proof: %w", err)
	}
	if v.execution == nil {
		return Verdict{}, fmt.Errorf("contextreceipt: execution proof decoder is unavailable")
	}
	execution, err := v.execution.DecodeExecutionProof(append([]byte{}, request.Proofs.ExecutionRequestBytes...))
	if err != nil {
		return Verdict{}, fmt.Errorf("contextreceipt: execution request proof: %w", err)
	}

	events, err := decodeExecutionEvents(request.Proofs.ExecutionEventBytes)
	if err != nil {
		return Verdict{}, err
	}
	receiptEvent, err := contextevent.DecodeEvent(bytes.NewReader(request.Proofs.ReceiptEventBytes))
	if err != nil {
		return Verdict{}, fmt.Errorf("contextreceipt: receipt event proof: %w", err)
	}

	requestDigest := request.Digest
	receiptDigest := request.Receipt.Digest
	operands := make([]Operand, 0, len(operandKinds))
	appendOperand := func(kind OperandKind, state State, expected, observed, code string, witnesses []Witness) {
		if state == StateProven {
			witnesses = []Witness{}
		} else if kind != "runner" || len(witnesses) == 0 {
			witnesses = []Witness{{Code: code, SourceID: "verdi.context-receipt-verify/" + string(kind), EvidenceDigest: observed, Detail: stateDetail(state)}}
		}
		operands = append(operands, Operand{Kind: kind, ID: string(kind), State: state, ExpectedDigest: expected, ObservedDigest: observed, Witnesses: witnesses})
	}

	appendOperand("receipt", receiptState(request.Receipt), receiptDigest, receiptDigest, receiptFindingCode(request.Receipt), nil)
	expectedCandidate := Candidate{
		BaseCommit: request.Receipt.InputCommit,
		BaseTree:   request.Receipt.InputTree,
		HeadCommit: request.Receipt.OutputCommit,
		HeadTree:   request.Receipt.OutputTree,
	}
	candidateExpectedDigest := mustProjectionDigest(expectedCandidate)
	candidateObservedDigest := mustProjectionDigest(request.Candidate)
	appendOperand("candidate", stateForEqual(candidateExpectedDigest, candidateObservedDigest), candidateExpectedDigest, candidateObservedDigest, "candidate-stale", nil)

	expectedExecution := executionProjectionFromReceipt(request.Receipt)
	executionExpectedDigest := mustProjectionDigest(expectedExecution)
	executionObservedDigest := mustProjectionDigest(executionReceiptProjection(execution))
	executionState := stateForEqual(executionExpectedDigest, executionObservedDigest)
	appendOperand("execution-request", executionState, executionExpectedDigest, executionObservedDigest, "execution-request-mismatch", nil)

	repositoryExpected := repositoryProjectionFromReceipt(request.Receipt)
	repositoryObserved := repositoryProjectionFromProof(repository)
	repositoryExpectedDigest := mustProjectionDigest(repositoryExpected)
	repositoryObservedDigest := mustProjectionDigest(repositoryObserved)
	repositoryState := StateProven
	if err := verifyRepositoryProof(repository, request.Candidate); err != nil || repositoryExpectedDigest != repositoryObservedDigest || verifyRepositoryObservation(repository.ExecutionObservation, events, request.Receipt) != nil {
		repositoryState = StateViolated
	}
	appendOperand("repository", repositoryState, repositoryExpectedDigest, repositoryObservedDigest, "repository-mismatch", nil)

	manifestExpected := struct {
		TerminalRevision uint64 `json:"terminal_revision"`
		ManifestDigest   string `json:"manifest_digest"`
	}{request.Receipt.TerminalManifestRevision, request.Receipt.ManifestDigest}
	manifestObserved := struct {
		TerminalRevision uint64 `json:"terminal_revision"`
		ManifestDigest   string `json:"manifest_digest"`
	}{execution.ManifestRevision, execution.ManifestDigest}
	manifestExpectedDigest := mustProjectionDigest(manifestExpected)
	manifestObservedDigest := mustProjectionDigest(manifestObserved)
	appendOperand("manifest", stateForEqual(manifestExpectedDigest, manifestObservedDigest), manifestExpectedDigest, manifestObservedDigest, "manifest-mismatch", nil)

	dispatchObserved := digestRaw(request.Proofs.ExecutionRequestBytes)
	appendOperand("dispatch", stateForEqual(request.Receipt.DispatchDigest, dispatchObserved), request.Receipt.DispatchDigest, dispatchObserved, "dispatch-mismatch", nil)

	eventState, eventChainState := verifyExecutionEventContinuity(events, request.Receipt, execution)
	eventDigestProjection := terminalEventDigestsFromEvents(events)
	eventsObservedDigest := mustProjectionDigest(eventDigestProjection)
	eventsExpectedDigest := mustProjectionDigest(terminalEventDigests(request.Receipt.RevisionSegments))
	if eventsExpectedDigest != eventsObservedDigest {
		eventState = StateViolated
	}
	appendOperand("events", eventState, eventsExpectedDigest, eventsObservedDigest, "event-mismatch", nil)
	eventChainObserved := mustProjectionDigest(revisionsFromEvents(events, request.Receipt.RevisionSegments))
	if eventChainObserved != request.Receipt.EventChainRoot {
		eventChainState = StateViolated
	}
	appendOperand("event-chain", eventChainState, request.Receipt.EventChainRoot, eventChainObserved, "event-chain-mismatch", nil)

	expansionExpected := mustProjectionDigest(request.Receipt.Expansions)
	expansionState, err := v.verifyExpansionDocuments(request.Proofs.ExpansionDataBytes, request.Receipt.Expansions)
	if err != nil {
		return Verdict{}, err
	}
	expansionObserved := observedProjectionDigest(expansionState, request.Receipt.Expansions, request.Proofs.ExpansionDataBytes)
	appendOperand("expansions", expansionState, expansionExpected, expansionObserved, "expansion-incomplete", nil)

	obligationExpected := mustProjectionDigest(request.Receipt.Obligations)
	obligationState, err := verifyObligationDocuments(request.Proofs.ObligationBytes, request.Receipt.Obligations)
	if err != nil {
		return Verdict{}, err
	}
	obligationObserved := observedProjectionDigest(obligationState, request.Receipt.Obligations, request.Proofs.ObligationBytes)
	appendOperand("obligations", obligationState, obligationExpected, obligationObserved, "obligation-mismatch", nil)

	evidenceExpected := mustProjectionDigest(request.Receipt.Evidence)
	evidenceState, err := verifyEvidenceDocuments(request.Proofs.EvidenceResultBytes, request.Receipt.Evidence)
	if err != nil {
		return Verdict{}, err
	}
	evidenceObserved := observedProjectionDigest(evidenceState, request.Receipt.Evidence, request.Proofs.EvidenceResultBytes)
	appendOperand("evidence", evidenceState, evidenceExpected, evidenceObserved, "evidence-mismatch", nil)

	receiptEventExpected, receiptEventObserved, receiptEventState, receiptAckExpected, receiptAckObserved, receiptAckState := receiptCompletionOperands(request.Receipt, receiptEvent, request.ReceiptEventAck)
	appendOperand("receipt-event", receiptEventState, receiptEventExpected, receiptEventObserved, "receipt-event-mismatch", nil)
	appendOperand("receipt-ack", receiptAckState, receiptAckExpected, receiptAckObserved, "receipt-ack-mismatch", nil)

	authority := localUnavailableAuthority(request.Receipt.RunnerPrincipalResolution.Claim.TrustSource)
	if v.authority != nil {
		query := AuthorityQuery{RequestDigest: requestDigest, ReceiptDigest: receiptDigest, CandidateCommit: request.Candidate.HeadCommit, CandidateTree: request.Candidate.HeadTree, ProfileRef: execution.ProfileRef, RunnerClaim: request.Receipt.RunnerPrincipalResolution.Claim}
		authority, err = v.authority.ResolveReceiptVerificationAuthority(ctx, query)
		if err != nil {
			return Verdict{}, fmt.Errorf("contextreceipt: resolve receipt verification authority: %w", err)
		}
	}
	profileState, profileExpected, profileObserved, runnerState, runnerExpected, runnerObserved, runnerFindingCode, runnerWitnesses, authErr := verifyGovernanceAuthority(ctx, authority, execution, request.Receipt)
	if authErr != nil {
		return Verdict{}, authErr
	}
	appendOperand("governance-profile", profileState, profileExpected, profileObserved, profileFinding(profileState), authority.Profile.Witnesses)
	appendOperand("runner", runnerState, runnerExpected, runnerObserved, runnerFindingCode, runnerWitnesses)
	isolationExpected := mustProjectionDigest(struct {
		ProfileID     string `json:"profile_id"`
		ProfileDigest string `json:"profile_digest"`
		Session       string `json:"session"`
		WorkspaceID   string `json:"workspace_id"`
	}{execution.ProfileRef.ID, execution.ProfileRef.Digest, execution.Session, request.Receipt.ExecutionWorkspaceID})
	isolationObserved := ""
	if authority.Isolation.State == StateProven {
		isolationObserved = mustProjectionDigest(struct {
			ProfileID     string `json:"profile_id"`
			ProfileDigest string `json:"profile_digest"`
			Session       string `json:"session"`
			WorkspaceID   string `json:"workspace_id"`
		}{authority.Isolation.ProfileID, authority.Isolation.ProfileDigest, authority.Isolation.Session, authority.Isolation.WorkspaceID})
	}
	isolationState := authority.Isolation.State
	if isolationState == StateProven && isolationExpected != isolationObserved {
		isolationState = StateViolated
	}
	appendOperand("isolation", isolationState, isolationExpected, isolationObserved, isolationFinding(isolationState), authority.Isolation.Witnesses)

	reviewProjection, err := v.reviewProjection(request)
	if err != nil {
		return Verdict{}, err
	}
	appendOperand("review-packet", reviewProjection.Packet.State, reviewProjection.Packet.ExpectedDigest, reviewProjection.Packet.ObservedDigest, "review-packet-mismatch", nil)
	appendOperand("review-link", reviewProjection.Link.State, reviewProjection.Link.ExpectedDigest, reviewProjection.Link.ObservedDigest, "review-link-mismatch", nil)
	appendOperand("freshness", reviewProjection.Freshness.State, reviewProjection.Freshness.ExpectedDigest, reviewProjection.Freshness.ObservedDigest, "review-stale", nil)

	applyPersistenceAuthority(operands, authority.Persistence, request.Receipt, receiptEvent, request.ReceiptEventAck)
	findings, witnesses := findingsAndWitnesses(operands, request.Receipt)
	verdict := Verdict{Schema: VerdictSchemaID, RequestDigest: requestDigest, ReceiptDigest: receiptDigest, ReceiptRole: request.Receipt.Role, ReceiptAuthority: request.Receipt.Authority, State: reducedState(operands), Operands: operands, Findings: findings, Witnesses: witnesses}
	encodedVerdict, err := EncodeVerdict(verdict)
	if err != nil {
		return Verdict{}, fmt.Errorf("contextreceipt: encode verdict: %w", err)
	}
	return DecodeVerdict(bytes.NewReader(encodedVerdict))
}

func (v *Verifier) reviewProjection(request VerifyRequest) (ReviewProofProjection, error) {
	if request.Receipt.Role == RoleBuilder {
		packetDigest := mustProjectionDigest([]ReviewInput{})
		linkDigest := mustProjectionDigest([]string{})
		freshnessDigest := mustProjectionDigest(struct{}{})
		return ReviewProofProjection{
			Packet:    ReviewOperandProjection{State: StateProven, ExpectedDigest: packetDigest, ObservedDigest: packetDigest},
			Link:      ReviewOperandProjection{State: StateProven, ExpectedDigest: linkDigest, ObservedDigest: linkDigest},
			Freshness: ReviewOperandProjection{State: StateProven, ExpectedDigest: freshnessDigest, ObservedDigest: freshnessDigest},
		}, nil
	}
	if v.review == nil {
		return ReviewProofProjection{
			Packet: ReviewOperandProjection{
				State:          StateUnproven,
				ExpectedDigest: mustProjectionDigest(request.Receipt.ReviewInputs),
				ObservedDigest: digestRaw(request.Proofs.ReviewPacketBytes),
			},
			Link: ReviewOperandProjection{
				State:          StateUnproven,
				ExpectedDigest: mustProjectionDigest(request.Receipt.ReviewOf),
			},
			Freshness: ReviewOperandProjection{
				State: StateUnproven,
				ExpectedDigest: mustProjectionDigest(struct {
					Candidate Candidate `json:"candidate"`
					ReviewOf  []string  `json:"review_of"`
				}{Candidate: request.Candidate, ReviewOf: request.Receipt.ReviewOf}),
			},
		}, nil
	}
	projection, err := v.review.VerifyReviewProof(append([]byte{}, request.Proofs.ReviewPacketBytes...), request.Receipt, request.Candidate)
	if err != nil {
		return ReviewProofProjection{}, fmt.Errorf("contextreceipt: review packet proof: %w", err)
	}
	for _, operand := range []struct {
		name       string
		projection ReviewOperandProjection
	}{
		{name: "packet", projection: projection.Packet},
		{name: "link", projection: projection.Link},
		{name: "freshness", projection: projection.Freshness},
	} {
		if err := validateReviewOperandProjection(operand.name, operand.projection); err != nil {
			return ReviewProofProjection{}, err
		}
	}
	return projection, nil
}

func validateReviewOperandProjection(name string, projection ReviewOperandProjection) error {
	if err := validateState(projection.State); err != nil {
		return fmt.Errorf("contextreceipt: review %s projection: %w", name, err)
	}
	if err := validateReceiptDigest("review "+name+" expected digest", projection.ExpectedDigest); err != nil {
		return err
	}
	if projection.ObservedDigest != "" {
		if err := validateReceiptDigest("review "+name+" observed digest", projection.ObservedDigest); err != nil {
			return err
		}
	}
	if projection.State == StateProven && (projection.ObservedDigest == "" || projection.ExpectedDigest != projection.ObservedDigest) {
		return fmt.Errorf("contextreceipt: proven review %s projection does not match", name)
	}
	if projection.State == StateViolated && projection.ExpectedDigest == projection.ObservedDigest {
		return fmt.Errorf("contextreceipt: violated review %s projection does not contradict", name)
	}
	return nil
}

func validateReceiptCompletion(receipt Receipt, eventBytes []byte, ack contextevent.ReceiptEventAck) error {
	if len(eventBytes) == 0 {
		return fmt.Errorf("contextreceipt: receipt event is unavailable")
	}
	event, err := contextevent.DecodeEvent(bytes.NewReader(eventBytes))
	if err != nil {
		return err
	}
	_, _, eventState, _, _, ackState := receiptCompletionOperands(receipt, event, ack)
	if eventState != StateProven || ackState != StateProven {
		return fmt.Errorf("contextreceipt: receipt event or acknowledgment does not bind exact receipt completion")
	}
	return nil
}

func receiptCompletionOperands(receipt Receipt, event contextevent.Event, ack contextevent.ReceiptEventAck) (string, string, State, string, string, State) {
	receiptBytes, err := EncodeReceipt(receipt)
	if err != nil {
		return "", "", StateViolated, "", "", StateViolated
	}
	represented := bytes.TrimSuffix(receiptBytes, []byte("\n"))
	representedDigest := digestRawNoFrame(represented)
	expectedEvent := struct {
		EventDigest            string `json:"event_digest"`
		ReceiptDigest          string `json:"receipt_digest"`
		RepresentedBytesDigest string `json:"represented_bytes_digest"`
	}{event.EventDigest, receipt.Digest, representedDigest}
	expectedEventDigest := mustProjectionDigest(expectedEvent)
	observedEventDigest := ""
	eventState := StateViolated
	if payload, ok := event.Payload.(*contextevent.ReceiptPayload); ok {
		observedEvent := struct {
			EventDigest            string `json:"event_digest"`
			ReceiptDigest          string `json:"receipt_digest"`
			RepresentedBytesDigest string `json:"represented_bytes_digest"`
		}{event.EventDigest, payload.ReceiptDigest, payload.Detail.Digest}
		observedEventDigest = mustProjectionDigest(observedEvent)
		detailMatches := payload.Detail.Digest == representedDigest
		switch payload.Detail.Mode {
		case contextevent.DetailInline:
			detailMatches = detailMatches && bytes.Equal(payload.Detail.RedactedJSON, represented)
		case contextevent.DetailSegment:
			detailMatches = detailMatches && payload.Detail.ByteCount == uint64(len(represented))
		}
		if detailMatches && payload.ReceiptDigest == receipt.Digest && payload.Role == receipt.Role && payload.Authority == receipt.Authority && payload.ExecutionEventChainRoot == receipt.EventChainRoot &&
			event.Kind == contextevent.KindReceipt && event.ManifestRevision == receipt.TerminalManifestRevision && event.ManifestDigest == receipt.ManifestDigest && event.SourceSequence == receipt.TerminalSourceSequence+1 && event.PriorEventDigest == receipt.RevisionSegments[len(receipt.RevisionSegments)-1].EventRoot &&
			event.ATCRunway == receipt.ATCRunway && event.ExecutionWorkspaceID == receipt.ExecutionWorkspaceID && event.CandidateCommit == receipt.OutputCommit && event.CandidateTree == receipt.OutputTree && event.Adapter == receipt.Adapter && event.AdapterVersion == receipt.AdapterVersion {
			eventState = StateProven
		}
	}
	expectedAck := struct {
		EventDigest       string `json:"event_digest"`
		ReceiptDigest     string `json:"receipt_digest"`
		ResultEventDigest string `json:"result_event_digest"`
		GlobalSequence    uint64 `json:"global_sequence"`
	}{event.EventDigest, receipt.Digest, receipt.RevisionSegments[len(receipt.RevisionSegments)-1].EventRoot, receipt.TerminalGlobalSequence + 1}
	observedAck := struct {
		EventDigest       string `json:"event_digest"`
		ReceiptDigest     string `json:"receipt_digest"`
		ResultEventDigest string `json:"result_event_digest"`
		GlobalSequence    uint64 `json:"global_sequence"`
	}{ack.EventDigest, ack.ReceiptDigest, receipt.RevisionSegments[len(receipt.RevisionSegments)-1].EventRoot, ack.GlobalSequence}
	expectedAckDigest, observedAckDigest := mustProjectionDigest(expectedAck), mustProjectionDigest(observedAck)
	ackState := StateViolated
	if _, err := contextevent.EncodeReceiptEventAck(ack); err == nil && ack.EventDigest == event.EventDigest && ack.ReceiptDigest == receipt.Digest && ack.ManifestRevision == event.ManifestRevision && ack.SourceSequence == event.SourceSequence && ack.GlobalSequence == receipt.TerminalGlobalSequence+1 && ack.Flight == event.Flight && ack.Lane == event.Lane && ack.Epoch == event.Epoch && ack.Session == event.Session && ack.Kind == event.Kind {
		ackState = StateProven
	}
	return expectedEventDigest, observedEventDigest, eventState, expectedAckDigest, observedAckDigest, ackState
}

func verifyRepositoryProof(proof RepositoryProof, candidate Candidate) error {
	if proof.Candidate != candidate {
		return fmt.Errorf("candidate mismatch")
	}
	objects := make(map[string]RepositoryObject, len(proof.Objects))
	for _, object := range proof.Objects {
		preimage := append([]byte(object.Type+" "+strconv.Itoa(len(object.Content))+"\x00"), object.Content...)
		sum := sha1.Sum(preimage)
		if hex.EncodeToString(sum[:]) != object.OID {
			return fmt.Errorf("object %s oid mismatch", object.OID)
		}
		objects[object.OID] = object
	}
	reachable := map[string]bool{}
	for _, pair := range []struct{ commit, tree string }{{candidate.BaseCommit, candidate.BaseTree}, {candidate.HeadCommit, candidate.HeadTree}} {
		object, ok := objects[pair.commit]
		if !ok || object.Type != "commit" {
			return fmt.Errorf("missing commit %s", pair.commit)
		}
		reachable[pair.commit] = true
		root, err := commitRootTree(object.Content)
		if err != nil || root != pair.tree {
			return fmt.Errorf("commit %s tree mismatch", pair.commit)
		}
		if err := visitTree(root, objects, reachable); err != nil {
			return err
		}
	}
	if len(reachable) != len(objects) {
		return fmt.Errorf("repository proof contains unreachable objects")
	}
	return nil
}

func verifyRepositoryObservation(observation ExecutionObservation, events []contextevent.Event, receipt Receipt) error {
	if len(events) == 0 {
		return fmt.Errorf("missing execution result event")
	}
	resultEvent := events[len(events)-1]
	result, ok := resultEvent.Payload.(*contextevent.ExecutionResultPayload)
	if !ok || resultEvent.Kind != contextevent.KindExecutionResult {
		return fmt.Errorf("terminal event is not execution-result")
	}
	if observation.WorkspaceID != receipt.ExecutionWorkspaceID || observation.Commit != receipt.OutputCommit || observation.Tree != receipt.OutputTree || observation.Clean != receipt.Clean || observation.EventDigest != resultEvent.EventDigest {
		return fmt.Errorf("repository observation contradicts receipt")
	}
	if result.InputCommit != receipt.InputCommit || result.OutputCommit != receipt.OutputCommit || result.OutputTree != receipt.OutputTree || result.Clean != receipt.Clean || result.ManifestDigest != receipt.ManifestDigest || resultEvent.CandidateCommit != receipt.OutputCommit || resultEvent.CandidateTree != receipt.OutputTree {
		return fmt.Errorf("execution-result observation contradicts receipt")
	}
	return nil
}

func commitRootTree(content []byte) (string, error) {
	headers := strings.SplitN(string(content), "\n\n", 2)[0]
	root := ""
	for _, line := range strings.Split(headers, "\n") {
		if strings.HasPrefix(line, "tree ") {
			if root != "" || len(line) != 45 || !isLowerHex(line[5:]) {
				return "", fmt.Errorf("commit has malformed tree header")
			}
			root = line[5:]
		}
	}
	if root == "" {
		return "", fmt.Errorf("commit has no tree header")
	}
	return root, nil
}

func visitTree(oid string, objects map[string]RepositoryObject, reachable map[string]bool) error {
	if reachable[oid] {
		return nil
	}
	object, ok := objects[oid]
	if !ok || object.Type != "tree" {
		return fmt.Errorf("missing tree %s", oid)
	}
	reachable[oid] = true
	content := object.Content
	for len(content) != 0 {
		space := bytes.IndexByte(content, ' ')
		nul := bytes.IndexByte(content, 0)
		if space <= 0 || nul <= space+1 || len(content) < nul+21 {
			return fmt.Errorf("tree %s has malformed entry", oid)
		}
		mode := string(content[:space])
		child := hex.EncodeToString(content[nul+1 : nul+21])
		if mode == "40000" || mode == "040000" {
			if err := visitTree(child, objects, reachable); err != nil {
				return err
			}
		}
		content = content[nul+21:]
	}
	return nil
}

func decodeExecutionEvents(documents [][]byte) ([]contextevent.Event, error) {
	events := make([]contextevent.Event, len(documents))
	for i, document := range documents {
		event, err := contextevent.DecodeEvent(bytes.NewReader(document))
		if err != nil {
			return nil, fmt.Errorf("contextreceipt: execution event proof[%d]: %w", i, err)
		}
		if event.Kind == contextevent.KindReceipt {
			return nil, fmt.Errorf("contextreceipt: execution event proof includes later receipt event")
		}
		events[i] = event
	}
	return events, nil
}

func verifyExecutionEventContinuity(events []contextevent.Event, receipt Receipt, execution ExecutionProjection) (State, State) {
	if len(events) == 0 {
		return StateViolated, StateViolated
	}
	revisionIndex := 0
	for i, event := range events {
		if revisionIndex >= len(receipt.RevisionSegments) {
			return StateViolated, StateViolated
		}
		revision := receipt.RevisionSegments[revisionIndex]
		if event.ManifestRevision != revision.ManifestRevision || event.ManifestDigest != revision.ManifestDigest || event.Flight != execution.Flight || event.Lane != execution.Lane || event.Epoch != execution.Epoch || event.Session != execution.Session || event.ATCRunway != receipt.ATCRunway || event.ExecutionWorkspaceID != receipt.ExecutionWorkspaceID || event.Adapter != receipt.Adapter || event.AdapterVersion != receipt.AdapterVersion {
			return StateViolated, StateViolated
		}
		if event.SourceSequence == 1 {
			if revisionIndex == 0 {
				if event.PriorRevision != nil {
					return StateViolated, StateViolated
				}
			} else {
				prior := receipt.RevisionSegments[revisionIndex-1]
				if event.PriorRevision == nil || event.PriorRevision.ManifestRevision != prior.ManifestRevision || event.PriorRevision.ManifestDigest != prior.ManifestDigest || event.PriorRevision.EventRoot != prior.EventRoot || event.PriorRevision.TerminalSourceSequence != prior.TerminalSourceSequence || event.PriorRevision.TerminalGlobalSequence != prior.TerminalGlobalSequence {
					return StateViolated, StateViolated
				}
			}
		} else if i == 0 || event.PriorEventDigest != events[i-1].EventDigest || events[i-1].ManifestRevision != event.ManifestRevision || event.SourceSequence != events[i-1].SourceSequence+1 {
			return StateViolated, StateViolated
		}
		if event.EventDigest == revision.EventRoot {
			if event.SourceSequence != revision.TerminalSourceSequence || event.Kind != revision.TerminalKind {
				return StateViolated, StateViolated
			}
			revisionIndex++
		}
	}
	if revisionIndex != len(receipt.RevisionSegments) || events[len(events)-1].Kind != contextevent.KindExecutionResult {
		return StateViolated, StateViolated
	}
	return StateProven, StateProven
}

func localUnavailableAuthority(source string) AuthorityFacts {
	witness := Witness{Code: "authority-unavailable", SourceID: "verdi.context-receipt-verify/controller", Detail: "unavailable"}
	return AuthorityFacts{
		Profile:     ProfileAuthority{State: StateUnproven, ProfileBytes: []byte{}, Witnesses: []Witness{witness}},
		TrustFact:   gp.TrustFact{SourceID: source, SourceKind: gp.TrustSourceIdentityProvider, Subjects: []string{}, Available: false, Valid: false, Reason: "controller unavailable"},
		Isolation:   IsolationAuthority{State: StateUnproven, Witnesses: []Witness{witness}},
		Persistence: PersistenceAuthority{State: StateUnproven, Witnesses: []Witness{witness}},
	}
}

type oneTrustFactReader struct{ fact gp.TrustFact }

func (r oneTrustFactReader) ReadTrustFact(context.Context, gp.TrustSource, gp.PrincipalClaim) (gp.TrustFact, error) {
	return r.fact, nil
}

func verifyGovernanceAuthority(ctx context.Context, authority AuthorityFacts, execution ExecutionProjection, receipt Receipt) (State, string, string, State, string, string, string, []Witness, error) {
	profileExpected := mustProjectionDigest(struct {
		ID         string `json:"id"`
		Digest     string `json:"digest"`
		Transition string `json:"transition"`
	}{execution.ProfileRef.ID, execution.ProfileRef.Digest, receiptVerificationTransition})
	if authority.Profile.State != StateProven {
		return authority.Profile.State, profileExpected, "", StateUnproven, mustProjectionDigest(receipt.RunnerPrincipalResolution), "", "runner-unavailable", nil, nil
	}
	profile, err := decodeAuthorityProfile(authority.Profile.ProfileBytes)
	if err != nil {
		return "", "", "", "", "", "", "", nil, fmt.Errorf("contextreceipt: governance profile: %w", err)
	}
	profileDigest, err := profile.Digest()
	if err != nil {
		return "", "", "", "", "", "", "", nil, err
	}
	profileObserved := mustProjectionDigest(struct {
		ID         string `json:"id"`
		Digest     string `json:"digest"`
		Transition string `json:"transition"`
	}{profile.ID, profileDigest, receiptVerificationTransition})
	profileState := stateForEqual(profileExpected, profileObserved)
	if profile.Class == gp.ClassExperimental {
		profileState = StateViolated
	}
	if authority.TrustFact.SourceKind != gp.TrustSourceIdentityProvider {
		profileState = StateViolated
	}
	resolution, err := gp.NewResolver(oneTrustFactReader{authority.TrustFact}).Resolve(ctx, profile, receipt.RunnerPrincipalResolution.Claim)
	if err != nil {
		return "", "", "", "", "", "", "", nil, fmt.Errorf("contextreceipt: governance resolver: %w", err)
	}
	runnerExpected := mustProjectionDigest(receipt.RunnerPrincipalResolution)
	runnerObserved := mustProjectionDigest(resolution)
	runnerState := resolutionState(resolution.State)
	runnerCode := "runner-untrusted"
	if runnerState == StateUnproven {
		runnerCode = "runner-unavailable"
	}
	if runnerState == StateProven && runnerExpected != runnerObserved {
		runnerState = StateViolated
	}
	if runnerState == StateProven {
		decision, err := gp.Authorize(profile, gp.AuthorizationRequest{Transition: receiptVerificationTransition, Posture: gp.PostureAuthoritative, Resolutions: []gp.PrincipalResolution{resolution}, Approvals: []gp.ApprovalRecord{{Role: receiptVerificationRole, PrincipalID: resolution.PrincipalID}}, RuleFacts: []gp.RuleFact{}, EvidenceSources: []string{}, EscalationMetrics: map[string]int{}})
		if err != nil {
			return "", "", "", "", "", "", "", nil, fmt.Errorf("contextreceipt: governance authorization: %w", err)
		}
		switch decision.State {
		case gp.AuthorizationAuthorized:
		case gp.AuthorizationUnproven:
			runnerState = StateUnproven
			runnerCode = "runner-unavailable"
		case gp.AuthorizationViolated:
			runnerState = StateViolated
			runnerCode = "runner-role-refused"
			resolution.Witnesses = nil
		default:
			return "", "", "", "", "", "", "", nil, fmt.Errorf("contextreceipt: unknown governance authorization state %q", decision.State)
		}
	}
	return profileState, profileExpected, profileObserved, runnerState, runnerExpected, runnerObserved, runnerCode, append([]Witness{}, resolution.Witnesses...), nil
}

func decodeAuthorityProfile(raw []byte) (gp.Profile, error) {
	var probe governanceProfileProbe
	var jsonProbe gp.Profile
	if err := artifact.DecodeExactJSON(raw, &jsonProbe); err == nil {
		probe = governanceProfileProbe{
			RoleMappings: jsonProbe.RoleMappings, ApplicableTransitions: jsonProbe.ApplicableTransitions,
			IdentityTrustSources: jsonProbe.IdentityTrustSources, EscalationThresholds: jsonProbe.EscalationThresholds,
		}
	} else if err := artifact.DecodeStrict(raw, &probe); err != nil {
		return gp.Profile{}, err
	}
	catalog := gp.Catalog{Roles: []string{receiptVerificationRole}, Transitions: []string{receiptVerificationTransition}}
	for _, mapping := range probe.RoleMappings {
		catalog.Roles = appendUnique(catalog.Roles, mapping.Role)
	}
	for _, transition := range probe.ApplicableTransitions {
		catalog.Transitions = appendUnique(catalog.Transitions, transition)
	}
	for _, source := range probe.IdentityTrustSources {
		catalog.EvidenceSources = appendUnique(catalog.EvidenceSources, source.ID)
	}
	for _, threshold := range probe.EscalationThresholds {
		catalog.EscalationMetrics = appendUnique(catalog.EscalationMetrics, threshold.Metric)
		for _, role := range threshold.RequiredRoles {
			catalog.Roles = appendUnique(catalog.Roles, role)
		}
		for _, transition := range threshold.Transitions {
			catalog.Transitions = appendUnique(catalog.Transitions, transition)
		}
	}
	return gp.DecodeProfile(raw, catalog)
}

type governanceProfileProbe struct {
	Schema                     string                         `yaml:"schema"`
	ID                         string                         `yaml:"id"`
	Class                      gp.Class                       `yaml:"class"`
	ApplicableTransitions      []string                       `yaml:"applicable_transitions"`
	IdentityTrustSources       []gp.TrustSource               `yaml:"identity_trust_sources"`
	RoleMappings               []gp.RoleMapping               `yaml:"role_mappings"`
	OwnershipSources           []gp.OwnershipSource           `yaml:"ownership_sources"`
	SignatureRequirements      []gp.SignatureRequirement      `yaml:"signature_requirements"`
	RequiredApprovers          []gp.ApproverRequirement       `yaml:"required_approvers"`
	DistinctnessRules          []gp.DistinctnessRule          `yaml:"distinctness_rules"`
	EvidenceSourceRestrictions []gp.EvidenceSourceRestriction `yaml:"evidence_source_restrictions"`
	EscalationThresholds       []gp.EscalationThreshold       `yaml:"escalation_thresholds"`
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func applyPersistenceAuthority(operands []Operand, persistence PersistenceAuthority, receipt Receipt, event contextevent.Event, ack contextevent.ReceiptEventAck) {
	for _, index := range []int{operandKindIndex("receipt-event"), operandKindIndex("receipt-ack")} {
		if persistence.State != StateProven && operands[index].State == StateProven {
			operands[index].State = persistence.State
			operands[index].Witnesses = append([]Witness{}, persistence.Witnesses...)
		}
	}
	if persistence.State == StateProven && (persistence.ReceiptDigest != receipt.Digest || persistence.ReceiptEventDigest != event.EventDigest || persistence.ReceiptAckDigest != mustProjectionDigest(ack)) {
		for _, index := range []int{operandKindIndex("receipt-event"), operandKindIndex("receipt-ack")} {
			operands[index].State = StateViolated
			operands[index].Witnesses = []Witness{{Code: "receipt-ack-mismatch", SourceID: "verdi.context-receipt-verify/" + string(operands[index].Kind), EvidenceDigest: operands[index].ObservedDigest, Detail: "contradicted"}}
		}
	}
}

func findingsAndWitnesses(operands []Operand, receipt Receipt) ([]Finding, []Witness) {
	findings := make([]Finding, 0)
	witnesses := make([]Witness, 0)
	for i := range operands {
		operand := &operands[i]
		if operand.State == StateProven {
			continue
		}
		code := defaultFindingCode(operand.Kind)
		switch {
		case operand.Kind == "receipt" && receipt.Authority == AuthorityAdvisory:
			code = "advisory-receipt"
		case operand.Kind == "governance-profile" && operand.State == StateUnproven:
			code = "authority-unavailable"
		case operand.Kind == "runner" && operand.State == StateUnproven:
			code = "runner-unavailable"
		case operand.Kind == "runner" && len(operand.Witnesses) == 1 && operand.Witnesses[0].Code == "runner-role-refused":
			code = "runner-role-refused"
		case operand.Kind == "isolation" && operand.State == StateUnproven:
			code = "isolation-unavailable"
		}
		if operand.Kind != "runner" || len(operand.Witnesses) == 0 || code == "runner-role-refused" || code == "runner-unavailable" {
			operand.Witnesses = []Witness{{Code: code, SourceID: "verdi.context-receipt-verify/" + string(operand.Kind), EvidenceDigest: operand.ObservedDigest, Detail: stateDetail(operand.State)}}
		}
		findings = append(findings, Finding{Code: code, OperandKind: operand.Kind, OperandID: operand.ID, State: operand.State})
		witnesses = append(witnesses, operand.Witnesses...)
	}
	witnesses = sortDeduplicateWitnesses(witnesses)
	return findings, witnesses
}

func sortDeduplicateWitnesses(witnesses []Witness) []Witness {
	sort.Slice(witnesses, func(i, j int) bool { return principalWitnessLess(witnesses[i], witnesses[j]) })
	out := make([]Witness, 0, len(witnesses))
	for _, witness := range witnesses {
		if len(out) == 0 || principalWitnessLess(out[len(out)-1], witness) {
			out = append(out, witness)
		}
	}
	return out
}

func defaultFindingCode(kind OperandKind) string {
	codes := []string{"receipt-mismatch", "candidate-stale", "execution-request-mismatch", "repository-mismatch", "manifest-mismatch", "dispatch-mismatch", "event-mismatch", "event-chain-mismatch", "expansion-incomplete", "obligation-mismatch", "evidence-mismatch", "receipt-event-mismatch", "receipt-ack-mismatch", "profile-mismatch", "runner-untrusted", "isolation-violated", "review-packet-mismatch", "review-link-mismatch", "review-stale"}
	index := operandKindIndex(kind)
	if index < 0 {
		return "receipt-mismatch"
	}
	return codes[index]
}

func receiptState(receipt Receipt) State {
	if receipt.Authority == AuthorityAdvisory {
		return StateUnproven
	}
	return StateProven
}
func receiptFindingCode(receipt Receipt) string {
	if receipt.Authority == AuthorityAdvisory {
		return "advisory-receipt"
	}
	return "receipt-mismatch"
}
func stateForEqual(expected, observed string) State {
	if expected == observed {
		return StateProven
	}
	return StateViolated
}
func stateDetail(state State) string {
	if state == StateUnproven {
		return "unavailable"
	}
	return "contradicted"
}
func profileFinding(state State) string {
	if state == StateUnproven {
		return "authority-unavailable"
	}
	return "profile-mismatch"
}
func isolationFinding(state State) string {
	if state == StateUnproven {
		return "isolation-unavailable"
	}
	return "isolation-violated"
}
func resolutionState(state gp.ResolutionState) State {
	switch state {
	case gp.ResolutionAuthenticated:
		return StateProven
	case gp.ResolutionViolated:
		return StateViolated
	default:
		return StateUnproven
	}
}

func mustProjectionDigest(value any) string { digest, _ := canonjson.Digest(value); return digest }
func digestRaw(data []byte) string          { return digestRawNoFrame(data) }
func digestRawNoFrame(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func digestDocumentSet(documents [][]byte) string {
	digests := make([]string, len(documents))
	for i := range documents {
		digests[i] = digestRaw(documents[i])
	}
	return mustProjectionDigest(digests)
}
func terminalEventDigestsFromEvents(events []contextevent.Event) []string {
	out := make([]string, 0)
	for i, event := range events {
		if i+1 == len(events) || events[i+1].ManifestRevision != event.ManifestRevision {
			out = append(out, event.EventDigest)
		}
	}
	return out
}
func terminalEventDigests(revisions []contextevent.Revision) []string {
	out := make([]string, len(revisions))
	for i := range revisions {
		out[i] = revisions[i].EventRoot
	}
	return out
}
func revisionsFromEvents(events []contextevent.Event, declared []contextevent.Revision) []contextevent.Revision {
	observed := make([]contextevent.Revision, 0, len(declared))
	start := 0
	for index, expected := range declared {
		end := start
		for end < len(events) && events[end].ManifestRevision == expected.ManifestRevision {
			end++
		}
		if end == start {
			observed = append(observed, contextevent.Revision{})
			continue
		}
		terminal := events[end-1]
		observed = append(observed, contextevent.Revision{
			Schema: contextevent.RevisionSchemaID, ManifestRevision: terminal.ManifestRevision, ManifestDigest: terminal.ManifestDigest,
			FirstGlobalSequence: expected.FirstGlobalSequence, TerminalGlobalSequence: expected.TerminalGlobalSequence,
			TerminalSourceSequence: terminal.SourceSequence, TerminalKind: terminal.Kind, EventRoot: terminal.EventDigest,
		})
		start = end
		_ = index
	}
	return observed
}

func (v *Verifier) verifyExpansionDocuments(documents [][]byte, expansions []Expansion) (State, error) {
	if len(documents) != 0 && v.expansion == nil {
		return "", fmt.Errorf("contextreceipt: expansion proof verifier is unavailable")
	}
	state := StateProven
	for i, document := range documents {
		expansion := Expansion{}
		if i < len(expansions) {
			expansion = expansions[i]
		} else {
			state = StateViolated
		}
		projection, err := v.expansion.VerifyExpansionProof(append([]byte{}, document...), expansion)
		if err != nil {
			return "", fmt.Errorf("contextreceipt: expansion data proof[%d]: %w", i, err)
		}
		for _, field := range []struct{ name, digest string }{
			{name: "data item", digest: projection.DataItemDigest},
			{name: "data", digest: projection.DataDigest},
			{name: "expansion", digest: projection.ExpansionDigest},
		} {
			if err := validateReceiptDigest("expansion proof "+field.name+" digest", field.digest); err != nil {
				return "", fmt.Errorf("contextreceipt: expansion data proof[%d]: %w", i, err)
			}
		}
		if i < len(expansions) && projection.ExpansionDigest != expansions[i].ExpansionDigest {
			state = StateViolated
		}
	}
	if len(documents) != len(expansions) {
		state = StateViolated
	}
	return state, nil
}

func verifyObligationDocuments(documents [][]byte, obligations []Obligation) (State, error) {
	state := StateProven
	for i, document := range documents {
		obligation, err := artifact.DecodeObligation(document)
		if err != nil {
			return "", fmt.Errorf("contextreceipt: obligation proof[%d]: %w", i, err)
		}
		if i >= len(obligations) || obligation.ID != obligations[i].Ref || obligation.ForKind != obligations[i].Kind || digestRaw(document) != obligations[i].ContentDigest {
			state = StateViolated
		}
	}
	if len(documents) != len(obligations) {
		state = StateViolated
	}
	return state, nil
}

func verifyEvidenceDocuments(documents [][]byte, evidence []Evidence) (State, error) {
	state := StateProven
	for i, document := range documents {
		if err := validateCanonicalJSONDocument(document, fmt.Sprintf("evidence result proof[%d]", i)); err != nil {
			return "", err
		}
		if i >= len(evidence) || digestRaw(document) != evidence[i].OutputDigest {
			state = StateViolated
		}
	}
	if len(documents) != len(evidence) {
		state = StateViolated
	}
	return state, nil
}

func validateCanonicalJSONDocument(document []byte, name string) error {
	var decoded any
	if err := artifact.DecodeExactJSON(document, &decoded); err != nil {
		return fmt.Errorf("contextreceipt: %s: %w", name, err)
	}
	canonical, err := canonjson.Marshal(decoded)
	if err != nil {
		return fmt.Errorf("contextreceipt: %s: %w", name, err)
	}
	if !bytes.Equal(document, canonical) {
		return fmt.Errorf("contextreceipt: %s is not byte-canonical", name)
	}
	return nil
}

func observedProjectionDigest(state State, expected any, documents [][]byte) string {
	if state == StateProven {
		return mustProjectionDigest(expected)
	}
	return digestDocumentSet(documents)
}

func executionProjectionFromReceipt(receipt Receipt) any {
	return struct {
		ATCRunway              string               `json:"atc_runway"`
		WorkspaceRequestDigest string               `json:"workspace_request_digest"`
		InputCommit            string               `json:"input_commit"`
		InputTree              string               `json:"input_tree"`
		ManifestDigest         string               `json:"manifest_digest"`
		Adapter                contextevent.Adapter `json:"adapter"`
		AdapterVersion         string               `json:"adapter_version"`
	}{receipt.ATCRunway, receipt.ExecutionWorkspaceRequestDigest, receipt.InputCommit, receipt.InputTree, receipt.ManifestDigest, receipt.Adapter, receipt.AdapterVersion}
}
func executionReceiptProjection(execution ExecutionProjection) any {
	return struct {
		ATCRunway              string               `json:"atc_runway"`
		WorkspaceRequestDigest string               `json:"workspace_request_digest"`
		InputCommit            string               `json:"input_commit"`
		InputTree              string               `json:"input_tree"`
		ManifestDigest         string               `json:"manifest_digest"`
		Adapter                contextevent.Adapter `json:"adapter"`
		AdapterVersion         string               `json:"adapter_version"`
	}{execution.ATCRunway, execution.ExecutionWorkspaceRequestDigest, execution.InputCommit, execution.InputTree, execution.ManifestDigest, execution.Adapter, execution.AdapterVersion}
}
func repositoryProjectionFromReceipt(receipt Receipt) any {
	return struct {
		InputCommit  string `json:"input_commit"`
		InputTree    string `json:"input_tree"`
		OutputCommit string `json:"output_commit"`
		OutputTree   string `json:"output_tree"`
		Clean        bool   `json:"clean"`
	}{receipt.InputCommit, receipt.InputTree, receipt.OutputCommit, receipt.OutputTree, receipt.Clean}
}
func repositoryProjectionFromProof(proof RepositoryProof) any {
	return struct {
		InputCommit  string `json:"input_commit"`
		InputTree    string `json:"input_tree"`
		OutputCommit string `json:"output_commit"`
		OutputTree   string `json:"output_tree"`
		Clean        bool   `json:"clean"`
	}{proof.Candidate.BaseCommit, proof.Candidate.BaseTree, proof.ExecutionObservation.Commit, proof.ExecutionObservation.Tree, proof.ExecutionObservation.Clean}
}
