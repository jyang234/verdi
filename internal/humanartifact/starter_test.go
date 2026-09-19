package humanartifact

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/constitutionimpact"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
)

func starterScaffold(t *testing.T, name string) Scaffold {
	t.Helper()
	s, err := ResolveScaffold(t.TempDir(), name)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// starterCatalog is the governance vocabulary the starter profiles are
// validated against. It is not an independent literal: TestRender-
// Constitution_RoundTrip asserts the constitution template declares
// exactly this catalog, so dropping policy-owner or policy-disposition-
// approval from policy-constitution.md fails there instead of leaving
// every test green while `verdi policy adopt --starter` writes a
// constitution whose vocabulary cannot validate the profile it selects.
func starterCatalog() governanceprincipal.Catalog {
	return governanceprincipal.Catalog{Roles: []string{"author", "reviewer", "policy-owner"}, Transitions: []string{"accept", "policy-disposition-approval"}}
}

func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func TestRenderConstitution_RoundTrip(t *testing.T) {
	s := starterScaffold(t, ConstitutionTemplate)
	out, err := RenderConstitution(s, ConstitutionScaffoldData{Title: "Starter constitution", Owners: []string{"local-operator"}, ProfileID: "starter-solo", TemplateIdentity: s.Identity, TemplateDigest: s.Digest})
	if err != nil {
		t.Fatal(err)
	}
	c, err := policyartifact.DecodeConstitution([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if c.SelectedProfile != "starter-solo" || c.Template == nil || c.Template.Digest != s.Digest || len(c.Adapters) != 0 || len(c.Environments) != 1 {
		t.Fatalf("decoded = %+v", c)
	}
	// The template's catalog IS the vocabulary the starter profiles are
	// validated against — they are written to select this constitution, so
	// the two cannot drift apart silently. DecodeConstitution canonicalizes
	// every catalog set by sorting it, so compare sorted copies: in a set
	// membership is the meaning, order is not.
	vocab := starterCatalog()
	wantCatalog := policyartifact.GovernanceCatalog{
		Roles:             sortedCopy(vocab.Roles),
		Transitions:       sortedCopy(vocab.Transitions),
		EvidenceSources:   []string{},
		EscalationMetrics: []string{},
	}
	if !reflect.DeepEqual(c.Catalog, wantCatalog) {
		t.Fatalf("constitution catalog = %+v, want exactly the vocabulary the starter profiles are validated against %+v", c.Catalog, wantCatalog)
	}
	// Anti-synthesis: a template that changes the selected profile fails by name.
	bad := s
	bad.Template = bytes.Replace(s.Template, []byte("selected_profile: {{safe .ProfileID}}"), []byte("selected_profile: other"), 1)
	if _, err := RenderConstitution(bad, ConstitutionScaffoldData{Title: "t", Owners: []string{"o"}, ProfileID: "starter-solo", TemplateIdentity: s.Identity, TemplateDigest: s.Digest}); err == nil || !strings.Contains(err.Error(), "selected_profile") {
		t.Fatalf("synthesized profile accepted: %v", err)
	}
	// Owners must be kebab handles (kernel rule), reported by the decoder.
	if _, err := RenderConstitution(s, ConstitutionScaffoldData{Title: "t", Owners: []string{"Not Kebab"}, ProfileID: "starter-solo", TemplateIdentity: s.Identity, TemplateDigest: s.Digest}); err == nil {
		t.Fatal("ungrammatical owner accepted")
	}
}

func TestRenderProfile_SoloBindsTheSubjectAndTeamMapsNoOne(t *testing.T) {
	solo := starterScaffold(t, ProfileSoloTemplate)
	out, err := RenderProfile(solo, ProfileScaffoldData{ProfileID: "starter-solo", Class: governanceprincipal.ClassSolo, Subject: "dev@example.invalid", TemplateIdentity: solo.Identity, TemplateDigest: solo.Digest}, starterCatalog())
	if err != nil {
		t.Fatal(err)
	}
	sp, err := policyartifact.DecodeStoredProfile([]byte(out), starterCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if sp.ID != "starter-solo" || sp.Profile.Class != governanceprincipal.ClassSolo || len(sp.Profile.RoleMappings) != 3 || sp.Profile.Template == nil || sp.Profile.Template.Identity != solo.Identity {
		t.Fatalf("solo = %+v", sp.Profile)
	}
	for _, m := range sp.Profile.RoleMappings {
		if len(m.Subjects) != 1 || m.Subjects[0] != "dev@example.invalid" {
			t.Fatalf("mapping %+v binds a subject the caller did not supply", m)
		}
	}
	// R-W4-1: the solo profile's one trust source is the local operator —
	// a bare self-assertion the resolver reports as local-operator-asserted,
	// never an independently verified identity.
	if len(sp.Profile.IdentityTrustSources) != 1 || sp.Profile.IdentityTrustSources[0].Kind != governanceprincipal.TrustSourceLocalOperator {
		t.Fatalf("solo identity_trust_sources = %+v, want exactly one %q source", sp.Profile.IdentityTrustSources, governanceprincipal.TrustSourceLocalOperator)
	}
	// A subject containing a quote or newline is rendered safely (printf %q), never a second key.
	if _, err := RenderProfile(solo, ProfileScaffoldData{ProfileID: "starter-solo", Class: governanceprincipal.ClassSolo, Subject: "a\"b\nrole_mappings: []", TemplateIdentity: solo.Identity, TemplateDigest: solo.Digest}, starterCatalog()); err != nil {
		t.Fatalf("quoted subject: %v", err)
	}
	// Anti-synthesis: a template that adds a subject the caller did not supply fails by name.
	bad := solo
	bad.Template = bytes.Replace(solo.Template, []byte(`subjects: [{{printf "%q" .Subject}}]}`), []byte(`subjects: [{{printf "%q" .Subject}}, alice]}`), 1)
	if _, err := RenderProfile(bad, ProfileScaffoldData{ProfileID: "starter-solo", Class: governanceprincipal.ClassSolo, Subject: "dev@example.invalid", TemplateIdentity: solo.Identity, TemplateDigest: solo.Digest}, starterCatalog()); err == nil || !strings.Contains(err.Error(), "subject") {
		t.Fatalf("synthesized subject accepted: %v", err)
	}

	team := starterScaffold(t, ProfileTeamTemplate)
	out, err = RenderProfile(team, ProfileScaffoldData{ProfileID: "starter-team", Class: governanceprincipal.ClassTeam, TemplateIdentity: team.Identity, TemplateDigest: team.Digest}, starterCatalog())
	if err != nil {
		t.Fatal(err)
	}
	tp, err := policyartifact.DecodeStoredProfile([]byte(out), starterCatalog())
	if err != nil {
		t.Fatal(err)
	}
	if tp.Profile.Class != governanceprincipal.ClassTeam || len(tp.Profile.RoleMappings) != 0 || len(tp.Profile.DistinctnessRules) != 2 || len(tp.Profile.RequiredApprovers) != 2 {
		t.Fatalf("team = %+v", tp.Profile)
	}
	// R-W4-1: the team profile authenticates through the forge.
	if len(tp.Profile.IdentityTrustSources) != 1 || tp.Profile.IdentityTrustSources[0].Kind != governanceprincipal.TrustSourceForge {
		t.Fatalf("team identity_trust_sources = %+v, want exactly one %q source", tp.Profile.IdentityTrustSources, governanceprincipal.TrustSourceForge)
	}
	// Class mismatch between data and template fails by name.
	if _, err := RenderProfile(team, ProfileScaffoldData{ProfileID: "starter-team", Class: governanceprincipal.ClassSolo, TemplateIdentity: team.Identity, TemplateDigest: team.Digest}, starterCatalog()); err == nil || !strings.Contains(err.Error(), "class") {
		t.Fatalf("class mismatch accepted: %v", err)
	}
	// Symmetry: a subject the rendered profile binds to no one is refused,
	// not silently discarded. Task 2 composes both renderers from one
	// ProfileScaffoldData, so passing the resolved local identity to the
	// team renderer is the natural mistake; this seam reports it by name
	// rather than returning a profile that names no one.
	if _, err := RenderProfile(team, ProfileScaffoldData{ProfileID: "starter-team", Class: governanceprincipal.ClassTeam, Subject: "dev@example.invalid", TemplateIdentity: team.Identity, TemplateDigest: team.Digest}, starterCatalog()); err == nil ||
		!strings.Contains(err.Error(), "role_mappings") || !strings.Contains(err.Error(), "dev@example.invalid") {
		t.Fatalf("supplied subject silently discarded: %v", err)
	}
}

func TestRenderStarterPolicy_ExactlyOneRealRule(t *testing.T) {
	s := starterScaffold(t, StarterPolicyTemplate)
	for _, mode := range []string{"draft-write", "proposal-only"} {
		out, err := RenderStarterPolicy(s, StarterPolicyScaffoldData{Name: "starter", Title: "Starter policy", Owners: []string{"local-operator"}, DesignAssistanceMode: mode, TemplateIdentity: s.Identity, TemplateDigest: s.Digest})
		if err != nil {
			t.Fatal(err)
		}
		p, err := policyartifact.DecodePolicy([]byte(out))
		if err != nil {
			t.Fatal(err)
		}
		da, ok := p.Payloads[policyartifact.DesignAssistancePayloadKind].(*policyartifact.DesignAssistancePayload)
		if !ok || da.Mode != mode || da.Layout || len(p.Claims) != 0 || len(p.Instructions) != 0 || len(p.Payloads) != 1 {
			t.Fatalf("mode %s: decoded = %+v", mode, p)
		}
	}
	// The existing placeholder scaffold is refused here: it renders no rule.
	placeholder := starterScaffold(t, "policy.md")
	if _, err := RenderStarterPolicy(placeholder, StarterPolicyScaffoldData{Name: "starter", Title: "t", Owners: []string{"o"}, DesignAssistanceMode: "draft-write", TemplateIdentity: placeholder.Identity, TemplateDigest: placeholder.Digest}); err == nil || !strings.Contains(err.Error(), "design_assistance") {
		t.Fatalf("placeholder accepted: %v", err)
	}
	// An unknown mode is refused by the payload's own validator, not silently written.
	if _, err := RenderStarterPolicy(s, StarterPolicyScaffoldData{Name: "starter", Title: "t", Owners: []string{"o"}, DesignAssistanceMode: "yes", TemplateIdentity: s.Identity, TemplateDigest: s.Digest}); err == nil {
		t.Fatal("bad mode accepted")
	}
	// Anti-synthesis: a template that adds a claim fails by name.
	bad := s
	bad.Template = bytes.Replace(s.Template, []byte("claims: []"), []byte("claims:\n  - {id: c, family: action, operator: required-values, subject: x, values: [y], scope: {phases: [], environments: [], paths: [], refs: []}, overridable: false}"), 1)
	if _, err := RenderStarterPolicy(bad, StarterPolicyScaffoldData{Name: "starter", Title: "t", Owners: []string{"o"}, DesignAssistanceMode: "draft-write", TemplateIdentity: s.Identity, TemplateDigest: s.Digest}); err == nil || !strings.Contains(err.Error(), "claims") {
		t.Fatalf("synthesized claim accepted: %v", err)
	}
}

func TestRenderConsumersInventory_EmptyCanonicalWithRecord(t *testing.T) {
	s := starterScaffold(t, InventoryTemplate)
	out, err := RenderConsumersInventory(s, InventoryScaffoldData{TemplateIdentity: s.Identity, TemplateDigest: s.Digest})
	if err != nil {
		t.Fatal(err)
	}
	inv, err := constitutionimpact.DecodeInventory(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Consumers) != 0 || inv.Template == nil || inv.Template.Digest != s.Digest {
		t.Fatalf("inventory = %+v", inv)
	}
	// A non-canonical override (pretty-printed) fails closed through DecodeInventory's canonical-bytes check.
	bad := s
	bad.Template = []byte("{\n  \"consumers\": [],\n  \"schema\": \"verdi.constitution-consumer-inventory/v1\"\n}\n")
	if _, err := RenderConsumersInventory(bad, InventoryScaffoldData{TemplateIdentity: s.Identity, TemplateDigest: s.Digest}); err == nil {
		t.Fatal("non-canonical override accepted")
	}
}

func TestStarterTemplates_StoreOverrideChangesIdentity(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".verdi", "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	canon, err := designscaffold.Canonical(StarterPolicyTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, StarterPolicyTemplate), bytes.Replace(canon, []byte("Starter policy: one real rule"), []byte("Our policy: one real rule"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := ResolveScaffold(root, StarterPolicyTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if s.Identity != "store:.verdi/templates/"+StarterPolicyTemplate {
		t.Fatalf("identity = %s", s.Identity)
	}
	out, err := RenderStarterPolicy(s, StarterPolicyScaffoldData{Name: "starter", Title: "t", Owners: []string{"o"}, DesignAssistanceMode: "draft-write", TemplateIdentity: s.Identity, TemplateDigest: s.Digest})
	if err != nil || !strings.Contains(out, "Our policy") || !strings.Contains(out, s.Identity) {
		t.Fatalf("override render: %v\n%s", err, out)
	}
}
