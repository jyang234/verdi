package policyintegration

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/humanartifact"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyauthority"
)

// TestProspective_TemplateChange is CO-5's template half: "Historical
// artifacts keep the template ... digests that shaped them. Verdi never
// rewrites history to make an old artifact appear born under a new
// rule." An artifact rendered from the embedded canonical policy.md
// scaffold is written to disk; a store override then changes what
// policy.md resolves to; a SECOND artifact rendered afterward records
// the override's own identity and digest, while the FIRST artifact's
// bytes and its own recorded template identity/digest are untouched —
// re-decoding it still produces the exact digest it had before the
// override ever existed.
func TestProspective_TemplateChange(t *testing.T) {
	root := t.TempDir()

	scaffold1, err := humanartifact.ResolveScaffold(root, "policy.md")
	if err != nil {
		t.Fatalf("ResolveScaffold (before override): %v", err)
	}
	if scaffold1.Identity != "embedded:policy.md" {
		t.Fatalf("scaffold1.Identity = %q, want the embedded identity", scaffold1.Identity)
	}
	data1 := humanartifact.PolicyScaffoldData{
		Name:             "first-policy",
		Title:            "First Policy",
		Owners:           []string{"platform-team"},
		TemplateIdentity: scaffold1.Identity,
		TemplateDigest:   scaffold1.Digest,
	}
	content1, err := humanartifact.RenderPolicy(scaffold1, data1)
	if err != nil {
		t.Fatalf("RenderPolicy (before override): %v", err)
	}
	decoded1, err := policyartifact.DecodePolicy([]byte(content1))
	if err != nil {
		t.Fatalf("DecodePolicy (before override): %v", err)
	}
	digest1, err := decoded1.Digest()
	if err != nil {
		t.Fatalf("Digest (before override): %v", err)
	}

	firstPath := filepath.Join(root, ".verdi", "policy", "policies", "first-policy.md")
	if err := os.MkdirAll(filepath.Dir(firstPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(firstPath, []byte(content1), 0o644); err != nil {
		t.Fatalf("writing first-policy.md: %v", err)
	}

	// A store override that changes policy.md's rationale placeholder
	// but keeps the rest of the frontmatter shape a valid render needs
	// (RenderPolicy's own kernel round trip requires it) — a legitimate
	// template evolution, not a sabotage case.
	override := strings.Replace(
		string(mustReadCanonicalPolicyTemplate(t)),
		"TODO: replace with the real rationale before accept.",
		"TODO: fill in the real rationale; this line moved under the new template.",
		1,
	)
	if override == string(mustReadCanonicalPolicyTemplate(t)) {
		t.Fatal("test setup: override template is byte-identical to the canonical one")
	}
	templatesDir := filepath.Join(root, ".verdi", "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "policy.md"), []byte(override), 0o644); err != nil {
		t.Fatalf("writing template override: %v", err)
	}

	scaffold2, err := humanartifact.ResolveScaffold(root, "policy.md")
	if err != nil {
		t.Fatalf("ResolveScaffold (after override): %v", err)
	}
	if scaffold2.Identity != "store:.verdi/templates/policy.md" {
		t.Fatalf("scaffold2.Identity = %q, want the store override identity", scaffold2.Identity)
	}
	if scaffold2.Digest == scaffold1.Digest {
		t.Fatal("scaffold2.Digest equals scaffold1.Digest; the override did not change the resolved template")
	}

	data2 := humanartifact.PolicyScaffoldData{
		Name:             "second-policy",
		Title:            "Second Policy",
		Owners:           []string{"platform-team"},
		TemplateIdentity: scaffold2.Identity,
		TemplateDigest:   scaffold2.Digest,
	}
	content2, err := humanartifact.RenderPolicy(scaffold2, data2)
	if err != nil {
		t.Fatalf("RenderPolicy (after override): %v", err)
	}
	decoded2, err := policyartifact.DecodePolicy([]byte(content2))
	if err != nil {
		t.Fatalf("DecodePolicy (after override): %v", err)
	}
	if decoded2.Template == nil || decoded2.Template.Identity != scaffold2.Identity || decoded2.Template.Digest != scaffold2.Digest {
		t.Fatalf("second artifact's template record = %+v, want identity %q digest %q", decoded2.Template, scaffold2.Identity, scaffold2.Digest)
	}

	// --- history not reinterpreted: the FIRST artifact's bytes on disk
	// are untouched by the override or the second render.
	gotFirstBytes, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("re-reading first-policy.md: %v", err)
	}
	if !bytes.Equal(gotFirstBytes, []byte(content1)) {
		t.Fatalf("first-policy.md bytes changed after the template override:\nwant=%q\ngot=%q", content1, gotFirstBytes)
	}
	redecoded1, err := policyartifact.DecodePolicy(gotFirstBytes)
	if err != nil {
		t.Fatalf("re-decoding first-policy.md: %v", err)
	}
	if redecoded1.Template == nil || redecoded1.Template.Identity != scaffold1.Identity || redecoded1.Template.Digest != scaffold1.Digest {
		t.Fatalf("first artifact's re-decoded template record = %+v, want the ORIGINAL embedded identity %q digest %q (not reinterpreted under the override)", redecoded1.Template, scaffold1.Identity, scaffold1.Digest)
	}
	redigest1, err := redecoded1.Digest()
	if err != nil {
		t.Fatalf("re-Digest of first-policy.md: %v", err)
	}
	if redigest1 != digest1 {
		t.Fatalf("first artifact's digest changed after the template override: was %q, now %q", digest1, redigest1)
	}
}

// mustReadCanonicalPolicyTemplate reads the embedded policy.md template's
// own source file directly (not through ResolveScaffold, which this
// test is exercising) so the override fixture can start from an exact
// copy of the real canonical template rather than a hand-duplicated
// approximation that could silently drift from it.
func mustReadCanonicalPolicyTemplate(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "designscaffold", "templates", "policy.md"))
	if err != nil {
		t.Fatalf("reading the canonical policy.md template source: %v", err)
	}
	return data
}

