package policyartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
)

// hexDigest returns a deterministic, shape-valid sha256:<64 hex> digest for
// seed — test-fixture convenience only, not a claim about any real
// artifact's content: Task 1 disposition fixtures are self-contained, and
// cross-artifact digest agreement is Task 2's load-time concern.
func hexDigest(seed string) string {
	sum := sha256.Sum256([]byte(seed))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// testDerivedWitnessDigest computes the canonical digest of w's witness
// identity with InputID cleared — the same shape the real Task 7 runtime
// semantic-input digest takes (§8). It exists only so dispoWitness's
// shared fixture value can be computed rather than hand-typed; SI-114
// removed the decoder's own equivalent self-derivation, so nothing in
// this package still ties input_id to this computation — see the
// dedicated non-derivable-input_id case in TestDecodeDisposition_StrictUnion.
func testDerivedWitnessDigest(w SemanticWitness) string {
	w.InputID = ""
	id, err := canonjson.Digest(w)
	if err != nil {
		panic(fmt.Sprintf("testDerivedWitnessDigest: %v", err))
	}
	return id
}

var (
	dispoTargetDigest      = hexDigest("test-dispo-target")
	dispoClaimACDigest     = hexDigest("test-dispo-claim-ac")
	dispoClaimACAuthority  = hexDigest("test-dispo-claim-ac-authority")
	dispoClaimPolDigest    = hexDigest("test-dispo-claim-policy")
	dispoClaimPolAuthority = hexDigest("test-dispo-claim-policy-authority")
	dispoExemptionDigest   = hexDigest("test-dispo-exemption")
	dispoJudgmentPrimary   = hexDigest("test-dispo-judgment-primary")
	dispoJudgmentChallenge = hexDigest("test-dispo-judgment-challenger")
	dispoTemplateDigest    = hexDigest("test-dispo-template")

	// dispoWitness is the canonical two-claim, one-exemption semantic
	// witness every valid test document shares.
	dispoWitness = SemanticWitness{
		TargetDigest: dispoTargetDigest,
		Claims: []SemanticClaimWitness{
			{
				ID:              "ac-review-approval",
				Digest:          dispoClaimACDigest,
				Category:        "acceptance-criterion",
				AuthorityDigest: dispoClaimACAuthority,
				Scope:           Scope{Phases: []string{"review"}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
				Values:          []string{"approved"},
			},
			{
				ID:              "policy-review-instruction",
				Digest:          dispoClaimPolDigest,
				Category:        "policy-instruction",
				AuthorityDigest: dispoClaimPolAuthority,
				Scope:           Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
				Values:          []string{},
			},
		},
		Exemptions: []SemanticExemptionWitness{
			{ID: "policy-exemption/legacy-service-go", Digest: dispoExemptionDigest},
		},
	}
	// dispoInputID happens to be the digest testDerivedWitnessDigest
	// computes from dispoWitness, purely as a stable, non-hand-typed
	// fixture value — SI-114 (§8) means the decoder no longer requires or
	// checks this agreement; see the dedicated
	// "witness input_id need not derive from..." case below for a
	// well-formed input_id that is deliberately NOT derivable and still
	// decodes.
	dispoInputID = testDerivedWitnessDigest(dispoWitness)
)

// dispoWitnessBlock is the shared "witness:" frontmatter block every valid
// test document embeds unmodified.
func dispoWitnessBlock() string {
	return fmt.Sprintf(`witness:
  input_id: %q
  target_digest: %q
  claims:
    - id: ac-review-approval
      digest: %q
      category: acceptance-criterion
      authority_digest: %q
      scope: {phases: [review], environments: [], paths: [], refs: []}
      values: ["approved"]
    - id: policy-review-instruction
      digest: %q
      category: policy-instruction
      authority_digest: %q
      scope: {phases: [], environments: [], paths: [], refs: []}
      values: []
  exemptions:
    - id: policy-exemption/legacy-service-go
      digest: %q
`, dispoInputID, dispoTargetDigest, dispoClaimACDigest, dispoClaimACAuthority, dispoClaimPolDigest, dispoClaimPolAuthority, dispoExemptionDigest)
}

// dispoTemplateLine is validJudgeResultDispositionDoc's scaffold-provenance
// line, named so negative cases can replace or remove exactly it.
var dispoTemplateLine = fmt.Sprintf(`template: {identity: "embedded:policy-disposition.md", digest: %q}`, dispoTemplateDigest)

// dispoJudgmentBlock is validJudgeResultDispositionDoc's judgment
// provenance block, named so negative cases can replace or remove it.
var dispoJudgmentBlock = fmt.Sprintf("judgment:\n  primary_digest: %q\n", dispoJudgmentPrimary)

// dispoApprovalsBlock is validJudgeResultDispositionDoc's single approval
// entry, named so cases can replace exactly the approvals block.
var dispoApprovalsBlock = "approvals:\n  - role: policy-owner\n    principal: principal/github-org/YWxpY2U\n"

// dispoExemptionEntry is validJudgeResultDispositionDoc's single exemption
// witness entry, named so cases can replace exactly that entry.
var dispoExemptionEntry = fmt.Sprintf("    - id: policy-exemption/legacy-service-go\n      digest: %q\n", dispoExemptionDigest)

// dispoScopeAnchor is validJudgeResultDispositionDoc's TOP-LEVEL scope
// line plus the line that follows it: the claim witness for
// ac-review-approval carries a byte-identical (but indented) scope, so a
// case targeting the disposition's own scope must anchor on the
// unindented occurrence.
var dispoScopeAnchor = "scope: {phases: [review], environments: [], paths: [], refs: []}\nwitness:"

// dispoClaimScopeAnchor is the ac-review-approval claim witness's scope
// line, distinguished from the disposition's own by its indentation.
var dispoClaimScopeAnchor = "      scope: {phases: [review], environments: [], paths: [], refs: []}"

// validJudgeResultDispositionDoc returns a complete, valid judge-result,
// no-conflict disposition document: two sorted claim witnesses in
// distinct closed categories, one exemption witness, a judgment
// provenance citation, and no fallback-only bound.
func validJudgeResultDispositionDoc() string {
	return "---\n" +
		"schema: verdi.policy-disposition/v1\n" +
		"id: policy-disposition/review-no-conflict\n" +
		"kind: policy-disposition\n" +
		"title: \"Review-phase claims coexist without conflict\"\n" +
		"owners: [platform-team]\n" +
		"scope: {phases: [review], environments: [], paths: [], refs: []}\n" +
		dispoWitnessBlock() +
		"conclusion: no-conflict\n" +
		"origin: judge-result\n" +
		dispoJudgmentBlock +
		"approvals:\n" +
		"  - role: policy-owner\n" +
		"    principal: principal/github-org/YWxpY2U\n" +
		dispoTemplateLine + "\n" +
		"---\n" +
		"The primary judge found the two review-phase claims mutually\n" +
		"satisfiable; no conflict exists in the current semantic input.\n"
}

// validHumanFallbackDispositionDoc returns a complete, valid human-fallback
// disposition, with compensating controls and — depending on
// withReviewCondition — either a real calendar-date expiry or a nonblank
// review condition, exactly as §8 requires (one bound is sufficient).
func validHumanFallbackDispositionDoc(withReviewCondition bool) string {
	bound := `expiry: "2026-12-31"` + "\n"
	if withReviewCondition {
		bound = `review_condition: "Re-evaluate once the judge transport is configured."` + "\n"
	}
	return "---\n" +
		"schema: verdi.policy-disposition/v1\n" +
		"id: policy-disposition/review-no-conflict\n" +
		"kind: policy-disposition\n" +
		"title: \"Review-phase claims coexist without conflict\"\n" +
		"owners: [platform-team]\n" +
		"scope: {phases: [review], environments: [], paths: [], refs: []}\n" +
		dispoWitnessBlock() +
		"conclusion: conflict\n" +
		"origin: human-fallback\n" +
		"compensating_controls:\n" +
		"  - \"Weekly manual review of the overlapping claims.\"\n" +
		"approvals:\n" +
		"  - role: policy-owner\n" +
		"    principal: principal/github-org/YWxpY2U\n" +
		bound +
		dispoTemplateLine + "\n" +
		"---\n" +
		"No current judge transport is configured, so a human recorded this\n" +
		"ruling directly, bounded by a compensating control and a review\n" +
		"bound.\n"
}

func TestDecodeDisposition_StrictUnion(t *testing.T) {
	t.Run("happy: judge-result", func(t *testing.T) {
		d, err := DecodeDisposition([]byte(validJudgeResultDispositionDoc()))
		if err != nil {
			t.Fatalf("DecodeDisposition: %v", err)
		}
		if d.ID != "policy-disposition/review-no-conflict" {
			t.Fatalf("ID = %q", d.ID)
		}
		if d.Name() != "review-no-conflict" {
			t.Fatalf("Name() = %q", d.Name())
		}
		if d.Conclusion != DispositionNoConflict {
			t.Fatalf("Conclusion = %q", d.Conclusion)
		}
		if d.Origin != DispositionJudgeResult {
			t.Fatalf("Origin = %q", d.Origin)
		}
		if d.Judgment == nil || d.Judgment.PrimaryDigest != dispoJudgmentPrimary {
			t.Fatalf("Judgment = %+v", d.Judgment)
		}
		if len(d.Witness.Claims) != 2 {
			t.Fatalf("witness claims = %d, want 2", len(d.Witness.Claims))
		}
		if d.Witness.Claims[0].ID != "ac-review-approval" || d.Witness.Claims[1].ID != "policy-review-instruction" {
			t.Fatalf("claim order = %+v", d.Witness.Claims)
		}
		if len(d.Witness.Exemptions) != 1 {
			t.Fatalf("witness exemptions = %d, want 1", len(d.Witness.Exemptions))
		}
		if len(d.CompensatingControls) != 0 {
			t.Fatalf("CompensatingControls = %v, want none for a judge-result ruling", d.CompensatingControls)
		}
		if len(d.Approvals) != 1 {
			t.Fatalf("Approvals = %d, want 1", len(d.Approvals))
		}
		digest, err := d.Digest()
		if err != nil {
			t.Fatalf("Digest: %v", err)
		}
		if !strings.HasPrefix(digest, "sha256:") {
			t.Fatalf("digest %q", digest)
		}
	})

	t.Run("happy: human-fallback with expiry", func(t *testing.T) {
		d, err := DecodeDisposition([]byte(validHumanFallbackDispositionDoc(false)))
		if err != nil {
			t.Fatalf("DecodeDisposition: %v", err)
		}
		if d.Origin != DispositionHumanFallback {
			t.Fatalf("Origin = %q", d.Origin)
		}
		if d.Judgment != nil {
			t.Fatalf("Judgment = %+v, want nil for human-fallback", d.Judgment)
		}
		if len(d.CompensatingControls) != 1 {
			t.Fatalf("CompensatingControls = %v, want 1", d.CompensatingControls)
		}
		if d.Expiry != "2026-12-31" {
			t.Fatalf("Expiry = %q", d.Expiry)
		}
		if d.ReviewCondition != "" {
			t.Fatalf("ReviewCondition = %q, want empty", d.ReviewCondition)
		}
	})

	t.Run("happy: human-fallback with review-condition-only bound", func(t *testing.T) {
		d, err := DecodeDisposition([]byte(validHumanFallbackDispositionDoc(true)))
		if err != nil {
			t.Fatalf("DecodeDisposition: %v", err)
		}
		if d.Expiry != "" {
			t.Fatalf("Expiry = %q, want empty", d.Expiry)
		}
		if d.ReviewCondition == "" {
			t.Fatalf("ReviewCondition is empty, want the recorded condition")
		}
	})

	t.Run("happy: judge-result with optional controls accepted", func(t *testing.T) {
		doc := strings.Replace(validJudgeResultDispositionDoc(), "approvals:\n",
			"compensating_controls:\n  - \"Optional extra review, not required for judge-result.\"\napprovals:\n", 1)
		d, err := DecodeDisposition([]byte(doc))
		if err != nil {
			t.Fatalf("DecodeDisposition: %v", err)
		}
		if len(d.CompensatingControls) != 1 {
			t.Fatalf("CompensatingControls = %v, want 1", d.CompensatingControls)
		}
	})

	t.Run("happy: judge-result citing a challenger judgment", func(t *testing.T) {
		doc := strings.Replace(validJudgeResultDispositionDoc(), dispoJudgmentBlock,
			fmt.Sprintf("judgment:\n  primary_digest: %q\n  challenger_digest: %q\n", dispoJudgmentPrimary, dispoJudgmentChallenge), 1)
		d, err := DecodeDisposition([]byte(doc))
		if err != nil {
			t.Fatalf("DecodeDisposition: %v", err)
		}
		if d.Judgment == nil {
			t.Fatal("Judgment = nil, want the cited provenance")
		}
		if d.Judgment.PrimaryDigest != dispoJudgmentPrimary {
			t.Fatalf("PrimaryDigest = %q, want %q", d.Judgment.PrimaryDigest, dispoJudgmentPrimary)
		}
		if d.Judgment.ChallengerDigest != dispoJudgmentChallenge {
			t.Fatalf("ChallengerDigest = %q, want %q", d.Judgment.ChallengerDigest, dispoJudgmentChallenge)
		}
	})

	// SI-114 (§8): the decoder validates witness.input_id's digest form
	// only. It no longer self-derives a second semantic-input identity
	// from the artifact's own smaller claim/exemption/target projection
	// and compares the two — that agreement (against the current runtime
	// semantic input) is Task 8's (internal/policyconflict) concern alone.
	// A well-formed input_id that this witness's own content could never
	// have produced must still decode successfully.
	t.Run("happy: witness input_id need not derive from the artifact's own projection", func(t *testing.T) {
		runtimeInputID := hexDigest("runtime-semantic-input-not-derivable-from-witness")
		doc := strings.Replace(validJudgeResultDispositionDoc(),
			fmt.Sprintf("input_id: %q", dispoInputID),
			fmt.Sprintf("input_id: %q", runtimeInputID), 1)
		d, err := DecodeDisposition([]byte(doc))
		if err != nil {
			t.Fatalf("DecodeDisposition: %v", err)
		}
		if d.Witness.InputID != runtimeInputID {
			t.Fatalf("Witness.InputID = %q, want %q", d.Witness.InputID, runtimeInputID)
		}
	})

	// Approvals are a normalized SET, not authored order: a document may
	// list them any way and still decode to one canonical role-then-principal
	// sequence, so two stores recording the same approval facts share a
	// digest.
	t.Run("happy: approvals normalize to role-then-principal order", func(t *testing.T) {
		doc := strings.Replace(validJudgeResultDispositionDoc(), dispoApprovalsBlock,
			"approvals:\n"+
				"  - role: security-owner\n    principal: principal/github-org/YWxpY2U\n"+
				"  - role: policy-owner\n    principal: principal/github-org/Ym9i\n"+
				"  - role: policy-owner\n    principal: principal/github-org/YWxpY2U\n", 1)
		d, err := DecodeDisposition([]byte(doc))
		if err != nil {
			t.Fatalf("DecodeDisposition: %v", err)
		}
		want := []Approval{
			{Role: "policy-owner", Principal: "principal/github-org/YWxpY2U"},
			{Role: "policy-owner", Principal: "principal/github-org/Ym9i"},
			{Role: "security-owner", Principal: "principal/github-org/YWxpY2U"},
		}
		if len(d.Approvals) != len(want) {
			t.Fatalf("Approvals = %+v, want %d entries", d.Approvals, len(want))
		}
		for i := range want {
			if d.Approvals[i] != want[i] {
				t.Fatalf("Approvals[%d] = %+v, want %+v (full: %+v)", i, d.Approvals[i], want[i], d.Approvals)
			}
		}
	})

	// judgmentDoc/approvalsDoc/exemptionsDoc/witnessDoc each swap exactly one
	// block of the valid judge-result document, so a negative case names a
	// single defect and the rest of the artifact stays conforming.
	judgmentDoc := func(block string) string {
		return strings.Replace(validJudgeResultDispositionDoc(), dispoJudgmentBlock, block, 1)
	}
	approvalsDoc := func(block string) string {
		return strings.Replace(validJudgeResultDispositionDoc(), dispoApprovalsBlock, block, 1)
	}
	exemptionsDoc := func(entry string) string {
		return strings.Replace(validJudgeResultDispositionDoc(), dispoExemptionEntry, entry, 1)
	}
	witnessDoc := func(block string) string {
		return strings.Replace(validJudgeResultDispositionDoc(), dispoWitnessBlock(), block, 1)
	}
	// blankBody replaces everything after the closing frontmatter fence with
	// whitespace, leaving a syntactically valid artifact with no rationale.
	blankBody := func(doc string) string {
		fence := "\n---\n"
		i := strings.LastIndex(doc, fence)
		return doc[:i+len(fence)] + "   \n\n"
	}

	negatives := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{"unknown top-level field", strings.Replace(validJudgeResultDispositionDoc(), "conclusion: no-conflict\n", "conclusion: no-conflict\nraw_result: {x: 1}\n", 1), "raw_result"},
		{"unknown claim-witness field (claim text)", strings.Replace(validJudgeResultDispositionDoc(), `values: ["approved"]`, "values: [\"approved\"]\n      text: \"the claim's authored prose\"", 1), "text"},
		{"duplicate top-level key", strings.Replace(validJudgeResultDispositionDoc(), "conclusion: no-conflict\n", "conclusion: no-conflict\nconclusion: no-conflict\n", 1), "already defined"},
		{"explicit null on a mandatory key", strings.Replace(validJudgeResultDispositionDoc(), "conclusion: no-conflict\n", "conclusion: null\n", 1), "conclusion"},
		{"yaml anchor", strings.Replace(validJudgeResultDispositionDoc(), "owners: [platform-team]", "owners: &o [platform-team]", 1), "anchor"},
		{"yaml custom tag", strings.Replace(validJudgeResultDispositionDoc(), "owners: [platform-team]", "owners: !mytag [platform-team]", 1), "tag"},
		{"missing schema", strings.Replace(validJudgeResultDispositionDoc(), "schema: verdi.policy-disposition/v1\n", "", 1), "schema"},
		{"missing id", strings.Replace(validJudgeResultDispositionDoc(), "id: policy-disposition/review-no-conflict\n", "", 1), "id"},
		{"missing kind", strings.Replace(validJudgeResultDispositionDoc(), "kind: policy-disposition\n", "", 1), "kind"},
		{"missing title", strings.Replace(validJudgeResultDispositionDoc(), "title: \"Review-phase claims coexist without conflict\"\n", "", 1), "title"},
		{"missing owners", strings.Replace(validJudgeResultDispositionDoc(), "owners: [platform-team]\n", "", 1), "owners"},
		{"missing scope", strings.Replace(validJudgeResultDispositionDoc(), "scope: {phases: [review], environments: [], paths: [], refs: []}\nwitness:", "witness:", 1), "scope"},
		{"missing conclusion", strings.Replace(validJudgeResultDispositionDoc(), "conclusion: no-conflict\n", "", 1), "conclusion"},
		{"missing origin", strings.Replace(validJudgeResultDispositionDoc(), "origin: judge-result\n", "", 1), "origin"},
		{"missing approvals", strings.Replace(validJudgeResultDispositionDoc(), "approvals:\n  - role: policy-owner\n    principal: principal/github-org/YWxpY2U\n", "", 1), "approval"},
		{"missing template", strings.Replace(validJudgeResultDispositionDoc(), dispoTemplateLine+"\n", "", 1), "template"},
		{"missing witness.input_id", strings.Replace(validJudgeResultDispositionDoc(), fmt.Sprintf("  input_id: %q\n", dispoInputID), "", 1), "input_id"},
		{"missing witness.target_digest", strings.Replace(validJudgeResultDispositionDoc(), fmt.Sprintf("  target_digest: %q\n", dispoTargetDigest), "", 1), "target_digest"},
		{"missing witness.claims key", strings.Replace(validJudgeResultDispositionDoc(), dispoWitnessBlock(), "witness:\n  input_id: \"x\"\n  target_digest: \"x\"\n  exemptions: []\n", 1), "claims"},
		{"missing witness.exemptions key", strings.Replace(validJudgeResultDispositionDoc(), dispoWitnessBlock(), fmt.Sprintf("witness:\n  input_id: %q\n  target_digest: %q\n  claims:\n    - id: ac-review-approval\n      digest: %q\n      category: acceptance-criterion\n      authority_digest: %q\n      scope: {phases: [review], environments: [], paths: [], refs: []}\n      values: [\"approved\"]\n", dispoInputID, dispoTargetDigest, dispoClaimACDigest, dispoClaimACAuthority), 1), "exemptions"},
		{"claim witness missing digest", strings.Replace(validJudgeResultDispositionDoc(), fmt.Sprintf("digest: %q\n      category: acceptance-criterion", dispoClaimACDigest), "category: acceptance-criterion", 1), "required"},
		{"unknown schema", strings.Replace(validJudgeResultDispositionDoc(), "verdi.policy-disposition/v1", "verdi.policy-disposition/v2", 1), "schema"},
		{"kind mismatch", strings.Replace(validJudgeResultDispositionDoc(), "kind: policy-disposition", "kind: policy-exemption", 1), "kind"},
		{"unknown conclusion", strings.Replace(validJudgeResultDispositionDoc(), "conclusion: no-conflict", "conclusion: maybe-conflict", 1), "conclusion"},
		{"unknown origin", strings.Replace(validJudgeResultDispositionDoc(), "origin: judge-result", "origin: coin-flip", 1), "origin"},
		{"unknown witness category", strings.Replace(validJudgeResultDispositionDoc(), "category: acceptance-criterion", "category: vibe-check", 1), "category"},
		{"bad target_digest form", strings.Replace(validJudgeResultDispositionDoc(), fmt.Sprintf("target_digest: %q", dispoTargetDigest), `target_digest: "nothex"`, 1), "target_digest"},
		{"bad claim digest form", strings.Replace(validJudgeResultDispositionDoc(), fmt.Sprintf("digest: %q\n      category: acceptance-criterion", dispoClaimACDigest), `digest: "nothex"
      category: acceptance-criterion`, 1), "digest"},
		{"bad exemption id form", strings.Replace(validJudgeResultDispositionDoc(), "id: policy-exemption/legacy-service-go", "id: legacy-service-go", 1), "form"},
		{"malformed witness.input_id", strings.Replace(validJudgeResultDispositionDoc(), fmt.Sprintf("input_id: %q", dispoInputID), `input_id: "nothex"`, 1), "input_id"},
		{"invalid expiry date", strings.Replace(validHumanFallbackDispositionDoc(false), `expiry: "2026-12-31"`, `expiry: "2026-02-31"`, 1), "calendar date"},
		{"blank compensating control", strings.Replace(validHumanFallbackDispositionDoc(false), `"Weekly manual review of the overlapping claims."`, `""`, 1), "empty control"},
		{"multiline compensating control", strings.Replace(validHumanFallbackDispositionDoc(false), `"Weekly manual review of the overlapping claims."`, "\"line one\\nline two\"", 1), "single line"},
		{"judgment on human-fallback", strings.Replace(validHumanFallbackDispositionDoc(false), "origin: human-fallback\n", "origin: human-fallback\n"+dispoJudgmentBlock, 1), "judgment"},
		{"human-fallback missing controls", strings.Replace(validHumanFallbackDispositionDoc(false), "compensating_controls:\n  - \"Weekly manual review of the overlapping claims.\"\n", "", 1), "compensating control"},
		{"human-fallback missing both bounds", strings.Replace(validHumanFallbackDispositionDoc(false), `expiry: "2026-12-31"`+"\n", "", 1), "expiry or a review condition"},

		{"no frontmatter delimiter", strings.TrimPrefix(validJudgeResultDispositionDoc(), "---\n"), "frontmatter delimiter"},

		// Disposition-level scope: both the presence grammar (toScope) and
		// the member grammar (Scope.Validate) apply to a disposition exactly
		// as they do to every other kernel artifact.
		{"missing witness", strings.Replace(validJudgeResultDispositionDoc(), dispoWitnessBlock(), "", 1), "field witness is missing"},
		{"scope dimension missing", strings.Replace(validJudgeResultDispositionDoc(), dispoScopeAnchor, "scope: {phases: [review], environments: [], paths: []}\nwitness:", 1), "disposition.scope.refs is missing"},
		{"scope with an unknown phase", strings.Replace(validJudgeResultDispositionDoc(), dispoScopeAnchor, "scope: {phases: [deploy], environments: [], paths: [], refs: []}\nwitness:", 1), "unknown phase"},

		// Judgment provenance: a present citation must name real judgment
		// records. challenger_digest is optional by ABSENCE only — an empty
		// or malformed value is a citation wearing the shape of provenance.
		{"judgment missing primary_digest", judgmentDoc(fmt.Sprintf("judgment:\n  challenger_digest: %q\n", dispoJudgmentChallenge)), "primary_digest is missing"},
		{"malformed primary_digest", judgmentDoc("judgment:\n  primary_digest: \"sha256:nothex\"\n"), `primary_digest "sha256:nothex" is not sha256`},
		{"malformed challenger_digest", judgmentDoc(fmt.Sprintf("judgment:\n  primary_digest: %q\n  challenger_digest: \"sha256:nothex\"\n", dispoJudgmentPrimary)), `challenger_digest "sha256:nothex" is not sha256`},
		{"empty challenger_digest", judgmentDoc(fmt.Sprintf("judgment:\n  primary_digest: %q\n  challenger_digest: \"\"\n", dispoJudgmentPrimary)), `challenger_digest "" is not sha256`},

		// Compensating controls are optional by ABSENCE only (§8: "when
		// present, remain nonempty").
		{"compensating_controls present but empty", strings.Replace(validJudgeResultDispositionDoc(), "approvals:\n", "compensating_controls: []\napprovals:\n", 1), "present but empty"},
		{"blank review_condition", strings.Replace(validHumanFallbackDispositionDoc(true), `review_condition: "Re-evaluate once the judge transport is configured."`, `review_condition: "  "`, 1), "not blank text"},

		// Approval facts: at least one, each complete, well-formed, and
		// distinct.
		{"approvals explicitly empty", approvalsDoc("approvals: []\n"), "at least one approval"},
		{"approval missing role", approvalsDoc("approvals:\n  - principal: principal/github-org/YWxpY2U\n"), "role and principal are both required"},
		{"approval missing principal", approvalsDoc("approvals:\n  - role: policy-owner\n"), "role and principal are both required"},
		{"non-kebab approval role", approvalsDoc("approvals:\n  - role: Policy_Owner\n    principal: principal/github-org/YWxpY2U\n"), "must be kebab-case"},
		{"invalid approval principal", approvalsDoc("approvals:\n  - role: policy-owner\n    principal: alice\n"), "want principal/<trust-source-id>/<base64url-subject>"},
		{"duplicate approval pair", approvalsDoc("approvals:\n  - role: policy-owner\n    principal: principal/github-org/YWxpY2U\n  - role: policy-owner\n    principal: principal/github-org/YWxpY2U\n"), "duplicate approval"},

		{"blank rationale body", blankBody(validJudgeResultDispositionDoc()), "rationale"},

		// Semantic witness: a witness always names claims, and every claim's
		// identity field carries its own grammar.
		{"witness claims explicitly empty", witnessDoc(fmt.Sprintf("witness:\n  input_id: %q\n  target_digest: %q\n  claims: []\n  exemptions: []\n", hexDigest("placeholder-input-id"), dispoTargetDigest)), "at least one claim"},
		{"blank claim id", strings.Replace(validJudgeResultDispositionDoc(), "- id: ac-review-approval", `- id: "   "`, 1), "must not be blank"},
		{"claim id with a control character", strings.Replace(validJudgeResultDispositionDoc(), "- id: ac-review-approval", `- id: "ac\tone"`, 1), "control character"},
		{"malformed claim authority_digest", strings.Replace(validJudgeResultDispositionDoc(), fmt.Sprintf("authority_digest: %q", dispoClaimACAuthority), `authority_digest: "nothex"`, 1), `authority_digest "nothex" is not sha256`},
		{"claim scope dimension missing", strings.Replace(validJudgeResultDispositionDoc(), dispoClaimScopeAnchor, "      scope: {phases: [review], environments: [], paths: []}", 1), "witness.claims[0].scope.refs is missing"},
		{"claim scope with an unknown phase", strings.Replace(validJudgeResultDispositionDoc(), dispoClaimScopeAnchor, "      scope: {phases: [deploy], environments: [], paths: [], refs: []}", 1), "unknown phase"},
		{"empty claim value member", strings.Replace(validJudgeResultDispositionDoc(), `values: ["approved"]`, `values: [""]`, 1), "empty value"},

		// Exemption witnesses carry an id/digest pair or nothing at all.
		{"exemption witness missing id", exemptionsDoc(fmt.Sprintf("    - digest: %q\n", dispoExemptionDigest)), "id and digest are both required"},
		{"exemption witness missing digest", exemptionsDoc("    - id: policy-exemption/legacy-service-go\n"), "id and digest are both required"},
		{"malformed exemption witness digest", exemptionsDoc("    - id: policy-exemption/legacy-service-go\n      digest: \"nothex\"\n"), `digest "nothex" is not sha256`},
	}
	for _, tt := range negatives {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeDisposition([]byte(tt.doc))
			if err == nil {
				t.Fatalf("DecodeDisposition = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("DecodeDisposition error = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}

// TestDecodeDisposition_WitnessOrdering proves the fail-closed ordering
// discipline: claims and exemptions must arrive strictly sorted by id with
// no duplicates, or decode rejects the document outright rather than
// silently reordering it.
func TestDecodeDisposition_WitnessOrdering(t *testing.T) {
	witnessBlock := func(claims, exemptions string) string {
		return "witness:\n" +
			fmt.Sprintf("  input_id: %q\n", hexDigest("placeholder-input-id")) +
			fmt.Sprintf("  target_digest: %q\n", dispoTargetDigest) +
			"  claims:\n" + claims +
			"  exemptions:\n" + exemptions
	}
	claimEntry := func(id, digest, category, authority string) string {
		return fmt.Sprintf("    - id: %s\n      digest: %q\n      category: %s\n      authority_digest: %q\n      scope: {phases: [], environments: [], paths: [], refs: []}\n      values: []\n",
			id, digest, category, authority)
	}
	exemptionEntry := func(id, digest string) string {
		return fmt.Sprintf("    - id: %s\n      digest: %q\n", id, digest)
	}
	baseExemptions := exemptionEntry("policy-exemption/legacy-service-go", dispoExemptionDigest)

	doc := func(claims, exemptions string) string {
		return strings.Replace(validJudgeResultDispositionDoc(), dispoWitnessBlock(), witnessBlock(claims, exemptions), 1)
	}

	tests := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{
			"unsorted claims",
			doc(claimEntry("policy-review-instruction", dispoClaimPolDigest, "policy-instruction", dispoClaimPolAuthority)+
				claimEntry("ac-review-approval", dispoClaimACDigest, "acceptance-criterion", dispoClaimACAuthority), baseExemptions),
			"sorted",
		},
		{
			"duplicate claim ids",
			doc(claimEntry("ac-review-approval", dispoClaimACDigest, "acceptance-criterion", dispoClaimACAuthority)+
				claimEntry("ac-review-approval", dispoClaimPolDigest, "policy-instruction", dispoClaimPolAuthority), baseExemptions),
			"duplicates",
		},
		{
			"unsorted exemptions",
			doc(claimEntry("ac-review-approval", dispoClaimACDigest, "acceptance-criterion", dispoClaimACAuthority)+
				claimEntry("policy-review-instruction", dispoClaimPolDigest, "policy-instruction", dispoClaimPolAuthority),
				exemptionEntry("policy-exemption/zzz-late", hexDigest("zzz"))+exemptionEntry("policy-exemption/aaa-early", hexDigest("aaa"))),
			"sorted",
		},
		{
			"duplicate exemption witnesses",
			doc(claimEntry("ac-review-approval", dispoClaimACDigest, "acceptance-criterion", dispoClaimACAuthority)+
				claimEntry("policy-review-instruction", dispoClaimPolDigest, "policy-instruction", dispoClaimPolAuthority),
				exemptionEntry("policy-exemption/legacy-service-go", dispoExemptionDigest)+exemptionEntry("policy-exemption/legacy-service-go", dispoExemptionDigest)),
			"duplicates",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeDisposition([]byte(tt.doc))
			if err == nil {
				t.Fatalf("DecodeDisposition = nil error, want error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("DecodeDisposition error = %v, want containing %q", err, tt.wantSub)
			}
		})
	}
}

// TestDispositionDigest_SealedAndMutationSafe proves the disposition seal:
// a hand-built value never yields a digest, an untouched decode round-trips
// a stable digest, and a post-decode mutation of any region — kernel field,
// scope, a witness claim, conclusion, or approvals/controls — makes
// Digest() fail.
func TestDispositionDigest_SealedAndMutationSafe(t *testing.T) {
	t.Run("hand-built value never yields a digest", func(t *testing.T) {
		var forged Disposition
		forged.ID = "policy-disposition/forged"
		if _, err := forged.Digest(); err == nil {
			t.Fatal("hand-built disposition yielded a digest")
		}
	})

	t.Run("untouched decode round-trips a stable digest", func(t *testing.T) {
		d, err := DecodeDisposition([]byte(validJudgeResultDispositionDoc()))
		if err != nil {
			t.Fatalf("DecodeDisposition: %v", err)
		}
		d1, err := d.Digest()
		if err != nil {
			t.Fatalf("Digest (1st): %v", err)
		}
		d2, err := d.Digest()
		if err != nil {
			t.Fatalf("Digest (2nd): %v", err)
		}
		if d1 != d2 {
			t.Fatalf("digest not stable across calls: %s vs %s", d1, d2)
		}
	})

	mutations := []struct {
		name   string
		mutate func(*Disposition)
	}{
		{"kernel field (title)", func(d *Disposition) { d.Title = "tampered" }},
		{"scope", func(d *Disposition) { d.Scope.Phases = append(d.Scope.Phases, "build") }},
		{"witness claim", func(d *Disposition) { d.Witness.Claims[0].Category = "constraint" }},
		{"conclusion", func(d *Disposition) { d.Conclusion = DispositionConflict }},
		{"approvals", func(d *Disposition) { d.Approvals[0].Role = "tampered-role" }},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			d, err := DecodeDisposition([]byte(validJudgeResultDispositionDoc()))
			if err != nil {
				t.Fatalf("DecodeDisposition: %v", err)
			}
			tt.mutate(d)
			if _, err := d.Digest(); err == nil || !strings.Contains(err.Error(), "modified") {
				t.Fatalf("mutated (%s) Digest err = %v, want modification error", tt.name, err)
			}
		})
	}

	t.Run("compensating controls (human-fallback)", func(t *testing.T) {
		d, err := DecodeDisposition([]byte(validHumanFallbackDispositionDoc(false)))
		if err != nil {
			t.Fatalf("DecodeDisposition: %v", err)
		}
		d.CompensatingControls[0] = "tampered control"
		if _, err := d.Digest(); err == nil || !strings.Contains(err.Error(), "modified") {
			t.Fatalf("mutated controls Digest err = %v, want modification error", err)
		}
	})
}

// --- exported vocabulary validation (Wave-3 policy-conflict-gate
// authority design §6/§8; a sibling package embeds SemanticClaimWitness
// and DispositionConclusion directly and needs to validate them without
// duplicating this package's closed vocabularies) -------------------------

func TestValidateWitnessCategory(t *testing.T) {
	valid := []string{
		"policy-instruction", "spec-problem", "spec-outcome",
		"acceptance-criterion", "open-question", "constraint", "decision",
		"adr-decision", "obligation-declaration",
	}
	for _, c := range valid {
		t.Run("valid/"+c, func(t *testing.T) {
			if err := ValidateWitnessCategory(c); err != nil {
				t.Fatalf("ValidateWitnessCategory(%q): %v", c, err)
			}
		})
	}

	invalid := []string{"", "spec-instruction", "POLICY-INSTRUCTION", "policy_instruction", "unknown-category", "Constraint"}
	for _, c := range invalid {
		t.Run("invalid/"+c, func(t *testing.T) {
			if err := ValidateWitnessCategory(c); err == nil {
				t.Fatalf("ValidateWitnessCategory(%q) = nil, want error", c)
			}
		})
	}
}

// validSemanticClaimWitness returns a fully valid, self-contained witness
// for TestSemanticClaimWitnessValidate's positive and mutation cases.
func validSemanticClaimWitness() SemanticClaimWitness {
	return SemanticClaimWitness{
		ID:              "ac-review-approval",
		Digest:          hexDigest("witness-validate-claim"),
		Category:        "acceptance-criterion",
		AuthorityDigest: hexDigest("witness-validate-authority"),
		Scope:           Scope{Phases: []string{"review"}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Values:          []string{"approved"},
	}
}

func TestSemanticClaimWitnessValidate(t *testing.T) {
	t.Run("valid witness", func(t *testing.T) {
		w := validSemanticClaimWitness()
		if err := w.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})
	t.Run("valid witness with bound and no values", func(t *testing.T) {
		w := validSemanticClaimWitness()
		bound := 3
		w.Values = []string{}
		w.Bound = &bound
		if err := w.Validate(); err != nil {
			t.Fatalf("Validate: %v", err)
		}
	})

	cases := []struct {
		name   string
		mutate func(*SemanticClaimWitness)
	}{
		{"blank id", func(w *SemanticClaimWitness) { w.ID = "" }},
		{"whitespace-only id", func(w *SemanticClaimWitness) { w.ID = "   " }},
		{"multiline id", func(w *SemanticClaimWitness) { w.ID = "line-one\nline-two" }},
		{"control character in id", func(w *SemanticClaimWitness) { w.ID = "ac-review\x00approval" }},
		{"malformed digest", func(w *SemanticClaimWitness) { w.Digest = "not-a-digest" }},
		{"digest missing sha256 prefix", func(w *SemanticClaimWitness) { w.Digest = strings.TrimPrefix(w.Digest, "sha256:") }},
		{"unknown category", func(w *SemanticClaimWitness) { w.Category = "bogus-category" }},
		{"malformed authority digest", func(w *SemanticClaimWitness) { w.AuthorityDigest = "not-a-digest" }},
		{"invalid scope (missing dimension)", func(w *SemanticClaimWitness) { w.Scope = Scope{} }},
		{"invalid scope (unknown phase)", func(w *SemanticClaimWitness) { w.Scope.Phases = []string{"bogus-phase"} }},
		{"empty value entry", func(w *SemanticClaimWitness) { w.Values = []string{""} }},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			w := validSemanticClaimWitness()
			tt.mutate(&w)
			if err := w.Validate(); err == nil {
				t.Fatalf("Validate() = nil, want error (%s)", tt.name)
			}
		})
	}
}

