package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/journey"
)

// journeyFeatureSpecMD is a minimal, valid, statusless active feature spec
// — mirrors specstate_test.go's specStateExactPaymentsMD (same shape, same
// v0 compatibility grammar), landed via fixturegit so its exact bytes are
// reachable from the default branch.
const journeyFeatureSpecMD = `---
id: spec/payments
kind: spec
class: feature
title: "Payments"
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`

// journeyComponentSpecMD is a component-class spec — no story, no
// acceptance criteria, refused outright by internal/journey (GatherFacts's
// decodeTargetSpec): a journey record projects only over feature/story
// targets.
const journeyComponentSpecMD = `---
id: spec/shared-lib
kind: spec
class: component
title: "Shared lib"
owners: [platform-team]
status: active
---
# body
`

// buildJourneyRepo mirrors specstate_test.go's buildSpecStateRepo: a
// minimal .verdi/verdi.yaml scaffold plus caller-supplied files, with
// CI_DEFAULT_BRANCH pinned so default-branch resolution is hermetic.
func buildJourneyRepo(t *testing.T, files map[string]string) *fixturegit.Repo {
	t.Helper()
	base := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n"}
	for k, v := range files {
		base[k] = v
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: base, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	t.Chdir(repo.Dir)
	return repo
}

// TestCmdJourney_Usage proves the single-positional-argument shape: zero
// or two-or-more arguments is a usage error, exit 2, with the exact usage
// line the task names — checked before any store root is resolved (no
// store exists in this test at all).
func TestCmdJourney_Usage(t *testing.T) {
	t.Chdir(t.TempDir())

	cases := []struct {
		name string
		args []string
	}{
		{"no arguments", nil},
		{"two arguments", []string{"spec/payments", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := cmdJourney(tc.args, &stdout, &stderr)
			if got != 2 {
				t.Fatalf("cmdJourney(%v) = %d, want 2", tc.args, got)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty on a usage error", stdout.String())
			}
			const want = "usage: verdi journey <feature-or-story-ref>\n"
			if stderr.String() != want {
				t.Fatalf("stderr = %q, want exactly %q", stderr.String(), want)
			}
		})
	}
}

// TestCmdJourney_HappyPath proves the end-to-end wiring over a real
// fixturegit repo: a statusless spec whose exact bytes are landed on the
// default branch produces exit 0, stdout that decodes via journey.Decode
// (a full canonical round trip, digest included), and a lifecycle state
// of accepted-pending-build (specstate's own statusless-landed reading).
func TestCmdJourney_HappyPath(t *testing.T) {
	buildJourneyRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})

	var stdout, stderr bytes.Buffer
	got := cmdJourney([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdJourney = %d, want 0; stderr=%s", got, stderr.String())
	}

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout = %q, want exactly one line", stdout.String())
	}

	rec, err := journey.Decode([]byte(lines[0]))
	if err != nil {
		t.Fatalf("journey.Decode(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if rec.Target.Ref != "spec/payments" {
		t.Fatalf("Target.Ref = %q, want spec/payments", rec.Target.Ref)
	}
	if rec.Lifecycle.State != "accepted-pending-build" {
		t.Fatalf("Lifecycle.State = %q, want accepted-pending-build", rec.Lifecycle.State)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", stderr.String())
	}
}

// TestCmdJourney_CanonicalEncoding_SortedKeys spot-checks the canonjson
// seam is actually used (sorted keys), mirroring specstate_test.go:93's
// idiom.
func TestCmdJourney_CanonicalEncoding_SortedKeys(t *testing.T) {
	buildJourneyRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})

	var stdout, stderr bytes.Buffer
	got := cmdJourney([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdJourney = %d, want 0; stderr=%s", got, stderr.String())
	}
	line := strings.TrimRight(stdout.String(), "\n")
	// A Record nests several field names at more than one level (e.g.
	// "disclosures" appears at the top level AND inside lifecycle,
	// blockers.eventual, and principals) — unlike specstate.Result's flat
	// shape, a plain strings.Index substring scan for each wanted key would
	// find the FIRST occurrence anywhere in the document, not necessarily
	// the top-level one, and misreport ordering. topLevelObjectKeys walks
	// only the outermost object's own keys, depth-aware, so this spot
	// check proves what specstate_test.go:93's idiom proves for its own
	// flatter schema: canonjson.Marshal's sorted-keys seam is actually
	// used, not Go's default map/struct marshal order.
	got2 := topLevelObjectKeys(t, line)
	want := []string{"actions", "blockers", "digest", "disclosures", "lifecycle", "principals", "repository", "schema", "target"}
	if len(got2) != len(want) {
		t.Fatalf("topLevelObjectKeys = %v, want %v", got2, want)
	}
	for i := range want {
		if got2[i] != want[i] {
			t.Fatalf("top-level key order = %v, want %v (canonjson seam not used or key set drifted)", got2, want)
		}
	}
}

