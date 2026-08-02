package specstate

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// gitReader is the Git-plumbing surface Projector needs, narrowed to
// exactly the internal/gitx entry points spec-acceptance state derivation
// requires (the 04 §port pattern: define the interface at the consumer,
// accept interfaces, return structs). Production callers get the real one
// through NewProjector; package tests construct a Projector directly over
// an in-process fake through the unexported newProjector, so the
// projector's own decision logic is characterized without executing real
// git (internal/fixturegit-backed integration tests separately prove the
// real adapter end to end).
type gitReader interface {
	Show(ctx context.Context, dir, commit, path string) ([]byte, error)
	BlobAt(ctx context.Context, dir, ref, path string) (oid string, found bool, err error)
	FirstParentBlobLanding(ctx context.Context, dir, ref, path, oid string) (commit string, found bool, err error)
	LsTree(ctx context.Context, dir, ref, path string) ([]string, error)
}

// realGitReader adapts internal/gitx's free functions to gitReader.
type realGitReader struct{}

func (realGitReader) Show(ctx context.Context, dir, commit, path string) ([]byte, error) {
	return gitx.Show(ctx, dir, commit, path)
}

func (realGitReader) BlobAt(ctx context.Context, dir, ref, path string) (string, bool, error) {
	return gitx.BlobAt(ctx, dir, ref, path)
}

func (realGitReader) FirstParentBlobLanding(ctx context.Context, dir, ref, path, oid string) (string, bool, error) {
	return gitx.FirstParentBlobLanding(ctx, dir, ref, path, oid)
}

func (realGitReader) LsTree(ctx context.Context, dir, ref, path string) ([]string, error) {
	return gitx.LsTree(ctx, dir, ref, path)
}

// Projector resolves Candidate bytes into an effective Result. Its zero
// value is not useful — construct it via NewProjector (production) or the
// package-private newProjector (tests, over a fake gitReader).
type Projector struct {
	git gitReader
}

// NewProjector returns a Projector backed by the real git plumbing
// (internal/gitx, execed against the process's system git). It is the
// only constructor production callers may use.
func NewProjector() Projector {
	return Projector{git: realGitReader{}}
}

// newProjector is the test-only seam: package tests construct a Projector
// over an in-process fake gitReader.
func newProjector(g gitReader) Projector {
	return Projector{git: g}
}

// specZonesPrefix is the store location scanSuccessors scans for
// potential successors: BOTH zones under .verdi/specs/, in one recursive
// LsTree call. The archive zone is included deliberately (final fix wave
// I3; design §Authority names archive records as authority): a successor
// that has itself CLOSED — moved to specs/archive/ — still supersedes its
// predecessor, and an active-only scan silently reverted such a
// predecessor to AcceptedPendingBuild the moment its successor archived.
const specZonesPrefix = ".verdi/specs"

// candidatePathPattern derives a candidate's zone and bare spec name from
// its path: group 1 is "active" or "archive", group 2 is the kebab-case
// name. Any other shape is refused outright rather than guessed at.
var candidatePathPattern = regexp.MustCompile(`^\.verdi/specs/(active|archive)/([a-z0-9]+(?:-[a-z0-9]+)*)/spec\.md$`)

// zone is the spec store location a candidate path names.
type zone int

const (
	zoneActive zone = iota
	zoneArchive
)

// parseCandidatePath derives a candidate's zone and bare spec name from
// its path, refusing any shape other than the two the store recognizes:
// .verdi/specs/active/<name>/spec.md or .verdi/specs/archive/<name>/spec.md.
func parseCandidatePath(path string) (zone, string, error) {
	m := candidatePathPattern.FindStringSubmatch(path)
	if m == nil {
		return 0, "", fmt.Errorf("specstate: candidate path %q does not match .verdi/specs/{active,archive}/<name>/spec.md", path)
	}
	if m[1] == "archive" {
		return zoneArchive, m[2], nil
	}
	return zoneActive, m[2], nil
}

