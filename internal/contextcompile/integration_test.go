// Lane 7C (sealed actor projection) plus Task 7 Step 2's hermetic
// end-to-end compiler integration coverage: real fixturegit repositories,
// a real installed policy store, and the production NewCompiler() ports
// (real gitx, real specstate.NewProjector, real policyauthority,
// real repositoryfacts.NewGatherer, real instructionprojection.Verify) —
// no fakes below this file's own git-plumbing test helpers.
package contextcompile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/specstate"
)

// --- fixtures: genuine sealed governanceprincipal resolutions --------------
//
// A PrincipalResolution's integrity seal can only be minted by
// governanceprincipal.Resolver.Resolve (see seal_test.go's
// TestResolutionSealRejectsForgery/TestResolutionSealDetectsMutation in
// that package). These fixtures build a real, minimal solo-class profile
// through the package's public DecodeProfile entry point and resolve real
// claims through a hermetic fake TrustFactReader — no reconstruction of the
// private seal, and no reliance on that package's unexported test helpers
// (authedRes et al.), which are not visible from this package.

// soloCatalog is the minimal catalog the fixture profile needs: solo-class
// profiles carry no approval/distinctness coverage requirement, so no
// roles or evidence sources need naming.
func soloCatalog() governanceprincipal.Catalog {
	return governanceprincipal.Catalog{Transitions: []string{"accept"}}
}

// soloProfileYAML is a minimal valid solo-class governance profile. Every
// top-level field must be present (governanceprincipal's strict decode
// rejects omission even where the value is the empty list).
const soloProfileYAML = `schema: verdi.governance-profile/v1
id: actors-fixture
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings: []
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`

func actorsFixtureProfile(t *testing.T) governanceprincipal.Profile {
	t.Helper()
	p, err := governanceprincipal.DecodeProfile([]byte(soloProfileYAML), soloCatalog())
	if err != nil {
		t.Fatalf("DecodeProfile: %v", err)
	}
	return p
}

// staticTrustFactReader is a hermetic fake governanceprincipal.TrustFactReader
// that always reports the wrapped fact, regardless of the source or claim
// asked about.
type staticTrustFactReader governanceprincipal.TrustFact

func (f staticTrustFactReader) ReadTrustFact(context.Context, governanceprincipal.TrustSource, governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
	return governanceprincipal.TrustFact(f), nil
}

const fixtureEvidenceDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func authenticatedFact(subject string) governanceprincipal.TrustFact {
	return governanceprincipal.TrustFact{
		SourceID:       "github",
		SourceKind:     governanceprincipal.TrustSourceForge,
		Subjects:       []string{subject},
		EvidenceDigest: fixtureEvidenceDigest,
		Available:      true,
		Valid:          true,
	}
}

func unprovenFact() governanceprincipal.TrustFact {
	return governanceprincipal.TrustFact{
		SourceID:   "github",
		SourceKind: governanceprincipal.TrustSourceForge,
		Available:  false,
		Reason:     "evidence unreachable",
	}
}

func violatedFact() governanceprincipal.TrustFact {
	return governanceprincipal.TrustFact{
		SourceID:       "github",
		SourceKind:     governanceprincipal.TrustSourceForge,
		Available:      true,
		Valid:          false,
		Reason:         "signature invalid",
		EvidenceDigest: fixtureEvidenceDigest,
	}
}

// mintResolution obtains a genuine sealed resolution the only way one
// exists: through governanceprincipal.Resolver.Resolve.
func mintResolution(t *testing.T, fact governanceprincipal.TrustFact, subject string) governanceprincipal.PrincipalResolution {
	t.Helper()
	profile := actorsFixtureProfile(t)
	r := governanceprincipal.NewResolver(staticTrustFactReader(fact))
	res, err := r.Resolve(context.Background(), profile, governanceprincipal.PrincipalClaim{TrustSource: "github", Subject: subject})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return res
}

// --- ActorResolver fakes ----------------------------------------------------

type fakeActorResolver struct {
	resolutions []governanceprincipal.PrincipalResolution
	err         error
}

func (f fakeActorResolver) Resolutions(context.Context) ([]governanceprincipal.PrincipalResolution, error) {
	return f.resolutions, f.err
}

// --- tests -------------------------------------------------------------

func TestProjectActorsNilResolverIsExplicitAbsence(t *testing.T) {
	got, err := projectActors(context.Background(), nil)
	if err != nil {
		t.Fatalf("projectActors(nil): unexpected error: %v", err)
	}
	if got.Posture != ResolutionUnproven {
		t.Errorf("Posture = %q, want %q", got.Posture, ResolutionUnproven)
	}
	if got.Resolutions == nil || len(got.Resolutions) != 0 {
		t.Errorf("Resolutions = %#v, want explicit empty slice", got.Resolutions)
	}
	if len(got.Disclosures) != 1 || got.Disclosures[0] != DisclosureActorResolutionUnproven {
		t.Errorf("Disclosures = %v, want [%q]", got.Disclosures, DisclosureActorResolutionUnproven)
	}
}

func TestProjectActorsEmptyResolverSetIsUnproven(t *testing.T) {
	got, err := projectActors(context.Background(), fakeActorResolver{})
	if err != nil {
		t.Fatalf("projectActors: unexpected error: %v", err)
	}
	if got.Posture != ResolutionUnproven {
		t.Errorf("Posture = %q, want %q", got.Posture, ResolutionUnproven)
	}
	if len(got.Resolutions) != 0 {
		t.Errorf("Resolutions = %#v, want empty", got.Resolutions)
	}
	if len(got.Disclosures) != 1 || got.Disclosures[0] != DisclosureActorResolutionUnproven {
		t.Errorf("Disclosures = %v, want [%q]", got.Disclosures, DisclosureActorResolutionUnproven)
	}
}

func TestProjectActorsAllAuthenticatedIsProven(t *testing.T) {
	author := mintResolution(t, authenticatedFact("user-123"), "user-123")
	reviewer := mintResolution(t, authenticatedFact("user-456"), "user-456")
	got, err := projectActors(context.Background(), fakeActorResolver{resolutions: []governanceprincipal.PrincipalResolution{author, reviewer}})
	if err != nil {
		t.Fatalf("projectActors: unexpected error: %v", err)
	}
	if got.Posture != ResolutionProven {
		t.Errorf("Posture = %q, want %q", got.Posture, ResolutionProven)
	}
	if len(got.Disclosures) != 0 {
		t.Errorf("Disclosures = %v, want empty", got.Disclosures)
	}
	if len(got.Resolutions) != 2 {
		t.Fatalf("Resolutions = %#v, want 2", got.Resolutions)
	}
}

func TestProjectActorsMixedAuthenticatedAndUnprovenIsUnproven(t *testing.T) {
	author := mintResolution(t, authenticatedFact("user-123"), "user-123")
	reviewer := mintResolution(t, unprovenFact(), "user-456")
	got, err := projectActors(context.Background(), fakeActorResolver{resolutions: []governanceprincipal.PrincipalResolution{author, reviewer}})
	if err != nil {
		t.Fatalf("projectActors: unexpected error: %v", err)
	}
	if got.Posture != ResolutionUnproven {
		t.Errorf("Posture = %q, want %q", got.Posture, ResolutionUnproven)
	}
	if len(got.Disclosures) != 1 || got.Disclosures[0] != DisclosureActorResolutionUnproven {
		t.Errorf("Disclosures = %v, want [%q]", got.Disclosures, DisclosureActorResolutionUnproven)
	}
}

func TestProjectActorsViolatedTakesPrecedence(t *testing.T) {
	orders := [][]string{{"violated", "unproven"}, {"unproven", "violated"}}
	for _, order := range orders {
		t.Run(strings.Join(order, "-then-"), func(t *testing.T) {
			violated := mintResolution(t, violatedFact(), "user-123")
			unproven := mintResolution(t, unprovenFact(), "user-456")
			var input []governanceprincipal.PrincipalResolution
			for _, kind := range order {
				if kind == "violated" {
					input = append(input, violated)
				} else {
					input = append(input, unproven)
				}
			}
			got, err := projectActors(context.Background(), fakeActorResolver{resolutions: input})
			if err != nil {
				t.Fatalf("projectActors: unexpected error: %v", err)
			}
			if got.Posture != ResolutionViolatedWithWitness {
				t.Errorf("Posture = %q, want %q", got.Posture, ResolutionViolatedWithWitness)
			}
			if len(got.Disclosures) != 0 {
				t.Errorf("Disclosures = %v, want empty (the disclosure is unproven-only)", got.Disclosures)
			}
		})
	}
}

func TestProjectActorsForgedResolutionRejected(t *testing.T) {
	genuine := mintResolution(t, authenticatedFact("user-123"), "user-123")
	forged := genuine
	forged.Witnesses = append([]governanceprincipal.Witness{}, genuine.Witnesses...)
	forged.Witnesses[0].EvidenceDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	_, err := projectActors(context.Background(), fakeActorResolver{resolutions: []governanceprincipal.PrincipalResolution{forged}})
	if err == nil {
		t.Fatal("projectActors(forged): want error, got nil")
	}
}

