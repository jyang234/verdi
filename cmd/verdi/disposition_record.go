// verdi disposition record --report PATH --row INPUT_ID --target-digest
// DIGEST --conclusion no-conflict|conflict --compensating-control TEXT
// [--compensating-control TEXT ...] --expiry DATE --approver
// ROLE=PRINCIPAL_ID [--approver ROLE=PRINCIPAL_ID ...] --id NAME --title
// TEXT --owner TEXT [--owner TEXT ...] [--root DIR]
// (Task 3, docs/superpowers/specs/2026-09-05-local-operator-disposition-
// design.md §2.3; ledger SI-178):
//
// Records a human's ruling over a kernel-printed policy-conflict witness as
// one `policy-disposition` artifact. It selects the semantic row named by
// --row (matched against the report's own SemanticEvaluation.InputID) from
// a `verdi.policy-conflict-report/v1` document strict-decoded through the
// existing internal/policyconflict codec, copies that row's input_id and
// claim identities into the new artifact's witness VERBATIM (byte-for-byte
// SemanticClaimWitness values, in the row's own order), reconstructs the
// witness's applicable-exemption set from the SAME report's mechanical
// rows (see reconstructApplicableExemptions's own doc comment for why this
// is the report's sole recorded source), derives origin from whether the
// row carries a primary judge exchange, resolves the disposition-family
// scaffold's template identity/digest through humanartifact.ResolveScaffold,
// and fills conclusion, compensating controls, approvals, and expiry from
// explicit operands. It evaluates nothing — no mechanical, semantic, or
// authority proof ever runs here — and never opens the target
// specification file: every value not already present in --report comes
// only from an operand.
//
// --target-digest is this file's one disclosed deviation from the literal
// task brief, which lists the operand set without it. The witness's
// target_digest (authority-design §8: "both accepted context and an
// acceptance candidate supply the target specification artifact's exact
// content digest... The accepted manifest digest remains separately
// bound... using it inside the disposition would recurse") is NOT, and
// structurally cannot be, present anywhere in a persisted
// verdi.policy-conflict-report/v1 document: SemanticEvaluation carries
// only {id, input_id, claims, unknown_mechanicals, primary, challenger,
// dispositions, state, reasons} (docs/superpowers/specs/2026-08-12-policy-
// conflict-gate-authority-design.md §10, unchanged here), the accepted
// target identity carries only the (explicitly wrong-for-this-purpose)
// manifest digest, and AuthorityInput.TargetDigest — the actual value
// policyconflict.authority.go's match rule compares against — is computed
// only in memory during evaluation and never serialized. Recomputing it
// independently would mean reading the target spec file, which the
// contract forbids outright, or re-running context-compile evaluation,
// which "it evaluates nothing" forbids in spirit. The smallest reversible
// fix is one more operand naming the value directly, cross-checked against
// the row's own claims (refused, naming --target-digest, as "an operand
// that would make the witness differ from the report" when the row carries
// claims but none share the given digest) rather than accepted blind. This
// is flagged for controller/owner review, not silently decided as settled
// design.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/humanartifact"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/store"
)

const dispositionRecordUsage = "disposition record: usage: verdi disposition record --report PATH --row INPUT_ID --target-digest DIGEST --conclusion no-conflict|conflict --compensating-control TEXT [--compensating-control TEXT ...] --expiry DATE --approver ROLE=PRINCIPAL_ID [--approver ROLE=PRINCIPAL_ID ...] --id NAME --title TEXT --owner TEXT [--owner TEXT ...] [--root DIR]"

// dispositionRecordArgs is parseDispositionRecordArgs's complete, validated
// operand set.
type dispositionRecordArgs struct {
	report               string
	row                  string
	targetDigest         string
	conclusion           string
	compensatingControls []string
	expiry               string
	approvers            []string // raw "ROLE=PRINCIPAL_ID" tokens, parsed later
	id                   string
	title                string
	owners               []string
	root                 string
	hasRoot              bool
}

