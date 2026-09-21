package lint

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// vl022 enforces spec/attest-helper AC-3 (spec/closure-ergonomics AC-2's
// enforcement half): a STORY-targeting attestation's own on-disk story-slug
// segment (parsed from its id, mirroring VL-011's compound-name split) must
// agree with store.RefSlug(target.Story) for the class: story spec its
// `verifies` edge names — the D6-18 failure class (a spec-name slug
// substituted for the story-ref slug) made a witness-carrying refusal
// instead of a silent fold-time absent
// (internal/evidence.AttestationExists/LoadAttestationState are bare
// filesystem checks; neither can tell "misfiled" from "never attested").
//
// Subject — STORY- and FEATURE-targeting attestations, each checked by its
// own class-appropriate rule (Controller adjudication ADJ-51, 2026-07-16,
// extended by spec/readiness-recovery ac-7/R-RR2-2): the rule fires on an
// attestation whose `verifies` edge resolves to a class: story spec — the
// D6-18 misfiling class the story rule exists to kill. `verdi attest`
// itself now scaffolds both classes (spec/readiness-recovery ac-6, ledger
// SI-213), and this rule covers both to match (ac-7, R-RR2-2, ledger
// SI-214, reversing ADJ-51's story-only scope below). ADJ-51's original
// ruling left a genuine residual gap for a
// `verifies` edge to a class: feature spec: this is the smallest reversible
// reading forced by the store's own real data — frozen dc-4 asserts "every
// attestation in the store as of this contract carries no verifies edge,"
// but that empirical premise was FALSE (11 legitimate, frozen feature-
// outcome attestations across this repo's own store and examples/showcase
// DO carry a verifies edge to a class: feature spec, and refusing them
// broke `make verify`), so ADJ-51 first ruled a non-story target
// unconditionally out of scope by CONSTRUCTION, filing mis-slug protection
// for feature-outcome attestations as a future story rather than inventing
// feature-slug-derivation semantics inside that build.
//
// R-RR2-2 closes that residual gap with the fold's own reading, not a
// guess: an attestation's `verifies` edge to a class: feature spec is
// checked exactly like a story edge for the "does the target declare the
// named AC" half (the same undeclared-AC finding, class-blind); the
// slug/mis-slug half applies ONLY when that AC's own declared evidence
// kinds include `attestation` — the fold's own consumer boundary
// (evidence.FoldFeature's FeatureSlug path, 03 §The feature fold): an AC
// that never declares the attestation kind has no fold reading this
// attestation's path at all, so a directory disagreement there is not a
// misfiling, it is simply outside every consumer, and VL-022 skips it
// (the showcase's own jira-loan-1482--ac-2 is exactly this pinned
// example). When the AC DOES declare attestation, the feature's own name
// (artifact.ParseRef(target.ID).Name — the same segment FoldFeature's
// caller passes as FeatureSlug, never a story-ref derivation) is the slug
// the attestation's own directory/id segment must agree with — a feature
// carries no story-ref slug of its own to compare against.
//
// Scoped, further, to attestations that carry a `verifies` edge AT ALL
// (DC-4): a hand-authored attestation with no edge at all is out of scope by
// construction. Mirrors vl019.go's own badVerifiesTarget pattern (an
// obligation's twin check), extended with the one genuinely new piece
// attestations need that obligations don't: a slug-derivation step. An
// obligation's verifies target IS named by its own directory (vl019.go: the
// obligation's story-slug segment is the target spec's own NAME) — but an
// attestation's path segment is the STORY's ref slug,
// store.RefSlug(target.Story), a different string entirely (I-6/D6-16).
type vl022 struct{}

func (vl022) ID() string { return "VL-022" }

func (vl022) Check(in *RunInput) []Finding {
	var findings []Finding
	for _, d := range in.Snapshot.Docs {
		if d.Grandfathered || d.DecodeErr != nil || d.Kind != "attestation" {
			continue
		}

		ref, err := artifact.ParseRef(d.Base.ID)
		if err != nil {
			continue // VL-002 already reports this
		}
		slugSeg, acID, ok := strings.Cut(ref.Name, "--")
		if !ok {
			continue // shape already enforced at decode (I-6 compoundNameRe)
		}

		// Validate EVERY verifies edge, not only the first (ADJ-51 finding 4):
		// AttestationFrontmatter.Validate places no cardinality constraint on
		// links, so a hand-annotated attestation may carry more than one, and
		// a misfiled edge after a clean one must not fold silently. An
		// attestation with no verifies edge at all iterates zero times — out
		// of scope by construction (DC-4).
		for _, l := range d.Base.Links {
			if l.Type != artifact.LinkVerifies {
				continue
			}
			if reason, bad := badAttestationVerifiesTarget(in.Root, l.Ref, slugSeg, acID, in.Model); bad {
				findings = append(findings, Finding{Rule: "VL-022", Path: d.RelPath, Message: fmt.Sprintf("attestation %s verifies %s, %s", d.Base.ID, l.Ref, reason)})
			}
		}
	}
	return findings
}

