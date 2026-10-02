package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/ritualwitness"
	"github.com/jyang234/verdi/internal/workbench"
	ws "github.com/jyang234/verdi/internal/writescope"
)

// uat036Ritual is one ritual UAT-036 names, driven through its real entry
// point (story dc-1): the registry declaration it is judged against, the
// store its fixture's seed commit carries, and the driver, built once the
// fixture exists.
type uat036Ritual struct {
	name   string
	ritual string
	base   map[string]string
	driver func(t *testing.T, fx *ritualwitness.Fixture) ritualwitness.Driver
}

func uat036Rituals(t *testing.T, bin string) []uat036Ritual {
	cli := func(args ...string) func(*testing.T, *ritualwitness.Fixture) ritualwitness.Driver {
		return func(*testing.T, *ritualwitness.Fixture) ritualwitness.Driver {
			return ritualwitness.Binary{Path: bin, Args: args}
		}
	}
	constitution := constitutionStoreFiles(t)
	return []uat036Ritual{
		{
			name: "design start", ritual: "design_start",
			base:   map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
			driver: cli("design", "start", "--kind", "feature", "--name", "uat036-scoped", "--defer-statements"),
		},
		{
			name: "design start --supersedes", ritual: "design_start",
			base: map[string]string{
				".verdi/verdi.yaml":                   minimalManifestYAML,
				".verdi/specs/active/lockbox/spec.md": lockboxPredecessor,
				".verdi/adr/0099-lockbox-exempt.md":   lockboxExemptADR,
			},
			driver: cli("design", "start", "--supersedes", "spec/lockbox", "--name", "lockbox-v2"),
		},
		{
			name: "verdi board commit", ritual: "commit_to_design",
			base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
			driver: func(t *testing.T, fx *ritualwitness.Fixture) ritualwitness.Driver {
				writeMutableBoard(t, fx.Dir, "jira:LOAN-1482", &artifact.Board{Schema: "verdi.board/v1"})
				return ritualwitness.Binary{Path: bin, Args: []string{"board", "commit", "jira:LOAN-1482", "--name", "uat036-from-board"}}
			},
		},
		{
			name: "the workbench's commit-to-design", ritual: "commit_to_design",
			base: map[string]string{".verdi/verdi.yaml": minimalManifestYAML},
			driver: func(t *testing.T, fx *ritualwitness.Fixture) ritualwitness.Driver {
				writeMutableBoard(t, fx.Dir, "STORY-1482", &artifact.Board{Schema: "verdi.board/v1"})
				body, err := json.Marshal(map[string]string{"name": "uat036-from-workbench", "story_ref": "jira:LOAN-1482"})
				if err != nil {
					t.Fatal(err)
				}
				return ritualwitness.Workbench{Serve: workbench.NewHandler, Method: http.MethodPost, Path: "/board/STORY-1482/commit", Body: body}
			},
		},
		{
			name: "accept diagram", ritual: "accept_diagram",
			base: map[string]string{
				".verdi/verdi.yaml": minimalManifestYAML,
				".verdi/diagrams/loansvc-target-topology.mermaid": proposedDiagramFile(),
			},
			driver: cli("accept", "diagram/loansvc-target-topology"),
		},
		{
			name: "constitution propose", ritual: "constitution_propose",
			base: constitution,
			driver: func(t *testing.T, _ *ritualwitness.Fixture) ritualwitness.Driver {
				return ritualwitness.Binary{Path: bin, Args: []string{"context", "constitution", "propose", "--request",
					constitutionProposeRequest(t, constitution, "policy/uat036-scoped")}}
			},
		},
	}
}

