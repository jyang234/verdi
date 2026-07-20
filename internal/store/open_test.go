package store

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

// TestOpen_Happy proves Open populates both Config fields from a valid
// store: Root exactly as given, and Manifest strict-decoded the same way
// DecodeManifest already proves in manifest_test.go (L-M3's config
// bottleneck: Open is the single place verbs load verdi.yaml).
func TestOpen_Happy(t *testing.T) {
	root := t.TempDir()
	writeVerdiYAML(t, root, validManifestYAML)

	cfg, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if cfg.Root != root {
		t.Fatalf("Config.Root = %q, want %q", cfg.Root, root)
	}
	if cfg.Manifest == nil {
		t.Fatal("Config.Manifest = nil, want populated")
	}
	if cfg.Manifest.Forge != "gitlab" {
		t.Fatalf("Config.Manifest.Forge = %q, want gitlab", cfg.Manifest.Forge)
	}
	if cfg.Manifest.Toolchain == nil || cfg.Manifest.Toolchain.Commit != "cd38b1a56bb782177a207d741a39807821cf2c1c" {
		t.Fatalf("Config.Manifest.Toolchain = %+v, unexpected", cfg.Manifest.Toolchain)
	}
	if cfg.Model == nil {
		t.Fatal("Config.Model = nil, want the embedded canonical default (Task 6: Model is always non-nil after a successful Open)")
	}
}

// TestOpenWithoutModelYAMLYieldsCanonical is Task 6's own load-bearing
// parity claim: a store with no .verdi/model.yaml at all resolves
// Config.Model to EXACTLY model.Canonical() — never nil, never a bare
// zero value — so a store with no manifest changes nothing about how it
// behaves today.
func TestOpenWithoutModelYAMLYieldsCanonical(t *testing.T) {
	root := t.TempDir()
	writeVerdiYAML(t, root, validManifestYAML)

	cfg, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if cfg.Model == nil {
		t.Fatal("Config.Model = nil, want the embedded canonical default")
	}
	want := model.Canonical()
	if !reflect.DeepEqual(*cfg.Model, *want) {
		t.Fatalf("Config.Model = %+v, want model.Canonical() = %+v", *cfg.Model, *want)
	}
}

// testModelYAML is structurally identical to the embedded canonical
// model (same classes/states/transitions/obligations) but names
// different per-class template files — dc-1's frontier passes it, and
// the different Template values give TestOpenPrefersStoreModelYAML a
// distinctive marker proving the STORE's own file was loaded, not the
// embedded default (which names feature.md/story.md, not the
// custom-*.md names below).
const testModelYAML = `schema: verdi.model/v1

classes:
  feature:
    display: Feature
    decomposes: stubs
    template: custom-feature.md
  story:
    display: Story
    parent: feature
    template: custom-story.md

lifecycle:
  feature:
    states: [draft, accepted-pending-build, closed, superseded]
    terminal: [closed, superseded]
    transitions:
      - verb: accept
        from: draft
        to: accepted-pending-build
        obligations:
          - { scheme: attestation, kind: author-vouch }
      - verb: close
        from: accepted-pending-build
        to: closed
        obligations:
          - { scheme: attestation, kind: countersign, count: 1 }
          - { scheme: behavioral, kind: fold-green }
  story:
    states: [draft, accepted-pending-build, closed, superseded]
    terminal: [closed, superseded]
    transitions:
      - verb: accept
        from: draft
        to: accepted-pending-build
        obligations:
          - { scheme: attestation, kind: author-vouch }
      - verb: close
        from: accepted-pending-build
        to: closed
        obligations:
          - { scheme: attestation, kind: countersign, count: 1 }
          - { scheme: behavioral, kind: fold-green }
`

// TestOpenPrefersStoreModelYAML proves a store's OWN .verdi/model.yaml,
// when present, wins over the embedded canonical default.
func TestOpenPrefersStoreModelYAML(t *testing.T) {
	root := t.TempDir()
	writeVerdiYAML(t, root, validManifestYAML)
	writeModelYAML(t, root, testModelYAML)

	cfg, err := Open(root)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if cfg.Model == nil {
		t.Fatal("Config.Model = nil, want the store's own model.yaml")
	}
	if got := cfg.Model.Classes["feature"].Template; got != "custom-feature.md" {
		t.Fatalf("Config.Model.Classes[feature].Template = %q, want %q (the store's own model.yaml, not the embedded canonical default)", got, "custom-feature.md")
	}
}