// cmdDispositionRecord is `verdi disposition record`'s entry point,
// dispatched from cmdDisposition (disposition.go) on the literal
// subcommand "record" — the same first-argument routing cmdContext
// (context.go) already established for its own "compile"/"conflict"/...
// subcommands.
func cmdDispositionRecord(args []string, stdout, stderr io.Writer) int {
	parsed, err := parseDispositionRecordArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "disposition record:", err)
		return 2
	}

	root := "."
	if parsed.hasRoot {
		root = parsed.root
	}
	resolvedRoot, err := store.FindRoot(root)
	if err != nil {
		fmt.Fprintln(stderr, "disposition record:", err)
		return 2
	}

	return runDispositionRecord(resolvedRoot, parsed, stdout, stderr)
}

// parseDispositionRecordArgs hand-parses args (mirroring this package's
// established loop-based style, e.g. disposition.go's
// parseDispositionArgs) rather than the stdlib flag package, since several
// operands repeat. Every failure here is a usage/argument-shape problem —
// operational, never touching any file — and is returned before any
// report is read.
func parseDispositionRecordArgs(args []string) (dispositionRecordArgs, error) {
	var a dispositionRecordArgs
	var hasReport, hasRow, hasTargetDigest, hasConclusion, hasExpiry, hasID, hasTitle bool

	next := func(i int) (int, string, error) {
		if i+1 >= len(args) {
			return i, "", fmt.Errorf("%s requires a value", args[i])
		}
		return i + 1, args[i+1], nil
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--report":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.report, hasReport = v, true
		case "--row":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.row, hasRow = v, true
		case "--target-digest":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.targetDigest, hasTargetDigest = v, true
		case "--conclusion":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.conclusion, hasConclusion = v, true
		case "--compensating-control":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.compensatingControls = append(a.compensatingControls, v)
		case "--expiry":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.expiry, hasExpiry = v, true
		case "--approver":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.approvers = append(a.approvers, v)
		case "--id":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.id, hasID = v, true
		case "--title":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.title, hasTitle = v, true
		case "--owner":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.owners = append(a.owners, v)
		case "--root":
			var v string
			var err error
			i, v, err = next(i)
			if err != nil {
				return dispositionRecordArgs{}, err
			}
			a.root, a.hasRoot = v, true
		default:
			if len(args[i]) >= 2 && args[i][0:2] == "--" {
				return dispositionRecordArgs{}, fmt.Errorf("unrecognized flag %q\n%s", args[i], dispositionRecordUsage)
			}
			return dispositionRecordArgs{}, fmt.Errorf("unexpected positional argument %q\n%s", args[i], dispositionRecordUsage)
		}
	}

	missing := func(flag string) error {
		return fmt.Errorf("%s is required\n%s", flag, dispositionRecordUsage)
	}
	switch {
	case !hasReport:
		return dispositionRecordArgs{}, missing("--report")
	case !hasRow:
		return dispositionRecordArgs{}, missing("--row")
	case !hasTargetDigest:
		return dispositionRecordArgs{}, missing("--target-digest")
	case !hasConclusion:
		return dispositionRecordArgs{}, missing("--conclusion")
	case len(a.compensatingControls) == 0:
		return dispositionRecordArgs{}, fmt.Errorf("at least one --compensating-control is required\n%s", dispositionRecordUsage)
	case !hasExpiry:
		return dispositionRecordArgs{}, missing("--expiry")
	case len(a.approvers) == 0:
		return dispositionRecordArgs{}, fmt.Errorf("at least one --approver is required\n%s", dispositionRecordUsage)
	case !hasID:
		return dispositionRecordArgs{}, missing("--id")
	case !hasTitle:
		return dispositionRecordArgs{}, missing("--title")
	case len(a.owners) == 0:
		return dispositionRecordArgs{}, fmt.Errorf("at least one --owner is required\n%s", dispositionRecordUsage)
	}
	return a, nil
}

