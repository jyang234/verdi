package policyauthority

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_IncompleteAdoption(t *testing.T) {
	files := minimalStoreFiles()
	delete(files, ".verdi/policy/constitution.md")
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want ErrIncompleteAdoption")
	}
	if !strings.Contains(err.Error(), "incomplete adoption") {
		t.Fatalf("error = %v, want incomplete-adoption text", err)
	}
}

func TestLoad_UnclassifiableEntryFailsClosed(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/policies/readme.txt"] = "not a policy artifact"
	root := t.TempDir()
	writeTree(t, root, files)
	if _, err := Load(root); err == nil {
		t.Fatal("Load() succeeded, want an unrecognized-entry error")
	}
}

func TestLoad_UnexpectedTopLevelDirectory(t *testing.T) {
	files := minimalStoreFiles()
	root := t.TempDir()
	writeTree(t, root, files)
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "policy", "junk"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want an unexpected-directory error")
	}
	if !strings.Contains(err.Error(), "unexpected directory") {
		t.Fatalf("error = %v, want unexpected-directory text", err)
	}
}

func TestLoad_FilenameStemMismatch(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/policies/go-toolchain.md"] = strings.Replace(
		files[".verdi/policy/policies/go-toolchain.md"], "id: policy/go-toolchain", "id: policy/renamed", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want a filename-stem-mismatch error")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %v, want stem-mismatch text", err)
	}
}

func TestLoad_UnregisteredClaimSubject(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/policies/go-toolchain.md"] = strings.Replace(
		files[".verdi/policy/policies/go-toolchain.md"], "subject: go-version", "subject: unregistered-subject", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want an unregistered-subject error")
	}
	if !strings.Contains(err.Error(), "is not registered") {
		t.Fatalf("error = %v, want subject-registration text", err)
	}
}

func TestLoad_UnregisteredScopeEnvironment(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/policies/go-toolchain.md"] = strings.Replace(
		files[".verdi/policy/policies/go-toolchain.md"],
		`scope: {phases: [], environments: [], paths: [], refs: []}
claims:`,
		`scope: {phases: [], environments: [nonexistent-env], paths: [], refs: []}
claims:`, 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want an unregistered-environment error")
	}
	if !strings.Contains(err.Error(), "registered environments") {
		t.Fatalf("error = %v, want environment-registration text", err)
	}
}

func TestLoad_OverlayRefinesMissingPolicy(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/overlays/frontend-go-version.md"] = strings.Replace(
		files[".verdi/policy/overlays/frontend-go-version.md"], "refines: policy/go-toolchain", "refines: policy/does-not-exist", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want a missing-refines-target error")
	}
	if !strings.Contains(err.Error(), "not a loaded policy") {
		t.Fatalf("error = %v, want missing-policy text", err)
	}
}

func TestLoad_OverlayRefinementMissingClaim(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/overlays/frontend-go-version.md"] = strings.Replace(
		files[".verdi/policy/overlays/frontend-go-version.md"], "claim: go-version", "claim: does-not-exist", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want a missing-claim error")
	}
	if !strings.Contains(err.Error(), "does not exist on policy") {
		t.Fatalf("error = %v, want missing-claim text", err)
	}
}

func TestLoad_OverlayOperandKindMismatch(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/overlays/frontend-go-version.md"] = strings.Replace(
		files[".verdi/policy/overlays/frontend-go-version.md"], `values: ["1.25"]`, `bound: 5`, 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want an operand-kind-mismatch error")
	}
	if !strings.Contains(err.Error(), "must carry a values operand") {
		t.Fatalf("error = %v, want operand-kind text", err)
	}
}

func TestLoad_StaleExemptionWitness(t *testing.T) {
	files := minimalStoreFiles()
	staleDigest := "sha256:" + strings.Repeat("0", 64)
	files[".verdi/policy/exemptions/legacy-service-go.md"] = strings.Replace(
		files[".verdi/policy/exemptions/legacy-service-go.md"], goVersionClaimDigest, staleDigest, 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want a stale-witness error")
	}
	if !strings.Contains(err.Error(), "stale witness") {
		t.Fatalf("error = %v, want stale-witness text", err)
	}
}

