package sealedreview

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// PacketCompiler compiles one exact packet over immutable, consumer-supplied
// repository and context-compiler ports.
type PacketCompiler struct {
	repository RepositoryReader
	compiler   ContextCompiler
}

// NewPacketCompiler constructs a packet compiler with all required ports.
func NewPacketCompiler(ports PacketCompilerPorts) (*PacketCompiler, error) {
	if ports.Repository == nil {
		return nil, fmt.Errorf("sealedreview: packet compiler repository port is required")
	}
	if ports.Compiler == nil {
		return nil, fmt.Errorf("sealedreview: packet compiler context compiler port is required")
	}
	return &PacketCompiler{repository: ports.Repository, compiler: ports.Compiler}, nil
}

// Compile reads only named Git objects and declared wrapper bytes, builds the
// fixed R0/R2 inventory, and compiles one packet-bound manifest/projection.
func (c *PacketCompiler) Compile(ctx context.Context, request PacketRequest) (PacketResult, error) {
	if ctx == nil {
		return PacketResult{}, fmt.Errorf("sealedreview: compile packet: nil context")
	}
	if c == nil || c.repository == nil || c.compiler == nil {
		return PacketResult{}, fmt.Errorf("sealedreview: packet compiler is not constructed")
	}
	if err := ctx.Err(); err != nil {
		return PacketResult{}, fmt.Errorf("sealedreview: compile packet: %w", err)
	}
	if err := validatePacketRequestShape(request); err != nil {
		return PacketResult{}, err
	}

	builderReceipt, err := contextreceipt.DecodeReceipt(bytes.NewReader(request.BuilderReceiptBytes))
	if err != nil {
		return PacketResult{}, fmt.Errorf("sealedreview: builder receipt: %w", err)
	}
	if builderReceipt.Role != contextreceipt.RoleBuilder {
		return PacketResult{}, fmt.Errorf("sealedreview: packet requires a builder receipt")
	}
	if builderReceipt.InputCommit != request.Candidate.BaseCommit || builderReceipt.InputTree != request.Candidate.BaseTree ||
		builderReceipt.OutputCommit != request.Candidate.HeadCommit || builderReceipt.OutputTree != request.Candidate.HeadTree || !builderReceipt.Clean {
		return PacketResult{}, fmt.Errorf("sealedreview: builder receipt does not bind the requested clean candidate")
	}

	view, err := c.readCandidate(ctx, request.Candidate)
	if err != nil {
		return PacketResult{}, err
	}
	diff, diffBytes, err := c.compileDiff(ctx, request.Candidate, view)
	if err != nil {
		return PacketResult{}, err
	}
	acceptedSpec, err := c.readPath(ctx, view.head, request.AcceptedSpecPath)
	if err != nil {
		return PacketResult{}, fmt.Errorf("sealedreview: read accepted spec: %w", err)
	}
	reviewPolicy, err := c.readPath(ctx, view.head, request.ReviewPolicyPath)
	if err != nil {
		return PacketResult{}, fmt.Errorf("sealedreview: read review policy: %w", err)
	}

	builderEvidence, builderEvidenceBytes, err := compileEvidenceBundle(
		EvidenceScopeBuilder,
		request.Candidate,
		request.BuilderEvidenceResultBytes,
		builderReceipt.Evidence,
	)
	if err != nil {
		return PacketResult{}, fmt.Errorf("sealedreview: builder evidence: %w", err)
	}

	items := []Item{
		newItem(ItemAcceptedSpec, "", "text/markdown; charset=utf-8", acceptedSpec),
		newItem(ItemCurrentDiff, request.Candidate.BaseCommit+".."+request.Candidate.HeadCommit, "application/json", diffBytes),
		newItem(ItemEvidenceBundle, request.Candidate.HeadCommit, "application/json", builderEvidenceBytes),
		newItem(ItemBuilderReceipt, builderReceipt.Digest, "application/json", request.BuilderReceiptBytes),
		newItem(ItemReviewPolicy, "", "text/markdown; charset=utf-8", reviewPolicy),
	}
	specID, err := decodeSpecID(acceptedSpec)
	if err != nil {
		return PacketResult{}, fmt.Errorf("sealedreview: decode accepted spec identity: %w", err)
	}
	items[0].ID = specID
	policy, err := decodeReviewPolicy(reviewPolicy)
	if err != nil {
		return PacketResult{}, err
	}
	items[4].ID = policy

	var currentEvidence *EvidenceBundle
	if request.Round == RoundR2 {
		prior, err := contextreceipt.DecodeReceipt(bytes.NewReader(request.PriorReviewReceiptBytes))
		if err != nil {
			return PacketResult{}, fmt.Errorf("sealedreview: prior R0 receipt: %w", err)
		}
		if err := validatePriorR0Receipt(prior, builderReceipt.Digest, request.Candidate, packetItemProjection(Packet{Items: items})); err != nil {
			return PacketResult{}, err
		}
		adjudication, err := DecodeAdjudication(bytes.NewReader(request.AdjudicationBytes))
		if err != nil {
			return PacketResult{}, fmt.Errorf("sealedreview: R2 adjudication: %w", err)
		}
		if adjudication.R0ReceiptDigest != prior.Digest {
			return PacketResult{}, fmt.Errorf("sealedreview: R2 adjudication does not bind the actual R0 receipt")
		}
		if err := validateAdjudicationCandidate(adjudication, request.Candidate); err != nil {
			return PacketResult{}, err
		}

		rebuilt, err := c.compiler.RebuildEvidence(ctx, EvidenceRebuildRequest{
			Candidate: request.Candidate,
			Commands:  cloneEvidenceSummaries(builderReceipt.Evidence),
		})
		if err != nil {
			return PacketResult{}, fmt.Errorf("sealedreview: rebuild current-candidate evidence: %w", err)
		}
		current, currentBytes, err := compileEvidenceBundle(
			EvidenceScopeCurrentCandidate,
			request.Candidate,
			rebuilt,
			builderReceipt.Evidence,
		)
		if err != nil {
			return PacketResult{}, fmt.Errorf("sealedreview: current-candidate evidence: %w", err)
		}
		if current.Digest == builderEvidence.Digest {
			return PacketResult{}, fmt.Errorf("sealedreview: current-candidate evidence reused the builder bundle")
		}
		currentEvidence = &current
		items = append(items,
			newItem(ItemAdjudication, prior.Digest, "application/json", request.AdjudicationBytes),
			newItem(ItemCurrentCandidateEvidence, request.Candidate.HeadCommit, "application/json", currentBytes),
		)
	}

	packet := Packet{
		Schema: PacketSchemaID, Round: request.Round, Candidate: request.Candidate,
		Reviewer: request.Reviewer, BuilderReceiptDigest: builderReceipt.Digest,
		Items: items, Exclusions: append([]string(nil), packetExclusions...),
	}
	packetBytes, err := EncodePacket(packet)
	if err != nil {
		return PacketResult{}, fmt.Errorf("sealedreview: encode packet: %w", err)
	}
	packet, err = DecodePacket(bytes.NewReader(packetBytes))
	if err != nil {
		return PacketResult{}, fmt.Errorf("sealedreview: decode compiled packet: %w", err)
	}
	binding, err := contextBinding(packet)
	if err != nil {
		return PacketResult{}, fmt.Errorf("sealedreview: bind packet context: %w", err)
	}
	compileRequest := ContextCompileRequest{
		Round: request.Round, Candidate: request.Candidate, Reviewer: request.Reviewer,
		PacketBytes: append([]byte(nil), packetBytes...), Binding: cloneContextBinding(binding),
	}
	compilation, err := c.compiler.Compile(ctx, compileRequest)
	if err != nil {
		return PacketResult{}, fmt.Errorf("sealedreview: compile packet-bound context: %w", err)
	}
	if err := validateContextCompilation(compilation, packet, binding); err != nil {
		return PacketResult{}, err
	}
	compilation.Binding = cloneContextBinding(binding)

	result := PacketResult{
		Packet: packet, PacketBytes: append([]byte(nil), packetBytes...),
		Diff: diff, DiffBytes: append([]byte(nil), diffBytes...),
		BuilderEvidence: builderEvidence, Compilation: cloneContextCompilation(compilation),
	}
	if currentEvidence != nil {
		current := cloneEvidenceBundle(*currentEvidence)
		result.CurrentCandidateEvidence = &current
	}
	return result, nil
}

