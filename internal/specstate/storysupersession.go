package specstate

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// Story supersession derived from the rung-3 records (SI-290, SI-291,
// SI-304; design docs/superpowers/specs/2026-09-29-story-supersession-
// proof-design.md; 02 §Kind registry and 03 §The amendment ladder rung 3 as
// ratified by PR #376). A story cannot carry the supersession: block that
// proves a feature's supersession (internal/artifact's validateStory), so
// an active-zone story predecessor is superseded when the default branch
// carries both records rung 3 already requires: a story successor whose
// top-level links: carry a whole-spec supersedes edge to it, and a conflict
// with status: superseded whose challenges links name the whole
// predecessor. Either record alone, or an unreadable conflict, leaves the
// predecessor disclosed-unproven. Both records are read from the same
// default-branch tree as every other lifecycle fact; nothing is ever
// written to the predecessor. A spike (class: story, spike: true) is out of
// scope on both sides and keeps today's behavior.

// conflictsDir returns the store directory the conflict scan reads on the
// default branch (.verdi/conflicts), derived from internal/store's layout.
func conflictsDir() string {
	return path.Dir(filepath.ToSlash(store.ConflictPath("", "x")))
}

// isConflictRecordPath reports whether p is where a conflict record sits:
// a direct <name>.md child of conflictsDir (store.ConflictPath). Any
// other entry the listing returns — a nested file, a non-.md file — is not
// a conflict record and is not read.
func isConflictRecordPath(p string) bool {
	name, ok := strings.CutSuffix(path.Base(p), ".md")
	return ok && name != "" && p == filepath.ToSlash(store.ConflictPath("", name))
}

// isRung3Story reports whether a decoded spec is in SI-290's scope: class
// story and not a spike.
func isRung3Story(fm *artifact.SpecFrontmatter) bool {
	return fm != nil && fm.Class == artifact.ClassStory && !fm.Spike
}

// scanConflicts reads and strict-decodes, through internal/artifact, every
// conflict record in the tree at rev — the same revision scanSuccessors
// reads the spec zones at, so a corpus, and the cache entry keyed on its
// commit, always covers exactly that commit's conflict set. The tree is
// listed NUL-terminated (LsTreeEntries), so a file name a plain listing
// would C-quote (a non-ASCII byte, a double quote, a backslash, a control
// character) is read by its real name, never skipped (review SS-R1). A
// conflict that fails strict decode is recorded in conflictFailures under
// its path (a scan failure, never a skipped file); a superseded conflict is
// credited to every spec its challenges links name as a whole spec. An
// operational read failure is an error.
func (p Projector) scanConflicts(ctx context.Context, root, rev string, corpus *successorCorpus) error {
	entries, err := p.git.LsTreeEntries(ctx, root, rev)
	if err != nil {
		return fmt.Errorf("specstate: scanning default-branch conflicts: %w", err)
	}
	var paths []string
	for _, e := range entries {
		if strings.HasPrefix(e.Path, conflictsDir()+"/") {
			paths = append(paths, e.Path)
		}
	}
	sort.Strings(paths)
	for _, cp := range paths {
		if !isConflictRecordPath(cp) {
			continue
		}
		content, err := p.git.Show(ctx, root, rev, cp)
		if err != nil {
			return fmt.Errorf("specstate: reading default-branch conflict %s: %w", cp, err)
		}
		fm, err := decodeConflictDocument(content)
		if err != nil {
			corpus.conflictFailures[cp] = fmt.Sprintf("default-branch conflict %s failed to decode: %v", cp, err)
			continue
		}
		if fm.Status != artifact.Status("superseded") {
			continue // open or dismissed: not the resolved record
		}
		for _, name := range wholeSpecChallengeNames(fm.Links) {
			corpus.resolvedBy[name] = append(corpus.resolvedBy[name], cp)
		}
	}
	return nil
}

// decodeConflictDocument splits a conflict file's frontmatter and strict-
// decodes it through internal/artifact.
func decodeConflictDocument(content []byte) (*artifact.ConflictFrontmatter, error) {
	rawFM, _, err := artifact.SplitFrontmatter(content)
	if err != nil {
		return nil, err
	}
	return artifact.DecodeConflict(rawFM)
}

// wholeSpecChallengeNames returns, once each and in link order, the bare
// names of the specs a conflict's challenges links name as a WHOLE spec. A
// challenges edge naming an object fragment (spec/x#dc-1) is the
// closed-spec object supersession route (SI-259..SI-265) and is excluded;
// so is any non-spec target. Like artifact.WholeSpecSupersedesRefs on the
// successor's side, a pinned whole-spec ref still names the whole spec.
func wholeSpecChallengeNames(links []artifact.Link) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range links {
		if l.Type != artifact.LinkChallenges {
			continue
		}
		ref, err := artifact.ParseRef(l.Ref)
		if err != nil || ref.Kind != artifact.KindSpec || ref.Fragment() || seen[ref.Name] {
			continue
		}
		seen[ref.Name] = true
		out = append(out, ref.Name)
	}
	return out
}