// TestProspective_PolicyChange is CO-5's policy half: a Resolve output
// held in memory from BEFORE a policy edit keeps digesting to its own
// old value (Digest() proves an unmodified seal, never recomputes from
// current disk state), a FRESH Resolve after the same edit yields the
// new value, and a previously generated projection file's bytes stay
// exactly what Generate last wrote until Generate runs again — editing
// canonical policy never silently rewrites an already-written
// projection (AC-1: "Editing a projection does not change authority;
// drift is a named blocking witness until the projection is regenerated
// or the canonical policy is changed through governance").
func TestProspective_PolicyChange(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, baseStoreFiles(t))

	store1, err := policyauthority.Load(root)
	if err != nil {
		t.Fatalf("Load (before edit): %v", err)
	}
	ep1, err := policyauthority.Resolve(store1)
	if err != nil {
		t.Fatalf("Resolve (before edit): %v", err)
	}
	d1, err := ep1.Digest()
	if err != nil {
		t.Fatalf("ep1.Digest() (before edit): %v", err)
	}

	if _, err := instructionprojection.Generate(root); err != nil {
		t.Fatalf("Generate (before edit): %v", err)
	}
	agentsPath := filepath.Join(root, "AGENTS.md")
	agentsBefore, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("reading AGENTS.md (before edit): %v", err)
	}

	// Edit the canonical policy directly: append a second instruction.
	policyPath := filepath.Join(root, ".verdi", "policy", "policies", "go-toolchain.md")
	raw, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(raw),
		`instructions:
  - "Run make verify before claiming completion."`,
		`instructions:
  - "Run make verify before claiming completion."
  - "Never claim a gate passed without command output."`,
		1)
	if edited == string(raw) {
		t.Fatal("test setup: instructions block not found to edit")
	}
	if err := os.WriteFile(policyPath, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// The OLD in-memory EffectivePolicy still digests to its own old
	// value: Digest() re-verifies ep1's own seal against ep1's own
	// content, never against whatever is currently on disk.
	d1Again, err := ep1.Digest()
	if err != nil {
		t.Fatalf("ep1.Digest() (after edit, same in-memory value): %v", err)
	}
	if d1Again != d1 {
		t.Fatalf("ep1.Digest() changed after an on-disk edit: was %q, now %q (a sealed value must never re-read disk state)", d1, d1Again)
	}

	// A FRESH Resolve sees the new value.
	store2, err := policyauthority.Load(root)
	if err != nil {
		t.Fatalf("Load (after edit): %v", err)
	}
	ep2, err := policyauthority.Resolve(store2)
	if err != nil {
		t.Fatalf("Resolve (after edit): %v", err)
	}
	d2, err := ep2.Digest()
	if err != nil {
		t.Fatalf("ep2.Digest() (after edit): %v", err)
	}
	if d2 == d1 {
		t.Fatal("a fresh Resolve after editing canonical policy produced the same digest as before the edit")
	}

	// The projection file Generate last wrote is UNCHANGED until
	// Generate runs again — the edit alone never rewrites it.
	agentsStillBefore, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("reading AGENTS.md (after edit, before re-Generate): %v", err)
	}
	if !bytes.Equal(agentsStillBefore, agentsBefore) {
		t.Fatal("AGENTS.md changed on disk from the policy edit alone, before any re-Generate call")
	}

	if _, err := instructionprojection.Generate(root); err != nil {
		t.Fatalf("Generate (after edit): %v", err)
	}
	agentsAfter, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("reading AGENTS.md (after re-Generate): %v", err)
	}
	if bytes.Equal(agentsAfter, agentsBefore) {
		t.Fatal("AGENTS.md did not change after re-Generate, even though the resolved authority changed")
	}
	if !strings.Contains(string(agentsAfter), "Never claim a gate passed without command output.") {
		t.Fatalf("regenerated AGENTS.md does not carry the new instruction:\n%s", agentsAfter)
	}
}