func TestDispositionConclusionValidate(t *testing.T) {
	valid := []DispositionConclusion{DispositionConflict, DispositionNoConflict}
	for _, c := range valid {
		t.Run("valid/"+string(c), func(t *testing.T) {
			if err := c.Validate(); err != nil {
				t.Fatalf("Validate(%q): %v", c, err)
			}
		})
	}

	invalid := []DispositionConclusion{"", "CONFLICT", "Conflict", "conflict ", "no_conflict", "bogus"}
	for _, c := range invalid {
		t.Run("invalid/"+string(c), func(t *testing.T) {
			if err := c.Validate(); err == nil {
				t.Fatalf("Validate(%q) = nil, want error", c)
			}
		})
	}
}

// TestDecodeDisposition_WitnessCategoryErrorPrefixOnce proves the delegated
// per-claim witness validation reports its package prefix exactly once. The
// decode path already labels its own error "policyartifact: disposition
// witness claims[i]: ..."; the delegated category check must not re-announce
// the package inside that label.
func TestDecodeDisposition_WitnessCategoryErrorPrefixOnce(t *testing.T) {
	doc := strings.Replace(validJudgeResultDispositionDoc(), "category: acceptance-criterion", "category: vibe-check", 1)
	_, err := DecodeDisposition([]byte(doc))
	if err == nil {
		t.Fatalf("DecodeDisposition(unknown witness category) = nil error, want failure")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unknown witness category") {
		t.Fatalf("error %q does not name the unknown witness category", msg)
	}
	if got := strings.Count(msg, "policyartifact:"); got != 1 {
		t.Fatalf("error %q carries the %q prefix %d times, want exactly 1", msg, "policyartifact:", got)
	}
}