func TestLoad_ExemptionWitnessUnloadedPolicy(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/exemptions/legacy-service-go.md"] = strings.Replace(
		files[".verdi/policy/exemptions/legacy-service-go.md"], "policy: policy/go-toolchain", "policy: policy/does-not-exist", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want an unloaded-policy witness error")
	}
	if !strings.Contains(err.Error(), "is not loaded") {
		t.Fatalf("error = %v, want unloaded-policy text", err)
	}
}

func TestLoad_ExemptionWitnessMissingClaim(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/exemptions/legacy-service-go.md"] = strings.Replace(
		files[".verdi/policy/exemptions/legacy-service-go.md"], "claim: go-version", "claim: does-not-exist", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want a missing-witness-claim error")
	}
	if !strings.Contains(err.Error(), "does not exist on policy") {
		t.Fatalf("error = %v, want missing-claim text", err)
	}
}

func TestLoad_UnregisteredApprovalRole(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/exemptions/legacy-service-go.md"] = strings.Replace(
		files[".verdi/policy/exemptions/legacy-service-go.md"], "role: policy-owner", "role: not-a-role", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want an unregistered-role error")
	}
	if !strings.Contains(err.Error(), "is not a member of the constitution catalog's roles") {
		t.Fatalf("error = %v, want role-registration text", err)
	}
}

// dispositionScopeLine is dispositionFile's OWN top-level scope line: the
// nested witness-claim scope is indented, so replacing this exact
// column-zero line rewrites only the disposition's own Scope.
const dispositionScopeLine = "\nscope: {phases: [], environments: [], paths: [], refs: []}\n"

// TestLoad_DispositionUnregisteredScopeEnvironment is
// TestLoad_UnregisteredScopeEnvironment's case for a disposition: §8 gives
// policyauthority cross-reference validation over dispositions exactly as
// over exemptions, so a disposition scoped to an environment the
// constitution never registered must be refused rather than sealed into the
// effective-authority digest.
func TestLoad_DispositionUnregisteredScopeEnvironment(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/dispositions/review-no-conflict.md"] = strings.Replace(
		dispositionFile(t, "review-no-conflict"),
		dispositionScopeLine,
		"\nscope: {phases: [], environments: [nonexistent-env], paths: [], refs: []}\n", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want an unregistered-environment error")
	}
	if !strings.Contains(err.Error(), "registered environments") {
		t.Fatalf("error = %v, want environment-registration text", err)
	}
	if !strings.Contains(err.Error(), "disposition policy-disposition/review-no-conflict scope") {
		t.Fatalf("error = %v, want the offending disposition and field named", err)
	}
}

// TestLoad_DispositionUnregisteredApprovalRole is
// TestLoad_UnregisteredApprovalRole's case for a disposition: §8 requires at
// least one approval on every disposition, and an approval naming a role
// outside the constitution catalog is the same unauthorized-authority defect
// the exemption loop already refuses.
func TestLoad_DispositionUnregisteredApprovalRole(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/dispositions/review-no-conflict.md"] = strings.Replace(
		dispositionFile(t, "review-no-conflict"), "role: policy-owner", "role: not-a-role", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want an unregistered-role error")
	}
	if !strings.Contains(err.Error(), "is not a member of the constitution catalog's roles") {
		t.Fatalf("error = %v, want role-registration text", err)
	}
	if !strings.Contains(err.Error(), "disposition policy-disposition/review-no-conflict approval") {
		t.Fatalf("error = %v, want the offending disposition and field named", err)
	}
}

// TestLoad_DispositionRegisteredScopeAndRoleLoads is the positive control
// for the two refusals above: a disposition naming a REGISTERED environment
// and a catalog role still loads, so the new cross-validation refuses only
// unregistered references and never any nonempty scope environment.
func TestLoad_DispositionRegisteredScopeAndRoleLoads(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/dispositions/review-no-conflict.md"] = strings.Replace(
		dispositionFile(t, "review-no-conflict"),
		dispositionScopeLine,
		"\nscope: {phases: [], environments: [production], paths: [], refs: []}\n", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	s, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error on a disposition naming a registered environment: %v", err)
	}
	d, ok := s.Dispositions["policy-disposition/review-no-conflict"]
	if !ok {
		t.Fatalf("Dispositions = %v, want policy-disposition/review-no-conflict loaded", s.Dispositions)
	}
	if len(d.Scope.Environments) != 1 || d.Scope.Environments[0] != "production" {
		t.Fatalf("loaded disposition scope environments = %v, want [production]", d.Scope.Environments)
	}
}