// badAttestationVerifiesTarget classifies one of an attestation's verifies
// target refs (expected to be a whole story-or-feature-spec ref) together
// with slugSeg (the attestation's own on-disk slug segment, parsed from its
// compound id) and acID (the acceptance-criterion id the attestation's own
// id/path names), and reports whether the attestation is refused, and why,
// always naming the offending value (D6-18: never a silent absence). The
// only accepted shape is a WHOLE spec ref (no object fragment) that
// resolves in the committed zone: a target that does not parse, carries a
// fragment, or does not resolve at all fails closed (a refusal naming what
// it found), and a target that resolves to any class other than story or
// feature — component, say, which declares no acceptance criteria to
// attest against at all — is outside this rule and is SKIPPED, never
// refused (R-RR2-2). For a story or feature target, the AC-declared check
// and the slug rule are then computed per class
// (badStoryAttestationTarget/badFeatureAttestationTarget below).
func badAttestationVerifiesTarget(root, verifiesRef, slugSeg, acID string, mdl *model.Model) (reason string, bad bool) {
	r, err := artifact.ParseRef(verifiesRef)
	if err != nil {
		return fmt.Sprintf("which does not parse as a ref: %v", err), true
	}
	if r.Kind != artifact.KindSpec || r.Fragment() {
		// The spoken class word is display and resolves (L-M13a(6) work
		// order); "closed edge vocabulary" is 02's own term for the edge
		// taxonomy being a closed SET — not the lifecycle state.
		return fmt.Sprintf("which is not a whole spec ref — an attestation verifies the whole %s spec (the AC is named by the attestation's own id, 02 §Link taxonomy's closed edge vocabulary)", mdl.DisplayClass("story")), true
	}

	target, err := storyresolve.LoadSpec(root, r.Name)
	if err != nil || target == nil {
		return "which does not resolve to a spec in the committed zone", true
	}

	switch target.Class {
	case artifact.ClassStory:
		return badStoryAttestationTarget(target, slugSeg, acID)
	case artifact.ClassFeature:
		return badFeatureAttestationTarget(target, slugSeg, acID, mdl)
	default:
		// Out of scope: neither class VL-022 tracks (e.g. component —
		// which has no acceptance criteria to attest against at all).
		return "", false
	}
}

// badStoryAttestationTarget is the original story-scoped rule (ADJ-51),
// unchanged: target's own declared acceptance criteria must include acID,
// and target's own story-ref slug (store.RefSlug(target.Story)) must equal
// slugSeg.
func badStoryAttestationTarget(target *artifact.SpecFrontmatter, slugSeg, acID string) (reason string, bad bool) {
	if _, declared := findDeclaredAC(target, acID); !declared {
		return fmt.Sprintf("but its own id names ac %q, which %s does not declare as an acceptance criterion", acID, target.ID), true
	}

	wantSlug := store.RefSlug(target.Story)
	if slugSeg != wantSlug {
		// vocab:identity — story-ref slug grammar (D6-16 path/id derivation)
		return fmt.Sprintf("whose own story-ref slug is %q, but this attestation's own directory/id segment is %q (D6-18: a spec-name/story-slug mismatch used to fold as a silent absent, never a misfiled attestation)", wantSlug, slugSeg), true
	}

	return "", false
}

// badFeatureAttestationTarget is R-RR2-2's feature-scoped rule: target's
// own declared acceptance criteria must include acID (the same undeclared-
// AC shape as the story rule, class-blind); when acID's own declared
// evidence kinds do NOT include attestation, the attestation sits outside
// every consumer of that AC (evidence.FoldFeature's own path never reads
// it) — skipped, never refused, mirroring ADJ-51's original construction-
// scoped skip one level down (a per-AC boundary, not a per-spec one).
// Otherwise the mis-slug check applies: the attestation's own directory/id
// segment must equal the feature's own name
// (artifact.ParseRef(target.ID).Name — the exact segment FoldFeature's
// caller passes as FeatureSlug), never a story-ref derivation a feature
// carries no obligation to set.
func badFeatureAttestationTarget(target *artifact.SpecFrontmatter, slugSeg, acID string, mdl *model.Model) (reason string, bad bool) {
	ac, declared := findDeclaredAC(target, acID)
	if !declared {
		return fmt.Sprintf("but its own id names ac %q, which %s does not declare as an acceptance criterion", acID, target.ID), true
	}
	if !hasAttestationKind(ac.Evidence) {
		// Out of scope (R-RR2-2): this AC has no fold reading an
		// attestation path at all, so a directory disagreement here is
		// not a misfiling — it is simply outside every consumer (the
		// showcase's own jira-loan-1482--ac-2 is this pinned example).
		return "", false
	}

	wantName := ""
	if ref, err := artifact.ParseRef(target.ID); err == nil {
		wantName = ref.Name
	}
	if slugSeg != wantName {
		// The spoken class word is display and routes through the model
		// (L-M13a(6), checkFeatureACAttestation's own pattern one file
		// over); everything else in this literal is identity the rename
		// must never touch — both quoted values, R-RR2-2's own
		// attestations/<feature-name>/<ac-id>.md path grammar, and 03
		// §The feature fold's section title, cited here as that section
		// names the reading consumer.
		return fmt.Sprintf("whose own name is %q, but this attestation's own directory/id segment is %q (a %s outcome attestation lives at attestations/<feature-name>/<ac-id>.md, the path the feature fold reads)", wantName, slugSeg, mdl.DisplayClass("feature")), true
	}

	return "", false
}

// findDeclaredAC returns target's own declared acceptance criterion whose
// id equals acID, and whether one was found — the "does the target declare
// this AC" check both class-scoped rules above share verbatim.
func findDeclaredAC(target *artifact.SpecFrontmatter, acID string) (artifact.AcceptanceCriterion, bool) {
	for _, ac := range target.AcceptanceCriteria {
		if ac.ID == acID {
			return ac, true
		}
	}
	return artifact.AcceptanceCriterion{}, false
}
