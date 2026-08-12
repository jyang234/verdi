package contextcompile

import (
	"bytes"
	"maps"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
)

func TestSelectAuthorityOperands_RetainsApplicableBasePolicy(t *testing.T) {
	scope := policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
	candidate := authorityOperandCandidate{Kind: PolicyEntryPolicy, ID: "policy/base", Path: ".verdi/policy/policies/base.md", Digest: "sha256:" + strings.Repeat("a", 64), Scope: scope}
	request := Request{Schema: RequestSchema, Adapter: AdapterRef{ID: "codex", Version: "1"}, Phase: PhaseBuild, Scope: scope, Spec: "spec/story"}
	target := ResolvedSpec{Ref: "spec/story", Path: ".verdi/specs/active/story/spec.md"}

	got, err := selectAuthorityOperands([]authorityOperandCandidate{candidate}, request, target)
	if err != nil {
		t.Fatalf("selectAuthorityOperands: %v", err)
	}
	want := authoritySelection{
		Operands:    []PolicyOperand{{Kind: PolicyEntryPolicy, ID: "policy/base", Path: ".verdi/policy/policies/base.md", Digest: "sha256:" + strings.Repeat("a", 64), Scope: scope}},
		Selection:   instructionprojection.Selection{PolicyIDs: []string{"policy/base"}},
		Disclosures: []DisclosureCode{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selection = %#v, want %#v", got, want)
	}
}

func TestSelectAuthorityOperands_RetainsUnknownWithDisclosure(t *testing.T) {
	scope := policyartifact.Scope{
		Phases:       []string{},
		Environments: []string{},
		Paths:        []string{"cmd/"},
		Refs:         []string{},
	}
	candidate := authorityOperandCandidate{
		Kind: PolicyEntryPolicy, ID: "policy/scoped",
		Path:   ".verdi/policy/policies/scoped.md",
		Digest: "sha256:" + strings.Repeat("b", 64), Scope: scope,
	}
	request := Request{
		Schema:  RequestSchema,
		Adapter: AdapterRef{ID: "codex", Version: "1"},
		Phase:   PhaseBuild,
		Scope: policyartifact.Scope{
			Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{},
		},
		Spec: "spec/story",
	}
	target := ResolvedSpec{Ref: "spec/story"}

	got, err := selectAuthorityOperands([]authorityOperandCandidate{candidate}, request, target)
	if err != nil {
		t.Fatalf("selectAuthorityOperands() error = %v", err)
	}
	want := authoritySelection{
		Operands: []PolicyOperand{{
			Kind: PolicyEntryPolicy, ID: "policy/scoped",
			Path:   ".verdi/policy/policies/scoped.md",
			Digest: "sha256:" + strings.Repeat("b", 64), Scope: scope,
		}},
		Selection:   instructionprojection.Selection{PolicyIDs: []string{"policy/scoped"}},
		Disclosures: []DisclosureCode{DisclosureApplicabilityUnknown},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectAuthorityOperands() = %#v, want %#v", got, want)
	}
}

func TestSelectAuthorityOperands_SortsCanonicalPolicyOrder(t *testing.T) {
	scope := policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
	candidates := []authorityOperandCandidate{
		{Kind: PolicyEntryPolicy, ID: "policy/zulu", Path: ".verdi/policy/policies/zulu.md", Digest: "sha256:" + strings.Repeat("c", 64), Scope: scope},
		{Kind: PolicyEntryExemption, ID: "policy-exemption/echo", Path: ".verdi/policy/exemptions/echo.md", Digest: "sha256:" + strings.Repeat("d", 64), Scope: scope},
		{Kind: PolicyEntryPolicy, ID: "policy/alpha", Path: ".verdi/policy/policies/alpha.md", Digest: "sha256:" + strings.Repeat("e", 64), Scope: scope},
		{Kind: PolicyEntryOverlay, ID: "policy-overlay/bravo", Path: ".verdi/policy/overlays/bravo.md", Digest: "sha256:" + strings.Repeat("f", 64), Scope: scope},
	}
	original := append([]authorityOperandCandidate(nil), candidates...)
	request := Request{Schema: RequestSchema, Adapter: AdapterRef{ID: "codex", Version: "1"}, Phase: PhaseBuild, Scope: scope, Spec: "spec/story"}
	target := ResolvedSpec{Ref: "spec/story", Path: ".verdi/specs/active/story/spec.md"}

	got, err := selectAuthorityOperands(candidates, request, target)
	if err != nil {
		t.Fatalf("selectAuthorityOperands() error = %v", err)
	}
	operand := func(c authorityOperandCandidate) PolicyOperand {
		return PolicyOperand{Kind: c.Kind, ID: c.ID, Path: c.Path, Digest: c.Digest, Scope: scope}
	}
	want := authoritySelection{
		Operands:    []PolicyOperand{operand(candidates[1]), operand(candidates[3]), operand(candidates[2]), operand(candidates[0])},
		Selection:   instructionprojection.Selection{PolicyIDs: []string{"policy/alpha", "policy/zulu"}},
		Disclosures: []DisclosureCode{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("selectAuthorityOperands() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(candidates, original) {
		t.Fatalf("selectAuthorityOperands() mutated candidates: got %#v, want %#v", candidates, original)
	}
}

// TestSelectAuthorityOperands_AcceptsRealDerivedCandidates pipes the real
// derived candidates of a loaded authority into the selector: the two halves
// must agree on the digest grammar policyartifact actually emits
// ("sha256:"+64 hex), not a synthetic bare-hex stand-in.
func TestSelectAuthorityOperands_AcceptsRealDerivedCandidates(t *testing.T) {
	root := installPolicyFixture(t)
	authority, err := ResolvePolicyAuthority(defaultAuthorityLoader{}, root, AdapterRef{ID: "codex", Version: "1"})
	if err != nil {
		t.Fatalf("ResolvePolicyAuthority() error = %v", err)
	}
	candidates, err := authorityOperandCandidates(authority)
	if err != nil {
		t.Fatalf("authorityOperandCandidates() error = %v", err)
	}
	if len(candidates) == 0 {
		t.Fatalf("authorityOperandCandidates() = empty, want the fixture's derived candidates")
	}
	request := Request{
		Schema:  RequestSchema,
		Adapter: AdapterRef{ID: "codex", Version: "1"},
		Phase:   PhaseBuild,
		Scope:   policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Spec:    "spec/story",
	}
	target := ResolvedSpec{Ref: "spec/story", Path: ".verdi/specs/active/story/spec.md"}

	got, err := selectAuthorityOperands(candidates, request, target)
	if err != nil {
		t.Fatalf("selectAuthorityOperands() error = %v", err)
	}
	if len(got.Operands) == 0 {
		t.Fatalf("selectAuthorityOperands() operands = empty, want the fixture's universally scoped policy retained")
	}
	selected := false
	for _, id := range got.Selection.PolicyIDs {
		if id == "policy/go-toolchain" {
			selected = true
		}
	}
	if !selected {
		t.Fatalf("selection.PolicyIDs = %#v, want it to contain %q", got.Selection.PolicyIDs, "policy/go-toolchain")
	}
	for _, operand := range got.Operands {
		if err := validateDigest("operand.digest", operand.Digest); err != nil {
			t.Fatalf("retained operand %s carries a digest the shared grammar rejects: %v", operand.ID, err)
		}
	}
}

func TestAuthorityOperandCandidates_DerivesLoadedAuthority(t *testing.T) {
	root := installPolicyFixture(t)
	authority, err := ResolvePolicyAuthority(defaultAuthorityLoader{}, root, AdapterRef{ID: "codex", Version: "1"})
	if err != nil {
		t.Fatalf("ResolvePolicyAuthority() error = %v", err)
	}
	policy := authority.Store.Policies["policy/go-toolchain"]
	overlay := authority.Store.Overlays["policy-overlay/frontend-go-version"]
	exemption := authority.Store.Exemptions["policy-exemption/legacy-service-go"]
	policyDigest, err := policy.Digest()
	if err != nil {
		t.Fatal(err)
	}
	overlayDigest, err := overlay.Digest()
	if err != nil {
		t.Fatal(err)
	}
	exemptionDigest, err := exemption.Digest()
	if err != nil {
		t.Fatal(err)
	}

	got, err := authorityOperandCandidates(authority)
	if err != nil {
		t.Fatalf("authorityOperandCandidates() error = %v", err)
	}
	want := []authorityOperandCandidate{
		{Kind: PolicyEntryExemption, ID: exemption.ID, Path: ".verdi/policy/exemptions/legacy-service-go.md", Digest: exemptionDigest, Scope: exemption.Scope},
		{Kind: PolicyEntryOverlay, ID: overlay.ID, Path: ".verdi/policy/overlays/frontend-go-version.md", Digest: overlayDigest, Scope: overlay.Scope},
		{Kind: PolicyEntryPolicy, ID: policy.ID, Path: ".verdi/policy/policies/go-toolchain.md", Digest: policyDigest, Scope: policy.Scope},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("authorityOperandCandidates() = %#v, want %#v", got, want)
	}
	got[0].Scope.Paths[0] = "mutated/"
	if exemption.Scope.Paths[0] != "services/legacy/" {
		t.Fatalf("authorityOperandCandidates() aliased store scope: %#v", exemption.Scope.Paths)
	}
}

func TestAuthorityOperandCandidates_RejectsMalformedAuthority(t *testing.T) {
	root := installPolicyFixture(t)
	authority, err := ResolvePolicyAuthority(defaultAuthorityLoader{}, root, AdapterRef{ID: "codex", Version: "1"})
	if err != nil {
		t.Fatalf("ResolvePolicyAuthority() error = %v", err)
	}

	cases := []struct {
		name string
		run  func() ([]authorityOperandCandidate, error)
		want string
	}{
		{
			name: "nil store",
			run: func() ([]authorityOperandCandidate, error) {
				mutated := authority
				mutated.Store = nil
				return authorityOperandCandidates(mutated)
			},
			want: "store",
		},
		{
			name: "nil policy entry",
			run: func() ([]authorityOperandCandidate, error) {
				mutated := authority
				newStore := *authority.Store
				clonedPolicies := maps.Clone(authority.Store.Policies)
				for k := range clonedPolicies {
					clonedPolicies[k] = nil
					break
				}
				newStore.Policies = clonedPolicies
				mutated.Store = &newStore
				return authorityOperandCandidates(mutated)
			},
			want: "policy",
		},
		{
			name: "nil overlay entry",
			run: func() ([]authorityOperandCandidate, error) {
				mutated := authority
				newStore := *authority.Store
				clonedOverlays := maps.Clone(authority.Store.Overlays)
				for k := range clonedOverlays {
					clonedOverlays[k] = nil
					break
				}
				newStore.Overlays = clonedOverlays
				mutated.Store = &newStore
				return authorityOperandCandidates(mutated)
			},
			want: "overlay",
		},
		{
			name: "nil exemption entry",
			run: func() ([]authorityOperandCandidate, error) {
				mutated := authority
				newStore := *authority.Store
				clonedExemptions := maps.Clone(authority.Store.Exemptions)
				for k := range clonedExemptions {
					clonedExemptions[k] = nil
					break
				}
				newStore.Exemptions = clonedExemptions
				mutated.Store = &newStore
				return authorityOperandCandidates(mutated)
			},
			want: "exemption",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.run()
			if err == nil {
				t.Fatalf("authorityOperandCandidates() error = nil, want error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("authorityOperandCandidates() error = %q, want substring containing %q", err.Error(), tt.want)
			}
			if len(got) != 0 {
				t.Fatalf("authorityOperandCandidates() candidates = %#v, want empty", got)
			}
		})
	}
}

func TestRenderSelectedProjection(t *testing.T) {
	root := installPolicyFixture(t)
	authority, err := ResolvePolicyAuthority(defaultAuthorityLoader{}, root, AdapterRef{ID: "codex", Version: "1"})
	if err != nil {
		t.Fatalf("ResolvePolicyAuthority() error = %v", err)
	}
	selection := instructionprojection.Selection{PolicyIDs: []string{"policy/go-toolchain"}}

	want, err := instructionprojection.Render(authority.Store, authority.Effective, authority.Adapter, selection)
	if err != nil {
		t.Fatalf("instructionprojection.Render() error = %v", err)
	}

	got, err := renderSelectedProjection(authority, selection)
	if err != nil {
		t.Fatalf("renderSelectedProjection() error = %v", err)
	}

	if len(got) != len(want.Files) {
		t.Fatalf("renderSelectedProjection() file count = %d, want %d", len(got), len(want.Files))
	}
	for i := range want.Files {
		if got[i].Path != want.Files[i].Path {
			t.Fatalf("file[%d].Path = %q, want %q", i, got[i].Path, want.Files[i].Path)
		}
		if !bytes.Equal(got[i].Content, want.Files[i].Content) {
			t.Fatalf("file[%d].Content = %q, want %q", i, got[i].Content, want.Files[i].Content)
		}
		if got[i].Digest != want.Files[i].Digest {
			t.Fatalf("file[%d].Digest = %q, want %q", i, got[i].Digest, want.Files[i].Digest)
		}
	}

	if len(got) == 0 || len(got[0].Content) == 0 {
		t.Fatalf("got[0].Content is empty; cannot exercise freshness assertion")
	}
	originalByte := got[0].Content[0]
	if want.Files[0].Content[0] != originalByte {
		t.Fatalf("want.Files[0].Content[0] = %v, want %v", want.Files[0].Content[0], originalByte)
	}

	got[0].Content[0] = originalByte + 1

	got2, err := renderSelectedProjection(authority, selection)
	if err != nil {
		t.Fatalf("renderSelectedProjection() second call error = %v", err)
	}
	if len(got2) != len(want.Files) {
		t.Fatalf("renderSelectedProjection() second call file count = %d, want %d", len(got2), len(want.Files))
	}
	for i := range want.Files {
		if !bytes.Equal(got2[i].Content, want.Files[i].Content) {
			t.Fatalf("second call file[%d].Content = %q, want %q (direct Render truth)", i, got2[i].Content, want.Files[i].Content)
		}
	}
	if want.Files[0].Content[0] != originalByte {
		t.Fatalf("mutating got[0].Content leaked into direct Render truth: want.Files[0].Content[0] = %v, want %v", want.Files[0].Content[0], originalByte)
	}

	if !reflect.DeepEqual(selection.PolicyIDs, []string{"policy/go-toolchain"}) {
		t.Fatalf("selection.PolicyIDs mutated: got %#v", selection.PolicyIDs)
	}

	t.Run("nil store", func(t *testing.T) {
		mutated := authority
		mutated.Store = nil
		if _, err := renderSelectedProjection(mutated, selection); err == nil {
			t.Fatalf("renderSelectedProjection() error = nil, want non-nil")
		}
	})

	t.Run("nil effective", func(t *testing.T) {
		mutated := authority
		mutated.Effective = nil
		if _, err := renderSelectedProjection(mutated, selection); err == nil {
			t.Fatalf("renderSelectedProjection() error = nil, want non-nil")
		}
	})
}

func TestSelectAuthorityOperands_RejectsMalformedInputs(t *testing.T) {
	baseScope := func() policyartifact.Scope {
		return policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
	}
	baseCandidate := func() authorityOperandCandidate {
		return authorityOperandCandidate{
			Kind: PolicyEntryPolicy, ID: "policy/base",
			Path: ".verdi/policy/policies/base.md", Digest: "sha256:" + strings.Repeat("a", 64), Scope: baseScope(),
		}
	}
	baseRequest := func() Request {
		return Request{Schema: RequestSchema, Adapter: AdapterRef{ID: "codex", Version: "1"}, Phase: PhaseBuild, Scope: baseScope(), Spec: "spec/story"}
	}
	baseTarget := func() ResolvedSpec {
		return ResolvedSpec{Ref: "spec/story", Path: ".verdi/specs/active/story/spec.md"}
	}

	cases := []struct {
		name  string
		build func() ([]authorityOperandCandidate, Request, ResolvedSpec)
		want  string
	}{
		{
			name: "unknown candidate kind",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				c := baseCandidate()
				c.Kind = "sidecar"
				return []authorityOperandCandidate{c}, baseRequest(), baseTarget()
			},
			want: "kind",
		},
		{
			name: "id prefix mismatched with kind",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				c := baseCandidate()
				c.ID = "policy-overlay/x"
				return []authorityOperandCandidate{c}, baseRequest(), baseTarget()
			},
			want: "id",
		},
		{
			name: "path mismatched with kind/name",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				c := baseCandidate()
				c.Path = ".verdi/policy/overlays/base.md"
				return []authorityOperandCandidate{c}, baseRequest(), baseTarget()
			},
			want: "path",
		},
		{
			name: "digest not a sha256 digest at all",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				c := baseCandidate()
				c.Digest = "zz"
				return []authorityOperandCandidate{c}, baseRequest(), baseTarget()
			},
			want: "digest",
		},
		{
			name: "digest missing sha256 prefix",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				c := baseCandidate()
				c.Digest = strings.Repeat("a", 64)
				return []authorityOperandCandidate{c}, baseRequest(), baseTarget()
			},
			want: "digest",
		},
		{
			name: "digest hex too short for the prefixed grammar",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				c := baseCandidate()
				c.Digest = "sha256:" + strings.Repeat("a", 63)
				return []authorityOperandCandidate{c}, baseRequest(), baseTarget()
			},
			want: "digest",
		},
		{
			name: "digest hex uppercase",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				c := baseCandidate()
				c.Digest = "sha256:" + strings.Repeat("A", 64)
				return []authorityOperandCandidate{c}, baseRequest(), baseTarget()
			},
			want: "digest",
		},
		{
			name: "duplicate kind+id across candidates",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				c1 := baseCandidate()
				c2 := baseCandidate()
				return []authorityOperandCandidate{c1, c2}, baseRequest(), baseTarget()
			},
			want: "duplicate",
		},
		{
			name: "invalid candidate scope",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				c := baseCandidate()
				c.Scope = policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{"cmd/", "cmd/"}, Refs: []string{}}
				return []authorityOperandCandidate{c}, baseRequest(), baseTarget()
			},
			want: "scope",
		},
		{
			name: "target ref empty",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				target := baseTarget()
				target.Ref = ""
				return []authorityOperandCandidate{baseCandidate()}, baseRequest(), target
			},
			want: "ref",
		},
		{
			name: "target ref malformed grammar",
			build: func() ([]authorityOperandCandidate, Request, ResolvedSpec) {
				target := baseTarget()
				target.Ref = "story"
				return []authorityOperandCandidate{baseCandidate()}, baseRequest(), target
			},
			want: "ref",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			candidates, request, target := tt.build()
			got, err := selectAuthorityOperands(candidates, request, target)
			if err == nil {
				t.Fatalf("selectAuthorityOperands() error = nil, want error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("selectAuthorityOperands() error = %q, want substring containing %q", err.Error(), tt.want)
			}
			if !reflect.DeepEqual(got, authoritySelection{}) {
				t.Fatalf("selectAuthorityOperands() result = %#v, want zero value", got)
			}
		})
	}
}

func TestRenderSelectedProjection_RejectsMalformedSelection(t *testing.T) {
	root := installPolicyFixture(t)
	authority, err := ResolvePolicyAuthority(defaultAuthorityLoader{}, root, AdapterRef{ID: "codex", Version: "1"})
	if err != nil {
		t.Fatalf("ResolvePolicyAuthority() error = %v", err)
	}

	cases := []struct {
		name      string
		selection instructionprojection.Selection
		want      string
	}{
		{
			name:      "unknown selected policy id",
			selection: instructionprojection.Selection{PolicyIDs: []string{"policy/does-not-exist"}},
			want:      "policy",
		},
		{
			name:      "duplicate selected policy ids",
			selection: instructionprojection.Selection{PolicyIDs: []string{"policy/go-toolchain", "policy/go-toolchain"}},
			want:      "policy",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderSelectedProjection(authority, tt.selection)
			if err == nil {
				t.Fatalf("renderSelectedProjection() error = nil, want error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("renderSelectedProjection() error = %q, want substring containing %q", err.Error(), tt.want)
			}
			if got != nil {
				t.Fatalf("renderSelectedProjection() result = %#v, want nil", got)
			}
		})
	}
}