func TestProjectActorsDuplicateClaimIdentityRejected(t *testing.T) {
	first := mintResolution(t, authenticatedFact("user-123"), "user-123")
	second := mintResolution(t, authenticatedFact("user-123"), "user-123")

	_, err := projectActors(context.Background(), fakeActorResolver{resolutions: []governanceprincipal.PrincipalResolution{first, second}})
	if err == nil {
		t.Fatal("projectActors(duplicate claim identity): want error, got nil")
	}
}

func TestProjectActorsDeterministicSort(t *testing.T) {
	a := mintResolution(t, authenticatedFact("zed"), "zed")
	b := mintResolution(t, authenticatedFact("alpha"), "alpha")
	got, err := projectActors(context.Background(), fakeActorResolver{resolutions: []governanceprincipal.PrincipalResolution{a, b}})
	if err != nil {
		t.Fatalf("projectActors: unexpected error: %v", err)
	}
	if !sort.SliceIsSorted(got.Resolutions, func(i, j int) bool {
		if got.Resolutions[i].Claim.TrustSource != got.Resolutions[j].Claim.TrustSource {
			return got.Resolutions[i].Claim.TrustSource < got.Resolutions[j].Claim.TrustSource
		}
		return got.Resolutions[i].Claim.Subject < got.Resolutions[j].Claim.Subject
	}) {
		t.Errorf("Resolutions not sorted by (claim.trust_source, claim.subject): %#v", got.Resolutions)
	}
	if got.Resolutions[0].Claim.Subject != "alpha" {
		t.Errorf("first resolution subject = %q, want %q", got.Resolutions[0].Claim.Subject, "alpha")
	}
}

func TestProjectActorsResolverErrorIsOperational(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := projectActors(context.Background(), fakeActorResolver{err: sentinel})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("projectActors: err = %v, want wrapped %v", err, sentinel)
	}
}

// ============================================================================
// Hermetic end-to-end Compile fixtures (Task 7 Step 2)
// ============================================================================

// integrationCommitEnv is a fixed author/committer/date environment for the
// git commits this file makes directly (outside fixturegit.Build's own
// layers): two independently built repos from identical file trees and this
// same fixed env always produce byte-identical commit objects — the same
// determinism fixturegit itself relies on (see fixturegit.go's own
// identity/date constants) — so TestCompile_Integration_DeterministicAcrossTwoRoots
// below can assert byte-identical manifests across two wholly separate
// t.TempDir() checkouts.
var integrationCommitEnv = []string{
	"TZ=UTC",
	"GIT_AUTHOR_NAME=Verdi Fixture",
	"GIT_AUTHOR_EMAIL=fixture@verdi.invalid",
	"GIT_AUTHOR_DATE=1704067200 +0000",
	"GIT_COMMITTER_NAME=Verdi Fixture",
	"GIT_COMMITTER_EMAIL=fixture@verdi.invalid",
	"GIT_COMMITTER_DATE=1704067200 +0000",
}

// runIntegrationGit execs git in dir with integrationCommitEnv applied,
// failing the test on a non-zero exit. Test-local and package-private,
// deliberately duplicated rather than shared with fixturegit's own
// unexported runGit (test-only, cheap to duplicate; CLAUDE.md's shared-code
// rule is about production code) or specstate's runGitForTest (a different
// package, same idiom).
func runIntegrationGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), integrationCommitEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// policyStoreFiles reads the same real, already-cross-validated policy
// fixture files installPolicyFixture installs (internal/policyartifact's
// own testdata/store), keyed by their repo-relative .verdi/policy/ path so
// they can be committed into a fixturegit layer rather than merely written
// to an ungoverned temp directory.
func policyStoreFiles(t *testing.T) map[string]string {
	t.Helper()
	files := []string{
		"constitution.md",
		"policies/go-toolchain.md",
		"overlays/frontend-go-version.md",
		"exemptions/legacy-service-go.md",
		"profiles/solo-default.md",
	}
	out := make(map[string]string, len(files))
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join("..", "policyartifact", "testdata", "store", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read policy fixture %s: %v", rel, err)
		}
		out[".verdi/policy/"+rel] = string(data)
	}
	return out
}

// buildCompilerRepo builds a fresh, deterministic hermetic repository
// carrying the real policy-store fixture plus specFiles (every spec this
// test needs landed on main, e.g. ".verdi/specs/active/<name>/spec.md"),
// then generates and commits the one real managed instruction projection
// (AGENTS.md plus its projection manifest) so a compile's stage 5 finds a
// clean, non-drifted projection. Every file lands in ONE scaffold commit
// (fixturegit.Build) plus ONE deterministic "generate projection" commit
// (this function's own runIntegrationGit calls) — both landing directly on
// main, which is sufficient for the real specstate.NewProjector() to
// resolve every spec as accepted-pending-build (SI-... / the same
// statusless-direct-landing shape internal/journey's own
// facts_integration_test.go TestGatherFacts_Integration_LandedSpec proves).
// CI_DEFAULT_BRANCH is pinned so default-branch resolution never depends on
// a configured "origin" remote or symbolic-ref (fixturegit repos carry
// neither).
func buildCompilerRepo(t *testing.T, specFiles map[string]string) *fixturegit.Repo {
	t.Helper()
	files := policyStoreFiles(t)
	for path, content := range specFiles {
		files[path] = content
	}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	runIntegrationGit(t, repo.Dir, "add", "-A")
	runIntegrationGit(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "generate instruction projection")
	repo.Head = strings.TrimSpace(runIntegrationGit(t, repo.Dir, "rev-parse", "HEAD"))
	return repo
}

// gitPorcelainStatus returns `git status --porcelain` output for dir,
// failing the test on a non-zero exit.
func gitPorcelainStatus(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status --porcelain: %v", err)
	}
	return string(out)
}

// integrationBuildRequest builds a grammar-valid Request naming spec as the
// build-phase target under an unrestricted (explicit-universal) scope.
func integrationBuildRequest(spec string) Request {
	return Request{
		Schema:  RequestSchema,
		Adapter: AdapterRef{ID: "codex", Version: "1"},
		Phase:   PhaseBuild,
		Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Spec:    spec,
	}
}

// multiParentStoryRepo builds the real hermetic fixture for the
// story-multi-parent story and its two governing parent features
// (feature-alpha, feature-beta), reusing the exact fixtures.go/fragments_test.go
// corpus already committed under testdata/fragments/.
func multiParentStoryRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	storyData, storyFM := decodeFragmentSpecFixture(t, "story-multi-parent.md")
	alphaData, _ := decodeFragmentSpecFixture(t, "feature-alpha.md")
	betaData, _ := decodeFragmentSpecFixture(t, "feature-beta.md")
	return buildCompilerRepo(t, map[string]string{
		".verdi/specs/active/" + strings.TrimPrefix(storyFM.ID, "spec/") + "/spec.md": string(storyData),
		".verdi/specs/active/feature-alpha/spec.md":                                   string(alphaData),
		".verdi/specs/active/feature-beta/spec.md":                                    string(betaData),
	})
}

// ============================================================================
// Declared-context (SI-91/SI-92) end-to-end fixture
// ============================================================================

// declaredContextADR is a minimal valid non-spec artifact for a parent
// feature to pin as declared context: SI-92 accepts every kind in the closed
// artifact registry, and store.NonSpecArtifactPath resolves an ADR's single
// fixed path (.verdi/adr/<name>.md).
const declaredContextADR = `---
id: adr/ctx-note
kind: adr
title: "Declared context note"
status: accepted
owners: [platform-team]
decided: 2026-01-01
frozen: { at: 2026-01-01, commit: 1111111111111111111111111111111111111111 }
---
# Declared context note

## Context

Prose the declared-context payload carries verbatim.

## Decision

Pin this ADR from a governing feature's context list.

## Consequences

The compile includes the whole artifact's exact pinned bytes.
`

// declaredContextSpec is a spec-kind declared-context target that is NOT one
// of the story's governing parents, so its declared lift survives (unlike
// the overlapping parent-feature pin below, which store authority outranks).
const declaredContextSpec = `---
id: spec/context-only
kind: spec
class: component
title: "Context-only component"
status: active
owners: [platform-team]
---
# Context-only component

Referenced only through a feature's declared context list.
`

// withDeclaredContext returns data (one fixture feature spec's exact bytes)
// with a `context:` frontmatter key carrying refs inserted directly after
// the class line. Building the refs at test time is unavoidable: an exact
// pinned ref names a commit that only exists once the artifact it pins has
// been committed, so no statically committed fixture file can carry one.
func withDeclaredContext(t *testing.T, data []byte, refs ...string) string {
	t.Helper()
	const anchor = "class: feature\n"
	text := string(data)
	if !strings.Contains(text, anchor) {
		t.Fatalf("fixture does not carry %q", anchor)
	}
	return strings.Replace(text, anchor, anchor+"context: ["+strings.Join(refs, ", ")+"]\n", 1)
}

