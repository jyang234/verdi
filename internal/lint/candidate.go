package lint

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// candidateRules is the fixed, closed rule set CheckCandidate runs a
// spec-import candidate through (spec-import-contract "Candidate and
// validation": "using existing VL-002/003/005/006 logic"). It is deliberately
// narrower than allRules: VL-001 is not listed because CheckCandidate
// performs the identical strict-decode check itself, directly on the
// candidate document only (see decodeDocumentBytes below) — running the full
// vl001{} rule over the augmented Snapshot would also re-report every
// OTHER already-broken document already sitting in the corpus, which is
// exactly the "unrelated pre-existing corpus finding" this seam must never
// fold into a candidate's own result.
var candidateRules = []Rule{vl002{}, vl003{}, vl005{}, vl006{}}

// CheckCandidate is the shared, read-only lint seam a spec-import candidate
// is checked through before it can be treated as valid (spec-import-contract
// "Candidate and validation", "Shared internal interfaces"). It builds the
// current corpus Snapshot exactly as `verdi lint` would, adds ONE additional
// in-memory Document decoded from content at relPath — never written to
// disk, never mutating anything the caller passed in — and runs the
// existing VL-002/003/005/006 rule implementations over the combined
// Snapshot: no copy of their logic, no independent parser (internal/
// specimport's Compose is the one caller; it must never re-implement ref
// resolution, configured tracker schemes, evidence floors or stub-target
// checks itself).
//
// The returned findings separate readiness from disclosure (spec-import-
// contract: "Unrelated pre-existing corpus findings are disclosed separately
// and cannot silently validate a candidate whose dependencies fail to decode
// or resolve"):
//
//   - SeverityViolation, and so blocking, for every finding whose own Path is
//     relPath (the candidate itself), plus any dependency the candidate's own
//     frontmatter links reference (a parent feature, an implements/resolves
//     target, ...) that is present in the committed zone but corrupt. Only
//     these determine whether the candidate is usable.
//   - SeverityDisclosure for every finding about some OTHER corpus document,
//     carrying its original rule, path and fact. These are reported, never
//     discarded and never blocking: a pre-existing corpus problem wholly
//     unconnected to this candidate is not the importing author's to fix, but
//     silence about it is not a pass either.
//
// A non-nil error is only ever operational (BuildSnapshot/service discovery
// failure), matching Engine.Run's own contract — a candidate that fails to
// decode is a Finding, never an error.
func CheckCandidate(ctx context.Context, root, relPath string, content []byte) ([]Finding, error) {
	snap, err := BuildSnapshot(root, Options{})
	if err != nil {
		return nil, err
	}

	doc := &Document{
		Kind:    candidateDocKind(relPath),
		Path:    filepath.Join(root, filepath.FromSlash(relPath)),
		RelPath: relPath,
	}
	decodeDocumentBytes(doc, content)
	insertCandidateDocument(snap, doc)

	// Display-only, best-effort — identical posture to Engine.Run's own
	// Model resolution: a store with no openable config checks exactly as
	// before, bare ids (see RunInput.Model's own doc comment in engine.go).
	var mdl *model.Model
	if cfg, err := store.Open(root); err == nil {
		mdl = cfg.Model
	}

	in := &RunInput{
		Ctx:       ctx,
		Root:      root,
		Snapshot:  snap,
		Opts:      Options{},
		Model:     mdl,
		Projector: specstate.NewProjector(),
	}

	var all []Finding
	if doc.DecodeErr != nil {
		// Mirrors vl001{}.Check's own Finding shape for exactly this one
		// document (never the whole-corpus rule: see candidateRules' doc
		// comment) — a candidate that fails strict decode must not be
		// silently waved through just because VL-001 itself is not among
		// the rules CheckCandidate runs.
		all = append(all, Finding{Rule: "VL-001", Path: relPath, Message: doc.DecodeErr.Error()})
	} else {
		for _, r := range candidateRules {
			all = append(all, r.Check(in)...)
		}
		all = append(all, candidateNewSpecFindings(doc, mdl)...)
	}

	findings := filterCandidateFindings(all, relPath)
	dependencyFindings := corruptDependencyFindings(snap, doc)
	findings = append(findings, dependencyFindings...)
	findings = append(findings, unrelatedCorpusDisclosures(all, snap, relPath, dependencyFindings)...)

	// Severity leads the ordering so the findings that decide readiness come
	// before the disclosures that do not; within each, the existing
	// rule/path/message ordering is unchanged.
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity < findings[j].Severity
		}
		if findings[i].Rule != findings[j].Rule {
			return findings[i].Rule < findings[j].Rule
		}
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Message < findings[j].Message
	})
	return findings, nil
}