// runDispositionRecord is the testable core: given an already-resolved
// store root and validated operands, read+decode the report, select the
// row, build the witness, render, and write.
func runDispositionRecord(root string, a dispositionRecordArgs, stdout, stderr io.Writer) int {
	data, err := os.ReadFile(a.report)
	if err != nil {
		fmt.Fprintf(stderr, "disposition record: --report: reading %s: %v\n", a.report, err)
		return 2
	}
	report, err := policyconflict.DecodeReport(data)
	if err != nil {
		fmt.Fprintf(stderr, "disposition record: --report: %s failed strict decode: %v\n", a.report, err)
		return 2
	}

	row, matches := findSemanticRow(report, a.row)
	if matches == 0 {
		fmt.Fprintf(stderr, "disposition record: --row: no semantic row with input_id %q in %s\n", a.row, a.report)
		return 2
	}
	if matches > 1 {
		fmt.Fprintf(stderr, "disposition record: --row: %d semantic rows share input_id %q in %s; the report is internally ambiguous, refusing rather than picking one\n", matches, a.row, a.report)
		return 2
	}

	if !validDispositionID(a.id) {
		fmt.Fprintf(stderr, "disposition record: --id: %q must be a single kebab-case path component (lowercase letters, digits, and single internal hyphens; no \"/\", \".\", or \"..\")\n", a.id)
		return 2
	}

	targetRef, haveTargetRef := targetRefFromReport(report)
	targetDigests := targetClaimAuthorityDigests(row.Claims, targetRef, haveTargetRef)
	switch {
	case len(targetDigests) == 0:
		fmt.Fprintf(stderr, "disposition record: --target-digest: the selected row's claims include none attributable to the target specification itself; the digest cannot be cross-checked for this report\n")
		return 2
	case len(targetDigests) > 1:
		fmt.Fprintf(stderr, "disposition record: --target-digest: the selected row's claims are ambiguous between the target specification and a governing parent (%d distinct authority digests); the digest cannot be safely cross-checked for this report\n", len(targetDigests))
		return 2
	case targetDigests[0] != a.targetDigest:
		fmt.Fprintf(stderr, "disposition record: --target-digest: %q does not match the target specification's own claim authority digest %q; an operand must never make the witness differ from the report\n", a.targetDigest, targetDigests[0])
		return 2
	}

	conclusion := policyartifact.DispositionConclusion(a.conclusion)
	if err := conclusion.Validate(); err != nil {
		fmt.Fprintf(stderr, "disposition record: --conclusion: %v\n", err)
		return 2
	}

	if _, err := time.Parse("2006-01-02", a.expiry); err != nil {
		fmt.Fprintf(stderr, "disposition record: --expiry: %q is not a real YYYY-MM-DD calendar date: %v\n", a.expiry, err)
		return 2
	}

	approvals := make([]humanartifact.DispositionApprovalData, 0, len(a.approvers))
	for _, raw := range a.approvers {
		role, principal, err := parseApprover(raw)
		if err != nil {
			fmt.Fprintf(stderr, "disposition record: --approver %q: %v\n", raw, err)
			return 2
		}
		approvals = append(approvals, humanartifact.DispositionApprovalData{Role: role, Principal: principal})
	}

	exemptions, err := reconstructApplicableExemptions(report.Mechanical)
	if err != nil {
		fmt.Fprintf(stderr, "disposition record: --report: reconstructing the applicable-exemption set from the report's mechanical rows: %v\n", err)
		return 2
	}

	destPath := store.PolicyDispositionPath(root, a.id)
	if _, err := os.Stat(destPath); err == nil {
		fmt.Fprintf(stderr, "disposition record: --id: an artifact already exists at %s; this verb never overwrites\n", destPath)
		return 2
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "disposition record: --id: checking %s: %v\n", destPath, err)
		return 2
	}

	scaffold, err := humanartifact.ResolveScaffold(root, "policy-disposition.md")
	if err != nil {
		fmt.Fprintln(stderr, "disposition record:", err)
		return 2
	}

	claims := make([]humanartifact.DispositionClaimData, len(row.Claims))
	for i, c := range row.Claims {
		claims[i] = humanartifact.DispositionClaimData{
			ID: c.ID, Digest: c.Digest, Category: c.Category, AuthorityDigest: c.AuthorityDigest,
			// Scope is copied verbatim from the row too — most witness
			// categories require scope.refs == [ID] exactly
			// (policyartifact.validateSemanticClaimScope), a per-claim fact
			// only the report's own claim carries.
			Scope: c.Scope,
		}
	}
	exemptionData := make([]humanartifact.DispositionExemptionData, len(exemptions))
	for i, e := range exemptions {
		exemptionData[i] = humanartifact.DispositionExemptionData{ID: e.ID, Digest: e.Digest}
	}

	scaffoldData := humanartifact.DispositionScaffoldData{
		Name:   a.id,
		Title:  a.title,
		Owners: a.owners,
		// InputID is copied VERBATIM from the selected row (design §2.3),
		// never recomputed — the row's own InputID already came from the
		// same strict-decoded, canonically-round-tripped report this
		// verb never re-evaluates.
		InputID:              row.InputID,
		TargetDigest:         a.targetDigest,
		Claims:               claims,
		Exemptions:           exemptionData,
		Conclusion:           a.conclusion,
		Origin:               dispositionOrigin(row),
		CompensatingControls: a.compensatingControls,
		Approvals:            approvals,
		Expiry:               a.expiry,
		TemplateIdentity:     scaffold.Identity,
		TemplateDigest:       scaffold.Digest,
	}

	content, err := humanartifact.RenderDisposition(scaffold, scaffoldData)
	if err != nil {
		fmt.Fprintln(stderr, "disposition record:", err)
		return 2
	}

	if err := atomicfile.Write(destPath, []byte(content), 0o644); err != nil {
		fmt.Fprintln(stderr, "disposition record:", err)
		return 2
	}

	fmt.Fprintf(stdout, "disposition record: wrote %s (policy-disposition/%s)\n", destPath, a.id)
	return 0
}