// declaredContextRepo builds the hermetic multi-parent story fixture in
// which BOTH governing parent features declare exact pinned context refs.
// The layers are strictly additive and each pins only already-committed
// commits:
//
//	commit 1: policy store, adr/ctx-note, spec/context-only
//	commit 2: spec/feature-beta, declaring adr/ctx-note@<commit 1>
//	commit 3: spec/feature-alpha, declaring adr/ctx-note@<commit 1> (the
//	          SAME exact ref beta declared — SI-91 union de-duplication),
//	          spec/context-only@<commit 1>, and spec/feature-beta@<commit 2>
//	          (an overlap with a store-authority path), plus the story
//	commit 4: the generated managed instruction projection
//
// Returns the repo and commit 1's exact hex SHA (baseCommit) — the caller
// needs it to assert the manifest's pinned declared-context refs by their
// EXACT byte value (logical + "@" + baseCommit), not merely by prefix and
// length.
func declaredContextRepo(t *testing.T) (*fixturegit.Repo, string) {
	t.Helper()
	storyData, storyFM := decodeFragmentSpecFixture(t, "story-multi-parent.md")
	alphaData, _ := decodeFragmentSpecFixture(t, "feature-alpha.md")
	betaData, _ := decodeFragmentSpecFixture(t, "feature-beta.md")

	base := policyStoreFiles(t)
	base[".verdi/adr/ctx-note.md"] = declaredContextADR
	base[".verdi/specs/active/context-only/spec.md"] = declaredContextSpec
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: base, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	baseCommit := repo.Head

	writeAndCommit := func(message string, files map[string]string) string {
		for rel, content := range files {
			dst := filepath.Join(repo.Dir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", rel, err)
			}
			if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
				t.Fatalf("write %s: %v", rel, err)
			}
		}
		runIntegrationGit(t, repo.Dir, "add", "-A")
		runIntegrationGit(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", message)
		return strings.TrimSpace(runIntegrationGit(t, repo.Dir, "rev-parse", "HEAD"))
	}

	betaCommit := writeAndCommit("land feature-beta", map[string]string{
		".verdi/specs/active/feature-beta/spec.md": withDeclaredContext(t, betaData, "adr/ctx-note@"+baseCommit),
	})
	writeAndCommit("land feature-alpha and the story", map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": withDeclaredContext(t, alphaData,
			"adr/ctx-note@"+baseCommit,
			"spec/context-only@"+baseCommit,
			"spec/feature-beta@"+betaCommit,
		),
		".verdi/specs/active/" + strings.TrimPrefix(storyFM.ID, "spec/") + "/spec.md": string(storyData),
	})

	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	repo.Head = writeAndCommit("generate instruction projection", nil)
	return repo, baseCommit
}

// TestCompile_Integration_DeclaredContextPinnedRefsSurviveIntoManifest is
// the end-to-end proof for SI-91/SI-92's declared-context path: a real
// multi-parent build story whose governing features declare exact pinned
// refs of a non-spec kind and a spec kind, plus one ref that overlaps a
// store-authority path.
func TestCompile_Integration_DeclaredContextPinnedRefsSurviveIntoManifest(t *testing.T) {
	repo, baseCommit := declaredContextRepo(t)
	c := NewCompiler()
	result, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest("spec/story-multi-parent"))
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}
	m := result.Manifest

	// (1) Candidate identity stays the UNPINNED logical ref, while the
	// manifest's included row carries the COMPLETE pinned ref (SI-92).
	for _, logical := range []string{"adr/ctx-note", "spec/context-only"} {
		row := requireIncludedEntry(t, m.Included, "ref:"+logical)
		if row.Source != SourceDeclaredContext || row.Kind != IncludedDeclaredContextRef {
			t.Errorf("%s row = %+v, want source=declared-context kind=declared-context-ref", logical, row)
		}
		if row.Ref == nil {
			t.Fatalf("%s row carries no ref", logical)
		}
		wantRef := logical + "@" + baseCommit
		if *row.Ref != wantRef {
			t.Errorf("%s row ref = %q, want the exact pinned %q", logical, *row.Ref, wantRef)
		}
		// The data item keeps the logical identity; only the manifest row
		// widens to the pinned ref.
		found := false
		for _, item := range result.DataItems {
			if item.ID != "ref:"+logical {
				continue
			}
			if found {
				t.Fatalf("data item %q appears more than once; SI-91 de-duplicates identical exact refs declared by two parents", logical)
			}
			found = true
			if item.Ref == nil || *item.Ref != logical {
				t.Errorf("data item %q ref = %v, want the unpinned logical ref %q", logical, item.Ref, logical)
			}
		}
		if !found {
			t.Errorf("no data item for declared-context ref %q", logical)
		}
	}

	// (2) Both parents declared adr/ctx-note@<same commit>: SI-91's union
	// de-duplicates identical exact refs into ONE candidate and payload.
	adrRows := 0
	for _, e := range m.Included {
		if e.ID == "ref:adr/ctx-note" {
			adrRows++
		}
	}
	if adrRows != 1 {
		t.Errorf("adr/ctx-note has %d included rows, want exactly 1 (two parents declared the identical exact ref)", adrRows)
	}

	// (3) The overlapping pin resolves the path store authority already
	// owns, so SI-92's source precedence classifies it store-authority and
	// suppresses the declared lift — it is not a duplicate candidate and
	// certainly not a compile failure.
	betaRow := requireIncludedEntry(t, m.Included, "ref:spec/feature-beta")
	if betaRow.Source != SourceStoreAuthority || betaRow.Kind != IncludedParentFeatureFragment {
		t.Errorf("spec/feature-beta row = %+v, want source=store-authority kind=parent-feature-fragment", betaRow)
	}
	for _, e := range m.Included {
		if e.Source == SourceDeclaredContext && e.Ref != nil && strings.HasPrefix(*e.Ref, "spec/feature-beta@") {
			t.Errorf("spec/feature-beta also appears as a declared-context row %+v; store authority outranks the declared lift for that path", e)
		}
	}
	// The lifted paths never reappear as repository files.
	for _, path := range []string{".verdi/adr/ctx-note.md", ".verdi/specs/active/context-only/spec.md", ".verdi/specs/active/feature-beta/spec.md"} {
		for _, e := range m.Included {
			if e.Path != nil && *e.Path == path {
				t.Errorf("%s is still an included %s/%s candidate; a lifted path is not duplicated as a repository file", path, e.Source, e.Kind)
			}
		}
	}
}

// requireManifestEntry returns the sole IncludedEntry in entries whose ID
// matches id, failing the test if it is absent or duplicated.
func requireIncludedEntry(t *testing.T, entries []IncludedEntry, id string) IncludedEntry {
	t.Helper()
	var found *IncludedEntry
	for i := range entries {
		if entries[i].ID == id {
			if found != nil {
				t.Fatalf("included entry %q appears more than once", id)
			}
			e := entries[i]
			found = &e
		}
	}
	if found == nil {
		t.Fatalf("no included entry with id %q: %+v", id, entries)
	}
	return *found
}

// containsDisclosure reports whether want appears in codes.
func containsDisclosure(codes []DisclosureCode, want DisclosureCode) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