// TestUAT036_RitualsNeverCarryForeignEntries is the producer of
// obligation ritual-effect-witness--ac-3--behavioral (spec/ritual-effect-
// witness ac-3, UAT-036): in the fully seeded state — an untracked file, a
// dirty tracked file, and a pre-staged foreign entry — design start, design
// start --supersedes, verdi board commit, the workbench's commit-to-design,
// accept diagram, and constitution propose each run to completion and
// create commits that omit the foreign entry, which stays staged; close
// exits 2 with no ref, index, or working-tree change; and the registry's
// only carried declaration is the board's Commit and push. CLI verbs run as
// the built binary and the workbench's commit through the running server's
// handler (dc-1), so no command log exists (until spec/gitx-recorder-seam):
// the claims rest on the state diff and each created commit's file list.
//
// Of the verdicts logged, the test asserts the index carry, which is ac-3's
// claim, and nothing else: every other effect is ac-2's (lane R3). Design
// start's `config: outside` verdicts (branch.design/<n>.remote and .merge,
// written because it cuts at the base's remote-tracking ref name) are a
// known effect carried to R3 (backlog BL-141).
//
// Beside the fixture's foreign entry at the repository root, each run also
// stages storeForeignFile directly inside .verdi/, outside every listed
// ritual's declared stage paths, so a commit whose pathspec is the whole
// .verdi/ cannot pass. A pathspec wider than the ritual's own paths but
// narrower than .verdi/ (.verdi/specs/, say) holds no foreign entry here,
// so this test does not tell it apart.
func TestUAT036_RitualsNeverCarryForeignEntries(t *testing.T) {
	bin := buildVerdiBinary(t)
	ctx := context.Background()
	for _, r := range uat036Rituals(t, bin) {
		t.Run(r.name, func(t *testing.T) {
			fx := ritualwitness.BuildWith(t, ctx, ritualwitness.SeedFull, r.base)
			stageStoreForeignEntry(t, fx)
			decl := ritualDeclaration(t, r.ritual)
			for _, p := range decl.StagePaths {
				if p.Matches(storeForeignFile) {
					t.Fatalf("%s lies under %s's declared stage path %s", storeForeignFile, r.ritual, p)
				}
			}
			res := ritualwitness.RunOn(t, ctx, fx, r.driver(t, fx), decl)
			logVerdicts(t, res)
			requireCleanRun(t, res)

			created := createdCommits(res)
			if len(created) == 0 {
				t.Fatal("the ritual created no commit")
			}
			for _, foreign := range []string{ritualwitness.ForeignFile, storeForeignFile} {
				for _, id := range created {
					if files := res.After.Commits[id].Files; slices.Contains(files, foreign) {
						t.Errorf("commit %s recorded the pre-staged foreign entry %s (UAT-036); its files: %v", id, foreign, files)
					}
				}
				if !stagedIn(res.Before, foreign) || !stagedIn(res.After, foreign) {
					t.Errorf("the foreign entry %s is not staged before and after the run: a scoped commit leaves it in the index", foreign)
				}
			}
			requireIndexCarryWithin(t, res)
		})
	}

	t.Run("close refuses before any mutation", func(t *testing.T) {
		fx := ritualwitness.BuildWith(t, ctx, ritualwitness.SeedFull, closeStoreFiles())
		writeCloseGateReport(t, fx.Dir, fx.BaseCommit, dispositionedFindingYAML)
		d := ritualwitness.Binary{Path: bin, Args: []string{"close", "spec/close-fixture", "--force-local"}}
		res := ritualwitness.RunOn(t, ctx, fx, d, ritualDeclaration(t, "close"))
		logVerdicts(t, res)
		if res.Exit != 2 {
			t.Fatalf("close exited %d, want 2: %v", res.Exit, res.Err)
		}
		if res.Err == nil || !strings.Contains(res.Err.Error(), ritualwitness.ForeignFile) {
			t.Fatalf("close's refusal = %v, want it to name the staged %s", res.Err, ritualwitness.ForeignFile)
		}
		for _, c := range []struct {
			what          string
			before, after any
		}{
			{"refs", res.Before.Refs, res.After.Refs},
			{"the remote's refs", res.Before.RemoteRefs, res.After.RemoteRefs},
			{"HEAD", res.Before.Head, res.After.Head},
			{"the index", res.Before.Index, res.After.Index},
			{"the status", res.Before.Status, res.After.Status},
			{"the working tree", res.Before.Files, res.After.Files},
			{"linked worktrees", res.Before.Worktrees, res.After.Worktrees},
		} {
			if !reflect.DeepEqual(c.before, c.after) {
				t.Errorf("close changed %s:\nbefore %+v\nafter  %+v", c.what, c.before, c.after)
			}
		}
		if created := createdCommits(res); len(created) != 0 {
			t.Errorf("close created commits %v", created)
		}
		requireIndexCarryWithin(t, res)
	})

	t.Run("the board's Commit and push is the one carried declaration", func(t *testing.T) {
		var carried []string
		var verbs []ws.Verb
		for _, d := range ws.Registry() {
			if d.IndexCarry == ws.CarryCarried {
				carried = append(carried, d.Ritual)
				verbs = append(verbs, d.Verbs...)
			}
		}
		if !slices.Equal(carried, []string{ws.RitualBoardCommitPush}) {
			t.Fatalf("carried declarations = %v, want exactly %s", carried, ws.RitualBoardCommitPush)
		}
		// The two literal routes, never ws.CarriedVerbs(): a verb added to
		// both that list and the declaration must not pass unnoticed.
		want := []ws.Verb{
			ws.Workbench("/board/spec/{name}/api/git-commit"),
			ws.Workbench("/b/{branch}/board/spec/{name}/api/git-commit"),
		}
		less := func(a, b ws.Verb) int { return strings.Compare(a.String(), b.String()) }
		slices.SortFunc(verbs, less)
		slices.SortFunc(want, less)
		if !slices.Equal(verbs, want) {
			t.Fatalf("the carried declaration names %v, want the board's Commit and push %v", verbs, want)
		}
	})
}

// closeStoreFiles is buildCloseFixtureRepo's store: a feature, a story
// ready to close, its elaborated obligations, and self-hosted bindings.
func closeStoreFiles() map[string]string {
	files := map[string]string{
		".verdi/verdi.yaml":                         "schema: verdi.layout/v1\nforge: github\n",
		".verdi/specs/active/loan-mgmt/spec.md":     featureV1SpecMD,
		".verdi/specs/active/close-fixture/spec.md": closeFixtureStorySpecMD,
		"verdi.bindings.yaml":                       closeFixtureBindingsYAML,
	}
	for _, kind := range []artifact.EvidenceKind{artifact.EvidenceStatic, artifact.EvidenceBehavioral} {
		producer := selfHostedStaticProducer
		if kind == artifact.EvidenceBehavioral {
			producer = selfHostedBehavioralProducer
		}
		files[".verdi/obligations/close-fixture/ac-1--"+string(kind)+".md"] = fixtureElaboratedObligationMD("close-fixture", "ac-1", kind, producer, "1", gateFakeFrozenCommit)
	}
	return files
}

// storeForeignFile is a colleague's unrelated entry inside the store,
// pre-staged beside the fixture's own foreign entry: inside .verdi/, and
// under none of the listed rituals' declared stage paths.
const storeForeignFile = ".verdi/colleague-staged.md"

// stageStoreForeignEntry writes and stages storeForeignFile in fx.
func stageStoreForeignEntry(t *testing.T, fx *ritualwitness.Fixture) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fx.Dir, filepath.FromSlash(storeForeignFile)), []byte("a colleague's unrelated staged store work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitTestOutput(t, fx.Dir, "add", "--", storeForeignFile)
}
