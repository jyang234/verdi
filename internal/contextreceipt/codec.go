package contextreceipt

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