// TestCompile_Integration_BuildStoryMultiParent_Succeeds is the primary
// hermetic end-to-end proof: a real fixturegit repository, a real
// installed policy store, and NewCompiler()'s real production ports
// compile the multi-parent build story to a complete Result whose manifest
// satisfies authority design §§5, 8, 9's structural invariants.
func TestCompile_Integration_BuildStoryMultiParent_Succeeds(t *testing.T) {
	repo := multiParentStoryRepo(t)
	statusBefore := gitPorcelainStatus(t, repo.Dir)

	c := NewCompiler()
	result, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest("spec/story-multi-parent"))
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}

	statusAfter := gitPorcelainStatus(t, repo.Dir)
	if statusBefore != statusAfter {
		t.Fatalf("Compile mutated the checkout: before=%q after=%q", statusBefore, statusAfter)
	}
	headAfter := strings.TrimSpace(runIntegrationGit(t, repo.Dir, "rev-parse", "HEAD"))
	if headAfter != repo.Head {
		t.Fatalf("Compile moved HEAD: before=%q after=%q", repo.Head, headAfter)
	}

	m := result.Manifest
	if m.Schema != ManifestSchema {
		t.Errorf("Schema = %q, want %q", m.Schema, ManifestSchema)
	}
	if m.Phase != PhaseBuild {
		t.Errorf("Phase = %q, want %q", m.Phase, PhaseBuild)
	}
	if m.Adapter != (AdapterRef{ID: "codex", Version: "1"}) {
		t.Errorf("Adapter = %+v", m.Adapter)
	}
	// The story's exact bytes were landed in the first (scaffold) commit;
	// repo.Head has since moved on to the second (generate-projection)
	// commit buildCompilerRepo adds, so the merge-signaled landing commit
	// AcceptedSpec.Commit names is repo.Heads[0], not repo.Head.
	if m.AcceptedSpec.Ref != "spec/story-multi-parent" || m.AcceptedSpec.Commit != repo.Heads[0] {
		t.Errorf("AcceptedSpec = %+v, want ref spec/story-multi-parent landed at %s", m.AcceptedSpec, repo.Heads[0])
	}
	if m.AcceptedSpec.Commit == "" || m.AcceptedSpec.Blob == "" || m.AcceptedSpec.ContentDigest == "" {
		t.Errorf("AcceptedSpec has an empty identity field: %+v", m.AcceptedSpec)
	}

	// revisions.authority binds; context is exactly 1 with no parent (the
	// domain type carries no field a parent could occupy at all).
	if err := validateDigest("revisions.authority", m.Revisions.Authority); err != nil {
		t.Errorf("Revisions.Authority is not a valid digest: %v", err)
	}
	if m.Revisions.Context != 1 {
		t.Errorf("Revisions.Context = %d, want 1", m.Revisions.Context)
	}

	// multi-parent story: sorted parent_features naming both feature-alpha
	// and feature-beta.
	if len(m.ParentFeatures) != 2 {
		t.Fatalf("ParentFeatures = %+v, want exactly 2 rows", m.ParentFeatures)
	}
	if m.ParentFeatures[0].Ref != "spec/feature-alpha" || m.ParentFeatures[1].Ref != "spec/feature-beta" {
		t.Fatalf("ParentFeatures refs = [%q, %q], want sorted [spec/feature-alpha, spec/feature-beta]", m.ParentFeatures[0].Ref, m.ParentFeatures[1].Ref)
	}
	for _, pf := range m.ParentFeatures {
		if pf.SourceDigest == "" || pf.FragmentDigest == "" || pf.PayloadDigest == "" {
			t.Errorf("ParentFeature %s has an empty digest: %+v", pf.Ref, pf)
		}
	}

	// dispositions/expansions are always [] in v1 (the domain type carries
	// no field for either, so this is a structural — not merely a value —
	// assertion: Manifest simply has no Dispositions/Expansions field).
	var hasDispositionsField, hasExpansionsField bool
	mt := reflect.TypeOf(m)
	for i := 0; i < mt.NumField(); i++ {
		switch mt.Field(i).Name {
		case "Dispositions":
			hasDispositionsField = true
		case "Expansions":
			hasExpansionsField = true
		}
	}
	if hasDispositionsField || hasExpansionsField {
		t.Errorf("Manifest carries a Dispositions/Expansions field (dispositions=%v expansions=%v); v1 must not be able to represent a nonempty value", hasDispositionsField, hasExpansionsField)
	}

	// evidence: advisory, consumed_reports [], fresh (v1 computes every
	// fact from the one HEAD gathered at stage 3 and reused unchanged).
	if m.Evidence.Authority != EvidenceAuthorityAdvisory {
		t.Errorf("Evidence.Authority = %q, want %q", m.Evidence.Authority, EvidenceAuthorityAdvisory)
	}
	if m.Evidence.ConsumedReports == nil || len(m.Evidence.ConsumedReports) != 0 {
		t.Errorf("Evidence.ConsumedReports = %#v, want explicit empty slice", m.Evidence.ConsumedReports)
	}
	if m.Evidence.Freshness != EvidenceFreshnessFresh {
		t.Errorf("Evidence.Freshness = %q, want %q", m.Evidence.Freshness, EvidenceFreshnessFresh)
	}

	// build phase: required_inputs is [].
	if len(m.RequiredInputs) != 0 {
		t.Errorf("RequiredInputs = %+v, want [] for phase build", m.RequiredInputs)
	}

	// actors: the production CLI supplies no principal-resolution port in
	// v1, so posture is explicitly unproven with its mandatory disclosure.
	if m.Actors.Posture != ResolutionUnproven {
		t.Errorf("Actors.Posture = %q, want %q (no ActorResolver wired in NewCompiler)", m.Actors.Posture, ResolutionUnproven)
	}
	if len(m.Actors.Resolutions) != 0 {
		t.Errorf("Actors.Resolutions = %+v, want empty", m.Actors.Resolutions)
	}
	found := false
	for _, d := range m.Actors.Disclosures {
		if d == DisclosureActorResolutionUnproven {
			found = true
		}
	}
	if !found {
		t.Errorf("Actors.Disclosures = %v, want %q", m.Actors.Disclosures, DisclosureActorResolutionUnproven)
	}

	// evidence: disclosures for every stale/unknown fact. fixturegit
	// repositories are never given an "origin" remote, so the computed
	// remote-origin fact is honestly unknown — this must surface both on
	// Repository.Disclosures and (unioned) on the manifest's top-level
	// Disclosures, never silently dropped or upgraded to a guessed value.
	if !containsDisclosure(m.Repository.Disclosures, DisclosureRepositoryRemoteUnknown) {
		t.Errorf("Repository.Disclosures = %v, want %q (fixturegit repos carry no origin remote)", m.Repository.Disclosures, DisclosureRepositoryRemoteUnknown)
	}
	if m.Repository.RemoteOrigin.Known {
		t.Errorf("Repository.RemoteOrigin = %+v, want unknown", m.Repository.RemoteOrigin)
	}
	if !containsDisclosure(m.Disclosures, DisclosureRepositoryRemoteUnknown) {
		t.Errorf("top-level Disclosures = %v, want the union to include %q", m.Disclosures, DisclosureRepositoryRemoteUnknown)
	}

	// included/excluded/opaque: every (source,id) candidate identity
	// (authority design §5: "candidate identity is (source, logical-id)")
	// appears in exactly one ledger — the SAME logical id legitimately
	// appears twice under two DIFFERENT sources (a managed projection
	// output's excluded head-tree copy plus its included projection
	// candidate is the documented case), so the key here is the compound
	// (source,id) pair, never id alone (internal duplicate-free union
	// check — universe_test.go/classify_test.go already prove
	// BuildUniverse/Classify's own total-partition invariant in isolation;
	// this reproves it survived compiler-level assembly).
	seen := make(map[string]string)
	key := func(source Source, id string) string { return string(source) + "\x00" + id }
	for _, e := range m.Included {
		if prior, dup := seen[key(e.Source, e.ID)]; dup {
			t.Errorf("(source,id) (%s,%q) appears in both %s and included", e.Source, e.ID, prior)
		}
		seen[key(e.Source, e.ID)] = "included"
	}
	for _, e := range m.Excluded {
		if prior, dup := seen[key(e.Source, e.ID)]; dup {
			t.Errorf("(source,id) (%s,%q) appears in both %s and excluded", e.Source, e.ID, prior)
		}
		seen[key(e.Source, e.ID)] = "excluded"
	}
	for _, e := range m.Opaque {
		if prior, dup := seen[key(SourceOpaque, e.ID)]; dup {
			t.Errorf("id %q appears in both %s and opaque", e.ID, prior)
		}
		seen[key(SourceOpaque, e.ID)] = "opaque"
	}

	// The accepted target, both parent fragments and the applicable
	// go-toolchain policy are included with a binding content digest;
	// the fixed opaque base names the requested adapter.
	acceptedRow := requireIncludedEntry(t, m.Included, "ref:spec/story-multi-parent")
	if acceptedRow.Source != SourceStoreAuthority || acceptedRow.Kind != IncludedAcceptedSpec || acceptedRow.PayloadChannel != ChannelData {
		t.Errorf("accepted-spec included row = %+v", acceptedRow)
	}
	if acceptedRow.ContentDigest != m.AcceptedSpec.ContentDigest {
		t.Errorf("accepted-spec included row content digest %q != AcceptedSpec.ContentDigest %q", acceptedRow.ContentDigest, m.AcceptedSpec.ContentDigest)
	}
	alphaRow := requireIncludedEntry(t, m.Included, "ref:spec/feature-alpha")
	if alphaRow.Kind != IncludedParentFeatureFragment {
		t.Errorf("feature-alpha included row = %+v", alphaRow)
	}
	policyRow := requireIncludedEntry(t, m.Included, "ref:policy/go-toolchain")
	if policyRow.Kind != IncludedPolicyArtifact {
		t.Errorf("policy/go-toolchain included row = %+v", policyRow)
	}
	if len(m.Opaque) != 1 || m.Opaque[0].Adapter != (AdapterRef{ID: "codex", Version: "1"}) {
		t.Fatalf("Opaque = %+v, want exactly one codex/1 harness-vendor-base row", m.Opaque)
	}

	// The generated instruction-projection file is the only authority-
	// channel included entry, and result.ProjectionFiles/ProjectionFileRef
	// digests agree.
	agentsRow := requireIncludedEntry(t, m.Included, "path:AGENTS.md")
	if agentsRow.Source != SourceProjection || agentsRow.Kind != IncludedInstructionProjection || agentsRow.PayloadChannel != ChannelAuthority {
		t.Errorf("AGENTS.md included row = %+v", agentsRow)
	}
	for _, e := range m.Included {
		if e.PayloadChannel == ChannelAuthority && (e.Source != SourceProjection || e.Kind != IncludedInstructionProjection) {
			t.Errorf("only source=projection/kind=instruction-projection may carry payload_channel authority, got %+v", e)
		}
	}
	if len(m.ProjectionFiles) != 1 || m.ProjectionFiles[0].Path != "AGENTS.md" {
		t.Fatalf("ProjectionFiles = %+v, want exactly one AGENTS.md row", m.ProjectionFiles)
	}
	if len(result.ProjectionFiles) != 1 || result.ProjectionFiles[0].Path != "AGENTS.md" {
		t.Fatalf("result.ProjectionFiles = %+v, want exactly one AGENTS.md row", result.ProjectionFiles)
	}
	if result.ProjectionFiles[0].Digest != m.ProjectionFiles[0].Digest {
		t.Errorf("result.ProjectionFiles digest %q != manifest ProjectionFiles digest %q", result.ProjectionFiles[0].Digest, m.ProjectionFiles[0].Digest)
	}
	if rawContentDigest(result.ProjectionFiles[0].Content) != result.ProjectionFiles[0].Digest {
		t.Errorf("result.ProjectionFiles[0].Digest does not bind Content")
	}

	// every included digest binds the returned data-item bytes: DataItem's
	// own `digest` field is the SHA-256 of its DIGESTLESS canonical form
	// (authority design §8.1), not of DataItemBytes[i]'s own full encoded
	// bytes (which include that digest field) — so the round-trip proof is
	// EncodeDataItem(item) == DataItemBytes[i], not rawContentDigest of the
	// outer bytes.
	byID := make(map[string]DataItem, len(result.DataItems))
	for i, item := range result.DataItems {
		byID[item.ID] = item
		reencoded, err := EncodeDataItem(item)
		if err != nil {
			t.Errorf("EncodeDataItem(DataItems[%d]): %v", i, err)
			continue
		}
		if !bytes.Equal(reencoded, result.DataItemBytes[i]) {
			t.Errorf("EncodeDataItem(DataItems[%d]) != DataItemBytes[%d] for id %q", i, i, item.ID)
		}
	}
	for _, e := range m.Included {
		if e.PayloadChannel != ChannelData {
			continue
		}
		item, ok := byID[e.ID]
		if !ok {
			t.Errorf("included data-channel row %q has no corresponding result.DataItems entry", e.ID)
			continue
		}
		if item.Digest != e.PayloadDigest {
			t.Errorf("data item %q digest %q != included row payload_digest %q", e.ID, item.Digest, e.PayloadDigest)
		}
		if item.ContentDigest != e.ContentDigest {
			t.Errorf("data item %q content digest %q != included row content_digest %q", e.ID, item.ContentDigest, e.ContentDigest)
		}
	}

	// Round-tripping the returned manifest bytes must reproduce the exact
	// same decoded Manifest and must byte-equal ManifestBytes.
	decoded, err := DecodeManifest(result.ManifestBytes)
	if err != nil {
		t.Fatalf("DecodeManifest(result.ManifestBytes): %v", err)
	}
	if !reflect.DeepEqual(decoded, m) {
		t.Errorf("DecodeManifest(result.ManifestBytes) != result.Manifest")
	}
	reencoded, err := EncodeManifest(m)
	if err != nil {
		t.Fatalf("EncodeManifest(result.Manifest): %v", err)
	}
	if !bytes.Equal(reencoded, result.ManifestBytes) {
		t.Errorf("EncodeManifest(result.Manifest) != result.ManifestBytes")
	}
}

