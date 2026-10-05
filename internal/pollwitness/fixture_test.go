package pollwitness

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// corpusDir is examples/showcase, the corpus the e2e harness provisions.
const corpusDir = "../../examples/showcase"

// moduleRoot is this repository's root, the source of the obligation-quality
// adoption commit the e2e harness grafts under its store.
const moduleRoot = "../.."

// corpusGoldenHeads are examples/showcase's four layer SHAs: built from
// layers.txt with fixturegit's fixed identity and date, the corpus carries
// these exact commits, so every frozen stamp and pin in it stays honest.
func corpusGoldenHeads() []string {
	return []string{
		"78e3161594fb31fdad17f2ea8a96b52f33dbf0f3",
		"f6dd4c4df724c0b16cae435e96f7e34ac94026c9",
		"16219044c9d6d41de9a0de9464ed24d49283b40c",
		"38cc28c9f7bdf4098bccc724caddd0acdc2d17f6",
	}
}

// ambientEnv lists every environment variable the derivations under test
// read (internal/lint's CI environment, specstate's CI_DEFAULT_BRANCH, the
// repository facts' CI ref). A CI runner sets several of them, so the
// witnesses clear them all: the pinned bytes answer to the fixture alone.
func ambientEnv() []string {
	return []string{
		"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF", "GITHUB_REF_NAME", "GITHUB_REF_TYPE",
		"CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME",
		"CI_COMMIT_REF_NAME", "CI_COMMIT_BRANCH", "CI_COMMIT_TAG",
	}
}

func neutralizeEnv(t *testing.T) {
	t.Helper()
	for _, key := range ambientEnv() {
		t.Setenv(key, "")
	}
}

// buildE2EStore builds the e2e fixture the way cmd/e2eharness's
// provisionStore builds its scratch store: examples/showcase's whole
// committed zone, the loansvc service root, the repository's
// .gitattributes and .verdi/.gitignore, as one commit (here with
// fixturegit's fixed identity and date, so its SHA is stable); then the
// obligation-quality adoption graft under that root commit; then the
// untracked mutable and derived zones. The corpus's frozen stamps and pins
// name the layered build's commits, which this history does not hold, so
// on this fixture they read unreachable — the e2e store's own shape. The
// harness's per-suite extras (its mermaid, draft-board and readiness
// fixtures) are not added: none of them is a wall pinned here.
func buildE2EStore(t *testing.T) *fixturegit.Repo {
	t.Helper()
	neutralizeEnv(t)
	files := harnessLayer(t).Files
	for rel, content := range committedZone(t) {
		files[rel] = content
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "e2e harness: the showcase store"}})
	graftAdoptionUnderRoot(t, repo.Dir, repo.Head)
	copyTree(t, filepath.Join(corpusDir, "mutable"), filepath.Join(repo.Dir, ".verdi", "data", "mutable"))
	copyTree(t, filepath.Join(corpusDir, "derived"), filepath.Join(repo.Dir, ".verdi", "data", "derived"))
	return repo
}

// committedZone reads every file under examples/showcase/.verdi, keyed by
// its repository-relative path.
func committedZone(t *testing.T) map[string]string {
	t.Helper()
	files := map[string]string{}
	root := filepath.Join(corpusDir, ".verdi")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(corpusDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("reading the showcase committed zone: %v", err)
	}
	return files
}

// realShapedStory is the real-store-shaped fixture's branch story.
const realShapedStory = "stale-decline-retry-audit"

// realShapedFeature is its branch feature, carrying the context pins.
const realShapedFeature = "stale-decline-audit-trail"

// realShapedArchivedStory is its archived, closed implementing story.
const realShapedArchivedStory = "stale-decline-legacy-retry"