// topLevelObjectKeys returns line's outermost JSON object's own keys, in
// their on-the-wire order, ignoring any nested object/array content
// (bracket-depth- and string-escape-aware, since canjson's compact output
// carries no whitespace to lean on).
func topLevelObjectKeys(t *testing.T, line string) []string {
	t.Helper()
	if len(line) < 2 || line[0] != '{' || line[len(line)-1] != '}' {
		t.Fatalf("topLevelObjectKeys: %q is not a JSON object", line)
	}
	var keys []string
	depth := 0
	inString := false
	escape := false
	expectKey := true
	var keyBuf strings.Builder
	inKey := false
	for i := 1; i < len(line)-1; i++ {
		c := line[i]
		if inString {
			if inKey {
				keyBuf.WriteByte(c)
			}
			if escape {
				escape = false
			} else if c == '\\' {
				escape = true
			} else if c == '"' {
				inString = false
				if inKey {
					inKey = false
					keys = append(keys, keyBuf.String()[:keyBuf.Len()-1]) // drop the closing quote just appended
					keyBuf.Reset()
				}
			}
			continue
		}
		switch c {
		case '"':
			inString = true
			if depth == 0 && expectKey {
				inKey = true
			}
		case '{', '[':
			depth++
		case '}', ']':
			depth--
		case ':':
			if depth == 0 {
				expectKey = false
			}
		case ',':
			if depth == 0 {
				expectKey = true
			}
		}
	}
	return keys
}

