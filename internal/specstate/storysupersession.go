package specstate

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strconv"
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

// isConflictFile reports whether p, repository-relative, is a conflict
// file as artifact.ClassifyPath (and so lint) classifies one: any .md under
// conflictsDir, at any depth, including an empty-named one. Any other entry
// — a non-.md file there, or anything outside it — is not a conflict file
// and is not read.
func isConflictFile(p string) bool {
	rel, ok := strings.CutPrefix(p, path.Dir(conflictsDir())+"/")
	if !ok {
		return false
	}
	kind, ok := artifact.ClassifyPath(rel)
	return ok && kind == string(artifact.KindConflict)
}

// isConflictRecordPath reports whether p is where a conflict record sits:
// a direct <name>.md child of conflictsDir (store.ConflictPath). A conflict
// file anywhere else — nested, or empty-named — is a scan failure, never
// read (SI-306 (4a)).
func isConflictRecordPath(p string) bool {
	name, ok := strings.CutSuffix(path.Base(p), ".md")
	return ok && name != "" && p == filepath.ToSlash(store.ConflictPath("", name))
}

// isGitQuoted reports whether a plain `git ls-tree` listing C-quoted p.
// Git wraps a listed path in double quotes whenever it escapes a byte of
// it (a double quote, a backslash, a control character, or, under
// core.quotePath, a non-ASCII byte), and a path it lists unquoted never
// begins with a double quote.
func isGitQuoted(p string) bool {
	return strings.HasPrefix(p, `"`)
}

// isQuotedConflictFile reports whether a git-quoted listing entry names a
// conflict file (isConflictFile) under its real name. Git's C-quoting —
// octal byte escapes and backslash escapes of a quote, a backslash, or a
// control character — is Go string-literal syntax, so strconv.Unquote
// recovers the real name. An entry that does not unquote fails closed: it
// may be a conflict file, so the scan cannot rule it out (review RR1).
func isQuotedConflictFile(listed string) bool {
	name, err := strconv.Unquote(listed)
	return err != nil || isConflictFile(name)
}

// isRung3Story reports whether a decoded spec is in SI-290's scope: class
// story and not a spike.
func isRung3Story(fm *artifact.SpecFrontmatter) bool {
	return fm != nil && fm.Class == artifact.ClassStory && !fm.Spike
}