// candidateNewSpecFindings applies the CURRENT new-spec floor to a candidate
// vl006's own isNewClassSpec would otherwise skip (spec-import-contract:
// "Strict decode, new-spec requiredness, anchors and project checks apply
// even to old native inputs; no archive grandfathering"). A native primary
// may be an old v0-shaped feature — no problem/outcome, no object anchors,
// no attestation kind — which isNewClassSpec reads as grandfathered because
// it carries no round-four surface field. That reading is correct for the
// existing corpus and is left exactly as it is; but a candidate is a spec
// being created NOW, so the floor applies to it.
//
// This calls vl006's OWN helpers rather than restating their checks:
// requiredness, anchor resolution and the feature outcome floor keep their
// single owner (spec-import-contract: "Ref resolution, configured tracker
// schemes, feature evidence floors and stub targets have one owner, never
// copied into the importer"). Scoped to the feature class: a story is
// already always new-class, and a component has no object model at all, so
// forcing the floor there would invent a requirement no rule states.
func candidateNewSpecFindings(doc *Document, mdl *model.Model) []Finding {
	if doc.Spec == nil || doc.Spec.Class != artifact.ClassFeature || isNewClassSpec(doc.Spec) {
		return nil
	}
	r := vl006{}
	findings := locusAll(r.checkRequiredness(doc), SpecLocus())
	return append(findings, r.checkFeatureACAttestation(doc, mdl)...)
}

// unrelatedCorpusDisclosures returns every finding about a document OTHER
// than the candidate, restated at SeverityDisclosure with its original rule,
// path and fact named in the message. Two sources feed it: the findings the
// candidateRules raised over the augmented Snapshot, and the existing
// Snapshot's own decode failures — candidateRules deliberately omit vl001
// (see candidateRules' doc comment), so an unrelated document that does not
// decode at all would otherwise be the one corpus problem this seam never
// mentions.
//
// dependencyFindings are the candidate's own corrupt dependencies, already
// returned as blocking above; their decode failure is not repeated here.
// Locus is dropped: a disclosure never badges a board card (finding.go's
// locusAll doc comment), and a disclosure about another document's object
// would otherwise claim a card on the candidate's wall.
func unrelatedCorpusDisclosures(all []Finding, snap *Snapshot, relPath string, dependencyFindings []Finding) []Finding {
	reportedDependency := make(map[string]bool, len(dependencyFindings))
	for _, f := range dependencyFindings {
		reportedDependency[f.Path] = true
	}

	var out []Finding
	disclose := func(f Finding) Finding {
		return Finding{
			Rule:     f.Rule,
			Path:     f.Path,
			Message:  fmt.Sprintf("unrelated existing corpus finding: %s %s: %s", f.Rule, f.Path, f.Message),
			Severity: SeverityDisclosure,
		}
	}
	for _, f := range all {
		if f.Path == relPath {
			continue
		}
		out = append(out, disclose(f))
	}
	for _, d := range snap.Docs {
		if d.RelPath == relPath || d.DecodeErr == nil || reportedDependency[d.RelPath] {
			continue
		}
		out = append(out, disclose(Finding{Rule: "VL-001", Path: d.RelPath, Message: d.DecodeErr.Error()}))
	}
	return out
}

// candidateDocKind classifies relPath into the artifact kind that governs
// how decodeDocumentBytes decodes it — the same table walkDocuments' own
// classifyArtifactPath already uses, applied to relPath's ".verdi/"-relative
// tail. An unrecognized location decodes as kind "" and unconditionally sets
// DecodeErr (decodeDocumentBytes's default case), so it still surfaces as a
// Finding rather than silently vanishing.
func candidateDocKind(relPath string) string {
	rel := strings.TrimPrefix(relPath, ".verdi/")
	kind, _ := classifyArtifactPath(rel)
	return kind
}