func validatePriorR0Receipt(receipt contextreceipt.Receipt, builderDigest string, candidate contextreceipt.Candidate, wantInputs []contextreceipt.ReviewInput) error {
	if receipt.Role != contextreceipt.RoleReviewer || len(receipt.ReviewOf) != 1 || receipt.ReviewOf[0] != builderDigest {
		return fmt.Errorf("sealedreview: prior R0 receipt does not link to the builder")
	}
	if receipt.OutputCommit != candidate.HeadCommit || receipt.OutputTree != candidate.HeadTree {
		return fmt.Errorf("sealedreview: prior R0 receipt is stale for the candidate")
	}
	if len(receipt.ReviewInputs) != len(wantInputs) || len(wantInputs) != 5 {
		return fmt.Errorf("sealedreview: prior R0 receipt must contain the exact five-kind review inventory")
	}
	for i, want := range wantInputs {
		if receipt.ReviewInputs[i] != want {
			return fmt.Errorf("sealedreview: prior R0 receipt review input[%d] does not match the actual R0 packet item", i)
		}
	}
	return nil
}

func validateAdjudicationCandidate(adjudication Adjudication, candidate contextreceipt.Candidate) error {
	for i, row := range adjudication.Rows {
		event, err := contextevent.DecodeEvent(bytes.NewReader(row.EventBytes))
		if err != nil {
			return fmt.Errorf("sealedreview: adjudication row[%d] event: %w", i, err)
		}
		if event.CandidateCommit != candidate.HeadCommit || event.CandidateTree != candidate.HeadTree {
			return fmt.Errorf("sealedreview: adjudication row[%d] candidate does not match packet", i)
		}
	}
	return nil
}

