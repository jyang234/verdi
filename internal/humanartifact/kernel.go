package humanartifact

import (
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// specBaseFields are internal/artifact's shared Base struct's own
// frontmatter keys (internal/artifact/common.go): id, kind, title,
// owners, schema, links, frozen, provenance. Every spec-store kind this
// package covers (feature, story, adr, attestation, waiver,
// reaffirmation, obligation) embeds Base — or, for feature/story, the
// SpecFrontmatter type that itself embeds Base — so these eight names
// are shared kernel across all seven.
var specBaseFields = []string{"id", "kind", "title", "owners", "schema", "links", "frozen", "provenance"}

// withStatus returns base plus "status", for the spec-store kinds whose
// own decoded Frontmatter type carries a status field: ADRFrontmatter,
// WaiverFrontmatter (both mandatory), and SpecFrontmatter's feature/story
// classes (optional, `omitempty` — merge-signaled acceptance's statusless
// posture, internal/artifact/spec.go). Attestation, reaffirmation, and
// obligation carry no status field at all ("existence is the record");
// their kernel is specBaseFields alone.
func withStatus(base []string) []string {
	return withFields(base, "status")
}

// withFields returns base plus extra, without mutating base — the
// shared append-a-copy helper withStatus and kernelFieldTable's own
// per-kind rows below both use.
func withFields(base []string, extra ...string) []string {
	out := make([]string, len(base), len(base)+len(extra))
	copy(out, base)
	return append(out, extra...)
}

// kernelFieldTable is the closed, per-kind kernel field-name table this
// package's Kernel documentation type materializes.
//
// Constitution kinds (policy, policy-overlay, policy-exemption) list
// their FULL L1 frontmatter key set (internal/policyartifact's
// kernelDoc/policyDoc/overlayDoc/exemptionDoc) — AC-1/DC-4: nothing in a
// policy artifact's own structured claims/refinements/witnesses grammar
// is generic free-form template content the way a spec's problem/
// outcome prose is, so the whole decoded shape is authority, not merely
// its identity fields.
//
// Spec-store kinds list their shared Base fields (internal/artifact's
// common.go: id, kind, title, owners, schema, links, frozen, provenance)
// plus status where the kind's own decoder carries one (withStatus),
// plus every OTHER identity/governance frontmatter key that kind's own
// decoder recognizes: class and custom for feature/story
// (SpecFrontmatter's own class discriminator, mandatory, and its
// reserved `custom:` extension namespace, internal/artifact/spec.go —
// a model-declared extension named "custom" would collide with an
// already-reserved key even though a given spec may omit the key
// itself); decided for adr (ADRFrontmatter); reason and expiry for
// waiver (WaiverFrontmatter); object and hash for reaffirmation
// (ReaffirmationFrontmatter); for_kind for obligation
// (ObligationFrontmatter). Deliberately excluded, per the accepted DC-4
// reasoning: each kind's own BODY-PROSE / content fields — problem,
// outcome, acceptance_criteria, constraints, decisions — since these are
// the scaffold's ordinary templated content, not identity/authority/
// governance kernel (the same optimize-for-authors/never-ambiguous-
// proof-formats line DC-4 draws generally, applied here to which fields
// count as an artifact's own immutable identity versus its authored
// body). This is a deliberately bounded correction, not a claim of
// completeness over every remaining SpecFrontmatter field (story, spike,
// impacts, context, open_questions, stubs, supersession, dispositions
// are not yet in this table); widening further is later, narrowly-
// scoped work.
var kernelFieldTable = map[string][]string{
	string(artifact.ClassFeature):      withFields(withStatus(specBaseFields), "class", "custom"),
	string(artifact.ClassStory):        withFields(withStatus(specBaseFields), "class", "custom"),
	string(artifact.KindADR):           withFields(withStatus(specBaseFields), "decided"),
	string(artifact.KindAttestation):   specBaseFields,
	string(artifact.KindWaiver):        withFields(withStatus(specBaseFields), "reason", "expiry"),
	string(artifact.KindReaffirmation): withFields(specBaseFields, "object", "hash"),
	string(artifact.KindObligation):    withFields(specBaseFields, "for_kind", "quality"),

	policyartifact.KindPolicy:    {"schema", "id", "kind", "title", "owners", "template", "scope", "claims", "instructions", "payloads"},
	policyartifact.KindOverlay:   {"schema", "id", "kind", "title", "owners", "template", "refines", "scope", "refinements"},
	policyartifact.KindExemption: {"schema", "id", "kind", "title", "owners", "template", "scope", "witnesses", "compensating_controls", "approvals", "expiry", "review_condition"},
}

// KernelFields returns the immutable kernel frontmatter field NAMES for
// kind — every name a model-declared extension must never shadow,
// rename, retype, or synthesize (AC-1) — and whether kind names a
// recognized artifact family. An unrecognized kind fails closed (ok
// false, fields nil), never a silently-empty kernel. The returned slice
// is a fresh copy: mutating it never corrupts kernelFieldTable or any
// other caller's own answer.
func KernelFields(kind string) ([]string, bool) {
	fields, ok := kernelFieldTable[kind]
	if !ok {
		return nil, false
	}
	out := make([]string, len(fields))
	copy(out, fields)
	return out, true
}