// TestCmdJourney_MissingRef proves a ref resolving nowhere at all (no
// working tree, no default branch) is operational exit 2 with a
// "journey: "-prefixed stderr line (journey.NotFoundError already
// self-prefixes; journeyErr must not double it).
func TestCmdJourney_MissingRef(t *testing.T) {
	buildJourneyRepo(t, nil)

	var stdout, stderr bytes.Buffer
	got := cmdJourney([]string{"spec/nowhere"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdJourney(spec/nowhere) = %d, want 2", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
	}
	if !strings.HasPrefix(stderr.String(), "journey: ") {
		t.Fatalf("stderr = %q, want a \"journey: \"-prefixed line", stderr.String())
	}
	if strings.Count(stderr.String(), "journey: ") != 1 {
		t.Fatalf("stderr = %q, want the \"journey: \" prefix exactly once (no doubling)", stderr.String())
	}
}

// TestCmdJourney_ComponentSpec proves a component-class target (no story,
// no acceptance criteria) is refused: operational exit 2.
func TestCmdJourney_ComponentSpec(t *testing.T) {
	buildJourneyRepo(t, map[string]string{".verdi/specs/active/shared-lib/spec.md": journeyComponentSpecMD})

	var stdout, stderr bytes.Buffer
	got := cmdJourney([]string{"spec/shared-lib"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdJourney(spec/shared-lib) = %d, want 2; stdout=%s", got, stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
	}
	if !strings.HasPrefix(stderr.String(), "journey: ") {
		t.Fatalf("stderr = %q, want a \"journey: \"-prefixed line", stderr.String())
	}
}

// TestCmdJourney_Rootless proves an unresolvable store root is operational
// exit 2 (store.FindRoot's own error, which carries no "journey: " prefix
// of its own — journeyErr must add exactly one).
func TestCmdJourney_Rootless(t *testing.T) {
	t.Chdir(t.TempDir())

	var stdout, stderr bytes.Buffer
	got := cmdJourney([]string{"spec/payments"}, &stdout, &stderr)
	if got != 2 {
		t.Fatalf("cmdJourney (rootless) = %d, want 2", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty on an operational error", stdout.String())
	}
	if !strings.HasPrefix(stderr.String(), "journey: ") {
		t.Fatalf("stderr = %q, want a \"journey: \"-prefixed line", stderr.String())
	}
}

// TestRun_JourneyDispatchesToRealVerb proves dispatch.go routes "journey"
// to the real implementation, mirroring specstate_test.go's
// TestRun_SpecStateDispatchesToRealVerb.
func TestRun_JourneyDispatchesToRealVerb(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer
	got := run([]string{"journey", "spec/x"}, &stderr)
	if got != 2 {
		t.Fatalf("run([journey spec/x]) outside a store = %d, want 2 (operational)", got)
	}
	if contains(stderr.String(), "usage") || contains(stderr.String(), "not implemented") {
		t.Fatalf("stderr = %q, want a real store-root error, not the generic stub message", stderr.String())
	}
}

// journeyCurrentHead and journeyPorcelainStatus mirror
// specstate_test.go's currentHead/porcelainStatus (already defined in
// that file, package main) — used by TestCmdJourney_ReadOnly below.
func journeyCurrentHead(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func journeyPorcelainStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status --porcelain: %v", err)
	}
	return string(out)
}

// TestCmdJourney_ReadOnly proves cmdJourney changes neither HEAD nor the
// working tree's status — the CLI-boundary half of DC-1's "removing the
// projection changes no canonical lifecycle artifact" witness (the fuller
// proof, with a full file-listing diff, lives in commit 5's determinism
// suite).
func TestCmdJourney_ReadOnly(t *testing.T) {
	repo := buildJourneyRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})

	headBefore := journeyCurrentHead(t, repo.Dir)
	statusBefore := journeyPorcelainStatus(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	got := cmdJourney([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdJourney = %d, want 0; stderr=%s", got, stderr.String())
	}

	if headBefore != journeyCurrentHead(t, repo.Dir) {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, journeyCurrentHead(t, repo.Dir))
	}
	if statusBefore != journeyPorcelainStatus(t, repo.Dir) {
		t.Fatalf("git status --porcelain changed: before=%q after=%q", statusBefore, journeyPorcelainStatus(t, repo.Dir))
	}
}

// --- Determinism / legacy-integration proof suite (commit 5) -------------

// TestCmdJourney_Deterministic_SameRepo proves two cmdJourney calls against
// the SAME fixturegit repo produce byte-identical stdout, including the
// digest line — the projection performs no wall-clock- or randomness-
// dependent derivation of its own (mirroring internal/journey's own
// TestProject_Integration_Deterministic, one layer up at the CLI
// boundary).
func TestCmdJourney_Deterministic_SameRepo(t *testing.T) {
	buildJourneyRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})

	var out1, out2, stderr bytes.Buffer
	if got := cmdJourney([]string{"spec/payments"}, &out1, &stderr); got != 0 {
		t.Fatalf("cmdJourney (1) = %d, want 0; stderr=%s", got, stderr.String())
	}
	stderr.Reset()
	if got := cmdJourney([]string{"spec/payments"}, &out2, &stderr); got != 0 {
		t.Fatalf("cmdJourney (2) = %d, want 0; stderr=%s", got, stderr.String())
	}
	if out1.String() != out2.String() {
		t.Fatalf("stdout differs across two cmdJourney calls against the same repo:\n1: %s\n2: %s", out1.String(), out2.String())
	}
}