// VerifyReviewProof verifies exact packet content and projects the three
// reviewer-only operands owned by contextreceipt.
func VerifyReviewProof(raw []byte, receipt contextreceipt.Receipt, candidate contextreceipt.Candidate) (contextreceipt.ReviewProofProjection, error) {
	packet, err := DecodePacket(bytes.NewReader(raw))
	if err != nil {
		return contextreceipt.ReviewProofProjection{}, err
	}
	if receipt.Role != contextreceipt.RoleReviewer {
		return contextreceipt.ReviewProofProjection{}, fmt.Errorf("sealedreview: review proof requires a reviewer receipt")
	}

	expectedPacket := projectionDigest(receipt.ReviewInputs)
	observedInputs := packetItemProjection(packet)
	observedPacket := projectionDigest(observedInputs)
	expectedLink := projectionDigest(receipt.ReviewOf)
	observedLink := projectionDigest([]string{packet.BuilderReceiptDigest})
	type freshnessExpectation struct {
		Candidate contextreceipt.Candidate `json:"candidate"`
		ReviewOf  []string                 `json:"review_of"`
	}
	expectedFreshness := projectionDigest(freshnessExpectation{Candidate: candidate, ReviewOf: receipt.ReviewOf})

	return contextreceipt.ReviewProofProjection{
		Packet:    reviewOperand(expectedPacket, observedPacket),
		Link:      reviewOperand(expectedLink, observedLink),
		Freshness: contextreceipt.ReviewOperandProjection{State: contextreceipt.StateUnproven, ExpectedDigest: expectedFreshness},
	}, nil
}

func validatePacketRequestShape(request PacketRequest) error {
	if err := validateRound(request.Round); err != nil {
		return err
	}
	if err := validateCandidate(request.Candidate); err != nil {
		return err
	}
	if err := validateReviewer(request.Reviewer); err != nil {
		return err
	}
	if err := validateSelectorPath(request.AcceptedSpecPath); err != nil {
		return fmt.Errorf("sealedreview: accepted spec path: %w", err)
	}
	if err := validateSelectorPath(request.ReviewPolicyPath); err != nil {
		return fmt.Errorf("sealedreview: review policy path: %w", err)
	}
	if request.AcceptedSpecPath == request.ReviewPolicyPath {
		return fmt.Errorf("sealedreview: accepted spec and review policy paths must differ")
	}
	if len(request.BuilderReceiptBytes) == 0 {
		return fmt.Errorf("sealedreview: builder receipt bytes must be nonempty")
	}
	if request.BuilderEvidenceResultBytes == nil {
		return fmt.Errorf("sealedreview: builder evidence result bytes must be non-null")
	}
	switch request.Round {
	case RoundR0:
		if len(request.PriorReviewReceiptBytes) != 0 || len(request.AdjudicationBytes) != 0 {
			return fmt.Errorf("sealedreview: R0 forbids prior review and adjudication bytes")
		}
	case RoundR2:
		if len(request.PriorReviewReceiptBytes) == 0 || len(request.AdjudicationBytes) == 0 {
			return fmt.Errorf("sealedreview: R2 requires prior review and adjudication bytes")
		}
	}
	return nil
}