// TestCompile_Integration_Golden_BuildStoryMultiParent ratchets the exact
// canonical manifest bytes a compile of the multi-parent build story
// produces (Task 7 Step 2: "deterministic goldens ... with exact-byte
// ratchets"). fixturegit's fixed author/committer/date identity plus this
// file's own integrationCommitEnv make every input (blob objects, commit
// SHAs, digests) byte-stable across machines and runs, so this ratchet is
// exact-byte, not merely structural.
func TestCompile_Integration_Golden_BuildStoryMultiParent(t *testing.T) {
	repo := multiParentStoryRepo(t)
	c := NewCompiler()
	result, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest("spec/story-multi-parent"))
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}

	golden := mustReadFixture(t, "golden/manifest-build-story-multi-parent.json")
	if !bytes.Equal(result.ManifestBytes, golden) {
		t.Fatalf("manifest bytes drifted from the committed golden ratchet\ngot:  %s\nwant: %s", result.ManifestBytes, golden)
	}

	if got := string(result.Manifest.Revisions.Authority); got != goldenAuthorityRevision {
		t.Fatalf("revisions.authority = %s, want %s", got, goldenAuthorityRevision)
	}
}

// goldenAuthorityRevision is the authority revision the golden fixture
// carried BEFORE the `.verdi/data` boundary row became unconditional, kept
// here as an independent, hand-transcribed constant rather than read back
// out of the fixture. Authority design §9 enumerates the authority
// revision's private preimage exhaustively — effective policy, accepted
// spec, parent fragments, decisions, obligations — and states outright that
// "repository state ... and payload classification ... are bound by the
// manifest self digest, not folded into authority identity". Adding an
// excluded repository-state row is therefore required to move the self
// digest and required NOT to move this value; a regeneration that moved
// both would mean the preimage had silently absorbed classification.
const goldenAuthorityRevision = "sha256:45ab861faa1f9a4c01b5dd16d3da4604c9b990faec29883779cc033a0cef5fb1"

// TestCompile_Integration_DeterministicAcrossTwoRoots proves identical
// trusted inputs (HEAD, request, authority) built independently under two
// wholly separate checkouts yield byte-identical manifests, data items and
// projections (authority design §10's first row).
func TestCompile_Integration_DeterministicAcrossTwoRoots(t *testing.T) {
	repoA := multiParentStoryRepo(t)
	repoB := multiParentStoryRepo(t)
	if repoA.Head != repoB.Head {
		t.Fatalf("fixture is not itself deterministic: rootA HEAD=%s rootB HEAD=%s", repoA.Head, repoB.Head)
	}

	c := NewCompiler()
	req := integrationBuildRequest("spec/story-multi-parent")
	resultA, err := c.Compile(context.Background(), repoA.Dir, req)
	if err != nil {
		t.Fatalf("Compile(rootA): %v", err)
	}
	resultB, err := c.Compile(context.Background(), repoB.Dir, req)
	if err != nil {
		t.Fatalf("Compile(rootB): %v", err)
	}

	if !bytes.Equal(resultA.ManifestBytes, resultB.ManifestBytes) {
		t.Fatalf("ManifestBytes differ across two identically-built roots:\nA: %s\nB: %s", resultA.ManifestBytes, resultB.ManifestBytes)
	}
	if len(resultA.DataItemBytes) != len(resultB.DataItemBytes) {
		t.Fatalf("DataItemBytes length differs: A=%d B=%d", len(resultA.DataItemBytes), len(resultB.DataItemBytes))
	}
	for i := range resultA.DataItemBytes {
		if !bytes.Equal(resultA.DataItemBytes[i], resultB.DataItemBytes[i]) {
			t.Errorf("DataItemBytes[%d] differs across two identically-built roots", i)
		}
	}
	if len(resultA.ProjectionFiles) != len(resultB.ProjectionFiles) {
		t.Fatalf("ProjectionFiles length differs: A=%d B=%d", len(resultA.ProjectionFiles), len(resultB.ProjectionFiles))
	}
	for i := range resultA.ProjectionFiles {
		if resultA.ProjectionFiles[i].Path != resultB.ProjectionFiles[i].Path || !bytes.Equal(resultA.ProjectionFiles[i].Content, resultB.ProjectionFiles[i].Content) {
			t.Errorf("ProjectionFiles[%d] differs across two identically-built roots", i)
		}
	}
}

// TestCompile_Integration_SpikeMultiParent_Succeeds proves the spike
// variant: `resolves` edges, oq-* targets, and no acceptance-criteria
// obligations to resolve.
func TestCompile_Integration_SpikeMultiParent_Succeeds(t *testing.T) {
	spikeData, spikeFM := decodeFragmentSpecFixture(t, "spike-multi-parent.md")
	alphaData, _ := decodeFragmentSpecFixture(t, "feature-alpha.md")
	betaData, _ := decodeFragmentSpecFixture(t, "feature-beta.md")
	repo := buildCompilerRepo(t, map[string]string{
		".verdi/specs/active/" + strings.TrimPrefix(spikeFM.ID, "spec/") + "/spec.md": string(spikeData),
		".verdi/specs/active/feature-alpha/spec.md":                                   string(alphaData),
		".verdi/specs/active/feature-beta/spec.md":                                    string(betaData),
	})

	c := NewCompiler()
	result, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest("spec/spike-multi-parent"))
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}
	m := result.Manifest
	if len(m.ParentFeatures) != 2 {
		t.Fatalf("ParentFeatures = %+v, want exactly 2 rows", m.ParentFeatures)
	}
	// A spike's fragments target the OPEN QUESTIONS its `resolves` edges
	// name, never acceptance criteria (authority design §6 Build: "a spike
	// uses each `resolves` target open-question fragment"). Each parent
	// fragment's payload is the canonical fragment JSON of §8.1, so the
	// target shape is assertable straight off the returned data item.
	payloadByID := make(map[string]string, len(result.DataItems))
	for _, item := range result.DataItems {
		payloadByID[item.ID] = item.Content
	}
	fragmentRows := 0
	for _, e := range m.Included {
		if e.Source != SourceStoreAuthority || e.Kind != IncludedParentFeatureFragment || e.Ref == nil {
			continue
		}
		fragmentRows++
		payload, ok := payloadByID[e.ID]
		if !ok {
			t.Errorf("parent fragment %s has no returned data item", e.ID)
			continue
		}
		var decoded struct {
			Targets []struct {
				ID string `json:"id"`
			} `json:"targets"`
		}
		if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
			t.Errorf("parent fragment %s payload is not the canonical fragment object: %v", e.ID, err)
			continue
		}
		if len(decoded.Targets) == 0 {
			t.Errorf("parent fragment %s carries no targets", e.ID)
		}
		for _, target := range decoded.Targets {
			if !strings.HasPrefix(target.ID, "oq-") {
				t.Errorf("parent fragment %s targets %q; a spike's fragment targets only oq-* open questions", e.ID, target.ID)
			}
		}
	}
	if fragmentRows != 2 {
		t.Errorf("found %d parent-feature-fragment included rows, want 2", fragmentRows)
	}
}