// successorCorpus is the default-branch spec corpus — BOTH zones, see
// specZonesPrefix — decoded at most once per ResolveMany call, over EVERY
// spec.md path with no exclusion at scan time (fix-round-1 finding 1:
// batch-wide exclusion at scan time hid a landed successor whenever it
// happened to also be one of the SAME call's own candidates, and silently
// dropped a malformed candidate's own decode failure from ever becoming a
// witness for any OTHER candidate). supersedesBy maps a predecessor
// spec's bare name to every default-branch path that carries BOTH a
// links: {type: supersedes} edge to that predecessor AND a validated
// supersession: block (the brief's two-signal requirement — this
// package does not additionally cross-check the supersession block's
// carried/amended/removed buckets against the predecessor's own object
// ids; internal/artifact already validates the block's own shape at
// decode time, and completeness against a specific predecessor's objects
// is a lint-layer concern, not this package's).
//
// linkOnlyBy maps a predecessor spec's bare name to every default-branch
// path that names it via a links: {type: supersedes} edge WITHOUT a
// validatable supersession: block — the story-class shape, which can
// never carry the block (internal/artifact's validateStory rejects it
// outright). One signal is not proof, so such a claim never projects
// Superseded; but discarding it entirely (the pre-fix behavior) silently
// accepted a predecessor a reviewed, merged successor claims to replace.
// resolveOne projects the predecessor Unproven with a disclosure naming
// the successor and the missing proof (final fix wave I4) — three-valued
// honesty, no invented mechanism.
//
// failures maps every corpus path that failed strict decode to a
// human-readable witness message, keyed by path so a per-candidate lookup
// (supersessorsFor/failuresExcluding below) can exclude EXACTLY that one
// candidate's own path — self-exclusion applied at lookup time, never at
// scan time, so a candidate's own decode failure never blocks its own
// verdict while still blocking every OTHER candidate's (a spec can never
// supersede itself, but it can very much be the undetected successor of
// something else in the same batch).
type successorCorpus struct {
	supersedesBy map[string][]string
	linkOnlyBy   map[string][]string
	failures     map[string]string
}

// supersessorsFor returns the sorted default-branch paths that validly
// name candidateName as their predecessor, excluding any entry whose own
// path equals candidatePath (defensive: a spec cannot supersede itself,
// even if malformed data somehow declared it).
func (c *successorCorpus) supersessorsFor(candidatePath, candidateName string) []string {
	var out []string
	for _, p := range c.supersedesBy[candidateName] {
		if p == candidatePath {
			continue
		}
		out = append(out, p)
	}
	return out
}

// linkOnlySupersessorsFor returns the sorted default-branch paths that
// name candidateName as their predecessor via a supersedes link alone —
// no validatable supersession: block — with the same self-exclusion
// supersessorsFor applies.
func (c *successorCorpus) linkOnlySupersessorsFor(candidatePath, candidateName string) []string {
	var out []string
	for _, p := range c.linkOnlyBy[candidateName] {
		if p == candidatePath {
			continue
		}
		out = append(out, p)
	}
	return out
}

// failuresExcluding returns every decode-witness message EXCEPT
// candidatePath's own (self-exclusion at lookup time — see successorCorpus's
// doc comment), sorted for deterministic output.
func (c *successorCorpus) failuresExcluding(candidatePath string) []string {
	var out []string
	for path, msg := range c.failures {
		if path == candidatePath {
			continue
		}
		out = append(out, msg)
	}
	sort.Strings(out)
	return out
}