// findSemanticRow returns the report's semantic row whose InputID equals
// inputID and the number of rows that matched. A report's Semantic slice
// carries at most one row in the current kernel
// (policyconflict.Service.Evaluate appends at most once), but this loop
// never assumes that: --row is an explicit, checked selector, and a
// report carrying two or more rows sharing one input_id is an internally
// ambiguous document this verb must refuse rather than silently resolve
// by picking the first (M-3, review finding). Callers must check matches
// before trusting the returned row: matches == 0 means not found,
// matches == 1 means the returned row is the unique match, and
// matches > 1 means the returned row is only the FIRST of several and
// must not be used.
func findSemanticRow(report policyconflict.Report, inputID string) (row policyconflict.SemanticEvaluation, matches int) {
	for _, r := range report.Semantic {
		if r.InputID == inputID {
			if matches == 0 {
				row = r
			}
			matches++
		}
	}
	return row, matches
}

// specClaimCategories is the closed set of witness categories only a
// governing SPECIFICATION — the target itself, or (per internal/
// contextcompile's buildFragmentProse, conflict.go:1000) a governing
// PARENT feature fragment — can contribute. policy-instruction
// (buildPolicyInstructionProse, conflict.go:938), adr-decision
// (buildADRDecisionProse, conflict.go:1044), and obligation-declaration
// (buildObligationProse, conflict.go:1061) claims are never the target
// specification's own identity claim, regardless of which report arm
// produced the row — review finding I-1.
var specClaimCategories = map[string]bool{
	"spec-problem":         true,
	"spec-outcome":         true,
	"acceptance-criterion": true,
	"open-question":        true,
	"constraint":           true,
	"decision":             true,
}

