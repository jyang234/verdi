package specstate

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// Story supersession derived from the rung-3 records (SI-290, SI-291,
// SI-304; design docs/superpowers/specs/2026-09-29-story-supersession-
// proof-design.md §2, §3, §6). Every row lands its tree on a real
// fixturegit repository's default branch and resolves one candidate over
// the real git plumbing, so each proof reads exactly what `git` holds.

const (
	ssFeaturePath    = ".verdi/specs/active/ss-feature/spec.md"
	ssFeatureV2Path  = ".verdi/specs/active/ss-feature-v2/spec.md"
	ssV1Path         = ".verdi/specs/active/ss-story/spec.md"
	ssV2Path         = ".verdi/specs/active/ss-story-v2/spec.md"
	ssV2ArchivePath  = ".verdi/specs/archive/ss-story-v2b/spec.md"
	ssSpikePath      = ".verdi/specs/active/ss-spike/spec.md"
	ssConflictPath   = ".verdi/conflicts/ss-story-wrong.md"
	ssGarbledPath    = ".verdi/conflicts/ss-garbled.md"
	ssBrokenSpecPath = ".verdi/specs/active/ss-broken/spec.md"
	ssFrozenStamp    = "frozen: { at: \"2026-09-29\", commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }\n"
	ssMissingRecord  = "no conflict with status: superseded challenges the whole spec"
	ssMissingSuccess = "no story spec on the default branch names it via a whole-spec links: supersedes edge"
	ssScanIncomplete = "the default-branch conflict scan is incomplete"
	ssOldLinkOnly    = "carries no validatable supersession: block"

	// SS-R1: conflict file names a plain `git ls-tree` C-quotes (a
	// non-ASCII byte, a double quote), which the scan must read by their
	// real names.
	ssNonASCIIPath = ".verdi/conflicts/résumé.md"
	ssQuotePath    = ".verdi/conflicts/a\"b.md"

	// SS-R2: the spec scan's own incompleteness, and the "no successor"
	// wording an incomplete spec scan can still prove.
	ssSpecScanIncomplete   = "the default-branch active-spec scan is incomplete"
	ssMissingSuccessHedged = "no story spec the default-branch scan could decode names it via a whole-spec links: supersedes edge"

	// SS-R3 (SI-306 (4a)): conflict files store layout never places —
	// nested, or empty-named — are scan failures, never read.
	ssNestedPath    = ".verdi/conflicts/sub/x.md"
	ssEmptyNamePath = ".verdi/conflicts/.md"
	ssNotRecord     = "is not at a conflict record path"
)

// ssStory is a schema-valid story spec implementing ss-feature#ac-1, with
// header spliced into its frontmatter and every extra link appended.
func ssStory(name, header string, links ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: spec/%s\nkind: spec\nclass: story\n%stitle: %s\nowners: [platform]\nstory: jira:SS-1\nproblem: { text: x, anchor: problem }\noutcome: { text: y, anchor: outcome }\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\nlinks:\n  - { type: implements, ref: \"spec/ss-feature#ac-1\" }\n", name, header, name)
	for _, l := range links {
		fmt.Fprintf(&b, "  - %s\n", l)
	}
	b.WriteString("---\nbody\n")
	return b.String()
}

// ssSpike is a schema-valid spike (class: story, spike: true) resolving
// ss-feature#oq-1, with every extra link appended.
func ssSpike(name string, links ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: spec/%s\nkind: spec\nclass: story\nspike: true\ntitle: %s\nowners: [platform]\nstory: jira:SS-2\nproblem: { text: x, anchor: problem }\noutcome: { text: y, anchor: outcome }\nlinks:\n  - { type: resolves, ref: \"spec/ss-feature#oq-1\" }\n", name, name)
	for _, l := range links {
		fmt.Fprintf(&b, "  - %s\n", l)
	}
	b.WriteString("---\nbody\n")
	return b.String()
}