type treeFile struct {
	mode string
	oid  string
}

type candidateView struct {
	base map[string]treeFile
	head map[string]treeFile
}

func (c *PacketCompiler) readCandidate(ctx context.Context, candidate contextreceipt.Candidate) (candidateView, error) {
	baseTree, err := c.readCommit(ctx, candidate.BaseCommit)
	if err != nil {
		return candidateView{}, fmt.Errorf("sealedreview: read base commit: %w", err)
	}
	if baseTree != candidate.BaseTree {
		return candidateView{}, fmt.Errorf("sealedreview: base commit tree does not match candidate")
	}
	headTree, err := c.readCommit(ctx, candidate.HeadCommit)
	if err != nil {
		return candidateView{}, fmt.Errorf("sealedreview: read head commit: %w", err)
	}
	if headTree != candidate.HeadTree {
		return candidateView{}, fmt.Errorf("sealedreview: head commit tree does not match candidate")
	}
	base, err := c.readTree(ctx, candidate.BaseTree, "", make(map[string]bool))
	if err != nil {
		return candidateView{}, fmt.Errorf("sealedreview: read base tree: %w", err)
	}
	head, err := c.readTree(ctx, candidate.HeadTree, "", make(map[string]bool))
	if err != nil {
		return candidateView{}, fmt.Errorf("sealedreview: read head tree: %w", err)
	}
	return candidateView{base: base, head: head}, nil
}

func (c *PacketCompiler) readCommit(ctx context.Context, oid string) (string, error) {
	object, err := c.readVerifiedObject(ctx, oid, "commit")
	if err != nil {
		return "", err
	}
	headers := strings.SplitN(string(object.Content), "\n\n", 2)[0]
	root := ""
	for _, line := range strings.Split(headers, "\n") {
		if !strings.HasPrefix(line, "tree ") {
			continue
		}
		if root != "" || len(line) != len("tree ")+40 {
			return "", fmt.Errorf("commit %s has malformed tree header", oid)
		}
		root = strings.TrimPrefix(line, "tree ")
		if err := requireGitOID("commit tree", root); err != nil {
			return "", err
		}
	}
	if root == "" {
		return "", fmt.Errorf("commit %s has no tree header", oid)
	}
	return root, nil
}

func (c *PacketCompiler) readTree(ctx context.Context, oid, prefix string, active map[string]bool) (map[string]treeFile, error) {
	if active[oid] {
		return nil, fmt.Errorf("tree cycle at %s", oid)
	}
	active[oid] = true
	defer delete(active, oid)
	object, err := c.readVerifiedObject(ctx, oid, "tree")
	if err != nil {
		return nil, err
	}
	entries, err := parseTreeEntries(object.Content)
	if err != nil {
		return nil, fmt.Errorf("tree %s: %w", oid, err)
	}
	files := make(map[string]treeFile)
	for _, entry := range entries {
		entryPath := entry.name
		if prefix != "" {
			entryPath = prefix + "/" + entry.name
		}
		if entry.mode == "40000" {
			children, err := c.readTree(ctx, entry.oid, entryPath, active)
			if err != nil {
				return nil, err
			}
			for name, child := range children {
				if _, duplicate := files[name]; duplicate {
					return nil, fmt.Errorf("duplicate tree path %q", name)
				}
				files[name] = child
			}
			continue
		}
		switch entry.mode {
		case "100644", "100755", "120000":
		default:
			return nil, fmt.Errorf("path %q has unsupported mode %q", entryPath, entry.mode)
		}
		if _, duplicate := files[entryPath]; duplicate {
			return nil, fmt.Errorf("duplicate tree path %q", entryPath)
		}
		files[entryPath] = treeFile{mode: entry.mode, oid: entry.oid}
	}
	return files, nil
}

