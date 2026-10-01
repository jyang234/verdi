package specstate

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// TestProjector_StorySupersession_DefaultBranchOnly_Integration is design
// §6 case 7, then case 1 over the same history: v2's edge and the resolved
// conflict on a design branch — checked out, so they are also in the
// working tree — have no effect on v1; merging that branch into the
// default branch supersedes v1.
func TestProjector_StorySupersession_DefaultBranchOnly_Integration(t *testing.T) {
	ctx := context.Background()
	v1 := ssStory("ss-story", "")
	repo := ssLand(t, map[string]string{ssV1Path: v1})
	candidate := Candidate{Path: ssV1Path, Content: []byte(v1)}

	runGitForTest(t, repo.Dir, "checkout", "--quiet", "-b", "design/ss-story-v2")
	writeFileForTest(t, repo.Dir, ssV2Path, ssStory("ss-story-v2", "", ssSupersedes("ss-story")))
	writeFileForTest(t, repo.Dir, ssConflictPath, ssConflict("ss-story-wrong", "superseded", "", "spec/ss-story"))
	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "propose ss-story-v2 and resolve its conflict")

	result, err := newProjector(realGitReader{}).Resolve(ctx, repo.Dir, candidate)
	if err != nil {
		t.Fatalf("Resolve on the design branch: %v", err)
	}
	ssAssert(t, ssV1Path, result, AcceptedPendingBuild, nil)

	runGitForTest(t, repo.Dir, "checkout", "--quiet", "main")
	runGitForTest(t, repo.Dir, "merge", "--no-ff", "--no-edit", "design/ss-story-v2")

	result, err = newProjector(realGitReader{}).Resolve(ctx, repo.Dir, candidate)
	if err != nil {
		t.Fatalf("Resolve after the merge: %v", err)
	}
	ssAssert(t, ssV1Path, result, Superseded, []ssWant{{"superseded by " + ssV2Path, ssConflictPath}})
}

// TestProjector_StorySupersession_IgnoresTheWorkingTree_Integration proves
// the conflict scan reads the default branch's tree, never the working
// tree: resolving the conflict only in the working tree leaves v1
// unproven, naming the missing resolved conflict.
func TestProjector_StorySupersession_IgnoresTheWorkingTree_Integration(t *testing.T) {
	v1 := ssStory("ss-story", "")
	repo := ssLand(t, map[string]string{
		ssV1Path:       v1,
		ssV2Path:       ssStory("ss-story-v2", "", ssSupersedes("ss-story")),
		ssConflictPath: ssConflict("ss-story-wrong", "open", "", "spec/ss-story"),
	})
	writeFileForTest(t, repo.Dir, ssConflictPath, ssConflict("ss-story-wrong", "superseded", "", "spec/ss-story"))

	result, err := newProjector(realGitReader{}).Resolve(context.Background(), repo.Dir, Candidate{Path: ssV1Path, Content: []byte(v1)})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	ssAssert(t, ssV1Path, result, Unproven, []ssWant{{ssV2Path, ssMissingRecord}})
}