// ssFeature is a schema-valid feature spec with every extra link appended.
func ssFeature(name string, links ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: spec/%s\nkind: spec\nclass: feature\ntitle: %s\nowners: [platform]\nacceptance_criteria:\n  - { id: ac-1, text: works, evidence: [static] }\n", name, name)
	if len(links) > 0 {
		b.WriteString("links:\n")
		for _, l := range links {
			fmt.Fprintf(&b, "  - %s\n", l)
		}
	}
	b.WriteString("---\nbody\n")
	return b.String()
}

// ssSupersedes is a whole-spec supersedes edge to name.
func ssSupersedes(name string) string {
	return fmt.Sprintf("{ type: supersedes, ref: \"spec/%s\" }", name)
}

// ssConflict is a schema-valid conflict with the given status, one
// challenges link per ref, and a frozen stamp when resolved; extra is
// spliced into the frontmatter (resolved_by).
func ssConflict(name, status, extra string, challenges ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: conflict/%s\nkind: conflict\ntitle: %s\nstatus: %s\nowners: [platform]\n%s", name, name, status, extra)
	if status != "open" {
		b.WriteString(ssFrozenStamp)
	}
	b.WriteString("links:\n")
	for _, ref := range challenges {
		fmt.Fprintf(&b, "  - { type: challenges, ref: \"%s\" }\n", ref)
	}
	b.WriteString("---\nwitness\n")
	return b.String()
}