// buildRealShapedStore builds the real-store-shaped fixture. The real
// store serves a checkout whose default branch is a remote-tracking
// origin/main, holds closed implementing stories in the archive zone, and
// sits on a feature branch ahead of origin/main with work not yet landed.
// So this fixture builds examples/showcase layer by layer with its golden
// SHAs (its frozen stamps and pins are honest here), adds the e2e harness's
// plumbing layer, lands an archived implementing story of
// spec/stale-decline on main, points origin/main and origin/HEAD at that
// commit under an origin remote, dangles one commit, and then commits, on
// feature/<story>, a new implementing story and a new feature whose
// context pins name a reachable commit, the same commit abbreviated, and
// the dangling commit. Every commit uses fixturegit's fixed identity and
// date, so every SHA is stable.
func buildRealShapedStore(t *testing.T) *fixturegit.Repo {
	t.Helper()
	neutralizeEnv(t)
	layers := append(corpusLayers(t), harnessLayer(t), fixturegit.Layer{
		Files: map[string]string{
			".verdi/specs/archive/" + realShapedArchivedStory + "/spec.md": archivedStorySpec,
		},
		Message: "real-store shape: a closed implementing story in the archive zone",
	})
	repo := fixturegit.Build(t, layers)
	for i, want := range corpusGoldenHeads() {
		if repo.Heads[i] != want {
			t.Fatalf("corpus layer %d SHA = %s, want golden %s", i+1, repo.Heads[i], want)
		}
	}
	graftAdoptionUnderRoot(t, repo.Dir, repo.Heads[0])
	git(t, repo.Dir, nil, "remote", "add", "origin", "https://github.com/example/showcase.git")
	git(t, repo.Dir, nil, "update-ref", "refs/remotes/origin/main", repo.Head)
	git(t, repo.Dir, nil, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	dangling := fixturegit.Dangle(t, repo, map[string]string{"scratch/abandoned.txt": "abandoned work\n"}, "real-store shape: an abandoned commit")

	git(t, repo.Dir, nil, "checkout", "--quiet", "-b", "feature/"+realShapedStory)
	feature := strings.NewReplacer(
		"@FULL@", corpusGoldenHeads()[0],
		"@ABBREV@", corpusGoldenHeads()[0][:7],
		"@DANGLING@", dangling,
	).Replace(branchFeatureSpec)
	writeFile(t, repo.Dir, ".verdi/specs/active/"+realShapedStory+"/spec.md", branchStorySpec)
	writeFile(t, repo.Dir, ".verdi/specs/active/"+realShapedFeature+"/spec.md", feature)
	git(t, repo.Dir, nil, "add", "-A")
	git(t, repo.Dir, commitEnv(), "commit", "--quiet", "--no-verify", "-m", "real-store shape: a new implementing story on its feature branch")
	repo.Head = strings.TrimSpace(gitOut(t, repo.Dir, "rev-parse", "HEAD"))
	repo.Heads = append(repo.Heads, repo.Head)

	copyTree(t, filepath.Join(corpusDir, "mutable"), filepath.Join(repo.Dir, ".verdi", "data", "mutable"))
	copyTree(t, filepath.Join(corpusDir, "derived"), filepath.Join(repo.Dir, ".verdi", "data", "derived"))
	return repo
}

// corpusLayers reads examples/showcase/layers.txt into fixturegit layers,
// in ascending layer order.
func corpusLayers(t *testing.T) []fixturegit.Layer {
	t.Helper()
	f, err := os.Open(filepath.Join(corpusDir, "layers.txt"))
	if err != nil {
		t.Fatalf("opening layers.txt: %v", err)
	}
	defer func() { _ = f.Close() }()

	byLayer := map[int][]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		num, rel, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("layers.txt: malformed line %q", line)
		}
		var n int
		if _, err := fmt.Sscanf(num, "%d", &n); err != nil {
			t.Fatalf("layers.txt: bad layer number in %q: %v", line, err)
		}
		byLayer[n] = append(byLayer[n], strings.TrimSpace(rel))
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning layers.txt: %v", err)
	}
	order := make([]int, 0, len(byLayer))
	for n := range byLayer {
		order = append(order, n)
	}
	sort.Ints(order)

	layers := make([]fixturegit.Layer, 0, len(order))
	for _, n := range order {
		files := map[string]string{}
		for _, rel := range byLayer[n] {
			files[rel] = readCorpus(t, rel)
		}
		layers = append(layers, fixturegit.Layer{Files: files, Message: fmt.Sprintf("layer %d", n)})
	}
	return layers
}

// harnessLayer carries what cmd/e2eharness commits beside the corpus.
func harnessLayer(t *testing.T) fixturegit.Layer {
	t.Helper()
	return fixturegit.Layer{
		Files: map[string]string{
			"loansvc/.flowmap.yaml":                   readCorpus(t, "loansvc/.flowmap.yaml"),
			"loansvc/.flowmap/boundary-contract.json": readCorpus(t, "loansvc/.flowmap/boundary-contract.json"),
			".gitattributes":                          readCorpus(t, ".gitattributes"),
			".verdi/.gitignore":                       "data/\n",
		},
		Message: "e2e harness: service root and repository plumbing",
	}
}