// TestProjector_StorySupersession_CacheSeesConflictChanges_Integration
// proves the corpus cache can never mask a changed conflict: a default-
// branch commit that changes only a conflict (the spec zone's tree is
// byte-identical before and after) is read again through the production
// constructor's shared process cache, and the verdict follows it.
func TestProjector_StorySupersession_CacheSeesConflictChanges_Integration(t *testing.T) {
	ctx := context.Background()
	v1 := ssStory("ss-story", "")
	repo := ssLand(t, map[string]string{
		ssV1Path:       v1,
		ssV2Path:       ssStory("ss-story-v2", "", ssSupersedes("ss-story")),
		ssConflictPath: ssConflict("ss-story-wrong", "open", "", "spec/ss-story"),
	})
	candidate := Candidate{Path: ssV1Path, Content: []byte(v1)}
	specTree := func() string {
		return strings.TrimSpace(runGitForTest(t, repo.Dir, "rev-parse", "main:"+specZonesPrefix))
	}

	for i := 0; i < 2; i++ {
		result, err := NewProjector().Resolve(ctx, repo.Dir, candidate)
		if err != nil {
			t.Fatalf("Resolve %d with the conflict open: %v", i, err)
		}
		ssAssert(t, ssV1Path, result, Unproven, []ssWant{{ssV2Path, ssMissingRecord}})
	}
	before := specTree()

	writeFileForTest(t, repo.Dir, ssConflictPath, ssConflict("ss-story-wrong", "superseded", "", "spec/ss-story"))
	runGitForTest(t, repo.Dir, "add", "-A")
	runGitForTest(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "resolve the conflict")
	if after := specTree(); after != before {
		t.Fatalf("spec-zone tree moved (%s -> %s); this row needs a conflict-only change", before, after)
	}

	for i := 0; i < 2; i++ {
		result, err := NewProjector().Resolve(ctx, repo.Dir, candidate)
		if err != nil {
			t.Fatalf("Resolve %d with the conflict resolved: %v", i, err)
		}
		ssAssert(t, ssV1Path, result, Superseded, []ssWant{{"superseded by " + ssV2Path, ssConflictPath}})
	}
}

// ssMemoCommits is two default-branch commits with byte-identical spec
// trees that differ only in the conflict's status.
func ssMemoCommits() map[string]map[string][]byte {
	specs := map[string][]byte{
		ssV1Path: []byte(ssStory("ss-story", "")),
		ssV2Path: []byte(ssStory("ss-story-v2", "", ssSupersedes("ss-story"))),
	}
	withConflict := func(status string) map[string][]byte {
		tree := map[string][]byte{ssConflictPath: []byte(ssConflict("ss-story-wrong", status, "", "spec/ss-story"))}
		for p, c := range specs {
			tree[p] = c
		}
		return tree
	}
	return map[string]map[string][]byte{"k-open": withConflict("open"), "k-resolved": withConflict("superseded")}
}

// TestProjector_StorySupersession_CorpusMemoKeysTheConflictSet drives one
// memoizing Projector across two commits that differ only in a conflict
// and checks each step against an unmemoized Projector: the conflict set
// is part of what the cache keys (the commit), so switching commits
// rescans and a cached entry is only ever its own commit's.
func TestProjector_StorySupersession_CorpusMemoKeysTheConflictSet(t *testing.T) {
	ctx := context.Background()
	repo := buildResolvableRepo(t)
	memoFake := newMemoGit(ssMemoCommits())
	plainFake := newMemoGit(ssMemoCommits())
	memo := newProjector(memoFake)
	plain := Projector{git: plainFake}
	candidate := Candidate{Path: ssV1Path, Content: ssMemoCommits()["k-open"][ssV1Path]}

	steps := []struct {
		head      string
		want      State
		wantScans int
	}{
		{head: "k-open", want: Unproven, wantScans: 1},
		{head: "k-resolved", want: Superseded, wantScans: 2},
		{head: "k-open", want: Unproven, wantScans: 2},
		{head: "k-resolved", want: Superseded, wantScans: 2},
	}
	for i, step := range steps {
		memoFake.set(step.head, nil, nil)
		plainFake.set(step.head, nil, nil)
		got, err := memo.Resolve(ctx, repo.Dir, candidate)
		if err != nil {
			t.Fatalf("step %d: memo Resolve: %v", i, err)
		}
		want, err := plain.Resolve(ctx, repo.Dir, candidate)
		if err != nil {
			t.Fatalf("step %d: plain Resolve: %v", i, err)
		}
		if got.State != step.want {
			t.Fatalf("step %d (head %s): state = %s, want %s (%+v)", i, step.head, got.State, step.want, got)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("step %d: memo result %+v differs from the unmemoized result %+v", i, got, want)
		}
		if n := memoFake.scans(); n != step.wantScans {
			t.Fatalf("step %d (head %s): %d corpus scans so far, want %d", i, step.head, n, step.wantScans)
		}
	}
}
