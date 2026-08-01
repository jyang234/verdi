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

// activeZonePrefix is the store location scanSuccessors scans for
// potential successors — only an active-zone spec can supersede another
// (01 §Directory layout; a superseded spec itself "stays in specs/active/",
// internal/artifact/status.go).
const activeZonePrefix = ".verdi/specs/active"

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

// successorCorpus is the default-branch active-zone spec corpus, decoded
// at most once per ResolveMany call. supersedesBy maps a predecessor
// spec's bare name to every default-branch path that carries BOTH a
// links: {type: supersedes} edge to that predecessor AND a validated
// supersession: block (the brief's two-signal requirement — this
// package does not additionally cross-check the supersession block's
// carried/amended/removed buckets against the predecessor's own object
// ids; internal/artifact already validates the block's own shape at
// decode time, and completeness against a specific predecessor's objects
// is a lint-layer concern, not this package's).
//
// failures lists every corpus path that failed strict decode, sorted. A
// non-empty failures list means the scan is INCOMPLETE: one of those
// unreadable specs might be the very successor a candidate needs ruled
// out before it can be reported accepted, so any candidate that would
// otherwise resolve accepted-pending-build from a clean scan instead
// resolves unproven, naming every decode witness (CLAUDE.md three-valued
// honesty: silence is never a pass).
type successorCorpus struct {
	supersedesBy map[string][]string
	failures     []string
}

// scanSuccessors reads and strict-decodes the default branch's active-zone
// spec corpus exactly once, skipping every path in excluded — a spec
// never supersedes itself, so a ResolveMany call's own candidate paths
// are excluded from the decode set: a candidate's own decodability never
// determines whether it itself is superseded, and (today, ahead of the
// sibling task that makes a persisted status optional) a statusless
// active candidate would otherwise fail this package's own corpus decode
// step for a reason that has nothing to do with supersession.
func (p Projector) scanSuccessors(ctx context.Context, root string, branch Branch, excluded map[string]bool) (*successorCorpus, error) {
	paths, err := p.git.LsTree(ctx, root, branch.Ref, activeZonePrefix)
	if err != nil {
		return nil, fmt.Errorf("specstate: scanning default-branch active specs: %w", err)
	}
	sort.Strings(paths)

	corpus := &successorCorpus{supersedesBy: map[string][]string{}}
	for _, path := range paths {
		if !strings.HasSuffix(path, "/spec.md") {
			continue // the corpus scan cares only about spec.md leaves
		}
		if excluded[path] {
			continue
		}

		content, err := p.git.Show(ctx, root, branch.Ref, path)
		if err != nil {
			return nil, fmt.Errorf("specstate: reading default-branch spec %s: %w", path, err)
		}

		fm, decodeErr := artifact.DecodeSpec(content)
		if decodeErr != nil {
			corpus.failures = append(corpus.failures, fmt.Sprintf("default-branch spec %s failed to decode: %v", path, decodeErr))
			continue
		}
		if fm.Supersession == nil {
			continue // no validated supersession: entry — never a successor
		}
		for _, l := range fm.Links {
			if l.Type != artifact.LinkSupersedes {
				continue
			}
			ref, parseErr := artifact.ParseRef(l.Ref)
			if parseErr != nil || ref.Kind != artifact.KindSpec {
				continue
			}
			corpus.supersedesBy[ref.Name] = append(corpus.supersedesBy[ref.Name], path)
		}
	}

	sort.Strings(corpus.failures)
	for name := range corpus.supersedesBy {
		sort.Strings(corpus.supersedesBy[name])
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

	excluded := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		excluded[c.Path] = true
	}

	var corpus *successorCorpus
	getCorpus := func() (*successorCorpus, error) {
		if corpus == nil {
			built, err := p.scanSuccessors(ctx, root, branch, excluded)
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
	// check supersession before falling back to the (now purely
	// informational) legacy status field.
	corpus, err := getCorpus()
	if err != nil {
		return Result{}, err
	}
	if successors := corpus.supersedesBy[name]; len(successors) > 0 {
		return Result{State: Superseded, Relation: RelationExact, Baseline: baseline}, nil
	}
	if len(corpus.failures) > 0 {
		disclosures := make([]string, 0, len(corpus.failures)+1)
		disclosures = append(disclosures, fmt.Sprintf(
			// vocab:identity — machinery diagnostic naming the lifecycle state this scan could not rule out
			"specstate: %s cannot be proven not-superseded — the default-branch active-spec scan is incomplete",
			c.Path,
		))
		disclosures = append(disclosures, corpus.failures...)
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

// probeLegacyStatus tolerantly reads a spec document's legacy frontmatter
// status: field through internal/artifact.DecodeSpec — this package's
// only sanctioned YAML decode seam (CLAUDE.md: "YAML via the single
// internal/artifact seam"; internal/specalign's import-seam witness
// enforces that no other package imports gopkg.in/yaml.v3 directly). Any
// decode failure — including a spec that omits status entirely, which
// internal/artifact.DecodeSpec cannot yet accept ahead of the sibling task
// in this same plan that makes the field optional — is read as "no legacy
// status available" rather than propagated: status is used here only to
// produce an optional compatibility disclosure, never to gate the
// git-derived verdict itself, so a decode failure here must never block
// or alter the state this package already proved from Git alone.
func probeLegacyStatus(content []byte) string {
	fm, err := artifact.DecodeSpec(content)
	if err != nil {
		return ""
	}
	return string(fm.Status)
}