// scanSuccessors reads and strict-decodes EVERY spec.md path under the
// default branch's two spec zones (specZonesPrefix — active AND archive,
// final fix wave I3), unconditionally — no candidate path is excluded at
// scan time (fix-round-1 finding 1; see successorCorpus's doc comment for
// why exclusion belongs at lookup time instead).
func (p Projector) scanSuccessors(ctx context.Context, root string, branch Branch) (*successorCorpus, error) {
	paths, err := p.git.LsTree(ctx, root, branch.Ref, specZonesPrefix)
	if err != nil {
		return nil, fmt.Errorf("specstate: scanning default-branch specs: %w", err)
	}
	sort.Strings(paths)

	corpus := &successorCorpus{supersedesBy: map[string][]string{}, linkOnlyBy: map[string][]string{}, failures: map[string]string{}}
	for _, path := range paths {
		if !strings.HasSuffix(path, "/spec.md") {
			continue // the corpus scan cares only about spec.md leaves
		}

		content, err := p.git.Show(ctx, root, branch.Ref, path)
		if err != nil {
			return nil, fmt.Errorf("specstate: reading default-branch spec %s: %w", path, err)
		}

		// artifact.DecodeSpec's documented input is frontmatter-only bytes
		// (every other call site in the module splits first — internal/
		// lint/walk.go's decodeDocument, internal/index's own walk, this
		// same file's probeLegacyStatus below): content here is a FULL
		// markdown file (opening/closing "---" delimiters plus the body).
		// Handing that straight to DecodeSpec used to make the underlying
		// yaml.v3 parser treat the body as a second, trailing YAML
		// document in the same stream — fine for a trivial one-line body,
		// but a real body (markdown headings, prose, backticks) routinely
		// fails to parse as YAML at all, surfacing as a spurious decode
		// failure for a document that is perfectly valid frontmatter.
		// SplitFrontmatter first, exactly like every other caller.
		rawFM, _, splitErr := artifact.SplitFrontmatter(content)
		if splitErr != nil {
			corpus.failures[path] = fmt.Sprintf("default-branch spec %s failed to decode: %v", path, splitErr)
			continue
		}
		fm, decodeErr := artifact.DecodeSpec(rawFM)
		if decodeErr != nil {
			corpus.failures[path] = fmt.Sprintf("default-branch spec %s failed to decode: %v", path, decodeErr)
			continue
		}
		for _, l := range fm.Links {
			if l.Type != artifact.LinkSupersedes {
				continue
			}
			ref, parseErr := artifact.ParseRef(l.Ref)
			if parseErr != nil || ref.Kind != artifact.KindSpec {
				continue
			}
			if fm.Supersession != nil {
				// The two-signal successor shape: a supersedes edge plus a
				// validated supersession: block — real, positive proof.
				corpus.supersedesBy[ref.Name] = append(corpus.supersedesBy[ref.Name], path)
			} else if !ref.Fragment() {
				// One WHOLE-SPEC signal only (the story-class shape, which
				// can never carry the block): recorded, never discarded —
				// the predecessor projects disclosed-unproven (fix wave
				// I4). An OBJECT-FRAGMENT supersedes edge (spec/x#object)
				// is excluded here on purpose: it is a decision-level
				// override (03 §Challenging closed decisions' rung-2
				// machinery), never a claim to replace the whole spec, so
				// it neither proves nor un-proves the predecessor's own
				// lifecycle state.
				corpus.linkOnlyBy[ref.Name] = append(corpus.linkOnlyBy[ref.Name], path)
			}
		}
	}

	for name := range corpus.supersedesBy {
		sort.Strings(corpus.supersedesBy[name])
	}
	for name := range corpus.linkOnlyBy {
		sort.Strings(corpus.linkOnlyBy[name])
	}
	return corpus, nil
}

// ResolveMany projects every candidate's effective state, reading and
// strict-decoding the default-branch active-spec corpus AT MOST ONCE for
// the whole call, never once per candidate — batch consumers therefore
// never trigger an O(specs²) Git+decode scan. The scan is built lazily,
// the first time some candidate actually needs a supersession answer (an
// active-zone candidate whose exact bytes are already provably reachable
// from the default branch): a call resolving only new proposals, diverged
// candidates, or archive-zone candidates never touches the corpus at all.
// Once built, the same corpus answers every remaining candidate in the
// call. Resolve delegates here with a single candidate.
func (p Projector) ResolveMany(ctx context.Context, root string, candidates []Candidate) ([]Result, error) {
	branch, ok := ResolveDefaultBranch(ctx, root)
	if !ok {
		results := make([]Result, len(candidates))
		for i := range candidates {
			results[i] = unresolvedDefaultBranchResult(root)
		}
		return results, nil
	}

	var corpus *successorCorpus
	getCorpus := func() (*successorCorpus, error) {
		if corpus == nil {
			built, err := p.scanSuccessors(ctx, root, branch)
			if err != nil {
				return nil, err
			}
			corpus = built
		}
		return corpus, nil
	}

	results := make([]Result, len(candidates))
	for i, c := range candidates {
		r, err := p.resolveOne(ctx, root, branch, c, getCorpus)
		if err != nil {
			return nil, err
		}
		results[i] = r
	}
	return results, nil
}