// TestCmdJourney_Deterministic_TwoRoots is the machine-independence proof
// at the binary boundary (F1(b)/CO-2/CO-4, mirroring internal/journey's
// own TestProject_Integration_TwoDistinctRootsByteIdentical): the SAME
// fixture layers, built in two DISTINCT temp dirs (different absolute
// paths) with no default branch resolvable (so specstate's own "no
// default branch could be resolved for <root>" disclosure is in play —
// exactly the string F1/F2 sanitize), must still produce byte-identical
// cmdJourney stdout. Before facts.go's sanitizeDisclosures this is RED:
// each repo's own absolute temp dir path leaks into Lifecycle.Disclosures
// verbatim.
func TestCmdJourney_Deterministic_TwoRoots(t *testing.T) {
	files := map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD}

	repo1 := buildJourneyRepoNoDefaultBranch(t, files)
	var out1, stderr1 bytes.Buffer
	if got := cmdJourney([]string{"spec/payments"}, &out1, &stderr1); got != 0 {
		t.Fatalf("cmdJourney (repo1) = %d, want 0; stderr=%s", got, stderr1.String())
	}

	repo2 := buildJourneyRepoNoDefaultBranch(t, files)
	var out2, stderr2 bytes.Buffer
	if got := cmdJourney([]string{"spec/payments"}, &out2, &stderr2); got != 0 {
		t.Fatalf("cmdJourney (repo2) = %d, want 0; stderr=%s", got, stderr2.String())
	}

	if repo1.Dir == repo2.Dir {
		t.Fatalf("test setup: want two distinct roots, got the same dir twice: %s", repo1.Dir)
	}
	if out1.String() != out2.String() {
		t.Fatalf("stdout differs across two distinct-root repos with identical semantic content (a root path leaked into the record):\nrepo1 (%s): %s\nrepo2 (%s): %s", repo1.Dir, out1.String(), repo2.Dir, out2.String())
	}
}

// buildJourneyRepoNoDefaultBranch mirrors buildJourneyRepo but deliberately
// leaves the default branch unresolvable (CI_DEFAULT_BRANCH cleared, no
// origin remote in a bare fixturegit repo) — internal/journey's own
// facts_integration_test.go buildFactsRepoNoDefaultBranch twin, copied
// rather than shared because that helper lives in package journey, not
// main.
func buildJourneyRepoNoDefaultBranch(t *testing.T, files map[string]string) *fixturegit.Repo {
	t.Helper()
	base := map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n"}
	for k, v := range files {
		base[k] = v
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: base, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "")
	t.Chdir(repo.Dir)
	return repo
}

// journeyVerdiFileListing returns a sorted "relpath|size|mtimeNS" tuple
// for every regular file under root/.verdi — a full, comparable snapshot
// cheap enough to take before and after a cmdJourney call, catching a
// create, delete, OR modify (a rewritten file gets a fresh mtime even
// when its size is unchanged) without reading every file's content.
func journeyVerdiFileListing(t *testing.T, root string) []string {
	t.Helper()
	dir := filepath.Join(root, ".verdi")
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		out = append(out, fmt.Sprintf("%s|%d|%d", filepath.ToSlash(rel), info.Size(), info.ModTime().UnixNano()))
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	sort.Strings(out)
	return out
}

// TestCmdJourney_RemovalNeutral is DC-1's "removing the projection changes
// no canonical lifecycle artifact" witness, taken from the CLI boundary
// (cmd/verdi/specstate_test.go:56-70's idiom, extended per commit 5's own
// brief with a full sorted .verdi/ file-listing diff — TestCmdJourney_
// ReadOnly in commit 4 already proves HEAD/porcelain-status neutrality;
// this proves no file under .verdi/ was created or modified either, the
// stronger claim a mutating verb's own listing-diff tests rely on
// elsewhere in this package).
func TestCmdJourney_RemovalNeutral(t *testing.T) {
	repo := buildJourneyRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})

	headBefore := journeyCurrentHead(t, repo.Dir)
	statusBefore := journeyPorcelainStatus(t, repo.Dir)
	listingBefore := journeyVerdiFileListing(t, repo.Dir)

	var stdout, stderr bytes.Buffer
	if got := cmdJourney([]string{"spec/payments"}, &stdout, &stderr); got != 0 {
		t.Fatalf("cmdJourney = %d, want 0; stderr=%s", got, stderr.String())
	}

	if headBefore != journeyCurrentHead(t, repo.Dir) {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, journeyCurrentHead(t, repo.Dir))
	}
	if statusBefore != journeyPorcelainStatus(t, repo.Dir) {
		t.Fatalf("git status --porcelain changed: before=%q after=%q", statusBefore, journeyPorcelainStatus(t, repo.Dir))
	}
	listingAfter := journeyVerdiFileListing(t, repo.Dir)
	if len(listingBefore) != len(listingAfter) {
		t.Fatalf(".verdi/ file count changed: before=%d after=%d\nbefore=%v\nafter=%v", len(listingBefore), len(listingAfter), listingBefore, listingAfter)
	}
	for i := range listingBefore {
		if listingBefore[i] != listingAfter[i] {
			t.Fatalf(".verdi/ file listing changed (a file was created or modified):\nbefore=%v\nafter=%v", listingBefore, listingAfter)
		}
	}
}