// countingRepoFactsGatherer wraps a RepositoryFactsGatherer, counting every
// Gather call and — from the SECOND call onward — returning a deliberately
// DIFFERENT snapshot than the first. A compile must disclose the repository
// facts it actually used, so it may gather exactly once (authority design
// §8.2's `repository` row plus §4's "computed once"); a second gather is
// both a redundant read and a window in which the manifest could publish
// facts no other stage ever consumed.
type countingRepoFactsGatherer struct {
	RepositoryFactsGatherer
	calls *int
}

func (g countingRepoFactsGatherer) Gather(ctx context.Context, in repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error) {
	*g.calls++
	snapshot, err := g.RepositoryFactsGatherer.Gather(ctx, in)
	if err != nil || *g.calls == 1 {
		return snapshot, err
	}
	snapshot.Facts.Branch = repositoryfacts.StringFact{Known: true, Value: "poisoned-second-gather"}
	return snapshot, nil
}

// --- .verdi/data/ subtree privacy over a real, gitignored zone ----------

// dataZoneSentinel is written into every file in the hermetic data zone
// below, so "no data-zone byte reached the manifest" is an assertion with
// something recognizable to find rather than a vacuous one.
const dataZoneSentinel = "SENTINEL-DATA-ZONE-BYTES"

// dataZoneGuardGitReader wraps a GitReader and panics the moment any port
// is asked about a path at or under the `.verdi/data` boundary — extending
// this package's existing panicking-fake discipline (compiler_test.go's
// panicGitReader, classify_test.go's forbidden-Show fake) from "this stage
// must not run" to "this SUBTREE must never be read". The manifest's
// boundary row must be produced from the fixed path identity alone, never
// from an inspection of the zone.
type dataZoneGuardGitReader struct {
	GitReader
}

func (g dataZoneGuardGitReader) Show(ctx context.Context, root, ref, path string) ([]byte, error) {
	if path == ".verdi/data" || strings.HasPrefix(path, ".verdi/data/") {
		panic("contextcompile: the compiler read a byte at or under the .verdi/data boundary: " + path)
	}
	return g.GitReader.Show(ctx, root, ref, path)
}

// gitignoredDataZoneRepo builds the conformant-store fixture: the standard
// multi-parent story repository plus a committed `.verdi/.gitignore` that
// ignores `data/`, and a populated `.verdi/data/` tree created AFTER the
// commits — exactly the shape a real store has. Because the zone is
// gitignored, `git ls-tree` never lists it and `git status --porcelain`
// (which the compiler runs without `--ignored`) never reports it, so NO
// git port can tell the compiler the zone exists.
func gitignoredDataZoneRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	storyData, storyFM := decodeFragmentSpecFixture(t, "story-multi-parent.md")
	alphaData, _ := decodeFragmentSpecFixture(t, "feature-alpha.md")
	betaData, _ := decodeFragmentSpecFixture(t, "feature-beta.md")
	repo := buildCompilerRepo(t, map[string]string{
		".verdi/.gitignore": "data/\n",
		".verdi/specs/active/" + strings.TrimPrefix(storyFM.ID, "spec/") + "/spec.md": string(storyData),
		".verdi/specs/active/feature-alpha/spec.md":                                   string(alphaData),
		".verdi/specs/active/feature-beta/spec.md":                                    string(betaData),
	})

	nested := filepath.Join(repo.Dir, ".verdi", "data", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("creating the data zone: %v", err)
	}
	for _, f := range []struct{ path, body string }{
		{filepath.Join(repo.Dir, ".verdi", "data", "secret-one.json"), `{"secret":"` + dataZoneSentinel + `"}` + "\n"},
		{filepath.Join(nested, "secret-two.json"), `{"secret":"` + dataZoneSentinel + `"}` + "\n"},
	} {
		if err := os.WriteFile(f.path, []byte(f.body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", f.path, err)
		}
	}
	return repo
}

// TestCompile_Integration_DataZoneBoundaryRow_GitignoredZone proves
// authority design §5's UNCONDITIONAL sentence — "`.verdi/data/` is
// represented by one excluded subtree-boundary candidate; its descendants
// are neither enumerated nor named" — holds in the store shape that
// actually ships: a `.verdi/.gitignore` ignoring `data/`. Gating the row on
// having observed a data-zone path made it unreachable here, since a
// gitignored zone is invisible to both `git ls-tree` and `git status
// --porcelain`.
//
// The row's source is `worktree-overlay`: the Wave-3 plan's binding
// constraint "Worktree-overlay candidates contain path identity and
// exclusion facts only" is exactly the boundary row's mandated shape, while
// head-tree candidates are HEAD tree entries carrying blob object identity
// — which a gitignored zone can never have.
func TestCompile_Integration_DataZoneBoundaryRow_GitignoredZone(t *testing.T) {
	repo := gitignoredDataZoneRepo(t)

	// Precondition: prove the zone really is invisible to both git ports,
	// so the boundary row below cannot have come from an observation.
	tree := runIntegrationGit(t, repo.Dir, "ls-tree", "-r", "--name-only", "HEAD")
	if strings.Contains(tree, ".verdi/data") {
		t.Fatalf("fixture is not a gitignored zone: ls-tree lists a data-zone path:\n%s", tree)
	}
	if status := gitPorcelainStatus(t, repo.Dir); strings.Contains(status, ".verdi/data") {
		t.Fatalf("fixture is not a gitignored zone: porcelain status reports a data-zone path:\n%s", status)
	}

	c := newCompilerWithPorts(
		dataZoneGuardGitReader{gitxGitReader{}},
		specstate.NewProjector(),
		defaultAuthorityLoader{},
		nil,
		repositoryfacts.NewGatherer(),
		defaultProjectionVerifier{},
	)
	result, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest("spec/story-multi-parent"))
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}

	entry := requireExcludedEntry(t, result.Manifest.Excluded, "path:"+dataZoneBoundaryPath)
	if entry.Source != SourceWorktreeOverlay {
		t.Fatalf("boundary row source = %q, want %q", entry.Source, SourceWorktreeOverlay)
	}
	if entry.Reason != ExclusionDataZoneDisposable {
		t.Fatalf("boundary row reason = %q, want %q", entry.Reason, ExclusionDataZoneDisposable)
	}
	if entry.Path == nil || *entry.Path != dataZoneBoundaryPath {
		t.Fatalf("boundary row path = %v, want %q", entry.Path, dataZoneBoundaryPath)
	}
	if entry.Ref != nil {
		t.Fatalf("boundary row carries ref %q, want none", *entry.Ref)
	}

	// The boundary appears exactly once, across ALL three ledgers.
	var boundaryRows int
	for _, e := range result.Manifest.Excluded {
		if e.ID == "path:"+dataZoneBoundaryPath {
			boundaryRows++
		}
	}
	for _, e := range result.Manifest.Included {
		if e.ID == "path:"+dataZoneBoundaryPath {
			t.Fatalf("the data-zone boundary appears in `included`: %+v", e)
		}
	}
	if boundaryRows != 1 {
		t.Fatalf("data-zone boundary rows = %d, want exactly 1", boundaryRows)
	}

	// No descendant is enumerated OR named, anywhere in the manifest or in
	// any data payload — and no data-zone byte was read.
	for _, forbidden := range []string{".verdi/data/", "secret-one", "secret-two", "nested", dataZoneSentinel} {
		if bytes.Contains(result.ManifestBytes, []byte(forbidden)) {
			t.Fatalf("manifest names %q, violating the collapsed-boundary privacy rule", forbidden)
		}
		for i, item := range result.DataItemBytes {
			if bytes.Contains(item, []byte(forbidden)) {
				t.Fatalf("data item %d names %q, violating the collapsed-boundary privacy rule", i, forbidden)
			}
		}
		for _, projection := range result.ProjectionFiles {
			if bytes.Contains(projection.Content, []byte(forbidden)) {
				t.Fatalf("projection %s names %q, violating the collapsed-boundary privacy rule", projection.Path, forbidden)
			}
		}
	}
}