// Resolve projects a single candidate's effective state.
func (p Projector) Resolve(ctx context.Context, root string, candidate Candidate) (Result, error) {
	results, err := p.ResolveMany(ctx, root, []Candidate{candidate})
	if err != nil {
		return Result{}, err
	}
	return results[0], nil
}

// unresolvedDefaultBranchResult is every candidate's result when the
// default branch itself cannot be resolved (the design's "unproven"
// clause): the missing witness is the default branch, never a silent
// fallback to a branch named "main".
func unresolvedDefaultBranchResult(root string) Result {
	return Result{
		State:       Unproven,
		Relation:    RelationUnproven,
		Disclosures: []string{fmt.Sprintf("specstate: no default branch could be resolved for %s", root)},
	}
}

// resolveOne projects one candidate given an already-resolved default
// branch. getCorpus lazily builds (and memoizes, across every candidate in
// the enclosing ResolveMany call) the default-branch successor corpus —
// called only when this candidate turns out to actually need a
// supersession answer.
func (p Projector) resolveOne(ctx context.Context, root string, branch Branch, c Candidate, getCorpus func() (*successorCorpus, error)) (Result, error) {
	zn, name, err := parseCandidatePath(c.Path)
	if err != nil {
		return Result{}, err
	}

	defaultOID, found, err := p.git.BlobAt(ctx, root, branch.Ref, c.Path)
	if err != nil {
		return Result{}, fmt.Errorf("specstate: resolving %s on %s: %w", c.Path, branch.Ref, err)
	}
	if !found {
		return Result{State: Proposed, Relation: RelationNew}, nil
	}

	defaultContent, err := p.git.Show(ctx, root, branch.Ref, c.Path)
	if err != nil {
		return Result{}, fmt.Errorf("specstate: reading %s at %s: %w", c.Path, branch.Ref, err)
	}

	// The exact-byte assertion compares candidate content against Show's
	// default-branch bytes directly — never a working-tree status field
	// (the brief's own binding requirement).
	if !bytes.Equal(defaultContent, c.Content) {
		return Result{
			State:    Proposed,
			Relation: RelationDiverged,
			Baseline: &Baseline{Path: c.Path, Blob: defaultOID},
		}, nil
	}

	landing, found, err := p.git.FirstParentBlobLanding(ctx, root, branch.Ref, c.Path, defaultOID)
	if err != nil {
		return Result{}, fmt.Errorf("specstate: proving first-parent landing for %s@%s: %w", c.Path, defaultOID, err)
	}
	if !found {
		return Result{
			State:    Unproven,
			Relation: RelationUnproven,
			Disclosures: []string{fmt.Sprintf(
				"specstate: %s matches the default branch's bytes (blob %s) but no first-parent landing commit could be proven on %s",
				c.Path, defaultOID, branch.Ref,
			)},
		}, nil
	}

	baseline := &Baseline{Path: c.Path, Blob: defaultOID, LandingCommit: landing}

	if zn == zoneArchive {
		return Result{State: Closed, Relation: RelationExact, Baseline: baseline}, nil
	}

	// Active zone, exact bytes reachable from default, landing proven:
	// check supersession before falling back to the legacy status field.
	// Both lookups apply THIS candidate's own self-exclusion at lookup
	// time (fix-round-1 finding 1) — the shared corpus itself was scanned
	// with no exclusion at all.
	corpus, err := getCorpus()
	if err != nil {
		return Result{}, err
	}
	if successors := corpus.supersessorsFor(c.Path, name); len(successors) > 0 {
		return Result{State: Superseded, Relation: RelationExact, Baseline: baseline}, nil
	}

	// fix-round-2 (Finding 4): THIS candidate's own bytes were ALSO
	// strict-decoded by the successor-corpus scan just above
	// (scanSuccessors decodes every active-zone spec.md unconditionally,
	// candidate paths included — fix-round-1 finding 1's own "no exclusion
	// at scan time") — reuse that witness directly rather than re-decoding.
	// Checked here, AFTER the successor-corpus proof above (which needs
	// none of this candidate's own content, so an externally,
	// positively-proven Superseded verdict still wins regardless of
	// whether this candidate's own bytes happen to decode) but BEFORE the
	// legacy-status read below (whose own probeLegacyStatus is tolerant
	// and would otherwise silently swallow this exact failure): a landed,
	// exact, active-zone spec whose frontmatter fails to decode must
	// project Unproven with a disclosure naming the failure, never
	// silently fall through to AcceptedPendingBuild just because its
	// legacy field (which might have said superseded or closed) couldn't
	// be read.
	if failMsg, malformed := corpus.failures[c.Path]; malformed {
		return Result{State: Unproven, Relation: RelationUnproven, Disclosures: []string{failMsg}}, nil
	}

	// fix-round-1 finding 1 (Task 5 review): a legacy EXPLICIT terminal
	// status — superseded or closed — is itself Git-derived-compatible
	// evidence once these exact bytes are proven reachable and landed: the
	// design's "Artifact compatibility" section preserves a legacy
	// terminal artifact's EXISTING meaning rather than silently
	// re-deriving a weaker one, and "Command behavior" forbids a consumer
	// re-checking the raw field itself — so this projection has to be the
	// one place that reads it. This is exactly the shape a rung-3
	// (story-level) supersession leaves behind: the OLD accept ritual
	// flips the predecessor's OWN status field to superseded as a real,
	// landed commit, but the successor a class: story predecessor is
	// bound to can never carry a validated supersession: block (internal/
	// artifact's validateStory rejects it outright), so the two-signal
	// successor-corpus proof above can NEVER independently confirm story-
	// level supersession — without this fallback, a landed, legacy-
	// superseded story predecessor would silently read as
	// AcceptedPendingBuild and every consumer that migrated onto this
	// projector would proceed as though it were still buildable. Checked
	// BEFORE the incomplete-scan Unproven case below: an explicit terminal
	// status is a direct, self-contained statement about THIS candidate
	// that does not depend on the corpus scan being complete to be
	// believed (unlike the corpus's own NEGATIVE "no successor found"
	// inference, which does).
	if legacy := probeLegacyStatus(c.Content); legacy == "superseded" || legacy == "closed" {
		state := Superseded
		if legacy == "closed" {
			state = Closed
		}
		return Result{
			State:       state,
			Relation:    RelationExact,
			Baseline:    baseline,
			Disclosures: legacyTerminalStatusDisclosure(c.Path, legacy, state),
		}, nil
	}

	// Final fix wave I4: a successor names this predecessor via a
	// links: supersedes edge but carries no validatable supersession:
	// block — the story-class shape, which can never carry the block, so
	// the two-signal proof above can never confirm it. One signal is not
	// proof (never Superseded — no invented mechanism), but a reviewed,
	// merged successor's claim is not nothing either (never silent
	// AcceptedPendingBuild): the predecessor projects disclosed-unproven,
	// naming each claiming successor and the missing proof. Checked AFTER
	// the legacy-terminal read above — an explicit persisted terminal
	// status is a positive, self-contained statement that still wins —
	// and BEFORE the scan-incompleteness fallback below (this is a more
	// specific witness than "the scan could not complete").
	if linkOnly := corpus.linkOnlySupersessorsFor(c.Path, name); len(linkOnly) > 0 {
		disclosures := make([]string, 0, len(linkOnly))
		for _, succ := range linkOnly {
			disclosures = append(disclosures, fmt.Sprintf(
				// vocab:identity — machinery diagnostic naming the frontmatter link/block fields and the lifecycle states involved
				"specstate: %s is named as a predecessor by %s via a links: supersedes edge, but that successor carries no validatable supersession: block — supersession cannot be proven from Git alone; reported unproven with this disclosure, never silently accepted-pending-build",
				c.Path, succ,
			))
		}
		return Result{State: Unproven, Relation: RelationUnproven, Disclosures: disclosures}, nil
	}

	if failures := corpus.failuresExcluding(c.Path); len(failures) > 0 {
		disclosures := make([]string, 0, len(failures)+1)
		disclosures = append(disclosures, fmt.Sprintf(
			// vocab:identity — machinery diagnostic naming the lifecycle state this scan could not rule out
			"specstate: %s cannot be proven not-superseded — the default-branch active-spec scan is incomplete",
			c.Path,
		))
		disclosures = append(disclosures, failures...)
		return Result{State: Unproven, Relation: RelationUnproven, Disclosures: disclosures}, nil
	}

	return Result{
		State:       AcceptedPendingBuild,
		Relation:    RelationExact,
		Baseline:    baseline,
		Disclosures: migrationDisclosures(c.Path, c.Content),
	}, nil
}

