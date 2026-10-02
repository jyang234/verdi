package readinessload

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

// boardHrefForTest is a deterministic board-href function: every shape and
// success row a board can correct routes to it.
func boardHrefForTest(branch, name string) string {
	return "/b/" + branch + "/board/spec/" + name
}

// TestLoad_SuccessCriteriaAndCoverage is SI-338 (4) and (5) through the
// real loader: feature-alpha declares ac-2, ac-1, and ac-untargeted, and
// its one non-spike stub lists ac-1 alone. success/criteria reads proven
// with the declared criteria as witnesses, and exactly the two criteria no
// non-spike stub lists carry a success/coverage row — unproven,
// non-blocking, routed to the wall, each naming its criterion as Object
// and carrying the shared coverage guidance. Neither moves the success
// area, and a second derivation is identical.
func TestLoad_SuccessCriteriaAndCoverage(t *testing.T) {
	repo, ref := readinessRepo(t, "feature")
	opts := Options{BoardHref: boardHrefForTest}
	snap, err := Load(context.Background(), repo.Dir, ref, opts)
	if err != nil {
		t.Fatal(err)
	}
	board := boardHrefForTest("main", "feature-alpha")
	if snap.BoardPath != board {
		t.Fatalf("snapshot BoardPath = %q, want %q", snap.BoardPath, board)
	}

	criteria := concernByID(t, snap, "success/criteria")
	if criteria.State != readinesspilot.StateProven || !criteria.Blocking || !reflect.DeepEqual(criteria.Witnesses, []string{"ac-1", "ac-2", "ac-untargeted"}) {
		t.Fatalf("success/criteria = %+v, want proven, blocking, witnessed by the declared criteria", criteria)
	}

	var coverage []string
	for _, c := range snap.AllConcerns {
		if strings.HasPrefix(c.ID, "success/coverage/") {
			coverage = append(coverage, c.ID)
		}
	}
	if want := []string{"success/coverage/ac-2", "success/coverage/ac-untargeted"}; !reflect.DeepEqual(coverage, want) {
		t.Fatalf("coverage rows = %q, want %q (ac-1 is listed by the alpha-story stub)", coverage, want)
	}
	for _, ac := range []string{"ac-2", "ac-untargeted"} {
		c := concernByID(t, snap, "success/coverage/"+ac)
		if c.State != readinesspilot.StateUnproven || c.Blocking || c.Timing != readinesspilot.TimingCurrent || c.Object != ac {
			t.Fatalf("coverage row = %+v, want unproven, non-blocking, current, Object %q", c, ac)
		}
		assertBoardDestination(t, c, board)
		if c.Guidance != readinesspilot.Guidance(readinesspilot.GuidanceCoverage, readinesspilot.GuidanceFacts{Object: ac}) {
			t.Fatalf("coverage guidance = %q", c.Guidance)
		}
	}
	if area := areaByID(snap, readinesspilot.AreaSuccess); area.State == readinesspilot.StateViolated {
		t.Fatalf("non-blocking coverage made the success area violated: %+v", area)
	}

	again, err := Load(context.Background(), repo.Dir, ref, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snap, again) {
		t.Fatal("the same ref at the same HEAD derived different snapshots")
	}
}

// TestLoad_CoverageCountsStubsNeverStories pins the wall's own stub-only
// rule against index-coverage dc-2's "no stub and no story": a story that
// implements feature-alpha#ac-2 does not cover ac-2 for readiness, so its
// coverage row stays.
func TestLoad_CoverageCountsStubsNeverStories(t *testing.T) {
	implementing := strings.Replace(readinessStorySpec, "spec/feature-alpha#ac-1", "spec/feature-alpha#ac-2", 1)
	if implementing == readinessStorySpec {
		t.Fatal("the story fixture no longer carries the implements edge this test re-points")
	}
	repo := buildCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": featureAlphaSpec(t),
		".verdi/specs/active/story-alpha/spec.md":   implementing,
	})
	snap, err := Load(context.Background(), repo.Dir, "spec/feature-alpha", Options{})
	if err != nil {
		t.Fatal(err)
	}
	c := concernByID(t, snap, "success/coverage/ac-2")
	if c.State != readinesspilot.StateUnproven {
		t.Fatalf("coverage row for a story-implemented criterion = %+v, want unproven", c)
	}
}

// TestLoad_StoryCarriesNoCoverage: coverage is a feature-only family.
func TestLoad_StoryCarriesNoCoverage(t *testing.T) {
	repo, ref := readinessRepo(t, "story")
	snap, err := Load(context.Background(), repo.Dir, ref, Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range snap.AllConcerns {
		if strings.HasPrefix(c.ID, "success/coverage/") {
			t.Fatalf("story derivation emitted coverage row %q", c.ID)
		}
	}
	if c := concernByID(t, snap, "success/criteria"); c.State != readinesspilot.StateProven {
		t.Fatalf("story success/criteria = %+v, want proven", c)
	}
}

// zeroCriteriaStorySpec is a story that declares no acceptance criteria —
// legal for the story class (artifact's validateStory requires none).
const zeroCriteriaStorySpec = `---
id: spec/story-bare
kind: spec
title: "A story with no criteria yet"
owners: [alpha-team]
class: story
story: jira:ALPHA-2
problem: {text: "The story has no criteria.", anchor: problem}
outcome: {text: "The story is reviewable.", anchor: outcome}
links:
  - {type: implements, ref: spec/feature-alpha#ac-1}
---
# Story Bare

## Problem

The story has no criteria.

## Outcome

The story is reviewable.
`

// TestLoad_ZeroCriteriaStoryReadsViolated pins SI-338 (5)'s reachable fix:
// a story with no acceptance criteria used to fail every readiness surface
// operationally (the success area was vacuous and Validate refused it); it
// now derives, with success/criteria violated with a witness, blocking,
// routed to the wall, and the success area violated.
func TestLoad_ZeroCriteriaStoryReadsViolated(t *testing.T) {
	repo := buildCompileRepo(t, map[string]string{".verdi/specs/active/story-bare/spec.md": zeroCriteriaStorySpec})
	snap, err := Load(context.Background(), repo.Dir, "spec/story-bare", Options{BoardHref: boardHrefForTest})
	if err != nil {
		t.Fatalf("Load(zero-criteria story) = %v, want a derived snapshot", err)
	}
	c := concernByID(t, snap, "success/criteria")
	if c.State != readinesspilot.StateViolated || !c.Blocking || len(c.Witnesses) == 0 {
		t.Fatalf("success/criteria = %+v, want violated with a witness, blocking", c)
	}
	assertBoardDestination(t, c, boardHrefForTest("main", "story-bare"))
	if c.Guidance != readinesspilot.Guidance(readinesspilot.GuidanceCriteria, readinesspilot.GuidanceFacts{}) {
		t.Fatalf("success/criteria guidance = %q", c.Guidance)
	}
	if area := areaByID(snap, readinesspilot.AreaSuccess); area.State != readinesspilot.StateViolated {
		t.Fatalf("success area = %+v, want violated", area)
	}
}