// TestOpen_ModelNegative proves Open's model-loading half surfaces
// model.DecodeModel's own errors UNWRAPPED (K5): loadModel (open.go) no
// longer re-wraps a DecodeModel failure with its own "decoding
// model.yaml: %w", since DecodeModel already self-prefixes the ONE case
// that needs it (its strict-decode-failure path: "model: decoding
// model.yaml: %w", decode.go) — re-wrapping THAT case doubled the prefix
// ("decoding model.yaml: model: decoding model.yaml: ..."), the exact
// defect this test now pins shut via the "at most one occurrence" check
// below. Validate/checkFrontier failures were never self-prefixed with
// "decoding model.yaml:" to begin with, so dropping loadModel's own wrap
// leaves them with NO such framing at all — each case's wantPrefix/
// wantDecodeMentions reflects its own real shape, not a uniform one.
func TestOpen_ModelNegative(t *testing.T) {
	cases := []struct {
		name       string
		modelYAML  string
		wantPrefix string
		// wantDecodeMentions is the exact number of times the literal
		// substring "decoding model.yaml:" appears in the final error —
		// 1 for the strict-decode-failure case (DecodeModel's own single
		// self-prefix survives, unrewrapped), 0 for Validate/checkFrontier
		// failures (never carried that text at all). Before the fix, the
		// strict-decode case doubled to 2 — a HasPrefix-only check never
		// would have caught that, which is why this test now also asserts
		// the exact count.
		wantDecodeMentions int
	}{
		{
			name:               "malformed YAML syntax",
			modelYAML:          "not: [valid",
			wantPrefix:         "model: decoding model.yaml: ",
			wantDecodeMentions: 1,
		},
		{
			name:               "structurally deviant (frontier violation)",
			modelYAML:          "schema: verdi.model/v1\nclasses: {}\nlifecycle: {}\n",
			wantPrefix:         model.ErrFrontier.Error(),
			wantDecodeMentions: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeVerdiYAML(t, root, validManifestYAML)
			writeModelYAML(t, root, tc.modelYAML)

			cfg, err := Open(root)
			if err == nil {
				t.Fatalf("Open(%s): want error, got nil (cfg=%+v)", tc.name, cfg)
			}
			if !strings.HasPrefix(err.Error(), tc.wantPrefix) {
				t.Fatalf("Open(%s) error = %q, want prefix %q", tc.name, err.Error(), tc.wantPrefix)
			}
			if got := strings.Count(err.Error(), "decoding model.yaml:"); got != tc.wantDecodeMentions {
				t.Fatalf("Open(%s) error = %q, contains \"decoding model.yaml:\" %d times, want exactly %d", tc.name, err.Error(), got, tc.wantDecodeMentions)
			}
		})
	}
}

// TestOpen_Negative pins the exact error wrapping cmd/verdi/forgeboot.go's
// pre-move loadManifest produced, copied verbatim from its source before
// the body moved here, so the promotion into store.Open is provably
// behavior-preserving (L-M3; ../../cmd/verdi/forgeboot.go's loadManifest
// is now a thin delegate to Open — see forgeboot.go):
//
//	data, err := os.ReadFile(filepath.Join(root, ".verdi", "verdi.yaml"))
//	if err != nil {
//	        return nil, fmt.Errorf("reading verdi.yaml: %w", err)
//	}
//	m, err := store.DecodeManifest(data)
//	if err != nil {
//	        return nil, fmt.Errorf("decoding verdi.yaml: %w", err)
//	}
func TestOpen_Negative(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, root string)
		wantPrefix string // loadManifest's own wrapping text, byte-identical
	}{
		{
			name:       "missing verdi.yaml",
			setup:      func(t *testing.T, root string) {},
			wantPrefix: "reading verdi.yaml: ",
		},
		{
			// Malformed YAML syntax: the same "not: [valid" fixture
			// manifest_test.go's TestDecodeManifest_Negative uses, so this
			// proves Open surfaces DecodeManifest's own strict-decode error
			// (artifact.DecodeStrict's yaml-parse failure) unchanged.
			name: "malformed YAML syntax",
			setup: func(t *testing.T, root string) {
				writeVerdiYAML(t, root, "not: [valid")
			},
			wantPrefix: "decoding verdi.yaml: ",
		},
		{
			// Syntactically valid YAML that strict-decode still rejects
			// (unknown top-level key) — proves the KnownFields(true) wall,
			// not just YAML-parse failures, surfaces through Open.
			name: "unknown field rejected by strict decode",
			setup: func(t *testing.T, root string) {
				writeVerdiYAML(t, root, "schema: verdi.layout/v1\nbogus: true\n")
			},
			wantPrefix: "decoding verdi.yaml: ",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)

			cfg, err := Open(root)
			if err == nil {
				t.Fatalf("Open(%s): want error, got nil (cfg=%+v)", tc.name, cfg)
			}
			if !strings.HasPrefix(err.Error(), tc.wantPrefix) {
				t.Fatalf("Open(%s) error = %q, want prefix %q", tc.name, err.Error(), tc.wantPrefix)
			}
		})
	}
}

// writeVerdiYAML materializes root/.verdi/verdi.yaml with data, mirroring
// the fixture-building convention cmd/verdi/sync_test.go's buildTestStore
// and this package's own manifest.go tests use.
func writeVerdiYAML(t *testing.T, root, data string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "verdi.yaml"), []byte(data), 0o644); err != nil {
		t.Fatalf("writing verdi.yaml: %v", err)
	}
}

// writeModelYAML materializes root/.verdi/model.yaml with data, mirroring
// writeVerdiYAML above (Task 6).
func writeModelYAML(t *testing.T, root, data string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "model.yaml"), []byte(data), 0o644); err != nil {
		t.Fatalf("writing model.yaml: %v", err)
	}
}