// requireExcludedEntry returns the sole ExcludedEntry in entries whose ID
// matches id, failing the test if it is absent or duplicated.
func requireExcludedEntry(t *testing.T, entries []ExcludedEntry, id string) ExcludedEntry {
	t.Helper()
	var found *ExcludedEntry
	for i := range entries {
		if entries[i].ID == id {
			if found != nil {
				t.Fatalf("excluded entry %q appears more than once", id)
			}
			e := entries[i]
			found = &e
		}
	}
	if found == nil {
		t.Fatalf("no excluded entry with id %q: %+v", id, entries)
	}
	return *found
}

// TestCompile_Integration_ConstitutionAndProfileAreStoreAuthority proves the
// resolved constitution and the SELECTED governance profile are lifted into
// store-authority, not left as ordinary head-tree repository files.
// Authority design §5's store-authority row is an exact enumeration —
// "Resolved constitution, profile, applicable policies/overlays/exemptions,
// accepted spec, parent feature fragments, and obligations" — and §5 also
// fixes that "a tracked path lifted here is not duplicated as a
// repository-file candidate".
func TestCompile_Integration_ConstitutionAndProfileAreStoreAuthority(t *testing.T) {
	repo := multiParentStoryRepo(t)
	c := NewCompiler()
	result, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest("spec/story-multi-parent"))
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}
	m := result.Manifest

	for _, ref := range []string{"policy-constitution/constitution", "governance-profile/solo-default"} {
		row := requireIncludedEntry(t, m.Included, "ref:"+ref)
		if row.Source != SourceStoreAuthority || row.Kind != IncludedPolicyArtifact {
			t.Errorf("%s included row = %+v, want source=store-authority kind=policy-artifact", ref, row)
		}
		if row.PayloadChannel != ChannelData {
			t.Errorf("%s payload channel = %q, want %q", ref, row.PayloadChannel, ChannelData)
		}
	}
	// The two lifted paths must NOT reappear as repository-file candidates.
	for _, path := range []string{".verdi/policy/constitution.md", ".verdi/policy/profiles/solo-default.md"} {
		for _, e := range m.Included {
			if e.Path != nil && *e.Path == path {
				t.Errorf("%s is still an included %s/%s candidate; a lifted path must not be duplicated as a repository file", path, e.Source, e.Kind)
			}
		}
		for _, e := range m.Excluded {
			if e.Path != nil && *e.Path == path {
				t.Errorf("%s is still an excluded %s candidate; a lifted path must not be duplicated as a repository file", path, e.Source)
			}
		}
	}
	// The two digests the manifest's own policy section already claims must
	// be exactly the ones the lifted candidates carry.
	constitutionRow := requireIncludedEntry(t, m.Included, "ref:policy-constitution/constitution")
	if constitutionRow.ContentDigest == "" {
		t.Error("lifted constitution carries no content digest")
	}
	if m.Policy.ProfileID != "solo-default" {
		t.Fatalf("Policy.ProfileID = %q, want solo-default", m.Policy.ProfileID)
	}
}

// TestCompile_Integration_NarrowedPathScopeKeepsAuthority proves declared
// scope bounds REPOSITORY material only (authority design §6: "Scope still
// bounds repository material"): a compile scoped to `cmd/` excludes ordinary
// tracked files as out-of-declared-scope while the applicable constitution
// and selected profile — §6 Build's required capsule content — remain
// included store-authority rows.
func TestCompile_Integration_NarrowedPathScopeKeepsAuthority(t *testing.T) {
	repo := multiParentStoryRepo(t)
	req := integrationBuildRequest("spec/story-multi-parent")
	req.Scope.Paths = []string{"cmd/"}

	c := NewCompiler()
	result, err := c.Compile(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}
	m := result.Manifest

	for _, ref := range []string{"policy-constitution/constitution", "governance-profile/solo-default", "spec/story-multi-parent"} {
		row := requireIncludedEntry(t, m.Included, "ref:"+ref)
		if row.Source != SourceStoreAuthority {
			t.Errorf("%s row source = %q, want store-authority under a narrowed path scope", ref, row.Source)
		}
	}
	// An ordinary tracked repository file outside cmd/ is scoped out.
	scopedOut := requireExcludedEntry(t, m.Excluded, "path:.verdi/policy/overlays/frontend-go-version.md")
	if scopedOut.Source != SourceHeadTree || scopedOut.Reason != ExclusionOutOfDeclaredScope {
		t.Errorf("scoped-out repository file = %+v, want head-tree/out-of-declared-scope", scopedOut)
	}
}

// editStoreArtifactInWorkingTreeOnly rewrites one .verdi/policy/ artifact in
// the WORKING TREE (never committing it), applying old->new exactly once,
// then regenerates the managed instruction projection so the compile's stage
// 5 still verifies clean against the edited store. The result is a
// reachable, otherwise-legal checkout in which the adopted authority
// policyauthority.Load reads differs from the exact HEAD bytes the compiler
// wraps as that operand's payload.
func editStoreArtifactInWorkingTreeOnly(t *testing.T, repo *fixturegit.Repo, rel, old, new string) {
	t.Helper()
	path := filepath.Join(repo.Dir, ".verdi", "policy", filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	edited := strings.Replace(string(data), old, new, 1)
	if edited == string(data) {
		t.Fatalf("fixture edit did not apply: %q is absent from %s", old, rel)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("regenerate instruction projection: %v", err)
	}
}

// TestCompile_Integration_AdoptedAuthorityDivergingFromHeadIsOperational
// proves the compiler never ships stale HEAD bytes under a fresh adopted
// digest. The adopted digest of every policy operand (and of the resolved
// constitution and selected profile) comes from the WORKING-TREE store load,
// while the payload bytes come from `git show HEAD`. When an adopted
// artifact has been edited but not committed those two disagree, and the
// manifest would otherwise claim the adopted digest over the older bytes.
// This is inconsistent authority — an operational (exit-2) failure per
// authority design §10's "Malformed/noncanonical request or authority",
// matching resolveOwners's and reverifyGoverningFeature's existing TOCTOU
// discipline — never a typed refusal.
func TestCompile_Integration_AdoptedAuthorityDivergingFromHeadIsOperational(t *testing.T) {
	cases := map[string]struct {
		rel, old, new string
		wantNamed     string
	}{
		"selected policy operand": {
			rel:       "policies/go-toolchain.md",
			old:       `title: "Go toolchain policy"`,
			new:       `title: "Go toolchain policy (uncommitted edit)"`,
			wantNamed: "policy/go-toolchain",
		},
		"resolved constitution": {
			rel:       "constitution.md",
			old:       `title: "Fixture project constitution"`,
			new:       `title: "Fixture project constitution (uncommitted edit)"`,
			wantNamed: "policy-constitution/constitution",
		},
		"selected governance profile": {
			rel:       "profiles/solo-default.md",
			old:       `{role: author, trust_source: github-org, subjects: [alice]}`,
			new:       `{role: author, trust_source: github-org, subjects: [alice, bob]}`,
			wantNamed: "governance-profile/solo-default",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := multiParentStoryRepo(t)
			editStoreArtifactInWorkingTreeOnly(t, repo, tc.rel, tc.old, tc.new)

			c := NewCompiler()
			_, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest("spec/story-multi-parent"))
			if err == nil {
				t.Fatal("Compile completed over authority whose adopted digest does not bind the HEAD bytes it wrapped")
			}
			if IsRefusal(err) {
				t.Fatalf("adopted/HEAD authority divergence classified as a refusal (want operational): %T %v", err, err)
			}
			if !strings.Contains(err.Error(), tc.wantNamed) {
				t.Errorf("error %q does not name the diverging operand %q", err, tc.wantNamed)
			}
		})
	}
}

// TestCompile_Integration_RepositoryFactsGatheredExactlyOnce proves Compile
// gathers repository facts exactly once and publishes THOSE facts in the
// manifest: the fixture's second (and any later) Gather deliberately reports
// a different branch, so a manifest naming "poisoned-second-gather" — or a
// call count above one — witnesses a manifest row inconsistent with the
// facts stage 3 actually checked the caller expectation against.
func TestCompile_Integration_RepositoryFactsGatheredExactlyOnce(t *testing.T) {
	repo := multiParentStoryRepo(t)
	calls := 0
	c := newCompilerWithPorts(
		gitxGitReader{}, specstate.NewProjector(), defaultAuthorityLoader{}, nil,
		countingRepoFactsGatherer{RepositoryFactsGatherer: repositoryfacts.NewGatherer(), calls: &calls},
		defaultProjectionVerifier{},
	)
	result, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest("spec/story-multi-parent"))
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("repositoryfacts Gather called %d times, want exactly 1 per Compile", calls)
	}
	if got := result.Manifest.Repository.Branch.Value; got != "main" {
		t.Errorf("Repository.Branch = %q, want the stage-3 fact %q (the manifest must disclose the facts this compile used)", got, "main")
	}
}