type parsedTreeEntry struct {
	mode string
	name string
	oid  string
}

func parseTreeEntries(content []byte) ([]parsedTreeEntry, error) {
	entries := make([]parsedTreeEntry, 0)
	for len(content) != 0 {
		space := bytes.IndexByte(content, ' ')
		nul := bytes.IndexByte(content, 0)
		if space <= 0 || nul <= space+1 || len(content) < nul+21 {
			return nil, fmt.Errorf("malformed tree entry")
		}
		mode, name := string(content[:space]), string(content[space+1:nul])
		if name == "" || name == "." || name == ".." || strings.Contains(name, "/") {
			return nil, fmt.Errorf("invalid tree entry name %q", name)
		}
		entries = append(entries, parsedTreeEntry{mode: mode, name: name, oid: hex.EncodeToString(content[nul+1 : nul+21])})
		content = content[nul+21:]
	}
	return entries, nil
}

func (c *PacketCompiler) readVerifiedObject(ctx context.Context, oid, wantType string) (RepositoryObject, error) {
	if err := ctx.Err(); err != nil {
		return RepositoryObject{}, err
	}
	object, err := c.repository.ReadObject(ctx, oid)
	if err != nil {
		return RepositoryObject{}, err
	}
	if object.Type != wantType {
		return RepositoryObject{}, fmt.Errorf("object %s type = %q, want %q", oid, object.Type, wantType)
	}
	if gitObjectOID(object.Type, object.Content) != oid {
		return RepositoryObject{}, fmt.Errorf("object %s content identity mismatch", oid)
	}
	object.Content = append([]byte(nil), object.Content...)
	return object, nil
}

func (c *PacketCompiler) readPath(ctx context.Context, tree map[string]treeFile, name string) ([]byte, error) {
	entry, ok := tree[name]
	if !ok {
		return nil, fmt.Errorf("path %q is absent", name)
	}
	object, err := c.readVerifiedObject(ctx, entry.oid, "blob")
	if err != nil {
		return nil, err
	}
	return object.Content, nil
}

func (c *PacketCompiler) compileDiff(ctx context.Context, candidate contextreceipt.Candidate, view candidateView) (Diff, []byte, error) {
	paths := make([]string, 0, len(view.base)+len(view.head))
	seen := make(map[string]bool, len(view.base)+len(view.head))
	for name := range view.base {
		seen[name] = true
		paths = append(paths, name)
	}
	for name := range view.head {
		if !seen[name] {
			paths = append(paths, name)
		}
	}
	sort.Strings(paths)
	entries := make([]DiffEntry, 0)
	for _, name := range paths {
		before, hadBefore := view.base[name]
		after, hasAfter := view.head[name]
		if hadBefore && hasAfter && before == after {
			continue
		}
		entry := DiffEntry{Path: []byte(name), BeforeBytes: []byte{}, AfterBytes: []byte{}}
		switch {
		case !hadBefore:
			entry.State = DiffAdded
		case !hasAfter:
			entry.State = DiffDeleted
		default:
			entry.State = DiffModified
		}
		if hadBefore {
			content, err := c.readVerifiedObject(ctx, before.oid, "blob")
			if err != nil {
				return Diff{}, nil, fmt.Errorf("sealedreview: read diff before blob %q: %w", name, err)
			}
			entry.BeforeMode, entry.BeforeBlob, entry.BeforeBytes = before.mode, before.oid, content.Content
		}
		if hasAfter {
			content, err := c.readVerifiedObject(ctx, after.oid, "blob")
			if err != nil {
				return Diff{}, nil, fmt.Errorf("sealedreview: read diff after blob %q: %w", name, err)
			}
			entry.AfterMode, entry.AfterBlob, entry.AfterBytes = after.mode, after.oid, content.Content
		}
		entries = append(entries, entry)
	}
	diff := Diff{
		Schema: DiffSchemaID, BaseCommit: candidate.BaseCommit, BaseTree: candidate.BaseTree,
		HeadCommit: candidate.HeadCommit, HeadTree: candidate.HeadTree, Entries: entries,
	}
	encoded, err := EncodeDiff(diff)
	if err != nil {
		return Diff{}, nil, err
	}
	decoded, err := DecodeDiff(bytes.NewReader(encoded))
	if err != nil {
		return Diff{}, nil, err
	}
	return decoded, encoded, nil
}