// ssLand builds a fixturegit repository whose default branch ("main",
// pinned through CI_DEFAULT_BRANCH) holds exactly tree.
func ssLand(t *testing.T, tree map[string]string) *fixturegit.Repo {
	t.Helper()
	files := map[string]string{"seed.txt": "seed\n"}
	for p, c := range tree {
		files[p] = c
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "land the rung-3 records"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	return repo
}

// ssWant is one expected disclosure: every substring must appear in it.
type ssWant []string

// ssAssert checks result's state, its baseline on a proven exact state,
// and that its disclosures match want one for one, in order.
func ssAssert(t *testing.T, candidate string, result Result, wantState State, want []ssWant) {
	t.Helper()
	if result.State != wantState {
		t.Fatalf("Resolve(%s).State = %s, want %s (result %+v)", candidate, result.State, wantState, result)
	}
	switch wantState {
	case Superseded, AcceptedPendingBuild, Closed:
		if result.Relation != RelationExact || result.Baseline == nil || result.Baseline.Path != candidate || result.Baseline.Blob == "" || result.Baseline.LandingCommit == "" {
			t.Fatalf("Resolve(%s) = %+v, want exact relation and a complete baseline for its own path", candidate, result)
		}
	case Unproven:
		if result.Relation != RelationUnproven || result.Baseline != nil {
			t.Fatalf("Resolve(%s) = %+v, want unproven relation and no baseline", candidate, result)
		}
	}
	if len(result.Disclosures) != len(want) {
		t.Fatalf("Resolve(%s).Disclosures = %q (%d), want %d", candidate, result.Disclosures, len(result.Disclosures), len(want))
	}
	for i, subs := range want {
		for _, sub := range subs {
			if !strings.Contains(result.Disclosures[i], sub) {
				t.Fatalf("Resolve(%s).Disclosures[%d] = %q, want it to contain %q", candidate, i, result.Disclosures[i], sub)
			}
		}
	}
}

// TestProjector_StorySupersession covers design §6 cases 1-9 and SI-304's
// precedence rows over real git.
func TestProjector_StorySupersession(t *testing.T) {
	v1 := ssStory("ss-story", "")
	v2 := ssStory("ss-story-v2", "", ssSupersedes("ss-story"))
	v2b := ssStory("ss-story-v2b", "status: closed\n"+ssFrozenStamp, ssSupersedes("ss-story"))
	resolved := ssConflict("ss-story-wrong", "superseded", "", "spec/ss-story")
	garbled := "---\nid: conflict/ss-garbled\nkind: conflict\ntitle: garbled\nstatus: superseded\nowners: [platform]\nlinks:\n  - { type: challenges, ref: \"spec/ss-story\" }\n---\n"
	legacySuperseded := ssStory("ss-story", "status: superseded\n"+ssFrozenStamp)
	legacyClosed := ssStory("ss-story", "status: closed\n"+ssFrozenStamp)
	legacyAccepted := ssStory("ss-story", "status: accepted-pending-build\n"+ssFrozenStamp)
	feature := ssFeature("ss-feature")
	// v2Broken carries v2's edge but fails strict decode (an unknown
	// field), so the spec scan is incomplete and cannot see the edge.
	v2Broken := ssStory("ss-story-v2", "bogus_field: nope\n", ssSupersedes("ss-story"))
	brokenSpec := "---\nid: spec/ss-broken\nkind: spec\nbogus_field: nope\n---\nbody\n"

	tests := []struct {
		name      string
		tree      map[string]string
		candidate string
		wantState State
		want      []ssWant
		absent    []string // no disclosure may contain any of these
	}{
		// Case 1.
		{
			name:      "case 1: story v2's edge and a superseded whole-spec conflict: superseded, naming v2 and the conflict",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssConflictPath: resolved},
			candidate: ssV1Path,
			wantState: Superseded,
			want:      []ssWant{{ssV1Path, "superseded by " + ssV2Path, ssConflictPath}},
		},
		{
			name:      "case 1: the successor itself stays accepted-pending-build",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssConflictPath: resolved},
			candidate: ssV2Path,
			wantState: AcceptedPendingBuild,
		},
		{
			name:      "case 1: a successor already closed into the archive zone still supersedes (discovery covers both zones)",
			tree:      map[string]string{ssV1Path: v1, ssV2ArchivePath: v2b, ssConflictPath: resolved},
			candidate: ssV1Path,
			wantState: Superseded,
			want:      []ssWant{{"superseded by " + ssV2ArchivePath, ssConflictPath}},
		},
		// Case 2.
		{
			name:      "case 2: decomposition, two successors and one conflict: superseded, both successors named",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssV2ArchivePath: v2b, ssConflictPath: resolved},
			candidate: ssV1Path,
			wantState: Superseded,
			want: []ssWant{
				{"superseded by " + ssV2Path, ssConflictPath},
				{"superseded by " + ssV2ArchivePath, ssConflictPath},
			},
		},
		// Case 3.
		{
			name:      "case 3: edge with no conflict: unproven, naming the successor and the missing resolved conflict",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV1Path, ssV2Path, ssMissingRecord}},
		},
		{
			name:      "case 3: edge with an open conflict: unproven, naming the missing resolved conflict",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssConflictPath: ssConflict("ss-story-wrong", "open", "", "spec/ss-story")},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV2Path, ssMissingRecord}},
		},
		{
			name:      "case 3: edge with a dismissed conflict: unproven, naming the missing resolved conflict",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssConflictPath: ssConflict("ss-story-wrong", "dismissed", "", "spec/ss-story")},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV2Path, ssMissingRecord}},
		},
		// Case 4.
		{
			name:      "case 4: superseded conflict with no successor: unproven, naming the conflict and the missing successor",
			tree:      map[string]string{ssV1Path: v1, ssConflictPath: resolved},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV1Path, ssConflictPath, ssMissingSuccess}},
		},
		// Case 5.
		{
			name:      "case 5: a superseded conflict challenging only a fragment of v1 is not proof",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssConflictPath: ssConflict("ss-story-wrong", "superseded", "resolved_by: spec/ss-story-v2\n", "spec/ss-story#ac-1")},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV2Path, ssMissingRecord}},
		},
		{
			name:      "case 5: a fragment-only conflict with no edge is not read at all",
			tree:      map[string]string{ssV1Path: v1, ssConflictPath: ssConflict("ss-story-wrong", "superseded", "resolved_by: spec/ss-story-v2\n", "spec/ss-story#ac-1")},
			candidate: ssV1Path,
			wantState: AcceptedPendingBuild,
		},
		// Case 6.
		{
			name:      "case 6: a conflict that fails strict decode: unproven, naming the failure",
			tree:      map[string]string{ssV1Path: v1, ssGarbledPath: garbled},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV1Path, ssScanIncomplete}, {ssGarbledPath, "failed to decode"}},
		},
		{
			name:      "case 6: a conflict with no frontmatter delimiters: unproven, naming the failure",
			tree:      map[string]string{ssV1Path: v1, ssGarbledPath: "no frontmatter here\n"},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssScanIncomplete}, {ssGarbledPath, "failed to decode"}},
		},
		// SI-304 precedence.
		{
			name:      "SI-304: an unreadable conflict does not unsettle a proven supersession",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssConflictPath: resolved, ssGarbledPath: garbled},
			candidate: ssV1Path,
			wantState: Superseded,
			want:      []ssWant{{"superseded by " + ssV2Path, ssConflictPath}},
		},
		{
			name:      "SI-304: an unreadable conflict does not unsettle a legacy explicit superseded status",
			tree:      map[string]string{ssV1Path: legacySuperseded, ssGarbledPath: garbled},
			candidate: ssV1Path,
			wantState: Superseded,
			want:      []ssWant{{"legacy status: superseded"}},
		},
		{
			name:      "SI-304: an unreadable conflict does not unsettle a legacy explicit closed status",
			tree:      map[string]string{ssV1Path: legacyClosed, ssGarbledPath: garbled},
			candidate: ssV1Path,
			wantState: Closed,
			want:      []ssWant{{"legacy status: closed"}},
		},
		{
			name:      "SI-304: a legacy non-terminal status settles nothing",
			tree:      map[string]string{ssV1Path: legacyAccepted, ssGarbledPath: garbled},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssScanIncomplete}, {ssGarbledPath}},
		},
		{
			name:      "SI-304: disclosures accumulate, naming the missing resolved conflict and the unreadable one",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssConflictPath: ssConflict("ss-story-wrong", "open", "", "spec/ss-story"), ssGarbledPath: garbled},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV2Path, ssMissingRecord}, {ssScanIncomplete}, {ssGarbledPath, "failed to decode"}},
		},
		{
			name:      "SI-304: a feature is unaffected by an unreadable conflict",
			tree:      map[string]string{ssFeaturePath: feature, ssGarbledPath: garbled},
			candidate: ssFeaturePath,
			wantState: AcceptedPendingBuild,
		},
		{
			name:      "SI-304: a spike is unaffected by an unreadable conflict",
			tree:      map[string]string{ssSpikePath: ssSpike("ss-spike"), ssGarbledPath: garbled},
			candidate: ssSpikePath,
			wantState: AcceptedPendingBuild,
		},
		// Case 8.
		{
			name:      "case 8: a legacy explicit superseded predecessor with an edge and an open conflict is unchanged",
			tree:      map[string]string{ssV1Path: legacySuperseded, ssV2ArchivePath: v2b, ssConflictPath: ssConflict("ss-story-wrong", "open", "", "spec/ss-story")},
			candidate: ssV1Path,
			wantState: Superseded,
			want:      []ssWant{{"legacy status: superseded"}},
		},
		{
			name:      "case 8: the legacy explicit status still wins first over a proven supersession",
			tree:      map[string]string{ssV1Path: legacySuperseded, ssV2Path: v2, ssConflictPath: resolved},
			candidate: ssV1Path,
			wantState: Superseded,
			want:      []ssWant{{"legacy status: superseded"}},
		},
		// Case 9.
		{
			name:      "case 9: a feature with a link-only feature successor and a superseded conflict keeps today's disclosure",
			tree:      map[string]string{ssFeaturePath: feature, ssFeatureV2Path: ssFeature("ss-feature-v2", ssSupersedes("ss-feature")), ".verdi/conflicts/ss-feature-wrong.md": ssConflict("ss-feature-wrong", "superseded", "", "spec/ss-feature")},
			candidate: ssFeaturePath,
			wantState: Unproven,
			want:      []ssWant{{ssFeatureV2Path, ssOldLinkOnly}},
		},
		{
			name:      "case 9: a feature challenged by a superseded conflict with no successor is unchanged",
			tree:      map[string]string{ssFeaturePath: feature, ".verdi/conflicts/ss-feature-wrong.md": ssConflict("ss-feature-wrong", "superseded", "", "spec/ss-feature")},
			candidate: ssFeaturePath,
			wantState: AcceptedPendingBuild,
		},
		{
			name:      "case 9: a feature superseded by the block route is unchanged",
			tree:      map[string]string{ssFeaturePath: feature, ssFeatureV2Path: string(validSuccessorSpec("ss-feature-v2", "ss-feature"))},
			candidate: ssFeaturePath,
			wantState: Superseded,
		},
		// Scope edges.
		{
			name:      "a spike successor is not a rung-3 story successor",
			tree:      map[string]string{ssV1Path: v1, ssSpikePath: ssSpike("ss-spike", ssSupersedes("ss-story")), ssConflictPath: resolved},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssSpikePath, ssOldLinkOnly}, {ssConflictPath, ssMissingSuccess}},
		},
		{
			name:      "a link-only feature successor of a story is not a rung-3 story successor",
			tree:      map[string]string{ssV1Path: v1, ssFeatureV2Path: ssFeature("ss-feature-v2", ssSupersedes("ss-story")), ssConflictPath: resolved},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssFeatureV2Path, ssOldLinkOnly}, {ssConflictPath, ssMissingSuccess}},
		},
		{
			name:      "a spike predecessor keeps today's link-only disclosure",
			tree:      map[string]string{ssSpikePath: ssSpike("ss-spike"), ssV2Path: ssStory("ss-story-v2", "", ssSupersedes("ss-spike")), ".verdi/conflicts/ss-spike-wrong.md": ssConflict("ss-spike-wrong", "superseded", "", "spec/ss-spike")},
			candidate: ssSpikePath,
			wantState: Unproven,
			want:      []ssWant{{ssV2Path, ssOldLinkOnly}},
		},
		{
			name:      "a spike predecessor challenged by a superseded conflict alone is unchanged",
			tree:      map[string]string{ssSpikePath: ssSpike("ss-spike"), ".verdi/conflicts/ss-spike-wrong.md": ssConflict("ss-spike-wrong", "superseded", "", "spec/ss-spike")},
			candidate: ssSpikePath,
			wantState: AcceptedPendingBuild,
		},
		{
			name:      "a superseded conflict challenging another spec has no effect",
			tree:      map[string]string{ssV1Path: v1, ssConflictPath: ssConflict("ss-story-wrong", "superseded", "", "spec/ss-other")},
			candidate: ssV1Path,
			wantState: AcceptedPendingBuild,
		},
		{
			name:      "a pinned whole-spec challenge names the whole predecessor",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssConflictPath: ssConflict("ss-story-wrong", "superseded", "", "spec/ss-story@abcdef0")},
			candidate: ssV1Path,
			wantState: Superseded,
			want:      []ssWant{{"superseded by " + ssV2Path, ssConflictPath}},
		},
		{
			// SI-306 (4a): a nested .md under the conflicts directory is a
			// conflict file (artifact.ClassifyPath) that store layout never
			// places there — a scan failure naming it, never read. A non-.md
			// entry there, and every path outside it, is not a conflict file.
			name: "a nested conflict file is a scan failure; entries that are not conflict files are not read",
			tree: map[string]string{
				ssV1Path:                         v1,
				".verdi/conflicts/notes.txt":     "not a record\n",
				".verdi/conflicts/nested/x.md":   "not a record either\n",
				".verdi/conflicts-extra/y.md":    "outside the directory\n",
				".verdi/specs/active/x/notes.md": "a spec-zone file, not a conflict\n",
			},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV1Path, ssScanIncomplete}, {".verdi/conflicts/nested/x.md", ssNotRecord}},
			absent:    []string{"notes.txt", "conflicts-extra", "specs/active/x/notes.md"},
		},
		// SS-R1: a conflict at a name plain `git ls-tree` C-quotes is read
		// by its real name.
		{
			name:      "SS-R1: a malformed conflict at a non-ASCII name: unproven, naming it",
			tree:      map[string]string{ssV1Path: v1, ssNonASCIIPath: garbled},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV1Path, ssScanIncomplete}, {ssNonASCIIPath, "failed to decode"}},
		},
		{
			name:      "SS-R1: a malformed conflict at a name holding a double quote: unproven, naming it",
			tree:      map[string]string{ssV1Path: v1, ssQuotePath: garbled},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV1Path, ssScanIncomplete}, {ssQuotePath, "failed to decode"}},
		},
		{
			name:      "SS-R1: a superseded conflict at a non-ASCII name plus v2's edge: superseded, naming it",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssNonASCIIPath: ssConflict("ss-story-resume", "superseded", "", "spec/ss-story")},
			candidate: ssV1Path,
			wantState: Superseded,
			want:      []ssWant{{"superseded by " + ssV2Path, ssNonASCIIPath}},
		},
		{
			name:      "SS-R1: a superseded conflict at a name holding a double quote plus v2's edge: superseded, naming it",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssQuotePath: ssConflict("ss-story-quote", "superseded", "", "spec/ss-story")},
			candidate: ssV1Path,
			wantState: Superseded,
			want:      []ssWant{{"superseded by " + ssV2Path, ssQuotePath}},
		},
		// SS-R2: an incomplete spec scan keeps its witness, and the
		// "no successor" disclosure asserts no negative it cannot prove.
		{
			name:      "SS-R2: a superseded conflict and a v2 that fails strict decode: unproven, naming the incomplete spec scan and v2's failure",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2Broken, ssConflictPath: resolved},
			candidate: ssV1Path,
			wantState: Unproven,
			want: []ssWant{
				{ssV1Path, ssConflictPath, ssMissingSuccessHedged},
				{ssV1Path, ssSpecScanIncomplete},
				{ssV2Path, "failed to decode"},
			},
			absent: []string{ssMissingSuccess},
		},
		{
			name:      "SS-R2: v2's edge with no resolved conflict and another spec failing strict decode: unproven, keeping the spec scan's witness",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssBrokenSpecPath: brokenSpec},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV2Path, ssMissingRecord}, {ssV1Path, ssSpecScanIncomplete}, {ssBrokenSpecPath, "failed to decode"}},
		},
		{
			name:      "SS-R2: an unreadable conflict and a spec failing strict decode: unproven, naming both incomplete scans",
			tree:      map[string]string{ssV1Path: v1, ssGarbledPath: garbled, ssBrokenSpecPath: brokenSpec},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssScanIncomplete}, {ssGarbledPath, "failed to decode"}, {ssSpecScanIncomplete}, {ssBrokenSpecPath, "failed to decode"}},
		},
		{
			name:      "SS-R2: a superseded conflict with no successor and a complete spec scan keeps the definite wording",
			tree:      map[string]string{ssV1Path: v1, ssConflictPath: resolved},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssConflictPath, ssMissingSuccess}},
			absent:    []string{ssSpecScanIncomplete, ssMissingSuccessHedged},
		},
		// SS-R3 (SI-306 (4a)): a nested or empty-named conflict file is a
		// scan failure — never read, never credited — whether or not its
		// bytes would decode.
		{
			name:      "SS-R3: a malformed nested conflict file: unproven, naming it",
			tree:      map[string]string{ssV1Path: v1, ssNestedPath: garbled},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV1Path, ssScanIncomplete}, {ssNestedPath, ssNotRecord}},
		},
		{
			name:      "SS-R3: a well-formed superseded nested conflict file is never credited: unproven, naming it",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssNestedPath: resolved},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV2Path, ssMissingRecord}, {ssV1Path, ssScanIncomplete}, {ssNestedPath, ssNotRecord}},
		},
		{
			name:      "SS-R3: a malformed empty-named conflict file: unproven, naming it",
			tree:      map[string]string{ssV1Path: v1, ssEmptyNamePath: garbled},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV1Path, ssScanIncomplete}, {ssEmptyNamePath, ssNotRecord}},
		},
		{
			name:      "SS-R3: a well-formed superseded empty-named conflict file is never credited: unproven, naming it",
			tree:      map[string]string{ssV1Path: v1, ssV2Path: v2, ssEmptyNamePath: resolved},
			candidate: ssV1Path,
			wantState: Unproven,
			want:      []ssWant{{ssV2Path, ssMissingRecord}, {ssV1Path, ssScanIncomplete}, {ssEmptyNamePath, ssNotRecord}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := ssLand(t, tt.tree)
			result, err := newProjector(realGitReader{}).Resolve(context.Background(), repo.Dir, Candidate{Path: tt.candidate, Content: []byte(tt.tree[tt.candidate])})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			ssAssert(t, tt.candidate, result, tt.wantState, tt.want)
			for _, sub := range tt.absent {
				for i, d := range result.Disclosures {
					if strings.Contains(d, sub) {
						t.Fatalf("Resolve(%s).Disclosures[%d] = %q, want no disclosure containing %q", tt.candidate, i, d, sub)
					}
				}
			}
		})
	}
}

