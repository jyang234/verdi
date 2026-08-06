package instructionprojection

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/policyauthority"
)

// writeTree materializes files (relative-path -> content) under root,
// creating parent directories as needed (the same pattern
// internal/policyauthority's own store_test.go uses; copied rather than
// imported per this lane's write-set boundary).
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

// copyTree copies every regular file under src into dst, preserving
// relative structure. Used to seed a t.TempDir() from the committed
// testdata/store/ fixture so a test can mutate its own private copy.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("reading %s: %v", src, err)
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", d, err)
			}
			copyTree(t, s, d)
			continue
		}
		data, err := os.ReadFile(s)
		if err != nil {
			t.Fatalf("reading %s: %v", s, err)
		}
		if err := os.MkdirAll(filepath.Dir(d), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", d, err)
		}
		if err := os.WriteFile(d, data, 0o644); err != nil {
			t.Fatalf("writing %s: %v", d, err)
		}
	}
}

// newFixtureRoot returns a fresh temp dir seeded from testdata/store/
// (one adapter "codex", managed [AGENTS.md, docs/AGENTS.md], discovery
// [AGENTS.md]; policy/go-toolchain with two instructions;
// policy/silent with zero).
func newFixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyTree(t, filepath.Join("testdata", "store"), root)
	return root
}

// newMultiFixtureRoot returns a fresh temp dir seeded from
// testdata/multi-store/ — the realistic two-adapter layout: codex
// (managed [AGENTS.md], discovery [AGENTS.md]) and claude-code (managed
// [CLAUDE.md], discovery [AGENTS.md, CLAUDE.md]), so one adapter
// discovers a file another adapter generates.
func newMultiFixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyTree(t, filepath.Join("testdata", "multi-store"), root)
	return root
}

// soloDefaultProfileDoc is the one governance profile every in-test
// store fixture in this package selects. Written once here rather than
// re-pasted per fixture: an incidental divergence between copies would
// change resolved authority (and therefore projected content) for
// reasons no test intended.
const soloDefaultProfileDoc = `---
schema: verdi.governance-profile/v1
id: solo-default
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - {id: github-org, kind: forge}
role_mappings:
  - {role: author, trust_source: github-org, subjects: [alice]}
  - {role: policy-owner, trust_source: github-org, subjects: [alice]}
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
---
The solo operator profile.
`

// loadResolve is the test-only equivalent of Generate/Verify's own
// Load+Resolve pair, used where a test needs the resolved authority
// itself (to render expected content, or to drive verify's store-
// agnostic core directly) rather than only Verify's verdict.
func loadResolve(t *testing.T, root string) (*policyauthority.Store, *policyauthority.EffectivePolicy) {
	t.Helper()
	store, err := policyauthority.Load(root)
	if err != nil {
		t.Fatalf("policyauthority.Load(%s): %v", root, err)
	}
	ep, err := policyauthority.Resolve(store)
	if err != nil {
		t.Fatalf("policyauthority.Resolve: %v", err)
	}
	return store, ep
}