// targetRefFromReport returns the report's own target ref when the report
// identifies it directly, and whether one was found.
//
// Only the acceptance-candidate arm carries this: CandidateIdentity.Ref
// (policyconflict.CandidateIdentity, schema.go) names the exact ref under
// evaluation. The accepted-context arm's identity
// (policyconflict.AcceptedIdentity) carries ONLY the manifest digest —
// deliberately: using it as (or to find) the target digest would recurse
// through the effective-policy digest, which folds every disposition
// (authority design §8; this file's header comment). Docs/superpowers/
// specs/2026-08-12-policy-conflict-gate-authority-design.md §10 confirms
// the semantic row itself carries no source-ref field either. So for the
// accepted-context arm this returns ok=false — see
// targetClaimAuthorityDigests's own doc comment for how that case is
// still handled safely, never by guessing a ref.
func targetRefFromReport(report policyconflict.Report) (ref string, ok bool) {
	if c := report.Input.Target.Candidate; c != nil && c.Ref != "" {
		return c.Ref, true
	}
	return "", false
}

// targetClaimAuthorityDigests returns the distinct, sorted AuthorityDigest
// values among claims attributable to the target specification itself —
// review finding I-1: claims come from five authority classes (policy
// instructions, the target spec, governing parent-feature fragments, ADRs,
// obligations — internal/contextcompile/conflict.go's buildProseClaims and
// its five builders), and only the target's OWN claims carry the target's
// content digest; accepting any claim's authority digest (this file's
// pre-fix behavior) let a parent-feature, policy, ADR, or obligation
// digest through, which would later resolve inert
// (policyconflict.ResolveDispositionAuthority / authority.go's
// resolveDisposition: violated-with-witness, no diagnostic).
//
// When targetRef is known (haveRef, the acceptance-candidate arm), this is
// EXACT: a claim qualifies only when its id's "<source-ref>#<object>"
// source-ref segment equals targetRef. A policy-instruction, adr-decision,
// or obligation-declaration claim's id never takes that shape against a
// spec ref, so those categories are excluded as a side effect of the ref
// comparison itself, never specially-cased.
//
// When targetRef is unknown (the accepted-context arm), this is a
// disclosed, conservative APPROXIMATION: claims are narrowed to
// specClaimCategories (excluding policy-instruction/adr-decision/
// obligation-declaration, which are never the target itself), which is as
// far as this can go without the ref — internal/contextcompile's
// buildFragmentProse mirrors buildSpecProse's exact categories and scope
// shape for a governing PARENT feature, so a parent's spec-shaped claim is
// structurally indistinguishable from the target's own by category alone.
// The caller (runDispositionRecord) therefore accepts a target digest only
// when EXACTLY ONE distinct digest remains after this narrowing: two
// different real files' content digests colliding by chance is not a risk
// this verb needs to entertain, so a single remaining digest is
// unambiguously the target's; two or more distinct digests is exactly the
// shape a governing parent feature's claims produce, and the caller must
// refuse rather than guess which one is the target's — fail closed, never
// accept unverified.
func targetClaimAuthorityDigests(claims []policyartifact.SemanticClaimWitness, targetRef string, haveRef bool) []string {
	seen := make(map[string]bool)
	for _, c := range claims {
		if haveRef {
			sourceRef, _, _ := strings.Cut(c.ID, "#")
			if sourceRef != targetRef {
				continue
			}
		} else if !specClaimCategories[c.Category] {
			continue
		}
		seen[c.AuthorityDigest] = true
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// dispositionIDRe is the store's disposition id grammar (M-2, review
// finding): a single kebab-case path component, mirroring
// policyartifact's own (unexported) kebabRe used by parseKindedID to
// validate the "<name>" half of "policy-disposition/<name>" — duplicated
// here, not imported, since it is unexported and internal/policyartifact
// is outside this fix's write set. --id ultimately names a file
// (store.PolicyDispositionPath joins root/.verdi/policy/dispositions/ with
// id+".md"), so this is checked and refused BY NAME before that path is
// ever built or stat'd, rather than relying on policyartifact's own
// (later, generic "failed strict decode") rejection of a malformed id.
var dispositionIDRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// validDispositionID reports whether id is a single, kebab-case path
// component — never empty, never containing "/" or a ".."/"." segment,
// never any character outside dispositionIDRe's grammar.
func validDispositionID(id string) bool {
	return dispositionIDRe.MatchString(id)
}

// dispositionOrigin derives the disposition's origin from whether row
// carries a primary judge exchange (design §2.3: "origin (human-fallback
// when no judgment is present, judge-result otherwise)"). This verb never
// populates a JudgmentProvenance citation even when origin is judge-result
// (see DispositionScaffoldData's own doc comment for why: the row's
// JudgmentExchange carries no persisted immutable judgment-record digest
// for this verb to cite) — decode accepts that omission unconditionally,
// regardless of origin.
func dispositionOrigin(row policyconflict.SemanticEvaluation) string {
	if row.Primary == nil {
		return string(policyartifact.DispositionHumanFallback)
	}
	return string(policyartifact.DispositionJudgeResult)
}

// dispositionExemption is reconstructApplicableExemptions's own return
// element — deliberately a package-local type (not
// policyartifact.SemanticExemptionWitness or humanartifact.
// DispositionExemptionData) so this pure aggregation stays independent of
// either consumer's own shape; runDispositionRecord converts to whichever
// its two call sites need.
type dispositionExemption struct {
	ID     string
	Digest string
}

// reconstructApplicableExemptions rebuilds the semantic input's applicable-
// exemption set from the SAME report's mechanical rows — the report's sole
// recorded source for this set. SemanticEvaluation itself carries no
// exemptions field (docs/superpowers/specs/2026-08-12-policy-conflict-
// gate-authority-design.md §10: "Each semantic row carries its semantic-
// input ID, normalized claim identities, the sorted UnknownMechanicalWitness
// rows when applicable, primary/challenger exchanges, applicable
// disposition resolution, state, and reason codes" — no exemptions), but
// internal/policyconflict's own semantic.go builds SemanticInput.Exemptions
// by collecting every ExemptionResolution named on every mechanical row
// (applicableExemptionWitnesses, unexported) — exactly what this function
// mirrors, over the SAME already-decoded, already-validated report data,
// so the reconstructed set is provably identical to what evaluation itself
// used, never a guess. Two different digests recorded for the same
// exemption id across mechanical rows is an internally inconsistent
// report, refused rather than silently resolved by picking one.
func reconstructApplicableExemptions(mechanical []policyconflict.MechanicalEvaluation) ([]dispositionExemption, error) {
	byID := make(map[string]dispositionExemption)
	for _, m := range mechanical {
		for _, res := range m.Exemptions {
			e := dispositionExemption{ID: res.ID, Digest: res.Digest}
			if prev, ok := byID[e.ID]; ok {
				if prev != e {
					return nil, fmt.Errorf("exemption %q carries two different digests across mechanical rows (%q and %q)", e.ID, prev.Digest, e.Digest)
				}
				continue
			}
			byID[e.ID] = e
		}
	}
	if len(byID) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]dispositionExemption, 0, len(ids))
	for _, id := range ids {
		out = append(out, byID[id])
	}
	return out, nil
}

// parseApprover splits raw ("ROLE=PRINCIPAL_ID") into its role and
// principal id, validating the principal through the kernel's own
// governanceprincipal.PrincipalID grammar. Role's own kebab-case grammar
// is enforced downstream by policyartifact.DecodeDisposition (reached via
// humanartifact.RenderDisposition), never duplicated here.
func parseApprover(raw string) (role, principal string, err error) {
	role, principal, ok := strings.Cut(raw, "=")
	if !ok {
		return "", "", fmt.Errorf("must have the form ROLE=PRINCIPAL_ID (no \"=\" found)")
	}
	if role == "" {
		return "", "", fmt.Errorf("must have the form ROLE=PRINCIPAL_ID (role is empty)")
	}
	if principal == "" {
		return "", "", fmt.Errorf("must have the form ROLE=PRINCIPAL_ID (principal is empty)")
	}
	if err := governanceprincipal.PrincipalID(principal).Validate(); err != nil {
		return "", "", fmt.Errorf("principal: %w", err)
	}
	return role, principal, nil
}