// TestCompile_Integration_DesignFeature_Succeeds proves a class:feature
// target compiles under phase design (unlike build, which
// DeclaredScopeRefusal forbids for a feature target): required_inputs is
// [], and the target has no parent_features of its own.
func TestCompile_Integration_DesignFeature_Succeeds(t *testing.T) {
	alphaData, alphaFM := decodeFragmentSpecFixture(t, "feature-alpha.md")
	repo := buildCompilerRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": string(alphaData),
	})

	req := integrationBuildRequest(alphaFM.ID)
	req.Phase = PhaseDesign
	c := NewCompiler()
	result, err := c.Compile(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}
	m := result.Manifest
	if m.Phase != PhaseDesign {
		t.Errorf("Phase = %q, want design", m.Phase)
	}
	if len(m.ParentFeatures) != 0 {
		t.Errorf("ParentFeatures = %+v, want [] for a feature target", m.ParentFeatures)
	}
	if len(m.RequiredInputs) != 0 {
		t.Errorf("RequiredInputs = %+v, want [] for phase design", m.RequiredInputs)
	}
}

// TestCompile_Integration_Review_RequiredInputsFiveRows proves the review
// capsule's exactly-five closed required_inputs rows (authority design §6,
// §8.2): accepted-spec and review-policy proven with a digest witness;
// result-diff, evidence-bundle and builder-receipt unproven with their
// fixed disclosure codes, and the compile still completes (exit-0
// advisory).
func TestCompile_Integration_Review_RequiredInputsFiveRows(t *testing.T) {
	repo := multiParentStoryRepo(t)
	req := integrationBuildRequest("spec/story-multi-parent")
	req.Phase = PhaseReview

	c := NewCompiler()
	result, err := c.Compile(context.Background(), repo.Dir, req)
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}
	m := result.Manifest
	wantKinds := []string{
		RequiredInputAcceptedSpec, RequiredInputBuilderReceipt, RequiredInputEvidenceBundle,
		RequiredInputResultDiff, RequiredInputReviewPolicy,
	}
	if len(m.RequiredInputs) != len(wantKinds) {
		t.Fatalf("RequiredInputs = %+v, want exactly %d rows", m.RequiredInputs, len(wantKinds))
	}
	for i, kind := range wantKinds {
		row := m.RequiredInputs[i]
		if row.Kind != kind {
			t.Fatalf("RequiredInputs[%d].Kind = %q, want %q (rows must be sorted by kind)", i, row.Kind, kind)
		}
		switch kind {
		case RequiredInputAcceptedSpec, RequiredInputReviewPolicy:
			if row.Resolution != ResolutionProven || row.Digest == nil {
				t.Errorf("RequiredInputs[%d] (%s) = %+v, want proven with a digest witness", i, kind, row)
			}
		default:
			if row.Resolution != ResolutionUnproven || row.Digest != nil || len(row.Witnesses) == 0 {
				t.Errorf("RequiredInputs[%d] (%s) = %+v, want unproven with a disclosure witness and no digest", i, kind, row)
			}
		}
	}
	wantDisclosures := []DisclosureCode{
		DisclosureReviewBuilderReceiptUnproven, DisclosureReviewEvidenceBundleUnproven, DisclosureReviewResultDiffUnproven,
	}
	for _, want := range wantDisclosures {
		found := false
		for _, d := range m.Disclosures {
			if d == want {
				found = true
			}
		}
		if !found {
			t.Errorf("top-level Disclosures = %v, want it to include %q", m.Disclosures, want)
		}
	}
}

// TestCompile_Integration_ExpectedRepositoryMismatch_RealFacts proves the
// expected-repository refusal against REAL computed repository facts (not
// the fake repositoryfacts.Snapshot compiler_test.go's stage-3 tests
// inject), exercising the real repositoryfacts.NewGatherer() wiring
// NewCompiler() constructs.
func TestCompile_Integration_ExpectedRepositoryMismatch_RealFacts(t *testing.T) {
	repo := multiParentStoryRepo(t)
	req := integrationBuildRequest("spec/story-multi-parent")
	req.Expected = &Expected{Branch: "not-main", Head: repo.Head}

	c := NewCompiler()
	_, err := c.Compile(context.Background(), repo.Dir, req)
	var refusal *ExpectedRepositoryMismatchRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *ExpectedRepositoryMismatchRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("ExpectedRepositoryMismatchRefusal not classified as a refusal")
	}
	if refusal.ComputedBranch != "main" || refusal.ComputedHead != repo.Head {
		t.Fatalf("refusal computed facts = %+v, want branch=main head=%s", refusal, repo.Head)
	}
}

// TestCompile_Integration_ProjectionDrift_RealFacts proves the projection-
// drift refusal against the REAL instructionprojection.Verify wiring: a
// repository whose managed AGENTS.md was never generated (or has since
// drifted from the constitution) refuses at stage 5, before any Git object
// read that stage 4/6 would otherwise need.
func TestCompile_Integration_ProjectionDrift_RealFacts(t *testing.T) {
	storyData, storyFM := decodeFragmentSpecFixture(t, "story-multi-parent.md")
	alphaData, _ := decodeFragmentSpecFixture(t, "feature-alpha.md")
	betaData, _ := decodeFragmentSpecFixture(t, "feature-beta.md")
	files := policyStoreFiles(t)
	files[".verdi/specs/active/"+strings.TrimPrefix(storyFM.ID, "spec/")+"/spec.md"] = string(storyData)
	files[".verdi/specs/active/feature-alpha/spec.md"] = string(alphaData)
	files[".verdi/specs/active/feature-beta/spec.md"] = string(betaData)
	// Deliberately skip buildCompilerRepo's Generate+commit step: no
	// AGENTS.md is ever written, so the managed projection is absent
	// (drifted) rather than clean.
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	c := NewCompiler()
	_, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest("spec/story-multi-parent"))
	var refusal *ProjectionDriftRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *ProjectionDriftRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("ProjectionDriftRefusal not classified as a refusal")
	}
	if len(refusal.Paths) == 0 {
		t.Fatal("ProjectionDriftRefusal.Paths is empty, want it to name AGENTS.md")
	}
}

// TestCompile_Integration_WrongTargetClass_RealFacts is
// TestCompilerStage4WrongClassRefusalShortCircuits's real-git counterpart:
// a feature specification requested as a build target refuses with
// *DeclaredScopeRefusal even when every port is the genuine production
// adapter over a real hermetic repository.
func TestCompile_Integration_WrongTargetClass_RealFacts(t *testing.T) {
	alphaData, alphaFM := decodeFragmentSpecFixture(t, "feature-alpha.md")
	repo := buildCompilerRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": string(alphaData),
	})

	c := NewCompiler()
	_, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest(alphaFM.ID))
	var refusal *DeclaredScopeRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected *DeclaredScopeRefusal, got %T %v", err, err)
	}
	if !IsRefusal(err) {
		t.Fatal("wrong-class DeclaredScopeRefusal not classified as a refusal")
	}
}

// TestCompile_Integration_ActorResolver_SealedResolutionsProjectDeterministically
// injects a real sealed ActorResolver (governanceprincipal.PrincipalResolution
// values minted the only way one exists: through
// governanceprincipal.Resolver.Resolve, mirroring this file's own
// mintResolution helper) alongside every other real production port,
// proving the manifest's actors section reflects the injected resolutions
// without needing repository/authority facts to be faked too.
func TestCompile_Integration_ActorResolver_SealedResolutionsProjectDeterministically(t *testing.T) {
	repo := multiParentStoryRepo(t)
	author := mintResolution(t, authenticatedFact("user-123"), "user-123")

	c := newCompilerWithPorts(
		gitxGitReader{}, specstate.NewProjector(), defaultAuthorityLoader{},
		fakeActorResolver{resolutions: []governanceprincipal.PrincipalResolution{author}},
		repositoryfacts.NewGatherer(), defaultProjectionVerifier{},
	)
	result, err := c.Compile(context.Background(), repo.Dir, integrationBuildRequest("spec/story-multi-parent"))
	if err != nil {
		t.Fatalf("Compile: unexpected error: %v", err)
	}
	m := result.Manifest
	if m.Actors.Posture != ResolutionProven {
		t.Errorf("Actors.Posture = %q, want %q", m.Actors.Posture, ResolutionProven)
	}
	if len(m.Actors.Resolutions) != 1 || m.Actors.Resolutions[0].Claim.Subject != "user-123" {
		t.Fatalf("Actors.Resolutions = %+v", m.Actors.Resolutions)
	}
	if len(m.Actors.Disclosures) != 0 {
		t.Errorf("Actors.Disclosures = %v, want empty (posture is proven)", m.Actors.Disclosures)
	}
}

func TestProjectActorsDoesNotAliasPortMemory(t *testing.T) {
	res := mintResolution(t, authenticatedFact("user-123"), "user-123")
	original := res

	got, err := projectActors(context.Background(), fakeActorResolver{resolutions: []governanceprincipal.PrincipalResolution{res}})
	if err != nil {
		t.Fatalf("projectActors: unexpected error: %v", err)
	}
	if len(got.Resolutions) != 1 || len(got.Resolutions[0].Witnesses) == 0 {
		t.Fatalf("Resolutions = %#v, want 1 resolution with witnesses", got.Resolutions)
	}

	got.Resolutions[0].Witnesses[0].Detail = "mutated by caller"

	if !reflect.DeepEqual(res.Witnesses, original.Witnesses) {
		t.Errorf("mutating the returned section aliased the port's resolution memory: %#v", res.Witnesses)
	}
}