// insertCandidateDocument adds doc to snap.Docs at its sorted position (the
// same RelPath ordering walkDocuments already produces) and, when it
// decoded cleanly with a non-empty id, indexes it into ByRef exactly as
// BuildSnapshot's own loop does — so every rule sees the candidate as an
// ordinary corpus member for both the duplicate-ref and the ref-resolution
// checks. snap is CheckCandidate's own freshly-built Snapshot, never a
// value the caller passed in, so mutating it here does not reach outside
// this function.
func insertCandidateDocument(snap *Snapshot, doc *Document) {
	i := sort.Search(len(snap.Docs), func(i int) bool { return snap.Docs[i].RelPath >= doc.RelPath })
	snap.Docs = append(snap.Docs, nil)
	copy(snap.Docs[i+1:], snap.Docs[i:])
	snap.Docs[i] = doc

	if doc.DecodeErr == nil && doc.Base.ID != "" {
		snap.ByRef[doc.Base.ID] = append(snap.ByRef[doc.Base.ID], doc)
	}
}

// filterCandidateFindings selects the findings that are about the candidate
// itself (Path == relPath) — VL-002's own-path checks, a dangling
// links[]/context[] ref on the candidate's own frontmatter, VL-005's own
// story-tracker check, VL-006's own requiredness/evidence/anchor checks.
// These are the only findings that decide whether the candidate is usable;
// every other finding the augmented Snapshot's corpus legitimately carries
// is routed to unrelatedCorpusDisclosures instead, never dropped.
func filterCandidateFindings(all []Finding, relPath string) []Finding {
	var out []Finding
	for _, f := range all {
		if f.Path == relPath {
			out = append(out, f)
		}
	}
	return out
}

// corruptDependencyFindings surfaces, on the dependency's OWN path, every
// spec doc.Base.Links names (a parent feature, an implements/resolves/
// supersedes/... target — never a type:story tracker ref, never a svc/...
// external ref, neither of which is a corpus Document at all) that exists
// on disk but fails to decode. filterCandidateFindings' own relPath-only
// filter would otherwise silently swallow this: VL-003's own ByRef lookup
// treats "exists but corrupt" identically to "does not exist at all" (both
// are simply absent from ByRef), so without this pass a broken dependency
// reads only as the candidate's own generic "does not resolve" — never
// naming which file is broken or why (spec-import-contract: "surface
// corrupt or unresolvable dependencies ... must not be silent"). The
// Finding mirrors vl001{}.Check's own shape verbatim for exactly this one
// document — not a second decode-failure vocabulary, the same fact vl001
// would report for this file if it ran over the whole corpus, scoped here
// to only the files this candidate actually depends on.
func corruptDependencyFindings(snap *Snapshot, doc *Document) []Finding {
	if doc.DecodeErr != nil {
		return nil
	}

	var findings []Finding
	reported := map[string]bool{}
	for _, l := range doc.Base.Links {
		if l.Type == artifact.LinkStory || externalRefShapeRe.MatchString(l.Ref) {
			continue
		}
		ref, err := artifact.ParseRef(l.Ref)
		if err != nil || ref.Kind != artifact.KindSpec {
			continue
		}
		for _, zone := range []string{store.ZoneActive, store.ZoneArchive} {
			depRelPath := store.SpecRelPath(zone, ref.Name)
			if reported[depRelPath] {
				continue
			}
			dep := findDocumentByRelPath(snap, depRelPath)
			if dep == nil || dep.DecodeErr == nil {
				continue
			}
			reported[depRelPath] = true
			findings = append(findings, Finding{Rule: "VL-001", Path: dep.RelPath, Message: dep.DecodeErr.Error()})
		}
	}
	return findings
}

// findDocumentByRelPath returns the Document in snap.Docs whose RelPath is
// relPath, or nil when no such document was walked at all (as opposed to
// one that was walked but failed to decode, which IS returned — callers
// distinguish the two via the returned Document's own DecodeErr).
func findDocumentByRelPath(snap *Snapshot, relPath string) *Document {
	for _, d := range snap.Docs {
		if d.RelPath == relPath {
			return d
		}
	}
	return nil
}