// migrationDisclosures returns the compatibility disclosure the design's
// "Artifact compatibility" section requires whenever a legacy
// `status: draft` document's exact bytes have already landed on the
// default branch: reported accepted-pending-build with this note, never
// misrepresented as an active draft. Every other legacy status (including
// an omitted one) needs no disclosure — the design treats Git
// reachability, not the persisted field, as authoritative.
func migrationDisclosures(path string, content []byte) []string {
	if probeLegacyStatus(content) != "draft" {
		return nil
	}
	return []string{fmt.Sprintf(
		// vocab:identity — machinery diagnostic naming the legacy field value and the state it is reported as instead
		"specstate: %s carries legacy status: draft, but its exact bytes are already reachable from the default branch — reported accepted-pending-build with this migration disclosure, never as an active draft",
		path,
	)}
}

// legacyTerminalStatusDisclosure returns the compatibility disclosure the
// design's "Artifact compatibility" section requires whenever a legacy
// EXPLICIT terminal status (superseded or closed) is the sole basis for an
// ACTIVE-zone candidate's projected state — reached only when no Git-
// derived successor proof (a validated links: supersedes + supersession:
// pair) independently confirmed it (resolveOne checks the corpus first).
// Mirrors migrationDisclosures' identical "the legacy field was the
// deciding signal, name it" shape for the draft/accepted-pending-build
// case, one level further along the lifecycle.
func legacyTerminalStatusDisclosure(path, legacyStatus string, projected State) []string {
	return []string{fmt.Sprintf(
		// vocab:identity — machinery diagnostic naming the legacy field value and the state it is reported as instead
		"specstate: %s carries legacy status: %s with exact bytes reachable from the default branch, but no Git-derived successor could independently confirm it — reported %s from the legacy field alone (compatibility reading)",
		path, legacyStatus, projected,
	)}
}