func compileEvidenceBundle(scope EvidenceScope, candidate contextreceipt.Candidate, documents [][]byte, summaries []contextreceipt.Evidence) (EvidenceBundle, []byte, error) {
	if documents == nil {
		return EvidenceBundle{}, nil, fmt.Errorf("result documents must be non-null")
	}
	if len(documents) != len(summaries) {
		return EvidenceBundle{}, nil, fmt.Errorf("result document count %d does not match command count %d", len(documents), len(summaries))
	}
	rows := make([]EvidenceRow, 0, len(documents))
	for i, raw := range documents {
		result, err := DecodeEvidenceResult(bytes.NewReader(raw))
		if err != nil {
			return EvidenceBundle{}, nil, fmt.Errorf("result[%d]: %w", i, err)
		}
		summary := summaries[i]
		if result.CommandID != summary.CommandID || !equalStrings(result.Argv, summary.Argv) {
			return EvidenceBundle{}, nil, fmt.Errorf("result[%d] command identity does not match receipt", i)
		}
		if result.ExitCode != summary.ExitCode || result.Verdict != summary.Verdict || result.OutputDigest != summary.OutputDigest {
			return EvidenceBundle{}, nil, fmt.Errorf("result[%d] summary does not match receipt", i)
		}
		rows = append(rows, EvidenceRow{CommandID: result.CommandID, ResultBytes: append([]byte(nil), raw...), ResultDigest: rawDigest(raw)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CommandID < rows[j].CommandID })
	bundle := EvidenceBundle{Schema: EvidenceBundleSchemaID, Scope: scope, Candidate: candidate, Rows: rows}
	encoded, err := EncodeEvidenceBundle(bundle)
	if err != nil {
		return EvidenceBundle{}, nil, err
	}
	decoded, err := DecodeEvidenceBundle(bytes.NewReader(encoded))
	if err != nil {
		return EvidenceBundle{}, nil, err
	}
	return decoded, encoded, nil
}

func newItem(kind ItemKind, id, mediaType string, content []byte) Item {
	return Item{Kind: kind, ID: id, MediaType: mediaType, ContentDigest: rawDigest(content), Content: append([]byte(nil), content...)}
}

func decodeReviewPolicy(content []byte) (string, error) {
	policy, err := policyartifact.DecodePolicy(content)
	if err != nil {
		return "", fmt.Errorf("sealedreview: decode review policy identity: %w", err)
	}
	return policy.ID, nil
}

func contextBinding(packet Packet) (ContextBinding, error) {
	binding := ContextBinding{
		Schema: ReviewBindingSchemaID, PacketDigest: packet.Digest, AcceptedSpecDigest: packet.Items[0].ContentDigest,
		ReviewPolicyDigest: packet.Items[4].ContentDigest, BuilderReceiptDigest: packet.BuilderReceiptDigest,
		HeadCommit: packet.Candidate.HeadCommit, HeadTree: packet.Candidate.HeadTree,
		ItemProjection: packetItemProjection(packet),
	}
	encoded, err := EncodeContextBinding(binding)
	if err != nil {
		return ContextBinding{}, err
	}
	return DecodeContextBinding(bytes.NewReader(encoded))
}

func packetItemProjection(packet Packet) []contextreceipt.ReviewInput {
	projection := make([]contextreceipt.ReviewInput, len(packet.Items))
	for i, item := range packet.Items {
		projection[i] = contextreceipt.ReviewInput{Kind: string(item.Kind), ContentDigest: item.ContentDigest}
	}
	sort.Slice(projection, func(i, j int) bool {
		if projection[i].Kind != projection[j].Kind {
			return projection[i].Kind < projection[j].Kind
		}
		return projection[i].ContentDigest < projection[j].ContentDigest
	})
	return projection
}

func validateContextCompilation(compilation ContextCompileResult, packet Packet, binding ContextBinding) error {
	bindingBytes, err := EncodeContextBinding(binding)
	if err != nil {
		return fmt.Errorf("sealedreview: encode expected context binding: %w", err)
	}
	wantProjection := []byte("<!-- verdi:review-binding " + base64.StdEncoding.EncodeToString(bindingBytes) + " -->\n")
	wantProjection = append(wantProjection, packet.Items[4].Content...)
	if !bytes.Equal(compilation.InstructionProjectionBytes, wantProjection) {
		return fmt.Errorf("sealedreview: context compiler returned a mismatched instruction projection")
	}
	if compilation.InstructionProjectionDigest != rawDigest(wantProjection) {
		return fmt.Errorf("sealedreview: context compiler returned a mismatched instruction projection digest")
	}

	manifest, err := contextcompile.DecodeManifest(compilation.ManifestBytes)
	if err != nil {
		return fmt.Errorf("sealedreview: context compiler returned an invalid manifest: %w", err)
	}
	if compilation.ManifestDigest != manifest.Digest {
		return fmt.Errorf("sealedreview: context compiler returned a mismatched manifest digest")
	}
	if manifest.Phase != contextcompile.PhaseReview {
		return fmt.Errorf("sealedreview: compiled manifest phase must be review")
	}
	if manifest.Repository.Head.Known != true || manifest.Repository.Head.Value != packet.Candidate.HeadCommit {
		return fmt.Errorf("sealedreview: compiled manifest repository head does not match packet candidate")
	}
	if manifest.AcceptedSpec.Ref != packet.Items[0].ID || manifest.AcceptedSpec.Commit != packet.Candidate.HeadCommit || manifest.AcceptedSpec.ContentDigest != packet.Items[0].ContentDigest {
		return fmt.Errorf("sealedreview: compiled manifest accepted spec does not match packet")
	}
	if manifest.Adapter.ID != string(packet.Reviewer.Adapter) || manifest.Adapter.Version != packet.Reviewer.AdapterVersion {
		return fmt.Errorf("sealedreview: compiled manifest adapter does not match packet reviewer")
	}
	if manifest.GovernanceProfile.ID != packet.Reviewer.ProfileID || manifest.GovernanceProfile.Digest != packet.Reviewer.ProfileDigest ||
		manifest.Policy.ProfileID != packet.Reviewer.ProfileID || manifest.Policy.ProfileDigest != packet.Reviewer.ProfileDigest {
		return fmt.Errorf("sealedreview: compiled manifest profile does not match packet reviewer")
	}
	wantProjectionDigest := rawDigest(wantProjection)
	for _, row := range manifest.ProjectionFiles {
		if row.Digest == wantProjectionDigest {
			return nil
		}
	}
	return fmt.Errorf("sealedreview: compiled manifest does not bind the instruction projection digest")
}

func reviewOperand(expected, observed string) contextreceipt.ReviewOperandProjection {
	state := contextreceipt.StateProven
	if expected != observed {
		state = contextreceipt.StateViolated
	}
	return contextreceipt.ReviewOperandProjection{State: state, ExpectedDigest: expected, ObservedDigest: observed}
}

func projectionDigest(value any) string {
	digest, err := canonjson.Digest(value)
	if err != nil {
		panic(fmt.Sprintf("sealedreview: canonical projection: %v", err))
	}
	return digest
}

func cloneEvidenceSummaries(rows []contextreceipt.Evidence) []contextreceipt.Evidence {
	if rows == nil {
		return nil
	}
	cloned := append([]contextreceipt.Evidence{}, rows...)
	for i := range cloned {
		cloned[i].Argv = append([]string(nil), cloned[i].Argv...)
	}
	return cloned
}

func cloneContextBinding(binding ContextBinding) ContextBinding {
	if binding.ItemProjection != nil {
		binding.ItemProjection = append([]contextreceipt.ReviewInput{}, binding.ItemProjection...)
	}
	return binding
}

func cloneContextCompilation(compilation ContextCompileResult) ContextCompileResult {
	compilation.ManifestBytes = append([]byte(nil), compilation.ManifestBytes...)
	compilation.InstructionProjectionBytes = append([]byte(nil), compilation.InstructionProjectionBytes...)
	compilation.Binding = cloneContextBinding(compilation.Binding)
	return compilation
}
