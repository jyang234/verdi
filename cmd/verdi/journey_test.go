package main

import (
	"bytes"
	"os/exec"
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