// conflictFailureMessages returns every conflict decode witness, sorted.
func (c *successorCorpus) conflictFailureMessages() []string {
	out := make([]string, 0, len(c.conflictFailures))
	for _, msg := range c.conflictFailures {
		out = append(out, msg)
	}
	sort.Strings(out)
	return out
}

// storyVerdict projects an active-zone story predecessor from the rung-3
// records, for a candidate whose exact bytes are landed and whose state no
// earlier rule settled (the supersession: block route, its own decode
// failure, a legacy explicit terminal status — I-40 still wins first).
// decided is false only when nothing names or challenges the predecessor
// and the conflict scan is complete; the caller then continues with the
// spec-scan fallback as before.
//
//   - Proven (SI-290): ≥1 story successor's whole-spec supersedes edge and
//     ≥1 superseded whole-spec conflict — Superseded, one disclosure per
//     successor naming it and the resolved conflict (rung 3's
//     decomposition names every successor). A proven supersession is
//     settled: an unreadable conflict cannot undo it (SI-304).
//   - Otherwise every present record is disclosed and the state is
//     Unproven (SI-291): each story successor with no resolved conflict;
//     each resolved conflict when no story successor exists; each other
//     link-only successor (a feature or spike naming a story) with today's
//     missing-block disclosure; and, when a conflict failed strict decode,
//     the incomplete conflict scan and every failure (SI-304).
func (c *successorCorpus) storyVerdict(candidatePath, name string, baseline *Baseline) (Result, bool) {
	var stories []string
	var disclosures []string
	resolved := c.resolvedBy[name]
	for _, succ := range c.linkOnlySupersessorsFor(candidatePath, name) {
		if c.rung3Stories[succ] {
			stories = append(stories, succ)
		}
	}
	if len(stories) > 0 && len(resolved) > 0 {
		for _, succ := range stories {
			disclosures = append(disclosures, fmt.Sprintf(
				// vocab:identity — machinery diagnostic naming the lifecycle state, the frontmatter link type, and the conflict status field
				"specstate: %s is superseded by %s — derived from both rung-3 records on the default branch: that story's whole-spec links: supersedes edge, and a conflict with status: superseded challenging the whole spec (%s); nothing is written to the predecessor",
				candidatePath, succ, strings.Join(resolved, ", "),
			))
		}
		return Result{State: Superseded, Relation: RelationExact, Baseline: baseline, Disclosures: disclosures}, true
	}

	for _, succ := range c.linkOnlySupersessorsFor(candidatePath, name) {
		if !c.rung3Stories[succ] {
			disclosures = append(disclosures, linkOnlyDisclosure(candidatePath, succ))
			continue
		}
		disclosures = append(disclosures, fmt.Sprintf(
			// vocab:identity — machinery diagnostic naming the frontmatter link type, the conflict status field, and the lifecycle states involved
			"specstate: %s is named as a predecessor by the story %s via a whole-spec links: supersedes edge, but no conflict with status: superseded challenges the whole spec on the default branch — story supersession needs both rung-3 records; reported unproven with this disclosure, never silently accepted-pending-build",
			candidatePath, succ,
		))
	}
	if len(stories) == 0 {
		for _, conflict := range resolved {
			disclosures = append(disclosures, fmt.Sprintf(
				// vocab:identity — machinery diagnostic naming the conflict status field, the frontmatter link type, and the lifecycle states involved
				"specstate: %s is challenged as a whole spec by %s, a conflict with status: superseded, but no story spec on the default branch names it via a whole-spec links: supersedes edge — story supersession needs both rung-3 records; reported unproven with this disclosure, never silently accepted-pending-build",
				candidatePath, conflict,
			))
		}
	}
	if failures := c.conflictFailureMessages(); len(failures) > 0 {
		disclosures = append(disclosures, fmt.Sprintf(
			// vocab:identity — machinery diagnostic naming the lifecycle state this scan could not rule out
			"specstate: %s cannot be proven not-superseded — the default-branch conflict scan is incomplete",
			candidatePath,
		))
		disclosures = append(disclosures, failures...)
	}
	if len(disclosures) == 0 {
		return Result{}, false
	}
	return Result{State: Unproven, Relation: RelationUnproven, Disclosures: disclosures}, true
}