// probeLegacyStatus tolerantly reads a spec document's legacy frontmatter
// status: field through internal/artifact.DecodeSpec — this package's
// only sanctioned YAML decode seam (CLAUDE.md: "YAML via the single
// internal/artifact seam"; internal/specalign's import-seam witness
// enforces that no other package imports gopkg.in/yaml.v3 directly).
// internal/artifact.DecodeSpec now accepts an omitted status on the
// feature/story classes (Task 4: spec.go's `status,omitempty` plus
// validateFeature/validateStory's empty-value tolerance), so a statusless
// spec decodes here too — its Status simply reads back as "", which
// migrationDisclosures' own `!= "draft"` check already treats as "no
// legacy draft compatibility question."
//
// fix-round-2 (Finding 4): the legacy status field DOES now gate the
// verdict for an active-zone candidate — resolveOne's own legacy-terminal
// branch (fix-round-1 finding 1) reads it directly to project Superseded
// or Closed, so the OLD claim this comment made ("used here only to
// produce an optional compatibility disclosure, never to gate the
// git-derived verdict itself... must never block or alter the state") no
// longer holds in general. What stays true, narrower than before: THIS
// function's own tolerant swallow-on-decode-failure ("" on any error) is
// safe only because resolveOne no longer reaches it blind for a candidate
// whose own content is malformed — it checks corpus.failures[c.Path] (the
// SAME strict decode the successor-corpus scan already performed) first,
// and projects Unproven with a disclosure before either of this
// function's two remaining callers (resolveOne's own legacy-terminal
// check, and migrationDisclosures) ever runs on unreadable content. Both
// are reached only once that same corpus.failures lookup has already come
// back empty for c.Path, so this function's error-swallowing branch is
// dead in that call path today — never a live fail-open gap — and stays
// tolerant here only as this function's own defensive posture.
func probeLegacyStatus(content []byte) string {
	rawFM, _, splitErr := artifact.SplitFrontmatter(content)
	if splitErr != nil {
		return ""
	}
	fm, err := artifact.DecodeSpec(rawFM)
	if err != nil {
		return ""
	}
	return string(fm.Status)
}
