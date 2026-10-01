package main

import (
	"bytes"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/matrixprojection"
)

// Story supersession derived from the rung-3 records (SI-290; design
// docs/superpowers/specs/2026-09-29-story-supersession-proof-design.md §4,
// §6 case 1): with story v2's whole-spec supersedes edge and the resolved
// conflict on the default branch, the consumers — unchanged — treat v1 as
// superseded. Every assertion drives the built binary.

const storySupersessionFeatureSpec = `---
id: spec/ss-feature
kind: spec
class: feature
title: "SS feature"
owners: [platform-team]
problem: { text: "a story's approach was wrong", anchor: problem }
outcome: { text: "the revised story carries the work", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "the work holds", evidence: [static] }
---
# SS feature
`

const storySupersessionV1Spec = `---
id: spec/ss-story
kind: spec
class: story
title: "SS story"
owners: [platform-team]
story: jira:SS-1
problem: { text: "p", anchor: problem }
outcome: { text: "o", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "the first approach", evidence: [static] }
links:
  - { type: implements, ref: "spec/ss-feature#ac-1" }
---
# SS story
`

const storySupersessionV2Spec = `---
id: spec/ss-story-v2
kind: spec
class: story
title: "SS story v2"
owners: [platform-team]
story: jira:SS-1
problem: { text: "p", anchor: problem }
outcome: { text: "o", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "the revised approach", evidence: [static] }
links:
  - { type: implements, ref: "spec/ss-feature#ac-1" }
  - { type: supersedes, ref: "spec/ss-story" }
---
# SS story v2
`

const storySupersessionConflict = `---
id: conflict/ss-story-wrong
kind: conflict
title: "ss-story's approach cannot work"
status: superseded
frozen: { at: 2026-09-29, commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }
owners: [platform-team]
links:
  - { type: challenges, ref: spec/ss-story }
---
The discovery that invalidates ss-story.
`

// storySupersessionRepo lands the feature, v1, v2, and the resolved
// conflict on a fixturegit repository's default branch.
func storySupersessionRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{
		".verdi/verdi.yaml":                       "schema: verdi.layout/v1\nforge: gitlab\n",
		".verdi/specs/active/ss-feature/spec.md":  storySupersessionFeatureSpec,
		".verdi/specs/active/ss-story/spec.md":    storySupersessionV1Spec,
		".verdi/specs/active/ss-story-v2/spec.md": storySupersessionV2Spec,
		".verdi/conflicts/ss-story-wrong.md":      storySupersessionConflict,
	}, Message: "land ss-story-v2 with its resolved conflict"}})
	pinFixtureDefaultBranch(t, repo.Dir)
	return repo
}

// storySupersessionRun runs the built binary in dir and returns its exit
// code, stdout, and stderr.
func storySupersessionRun(t *testing.T, bin, dir string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CI_DEFAULT_BRANCH=main")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("running verdi %v: %v", args, err)
	}
	return exitErr.ExitCode(), stdout.String(), stderr.String()
}

func TestStorySupersession_BuildStartRefusesV1NamingV2_BuiltBinary(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	repo := storySupersessionRepo(t)

	exit, _, stderr := storySupersessionRun(t, bin, repo.Dir, "build", "start", "spec/ss-story")
	if exit != 1 {
		t.Fatalf("verdi build start spec/ss-story exit = %d, want 1 (superseded is a verdict refusal); stderr=%s", exit, stderr)
	}
	if !strings.Contains(stderr, "refused") || !strings.Contains(stderr, "spec/ss-story-v2") {
		t.Fatalf("verdi build start spec/ss-story stderr = %q, want a refusal naming the successor spec/ss-story-v2", stderr)
	}

	exit, stdout, stderr := storySupersessionRun(t, bin, repo.Dir, "spec", "state", "spec/ss-story")
	if exit != 0 || !strings.Contains(stdout, `"state":"superseded"`) || !strings.Contains(stdout, ".verdi/specs/active/ss-story-v2/spec.md") || !strings.Contains(stdout, ".verdi/conflicts/ss-story-wrong.md") {
		t.Fatalf("verdi spec state spec/ss-story exit=%d stdout=%s stderr=%s, want superseded naming v2 and the conflict", exit, stdout, stderr)
	}
}

func TestStorySupersession_MatrixExcludesV1FoldsV2_BuiltBinary(t *testing.T) {
	t.Parallel()
	bin := buildVerdiBinary(t)
	repo := storySupersessionRepo(t)

	exit, stdout, stderr := storySupersessionRun(t, bin, repo.Dir, "matrix", "--json", "spec/ss-feature")
	if exit != 0 {
		t.Fatalf("verdi matrix --json spec/ss-feature exit = %d, want 0; stderr=%s", exit, stderr)
	}
	record, err := matrixprojection.Decode([]byte(stdout))
	if err != nil {
		t.Fatalf("decoding the matrix record: %v\n%s", err, stdout)
	}
	if record.Feature == nil || len(record.Feature.ACs) != 1 {
		t.Fatalf("matrix record = %s, want one feature AC", stdout)
	}
	if got, want := record.Feature.ACs[0].ImplementingStories, []string{"spec/ss-story-v2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ac-1 implementing stories = %q, want %q (v1 excluded, v2 folded)", got, want)
	}

	exit, stdout, stderr = storySupersessionRun(t, bin, repo.Dir, "matrix", "spec/ss-feature")
	if exit != 0 {
		t.Fatalf("verdi matrix spec/ss-feature exit = %d, want 0; stderr=%s", exit, stderr)
	}
	if !strings.Contains(stdout, "spec/ss-story-v2, spec/ss-story [superseded]") {
		t.Fatalf("verdi matrix spec/ss-feature stdout = %s, want ac-1 folding v2 and marking v1 superseded", stdout)
	}
}