// TestProjector_StorySupersession_BatchAgreesWithSingle resolves the
// decomposition tree's predecessor and both successors in one ResolveMany
// call and proves each result equals its single Resolve.
func TestProjector_StorySupersession_BatchAgreesWithSingle(t *testing.T) {
	tree := map[string]string{
		ssV1Path:        ssStory("ss-story", ""),
		ssV2Path:        ssStory("ss-story-v2", "", ssSupersedes("ss-story")),
		ssV2ArchivePath: ssStory("ss-story-v2b", "status: closed\n"+ssFrozenStamp, ssSupersedes("ss-story")),
		ssConflictPath:  ssConflict("ss-story-wrong", "superseded", "", "spec/ss-story"),
	}
	repo := ssLand(t, tree)
	ctx := context.Background()
	paths := []string{ssV1Path, ssV2Path, ssV2ArchivePath}
	candidates := make([]Candidate, len(paths))
	for i, p := range paths {
		candidates[i] = Candidate{Path: p, Content: []byte(tree[p])}
	}
	batch, err := newProjector(realGitReader{}).ResolveMany(ctx, repo.Dir, candidates)
	if err != nil {
		t.Fatalf("ResolveMany: %v", err)
	}
	wantStates := []State{Superseded, AcceptedPendingBuild, Closed}
	for i, c := range candidates {
		single, err := newProjector(realGitReader{}).Resolve(ctx, repo.Dir, c)
		if err != nil {
			t.Fatalf("Resolve(%s): %v", c.Path, err)
		}
		if !reflect.DeepEqual(batch[i], single) {
			t.Fatalf("ResolveMany[%d] = %+v, single Resolve = %+v", i, batch[i], single)
		}
		if batch[i].State != wantStates[i] {
			t.Fatalf("ResolveMany[%d] (%s) = %s, want %s", i, c.Path, batch[i].State, wantStates[i])
		}
	}
}