// journeyLegacyDraftMD is a legacy-status active feature spec (v0's
// pre-merge-signaled `status:` field, still readable per specstate's own
// compatibility grammar): landed via fixturegit exactly as-is, its exact
// bytes reachable from the default branch.
const journeyLegacyDraftMD = `---
id: spec/legacydraft
kind: spec
class: feature
title: "Legacy Draft"
owners: [platform-team]
status: draft
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`

// TestCmdJourney_LegacyDraft_MigrationDisclosure is legacy-source
// integration case (a): a fixture spec with legacy `status: draft` whose
// exact bytes are landed on the default branch resolves
// Lifecycle.State == accepted-pending-build (specstate's own statusless/
// legacy-draft compatibility reading — internal/specstate/resolve.go's
// migrationDisclosures) and Lifecycle.Disclosures carries specstate's
// migration disclosure verbatim.
//
// The task brief for this case additionally called for a root-sanitized
// "<store-root>" token assertion; empirically (this test was written
// against real cmdJourney output, not assumed) migrationDisclosures'
// own path argument is already GatherFacts's store-RELATIVE path, never
// an absolute one, so no root ever appears in THIS disclosure to sanitize
// — sanitizeDisclosures (facts.go) is a no-op here by construction, not a
// gap. The "<store-root>" token DOES appear for real in the unproven
// (no-default-branch) case below (specstate's OWN "no default branch
// could be resolved for <root>" disclosure), so that positive assertion
// moved there instead of being asserted falsely here. This test still
// proves the "never an absolute path" half directly: no fixture temp dir
// path leaks into stdout.
func TestCmdJourney_LegacyDraft_MigrationDisclosure(t *testing.T) {
	repo := buildJourneyRepo(t, map[string]string{".verdi/specs/active/legacydraft/spec.md": journeyLegacyDraftMD})

	var stdout, stderr bytes.Buffer
	if got := cmdJourney([]string{"spec/legacydraft"}, &stdout, &stderr); got != 0 {
		t.Fatalf("cmdJourney = %d, want 0; stderr=%s", got, stderr.String())
	}

	rec, err := journey.Decode([]byte(strings.TrimRight(stdout.String(), "\n")))
	if err != nil {
		t.Fatalf("journey.Decode(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if rec.Lifecycle.State != "accepted-pending-build" {
		t.Fatalf("Lifecycle.State = %q, want accepted-pending-build", rec.Lifecycle.State)
	}
	wantDisclosure := "specstate: .verdi/specs/active/legacydraft/spec.md carries legacy status: draft, but its exact bytes are already reachable from the default branch — reported accepted-pending-build with this migration disclosure, never as an active draft"
	if !containsString(rec.Lifecycle.Disclosures, wantDisclosure) {
		t.Fatalf("Lifecycle.Disclosures = %v, want it to contain the migration disclosure %q", rec.Lifecycle.Disclosures, wantDisclosure)
	}
	if strings.Contains(stdout.String(), repo.Dir) {
		t.Fatalf("stdout leaks the fixture's absolute temp dir path:\n%s", stdout.String())
	}
}

// journeyLegacySupersededMD is a legacy `status: superseded` active
// feature spec carrying the frozen stamp internal/artifact's validateSpec
// requires for any terminal status (superseded/closed) — without it,
// decodeTargetSpec's own strict-decode seam refuses the document outright
// before internal/journey ever reaches specstate's legacy-terminal path
// (confirmed empirically, not assumed).
const journeyLegacySupersededMD = `---
id: spec/legacysup
kind: spec
class: feature
title: "Legacy Superseded"
owners: [platform-team]
status: superseded
frozen: { at: "2024-01-01", commit: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa }
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
---
# body
`

// TestCmdJourney_LegacySuperseded_NoCatalogBlockers is legacy-source
// integration case (b): a landed spec with legacy `status: superseded`
// resolves Lifecycle.State == superseded, no catalog blockers (superseded
// is terminal in the canonical operating model — no transition's From
// equals it, so candidateTransitions yields nothing for deriveBlockers/
// deriveActions to act on), and empty safe actions.
func TestCmdJourney_LegacySuperseded_NoCatalogBlockers(t *testing.T) {
	buildJourneyRepo(t, map[string]string{".verdi/specs/active/legacysup/spec.md": journeyLegacySupersededMD})

	var stdout, stderr bytes.Buffer
	if got := cmdJourney([]string{"spec/legacysup"}, &stdout, &stderr); got != 0 {
		t.Fatalf("cmdJourney = %d, want 0; stderr=%s", got, stderr.String())
	}

	rec, err := journey.Decode([]byte(strings.TrimRight(stdout.String(), "\n")))
	if err != nil {
		t.Fatalf("journey.Decode(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if rec.Lifecycle.State != "superseded" {
		t.Fatalf("Lifecycle.State = %q, want superseded", rec.Lifecycle.State)
	}
	if len(rec.Blockers.Current) != 0 {
		t.Fatalf("Blockers.Current = %v, want empty (superseded is terminal — no catalog transition applies)", rec.Blockers.Current)
	}
	if len(rec.Actions.Safe) != 0 {
		t.Fatalf("Actions.Safe = %v, want empty", rec.Actions.Safe)
	}
}

// TestCmdJourney_Unproven_DefaultBranchUnresolved is legacy-source
// integration case (c): no resolvable default branch at all yields the
// default-branch-unresolved and lifecycle-state-unproven blockers,
// Relationship == unknown, and — the point of this whole exit-
// classification design — exit STILL 0: an unproven state is a projected
// FACT, never an operational failure. This is also where specstate's own
// "no default branch could be resolved for <root>" disclosure actually
// embeds this checkout's absolute store root, so it is the real, provable
// site of the "<store-root>" sanitization assertion (see
// TestCmdJourney_LegacyDraft_MigrationDisclosure's doc comment for why it
// is not asserted there instead).
func TestCmdJourney_Unproven_DefaultBranchUnresolved(t *testing.T) {
	repo := buildJourneyRepoNoDefaultBranch(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})

	var stdout, stderr bytes.Buffer
	got := cmdJourney([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdJourney (unresolved default branch) = %d, want 0; stderr=%s", got, stderr.String())
	}

	rec, err := journey.Decode([]byte(strings.TrimRight(stdout.String(), "\n")))
	if err != nil {
		t.Fatalf("journey.Decode(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if rec.Lifecycle.State != "unproven" {
		t.Fatalf("Lifecycle.State = %q, want unproven", rec.Lifecycle.State)
	}
	if rec.Repository.Relationship != "unknown" {
		t.Fatalf("Repository.Relationship = %q, want unknown", rec.Repository.Relationship)
	}
	if journeyFindBlocker(rec.Blockers.Current, "default-branch-unresolved/unknown") == nil {
		t.Errorf("blockers missing default-branch-unresolved/unknown; got %v", journeyBlockerIDs(rec.Blockers.Current))
	}
	if journeyFindBlocker(rec.Blockers.Current, "lifecycle-state-unproven/unknown") == nil {
		t.Errorf("blockers missing lifecycle-state-unproven/unknown; got %v", journeyBlockerIDs(rec.Blockers.Current))
	}

	const storeRootToken = "<store-root>"
	found := false
	for _, d := range rec.Lifecycle.Disclosures {
		if strings.Contains(d, storeRootToken) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Lifecycle.Disclosures = %v, want at least one disclosure carrying the sanitized %q token", rec.Lifecycle.Disclosures, storeRootToken)
	}
	if strings.Contains(stdout.String(), repo.Dir) {
		t.Fatalf("stdout leaks the fixture's absolute temp dir path even though sanitizeDisclosures ran:\n%s", stdout.String())
	}
}

// TestCmdJourney_DirtyWorktree is negative-coverage completion (4a): an
// untracked file in the working tree makes the repository dirty while the
// evaluated spec's own bytes are untouched — Repository.Dirty.Value ==
// true, Source stays "head" (the target content itself still matches
// HEAD), exit 0 (dirty is a fact, never a verdict).
func TestCmdJourney_DirtyWorktree(t *testing.T) {
	repo := buildJourneyRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})
	if err := os.WriteFile(filepath.Join(repo.Dir, "scratch.txt"), []byte("untracked\n"), 0o644); err != nil {
		t.Fatalf("writing untracked file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdJourney([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdJourney (dirty worktree) = %d, want 0; stderr=%s", got, stderr.String())
	}
	rec, err := journey.Decode([]byte(strings.TrimRight(stdout.String(), "\n")))
	if err != nil {
		t.Fatalf("journey.Decode(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if !rec.Repository.Dirty.Known || !rec.Repository.Dirty.Value {
		t.Fatalf("Repository.Dirty = %+v, want known/true", rec.Repository.Dirty)
	}
	if rec.Repository.Source != "head" {
		t.Fatalf("Repository.Source = %q, want head (the evaluated spec's own bytes are untouched)", rec.Repository.Source)
	}
}

// TestCmdJourney_DivergedSpec is negative-coverage completion (4b): a
// working-tree edit of a landed spec diverges the candidate from what the
// default branch actually holds at that path — Lifecycle.Relation ==
// diverged, Lifecycle.Posture == advisory (never authoritative on a
// diverged working tree — DC-2's wrong-checkout ambiguity), Repository.
// Source == working-tree, exit 0.
func TestCmdJourney_DivergedSpec(t *testing.T) {
	repo := buildJourneyRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})
	edited := journeyFeatureSpecMD + "\n<!-- local, uncommitted edit -->\n"
	if err := os.WriteFile(filepath.Join(repo.Dir, ".verdi", "specs", "active", "payments", "spec.md"), []byte(edited), 0o644); err != nil {
		t.Fatalf("diverging payments spec.md: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdJourney([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdJourney (diverged) = %d, want 0; stderr=%s", got, stderr.String())
	}
	rec, err := journey.Decode([]byte(strings.TrimRight(stdout.String(), "\n")))
	if err != nil {
		t.Fatalf("journey.Decode(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if rec.Lifecycle.Relation != "diverged" {
		t.Fatalf("Lifecycle.Relation = %q, want diverged", rec.Lifecycle.Relation)
	}
	if rec.Lifecycle.Posture != "advisory" {
		t.Fatalf("Lifecycle.Posture = %q, want advisory", rec.Lifecycle.Posture)
	}
	if rec.Repository.Source != "working-tree" {
		t.Fatalf("Repository.Source = %q, want working-tree", rec.Repository.Source)
	}
}

// TestCmdJourney_RemoteOnlySpec is negative-coverage completion (4c): a
// spec present at the default branch but ABSENT from the working tree
// resolves Repository.Source == remote-ref (never a NotFound refusal, and
// never a fabricated empty candidate), exit 0.
func TestCmdJourney_RemoteOnlySpec(t *testing.T) {
	repo := buildJourneyRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})
	if err := os.Remove(filepath.Join(repo.Dir, ".verdi", "specs", "active", "payments", "spec.md")); err != nil {
		t.Fatalf("removing working-tree copy: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdJourney([]string{"spec/payments"}, &stdout, &stderr)
	if got != 0 {
		t.Fatalf("cmdJourney (remote-only) = %d, want 0; stderr=%s", got, stderr.String())
	}
	rec, err := journey.Decode([]byte(strings.TrimRight(stdout.String(), "\n")))
	if err != nil {
		t.Fatalf("journey.Decode(stdout): %v\nstdout=%s", err, stdout.String())
	}
	if rec.Repository.Source != "remote-ref" {
		t.Fatalf("Repository.Source = %q, want remote-ref", rec.Repository.Source)
	}
}

// TestCmdJourney_NoAbsolutePathLeak is commit 5's absolute-path leak scan:
// across every scenario this file exercises stdout for (happy path,
// legacy draft/superseded, the unresolved-default-branch unproven case,
// dirty, diverged, and remote-only), stdout must never carry the
// fixture's own temp dir path, a bare macOS temp-root prefix, or a raw
// git/gitx error fragment — every one of those would be a leaked
// filesystem detail or an unwrapped plumbing error, neither of which
// belongs in a canonical, machine-independent record.
func TestCmdJourney_NoAbsolutePathLeak(t *testing.T) {
	scenarios := map[string]func(t *testing.T) (root string, stdout string){
		"happy_path": func(t *testing.T) (string, string) {
			repo := buildJourneyRepo(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})
			var out, errBuf bytes.Buffer
			if got := cmdJourney([]string{"spec/payments"}, &out, &errBuf); got != 0 {
				t.Fatalf("cmdJourney = %d, want 0; stderr=%s", got, errBuf.String())
			}
			return repo.Dir, out.String()
		},
		"legacy_draft": func(t *testing.T) (string, string) {
			repo := buildJourneyRepo(t, map[string]string{".verdi/specs/active/legacydraft/spec.md": journeyLegacyDraftMD})
			var out, errBuf bytes.Buffer
			if got := cmdJourney([]string{"spec/legacydraft"}, &out, &errBuf); got != 0 {
				t.Fatalf("cmdJourney = %d, want 0; stderr=%s", got, errBuf.String())
			}
			return repo.Dir, out.String()
		},
		"legacy_superseded": func(t *testing.T) (string, string) {
			repo := buildJourneyRepo(t, map[string]string{".verdi/specs/active/legacysup/spec.md": journeyLegacySupersededMD})
			var out, errBuf bytes.Buffer
			if got := cmdJourney([]string{"spec/legacysup"}, &out, &errBuf); got != 0 {
				t.Fatalf("cmdJourney = %d, want 0; stderr=%s", got, errBuf.String())
			}
			return repo.Dir, out.String()
		},
		"unresolved_default_branch": func(t *testing.T) (string, string) {
			repo := buildJourneyRepoNoDefaultBranch(t, map[string]string{".verdi/specs/active/payments/spec.md": journeyFeatureSpecMD})
			var out, errBuf bytes.Buffer
			if got := cmdJourney([]string{"spec/payments"}, &out, &errBuf); got != 0 {
				t.Fatalf("cmdJourney = %d, want 0; stderr=%s", got, errBuf.String())
			}
			return repo.Dir, out.String()
		},
	}

	forbidden := []string{"/var/folders", "/private/var", "fatal:", "gitx:"}

	for name, run := range scenarios {
		t.Run(name, func(t *testing.T) {
			root, stdout := run(t)
			if strings.Contains(stdout, root) {
				t.Fatalf("stdout leaks the fixture's own temp dir path %q:\n%s", root, stdout)
			}
			for _, bad := range forbidden {
				if strings.Contains(stdout, bad) {
					t.Fatalf("stdout contains forbidden substring %q:\n%s", bad, stdout)
				}
			}
		})
	}
}

// containsString reports whether ss contains s exactly.
func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// journeyFindBlocker and journeyBlockerIDs mirror internal/journey/
// derive_test.go's identically-shaped findBlocker/blockerIDs helpers
// (that package's own copies are unexported to package journey; this
// package cannot import _test.go symbols across packages, so the small
// idiom is copied, not shared).
func journeyFindBlocker(blockers []journey.Blocker, id string) *journey.Blocker {
	for i := range blockers {
		if blockers[i].ID == id {
			return &blockers[i]
		}
	}
	return nil
}

func journeyBlockerIDs(blockers []journey.Blocker) []string {
	ids := make([]string, 0, len(blockers))
	for _, b := range blockers {
		ids = append(ids, b.ID)
	}
	return ids
}
