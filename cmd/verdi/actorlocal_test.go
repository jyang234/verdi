package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// --- fixture plumbing --------------------------------------------------

// initGitRepoNoIdentity creates a fresh git repository with NO local
// user.email/user.name configured. gitx.ConfigValue reads `--local` scope
// only, which git never widens to the host's global/system configuration
// regardless of environment, so this reliably reproduces "absent identity"
// on any machine, including one with a real global git identity configured.
func initGitRepoNoIdentity(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return dir
}

func setLocalGitConfig(t *testing.T, dir, key, value string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", dir, "config", key, value).CombinedOutput(); err != nil {
		t.Fatalf("git config %s %s: %v\n%s", key, value, err, out)
	}
}

func addLocalGitConfig(t *testing.T, dir, key, value string) {
	t.Helper()
	if out, err := exec.Command("git", "-C", dir, "config", "--add", key, value).CombinedOutput(); err != nil {
		t.Fatalf("git config --add %s %s: %v\n%s", key, value, err, out)
	}
}

// actorLocalCatalog covers every catalog reference the two test profiles
// below need.
func actorLocalCatalog() governanceprincipal.Catalog {
	return governanceprincipal.Catalog{
		Roles:       []string{"policy-owner", "author"},
		Transitions: []string{"policy-disposition-approval", "accept"},
	}
}

// actorLocalProfileYAML declares one local-operator trust source ("local")
// bound to "bound@example.com" only for role policy-owner.
const actorLocalProfileYAML = `schema: verdi.governance-profile/v1
id: solo-local-test
class: solo
applicable_transitions: [policy-disposition-approval]
identity_trust_sources:
  - { id: local, kind: local-operator }
role_mappings:
  - role: policy-owner
    trust_source: local
    subjects: ["bound@example.com"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`

// actorLocalTwoSourceProfileYAML declares TWO local-operator trust sources
// — an ambiguous declaration the design never anticipates a single subject
// for (proof point: the ambient wiring must fail closed, never guess).
const actorLocalTwoSourceProfileYAML = `schema: verdi.governance-profile/v1
id: solo-local-ambiguous
class: solo
applicable_transitions: [policy-disposition-approval]
identity_trust_sources:
  - { id: local, kind: local-operator }
  - { id: local2, kind: local-operator }
role_mappings:
  - role: policy-owner
    trust_source: local
    subjects: ["bound@example.com"]
  - role: policy-owner
    trust_source: local2
    subjects: ["bound@example.com"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`

// actorLocalNoSourceProfileYAML declares no local-operator trust source at
// all (a forge source only) — the profile shape every non-opted-in store
// carries today.
const actorLocalNoSourceProfileYAML = `schema: verdi.governance-profile/v1
id: solo-forge-test
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["alice"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`

func mustDecodeActorLocalProfile(t *testing.T, raw string) governanceprincipal.Profile {
	t.Helper()
	p, err := governanceprincipal.DecodeProfile([]byte(raw), actorLocalCatalog())
	if err != nil {
		t.Fatalf("DecodeProfile: %v", err)
	}
	return p
}

// --- resolveLocalActors --------------------------------------------------

func TestResolveLocalActors_NoLocalOperatorSource(t *testing.T) {
	dir := initGitRepoNoIdentity(t)
	setLocalGitConfig(t, dir, "user.email", "bound@example.com")
	profile := mustDecodeActorLocalProfile(t, actorLocalNoSourceProfileYAML)

	actors, err := resolveLocalActors(context.Background(), dir, profile)
	if err != nil {
		t.Fatalf("resolveLocalActors: %v", err)
	}
	if actors != nil {
		t.Fatalf("actors = %#v, want nil for a profile with no local-operator source", actors)
	}
}

func TestResolveLocalActors_BoundIdentity_Authenticated(t *testing.T) {
	dir := initGitRepoNoIdentity(t)
	setLocalGitConfig(t, dir, "user.email", "bound@example.com")
	profile := mustDecodeActorLocalProfile(t, actorLocalProfileYAML)

	actors, err := resolveLocalActors(context.Background(), dir, profile)
	if err != nil {
		t.Fatalf("resolveLocalActors: %v", err)
	}
	if len(actors) != 1 {
		t.Fatalf("actors = %#v, want exactly one", actors)
	}
	got := actors[0]
	if got.State != governanceprincipal.ResolutionAuthenticated {
		t.Fatalf("state = %q, want authenticated", got.State)
	}
	wantID, err := governanceprincipal.CanonicalPrincipalID("local", "bound@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.PrincipalID != wantID {
		t.Fatalf("principal id = %q, want %q", got.PrincipalID, wantID)
	}
	foundWitness := false
	for _, w := range got.Witnesses {
		if w.Code == governanceprincipal.ReasonLocalOperatorAsserted {
			foundWitness = true
		}
		if w.Code == governanceprincipal.ReasonTrustSubjectVerified {
			t.Fatalf("witness carries trust-subject-verified; local-operator resolutions must never overclaim that code")
		}
	}
	if !foundWitness {
		t.Fatalf("witnesses = %#v, want local-operator-asserted", got.Witnesses)
	}
}