// TestConflictPathClassification pins which default-branch paths the
// conflict scan treats as conflict files (artifact.ClassifyPath's reading)
// and which of those sit at a conflict record path (store.ConflictPath); a
// conflict file off a record path is a scan failure (SI-306 (4a)).
func TestConflictPathClassification(t *testing.T) {
	tests := []struct {
		path       string
		wantFile   bool
		wantRecord bool
	}{
		{".verdi/conflicts/ss-story-wrong.md", true, true},
		{ssNonASCIIPath, true, true},
		{ssQuotePath, true, true},
		{".verdi/conflicts/a b.md", true, true},
		{ssNestedPath, true, false},
		{".verdi/conflicts/a/b/c.md", true, false},
		{ssEmptyNamePath, true, false},
		{".verdi/conflicts/notes.txt", false, false},
		{".verdi/conflicts/x.md.bak", false, false},
		{".verdi/conflicts", false, false},
		{".verdi/conflicts-extra/y.md", false, false},
		{".verdi/specs/active/x/spec.md", false, false},
		{"sub/.verdi/conflicts/x.md", false, false},
		{"conflicts/x.md", false, false},
		{"", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isConflictFile(tt.path); got != tt.wantFile {
				t.Fatalf("isConflictFile(%q) = %v, want %v", tt.path, got, tt.wantFile)
			}
			if got := tt.wantFile && isConflictRecordPath(tt.path); got != tt.wantRecord {
				t.Fatalf("isConflictFile && isConflictRecordPath(%q) = %v, want %v", tt.path, got, tt.wantRecord)
			}
		})
	}
	if got, want := conflictsDir(), ".verdi/conflicts"; got != want {
		t.Fatalf("conflictsDir() = %q, want %q", got, want)
	}
}
