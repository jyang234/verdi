package lint

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

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
// The returned findings are filtered to what the candidate's own author can
// act on: every finding whose own Path is relPath (the candidate itself),
// plus an explicit surfacing of any dependency the candidate's own
// frontmatter links reference (a parent feature, an implements/resolves
// target, ...) that turns out to be present in the committed zone but
// corrupt — so a broken dependency can never be silently absorbed into the
// large set of unrelated pre-existing corpus findings this seam does NOT
// return. A non-nil error is only ever operational (BuildSnapshot/service
// discovery failure), matching Engine.Run's own contract — a candidate that
// fails to decode is a Finding, never an error.
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
	}

	findings := filterCandidateFindings(all, relPath)
	findings = append(findings, corruptDependencyFindings(snap, doc)...)

	sort.Slice(findings, func(i, j int) bool {
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

// filterCandidateFindings keeps only the findings that are about the
// candidate itself (Path == relPath) — VL-002's own-path checks, a dangling
// links[]/context[] ref on the candidate's own frontmatter, VL-005's own
// story-tracker check, VL-006's own requiredness/evidence/anchor checks —
// discarding every other, unrelated finding the augmented Snapshot's corpus
// may still legitimately carry (a pre-existing corpus document's own
// problem, wholly unconnected to this candidate).
func filterCandidateFindings(all []Finding, relPath string) []Finding {
	var out []Finding
	for _, f := range all {
		if f.Path == relPath {
			out = append(out, f)
		}
	}
	return out
}

// corruptDependencyFindings is filled in by a following commit (its own
// RED test drives the real body): it will surface a Finding, on the
// dependency's OWN path, for any spec doc.Base.Links names that exists on
// disk but fails to decode — the one case filterCandidateFindings' own
// relPath-only filter would otherwise silently swallow. A placeholder
// empty slice for now changes nothing about the candidate result.
func corruptDependencyFindings(snap *Snapshot, doc *Document) []Finding {
	return nil
}