// graftAdoptionUnderRoot reproduces cmd/e2eharness's
// attachObligationQualityAdoptionAncestry: the store is newer than the
// owner merge that adopted obligation quality, so that commit is imported
// and grafted as the parent of the store's root commit.
func graftAdoptionUnderRoot(t *testing.T, dir, root string) {
	t.Helper()
	adoption := evidence.ObligationQualityAdoptionCommit
	source, err := filepath.Abs(moduleRoot)
	if err != nil {
		t.Fatal(err)
	}
	git(t, dir, nil, "fetch", "--quiet", "--no-tags", source, adoption)
	git(t, dir, nil, "replace", "--graft", adoption)
	git(t, dir, nil, "replace", "--graft", root, adoption)
	git(t, dir, nil, "merge-base", "--is-ancestor", adoption, root)
}

func readCorpus(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(corpusDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading corpus file %s: %v", rel, err)
	}
	return string(data)
}

// commitEnv is fixturegit's fixed commit identity and date, so a commit
// made after Build is as stable as the layers it follows.
func commitEnv() []string {
	return []string{
		"TZ=UTC",
		"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid", "GIT_AUTHOR_DATE=1704067200 +0000",
		"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid", "GIT_COMMITTER_DATE=1704067200 +0000",
	}
}

func git(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// copyTree copies every regular file under src to dst, never through git.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying %s to %s: %v", src, dst, err)
	}
}

const archivedStorySpec = `---
id: spec/stale-decline-legacy-retry
kind: spec
class: story
title: "Legacy stale-decline retry (fixture, closed implementing story)"
status: closed
owners: [platform-team]
problem: { text: "the first stale-decline retry ran inline rather than through the outbox", anchor: "#problem" }
outcome: { text: "the retry runs through the outbox", anchor: "#outcome" }
story: jira:LOAN-1490
links:
  - { type: implements, ref: "spec/stale-decline#ac-2" }
acceptance_criteria:
  - { id: ac-1, text: "the retry runs through the outbox", evidence: [static], anchor: "#ac-1" }
frozen: { at: 2026-07-01, commit: 16219044c9d6d41de9a0de9464ed24d49283b40c }
---
# Legacy stale-decline retry

## Problem

The first stale-decline retry ran inline rather than through the outbox.

## Outcome

The retry runs through the outbox.

## AC-1

The retry runs through the outbox.
`

const branchStorySpec = `---
id: spec/stale-decline-retry-audit
kind: spec
class: story
title: "Stale-decline retry audit (fixture, branch story)"
owners: [platform-team]
problem: { text: "nobody can tell which stale declines were retried", anchor: "#problem" }
outcome: { text: "every stale-decline retry leaves an audit record", anchor: "#outcome" }
story: jira:LOAN-1491
links:
  - { type: implements, ref: "spec/stale-decline#ac-2" }
acceptance_criteria:
  - { id: ac-1, text: "every stale-decline retry leaves an audit record", evidence: [static, behavioral], anchor: "#ac-1" }
---
# Stale-decline retry audit

## Problem

Nobody can tell which stale declines were retried.

## Outcome

Every stale-decline retry leaves an audit record.

## AC-1

Every stale-decline retry leaves an audit record.
`

const branchFeatureSpec = `---
id: spec/stale-decline-audit-trail
kind: spec
class: feature
title: "Stale-decline audit trail (fixture, branch feature)"
owners: [platform-team]
story: jira:LOAN-1492
problem: { text: "servicing cannot reconstruct why a stale decline was retried", anchor: "#problem" }
outcome: { text: "every stale-decline decision is reconstructable from its audit trail", anchor: "#outcome" }
context:
  - adr/0002-outbox-events@@FULL@
  - adr/0002-outbox-events@@ABBREV@
  - adr/0002-outbox-events@@DANGLING@
acceptance_criteria:
  - { id: ac-1, text: "every stale-decline decision is reconstructable from its audit trail", evidence: [static, attestation], anchor: "#ac-1" }
---
# Stale-decline audit trail

## Problem

Servicing cannot reconstruct why a stale decline was retried.

## Outcome

Every stale-decline decision is reconstructable from its audit trail.

## AC-1

Every stale-decline decision is reconstructable from its audit trail.
`