// scanConflicts reads and strict-decodes, through internal/artifact, every
// conflict file in the tree at rev — the same revision scanSuccessors
// reads the spec zones at, so a corpus, and the cache entry keyed on its
// commit, always covers exactly that commit's conflict set. Every failure
// is recorded in conflictFailures under its path — a scan failure, never a
// skipped file: a conflict file the plain listing C-quotes cannot be read
// by its real name, so it is never read and is named as listed (review
// SS-R1; a NUL-terminated listing would widen lint's git reads beyond what
// internal/disclosureview's cache-key guard pins), while a quoted entry
// that is not a conflict file is not read at all, like any other
// non-conflict entry (review RR1); a conflict file that is not at a
// conflict record path is never read (SI-306 (4a)); and a record that
// fails strict decode is never credited. A superseded conflict is credited
// to every spec its challenges links name as a whole spec. An operational
// read failure is an error.
func (p Projector) scanConflicts(ctx context.Context, root, rev string, corpus *successorCorpus) error {
	paths, err := p.git.LsTree(ctx, root, rev, conflictsDir())
	if err != nil {
		return fmt.Errorf("specstate: scanning default-branch conflicts: %w", err)
	}
	sort.Strings(paths)
	for _, cp := range paths {
		if isGitQuoted(cp) {
			if !isQuotedConflictFile(cp) {
				continue
			}
			corpus.conflictFailures[cp] = fmt.Sprintf("default-branch conflicts entry %s is listed git-quoted, so the scan cannot read it by its real name — never read, never credited", cp)
			continue
		}
		if !isConflictFile(cp) {
			continue
		}
		if !isConflictRecordPath(cp) {
			corpus.conflictFailures[cp] = fmt.Sprintf("default-branch conflict file %s is not at a conflict record path (%s/<name>.md) — never read, never credited", cp, conflictsDir())
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
//     Unproven (SI-291): each story successor with no resolved conflict
//     (claiming only what the conflict scan could decode when it is
//     incomplete); each resolved conflict when no story successor exists;
//     each other link-only successor (a feature or spike naming a story)
//     with today's missing-block disclosure; when a conflict failed strict
//     decode, the incomplete conflict scan and every failure (SI-304);
//     and, when a spec failed strict decode, the incomplete spec scan and
//     every failure, as resolveOne's fallback would report them (review
//     SS-R2). An incomplete spec scan cannot prove that no story successor
//     exists, so a resolved conflict's disclosure then claims only that no
//     spec the scan could decode names the predecessor.
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

	conflictScanComplete := len(c.conflictFailures) == 0
	for _, succ := range c.linkOnlySupersessorsFor(candidatePath, name) {
		if !c.rung3Stories[succ] {
			disclosures = append(disclosures, linkOnlyDisclosure(candidatePath, succ))
			continue
		}
		disclosures = append(disclosures, missingResolvedConflictDisclosure(candidatePath, succ, conflictScanComplete))
	}
	specFailures := c.failuresExcluding(candidatePath)
	if len(stories) == 0 {
		for _, conflict := range resolved {
			disclosures = append(disclosures, missingSuccessorDisclosure(candidatePath, conflict, len(specFailures) == 0))
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
	if len(specFailures) > 0 {
		disclosures = append(disclosures, specScanIncompleteDisclosure(candidatePath))
		disclosures = append(disclosures, specFailures...)
	}
	return Result{State: Unproven, Relation: RelationUnproven, Disclosures: disclosures}, true
}

// missingResolvedConflictDisclosure names a story successor whose
// whole-spec supersedes edge names candidatePath when no superseded
// conflict challenges the whole spec. With a complete conflict scan the
// negative is proven; with an incomplete one it covers only the conflicts
// the scan could decode, and says so (SS-R2's conflict-scan analog).
func missingResolvedConflictDisclosure(candidatePath, succ string, conflictScanComplete bool) string {
	if conflictScanComplete {
		return fmt.Sprintf(
			// vocab:identity — machinery diagnostic naming the frontmatter link type, the conflict status field, and the lifecycle states involved
			"specstate: %s is named as a predecessor by the story %s via a whole-spec links: supersedes edge, but no conflict with status: superseded challenges the whole spec on the default branch — story supersession needs both rung-3 records; reported unproven with this disclosure, never silently accepted-pending-build",
			candidatePath, succ,
		)
	}
	return fmt.Sprintf(
		// vocab:identity — machinery diagnostic naming the frontmatter link type, the conflict status field, and the lifecycle states involved
		"specstate: %s is named as a predecessor by the story %s via a whole-spec links: supersedes edge, but no conflict the default-branch scan could decode has status: superseded and challenges the whole spec, and that scan is incomplete, so a resolved conflict among the entries it could not decode is not ruled out — story supersession needs both rung-3 records; reported unproven with this disclosure, never silently accepted-pending-build",
		candidatePath, succ,
	)
}

// missingSuccessorDisclosure names a superseded conflict that challenges
// candidatePath as a whole spec when no story successor names it. With a
// complete spec scan the negative is proven; with an incomplete one it
// covers only the specs the scan could decode, and says so (review SS-R2).
func missingSuccessorDisclosure(candidatePath, conflict string, specScanComplete bool) string {
	if specScanComplete {
		return fmt.Sprintf(
			// vocab:identity — machinery diagnostic naming the conflict status field, the frontmatter link type, and the lifecycle states involved
			"specstate: %s is challenged as a whole spec by %s, a conflict with status: superseded, but no story spec on the default branch names it via a whole-spec links: supersedes edge — story supersession needs both rung-3 records; reported unproven with this disclosure, never silently accepted-pending-build",
			candidatePath, conflict,
		)
	}
	return fmt.Sprintf(
		// vocab:identity — machinery diagnostic naming the conflict status field, the frontmatter link type, and the lifecycle states involved
		"specstate: %s is challenged as a whole spec by %s, a conflict with status: superseded, but no story spec the default-branch scan could decode names it via a whole-spec links: supersedes edge, and that scan is incomplete, so a successor among the specs it could not decode is not ruled out — story supersession needs both rung-3 records; reported unproven with this disclosure, never silently accepted-pending-build",
		candidatePath, conflict,
	)
}