func TestResolveLocalActors_FallsBackToUserName(t *testing.T) {
	dir := initGitRepoNoIdentity(t)
	setLocalGitConfig(t, dir, "user.name", "bound@example.com")
	profile := mustDecodeActorLocalProfile(t, actorLocalProfileYAML)

	actors, err := resolveLocalActors(context.Background(), dir, profile)
	if err != nil {
		t.Fatalf("resolveLocalActors: %v", err)
	}
	if len(actors) != 1 || actors[0].State != governanceprincipal.ResolutionAuthenticated {
		t.Fatalf("actors = %#v, want one authenticated resolution via user.name fallback", actors)
	}
}

func TestResolveLocalActors_EmailPreferredOverName(t *testing.T) {
	dir := initGitRepoNoIdentity(t)
	setLocalGitConfig(t, dir, "user.email", "bound@example.com")
	setLocalGitConfig(t, dir, "user.name", "someone-else@example.com")
	profile := mustDecodeActorLocalProfile(t, actorLocalProfileYAML)

	actors, err := resolveLocalActors(context.Background(), dir, profile)
	if err != nil {
		t.Fatalf("resolveLocalActors: %v", err)
	}
	wantID, err := governanceprincipal.CanonicalPrincipalID("local", "bound@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(actors) != 1 || actors[0].PrincipalID != wantID {
		t.Fatalf("actors = %#v, want the email subject, not user.name", actors)
	}
}

func TestResolveLocalActors_UnboundIdentity_Violated(t *testing.T) {
	dir := initGitRepoNoIdentity(t)
	setLocalGitConfig(t, dir, "user.email", "unbound@example.com")
	profile := mustDecodeActorLocalProfile(t, actorLocalProfileYAML)

	actors, err := resolveLocalActors(context.Background(), dir, profile)
	if err != nil {
		t.Fatalf("resolveLocalActors: %v", err)
	}
	if len(actors) != 1 {
		t.Fatalf("actors = %#v, want exactly one", actors)
	}
	got := actors[0]
	if got.State != governanceprincipal.ResolutionViolated {
		t.Fatalf("state = %q, want violated-with-witness (never a favorable default)", got.State)
	}
	if got.PrincipalID != "" {
		t.Fatalf("principal id = %q, want empty for a non-authenticated resolution", got.PrincipalID)
	}
}

func TestResolveLocalActors_AbsentIdentity_Unproven(t *testing.T) {
	dir := initGitRepoNoIdentity(t)
	profile := mustDecodeActorLocalProfile(t, actorLocalProfileYAML)

	actors, err := resolveLocalActors(context.Background(), dir, profile)
	if err != nil {
		t.Fatalf("resolveLocalActors: %v", err)
	}
	if len(actors) != 1 {
		t.Fatalf("actors = %#v, want exactly one (an attempted, unproven resolution, not silently omitted)", actors)
	}
	if actors[0].State != governanceprincipal.ResolutionUnproven {
		t.Fatalf("state = %q, want unproven", actors[0].State)
	}
}

func TestResolveLocalActors_AmbiguousConfig_Operational(t *testing.T) {
	dir := initGitRepoNoIdentity(t)
	addLocalGitConfig(t, dir, "user.email", "a@example.com")
	addLocalGitConfig(t, dir, "user.email", "b@example.com")
	profile := mustDecodeActorLocalProfile(t, actorLocalProfileYAML)

	_, err := resolveLocalActors(context.Background(), dir, profile)
	if err == nil {
		t.Fatal("resolveLocalActors: got nil error, want an operational failure for ambiguous git config")
	}
}

func TestResolveLocalActors_MultipleLocalOperatorSources_Operational(t *testing.T) {
	dir := initGitRepoNoIdentity(t)
	setLocalGitConfig(t, dir, "user.email", "bound@example.com")
	profile := mustDecodeActorLocalProfile(t, actorLocalTwoSourceProfileYAML)

	_, err := resolveLocalActors(context.Background(), dir, profile)
	if err == nil {
		t.Fatal("resolveLocalActors: got nil error, want a refusal for an ambiguous multi-source declaration")
	}
}

// TestResolveLocalActors_RefusesNonTopLevelRoot proves the store root must
// be a Git repository top level in its own right: a store nested inside a
// larger enclosing repository (no .git of its own) must never have that
// ENCLOSING repository's identity read on its behalf.
func TestResolveLocalActors_RefusesNonTopLevelRoot(t *testing.T) {
	outer := initGitRepoNoIdentity(t)
	setLocalGitConfig(t, outer, "user.email", "enclosing-repo@example.com")
	nested := filepath.Join(outer, "nested-store")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	profile := mustDecodeActorLocalProfile(t, actorLocalProfileYAML)

	_, err := resolveLocalActors(context.Background(), nested, profile)
	if err == nil {
		t.Fatal("resolveLocalActors: got nil error, want a refusal for a non-top-level store root")
	}
	if !strings.Contains(err.Error(), "top level") {
		t.Fatalf("error = %q, want it to name the top-level check", err.Error())
	}
}

// TestResolveLocalActors_NotAGitRepository proves a store root that is not
// inside any Git repository at all fails operationally rather than
// silently proceeding.
func TestResolveLocalActors_NotAGitRepository(t *testing.T) {
	dir := t.TempDir()
	profile := mustDecodeActorLocalProfile(t, actorLocalProfileYAML)

	_, err := resolveLocalActors(context.Background(), dir, profile)
	if err == nil {
		t.Fatal("resolveLocalActors: got nil error, want a refusal for a non-Git store root")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want a legible git-related refusal, not a bare filesystem error", err)
	}
}
