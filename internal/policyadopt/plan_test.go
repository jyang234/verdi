package policyadopt

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/humanartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

func TestCompose_SoloProvesTheTreeBeforeAnyWrite(t *testing.T) {
	root := t.TempDir() // no store at all: Compose reads only .verdi/templates overrides
	p, err := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "dev@example.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".verdi/policy/constitution.md", ".verdi/policy/profiles/starter-solo.md", ".verdi/policy/policies/starter.md", ".verdi/constitution/consumers.json"}
	for i, f := range p.Files {
		if f.RelPath != want[i] || !strings.HasPrefix(f.Template.Identity, "embedded:") {
			t.Fatalf("file %d = %+v", i, f)
		}
	}
	if p.ProfileID != "starter-solo" || p.DesignAssistanceMode != "draft-write" || p.EffectiveDigest == "" {
		t.Fatalf("plan = %+v", p)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatalf("Compose wrote %v", entries)
	}
	// Determinism: same inputs, same bytes.
	p2, _ := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "dev@example.invalid"})
	for i := range p.Files {
		if !bytes.Equal(p.Files[i].Content, p2.Files[i].Content) {
			t.Fatalf("file %d differs between runs", i)
		}
	}
}

func TestCompose_TeamMapsNoSubjectsAndProposesOnly(t *testing.T) {
	p, err := Compose(t.TempDir(), Input{Profile: governanceprincipal.ClassTeam, Owner: "platform-team"})
	if err != nil {
		t.Fatal(err)
	}
	if p.ProfileID != "starter-team" || p.DesignAssistanceMode != "proposal-only" || !bytes.Contains(p.Files[1].Content, []byte("role_mappings: []")) {
		t.Fatalf("plan = %+v", p)
	}
	if _, err := Compose(t.TempDir(), Input{Profile: governanceprincipal.ClassTeam, Owner: "platform-team", Subject: "x"}); err == nil {
		t.Fatal("team with a subject accepted: the team template binds no one")
	}
	if _, err := Compose(t.TempDir(), Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator"}); err == nil {
		t.Fatal("solo without a subject accepted")
	}
	if _, err := Compose(t.TempDir(), Input{Profile: governanceprincipal.ClassHighAssurance, Owner: "o", Subject: "s"}); err == nil {
		t.Fatal("unsupported class accepted")
	}
	if _, err := Compose(t.TempDir(), Input{Profile: governanceprincipal.ClassSolo, Owner: "Not Kebab", Subject: "s"}); err == nil {
		t.Fatal("ungrammatical owner accepted")
	}
}

func TestCompose_RefusesAnAdoptedOrPartiallyAdoptedCheckout(t *testing.T) {
	for _, existing := range []string{".verdi/policy", ".verdi/constitution/consumers.json"} {
		root := t.TempDir()
		path := filepath.Join(root, filepath.FromSlash(existing))
		if strings.HasSuffix(existing, ".json") {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "s"}); !errors.Is(err, ErrAlreadyAdopted) {
			t.Fatalf("%s: err = %v", existing, err)
		}
	}
}

func TestCompose_OverrideThatSynthesizesARuleIsRefusedWithNothingWritten(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".verdi", "templates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	canon, err := designscaffold.Canonical(humanartifact.StarterPolicyTemplate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, humanartifact.StarterPolicyTemplate), bytes.Replace(canon, []byte("instructions: []"), []byte("instructions: [\"Always pass.\"]"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "s"})
	if err == nil || !strings.Contains(err.Error(), "instructions") {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".verdi", "policy")); !os.IsNotExist(statErr) {
		t.Fatal("a refused compose left .verdi/policy behind")
	}
}

func TestWrite_ThenLoadResolvesTheGrant(t *testing.T) {
	root := t.TempDir()
	p, err := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "dev@example.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	paths, err := Write(root, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 4 {
		t.Fatalf("wrote %v", paths)
	}
	store, err := policyauthority.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	eff, err := policyauthority.Resolve(store)
	if err != nil {
		t.Fatal(err)
	}
	digest, _ := eff.Digest()
	if digest != p.EffectiveDigest {
		t.Fatalf("on-disk effective digest %s != planned %s", digest, p.EffectiveDigest)
	}
	// Every file carries the record of the scaffold it came from.
	for _, f := range p.Files {
		data, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.RelPath)))
		if !bytes.Contains(data, []byte(f.Template.Digest)) {
			t.Fatalf("%s does not record its template digest", f.RelPath)
		}
	}
	// Writing twice is refused (the checkout is now adopted).
	if _, err := Compose(root, Input{Profile: governanceprincipal.ClassSolo, Owner: "local-operator", Subject: "s"}); !errors.Is(err, ErrAlreadyAdopted) {
		t.Fatalf("second compose: %v", err)
	}
}