func TestLoad_SelectedProfileMissing(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/constitution.md"] = strings.Replace(
		files[".verdi/policy/constitution.md"], "selected_profile: solo-default", "selected_profile: nonexistent-profile", 1)
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want a missing-selected-profile error")
	}
	if !strings.Contains(err.Error(), "does not resolve to a loaded stored profile") {
		t.Fatalf("error = %v, want selected-profile text", err)
	}
}

func TestLoad_DuplicatePayloadKindAcrossPolicies(t *testing.T) {
	files := minimalStoreFiles()
	files[".verdi/policy/policies/go-toolchain.md"] = strings.Replace(
		files[".verdi/policy/policies/go-toolchain.md"], "payloads: {}", "payloads: {design_assistance: {mode: off, layout: false}}", 1)
	files[".verdi/policy/policies/second.md"] = `---
schema: verdi.policy/v1
id: policy/second
kind: policy
title: "Second policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads: {design_assistance: {mode: proposal-only, layout: false}}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
A second policy that collides on payload kind.
`
	root := t.TempDir()
	writeTree(t, root, files)
	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() succeeded, want a duplicate-payload-kind error")
	}
	if !strings.Contains(err.Error(), "policy/go-toolchain") || !strings.Contains(err.Error(), "policy/second") {
		t.Fatalf("error = %v, want both policy ids named", err)
	}
}

// TestLoad_SymlinkedArtifactRejected proves a symlink inside the
// constitution store fails closed rather than being followed: the store
// is the Git-governed authority, and a link lets content that is not
// committed under .verdi/policy/ — or that resolves differently on
// another checkout — enter the loaded authority. The walker already fails
// closed on unexpected directories; a linked FILE gets the same treatment,
// named by its path.
func TestLoad_SymlinkedArtifactRejected(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, minimalStoreFiles())

	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	target := filepath.Join(outside, "extra.md")
	if err := os.WriteFile(target, []byte(`---
schema: verdi.policy/v1
id: policy/extra
kind: policy
title: "Extra policy"
owners: [platform-team]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads: {}
template: {identity: "embedded:policy.md", digest: "sha256:0e1b83a8e41d5ecfe9f14cb4973b7a584bfcb471247fa064b5fe273e4d322561"}
---
A perfectly valid policy that simply does not live in the store.
`), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(root, ".verdi", "policy", "policies", "extra.md")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable on this platform: %v", err)
	}

	_, err := Load(root)
	if err == nil {
		t.Fatal("Load() followed a symlinked artifact, want an error")
	}
	if !strings.Contains(err.Error(), "symlink") || !strings.Contains(err.Error(), "policies/extra.md") {
		t.Fatalf("error = %v, want a symlink error naming the path", err)
	}
}

// TestLoad_RefinementOnNonRefinableOperatorRejected proves Load and
// Resolve agree on WHICH operators admit a refinement at all. Load's
// structural operand-kind check used to accept refinements of path-read
// and path-write (they do take a values operand) that Resolve then
// rejected as non-refinable, so a store could Load clean and never
// resolve. A store that Loads must Resolve, absent a genuine narrowing
// violation, so the same named error class now fires at Load.
func TestLoad_RefinementOnNonRefinableOperatorRejected(t *testing.T) {
	cases := []struct {
		name    string
		claim   string
		operand string
	}{
		{"equals", "exact-version", `values: ["1.24"]`},
		{"not-equals", "not-legacy", `values: ["ancient"]`},
		{"path-read", "readable-paths", `values: ["docs/sub/"]`},
		{"path-write", "writable-paths", `values: ["scripts/sub/"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := rulesStoreFiles()
			files[".verdi/policy/overlays/o.md"] = overlayFile("o", `"web/"`, tc.claim, tc.operand)
			root := t.TempDir()
			writeTree(t, root, files)
			_, err := Load(root)
			if err == nil {
				t.Fatal("Load() succeeded on a refinement of a non-refinable operator, want error")
			}
			if !strings.Contains(err.Error(), "is not refinable") {
				t.Fatalf("error = %v, want the not-refinable error class", err)
			}
		})
	}
}
