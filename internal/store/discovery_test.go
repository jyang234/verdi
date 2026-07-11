package store

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

const svcFlowmapYAML = `version: 1
service: svcfix
obligations:
  - name: audit-before-publish
    require: "example.com/svcfix/internal/audit#Write"
    before: "example.com/svcfix/internal/bus#Publish"
`

const svcBoundaryContractJSON = `{
  "service": "svcfix",
  "schema_version": "flowmap.boundary/v1",
  "entrypoints": { "http": [], "consumers": [] },
  "published": [],
  "consumed": [],
  "external_dependencies": [],
  "blind_spots": []
}
`

const svcBindingsYAML = `schema: verdi.bindings/v1
spec: spec/stale-decline
bindings:
  - { producer: audit-before-publish, kind: static, acs: [ac-1] }
`

func TestDiscoverServices_Happy(t *testing.T) {
	root := t.TempDir()

	writeFileT(t, filepath.Join(root, "svcfix", flowmapFile), svcFlowmapYAML)
	writeFileT(t, filepath.Join(root, "svcfix", BoundaryContractRelPath), svcBoundaryContractJSON)
	writeFileT(t, filepath.Join(root, "svcfix", bindingsFile), svcBindingsYAML)
	writeFileT(t, filepath.Join(root, "svcfix", "api", "openapi.yaml"), "openapi: 3.0.3\ninfo:\n  title: x\n  version: \"1\"\npaths: {}\n")

	// A second, minimal service with no companion files and no `service:`
	// key, to prove the dir-name default and the presence-only fields.
	writeFileT(t, filepath.Join(root, "barefix", flowmapFile), "version: 1\n")

	// Noise that must not be discovered as (or descended into for) services.
	writeFileT(t, filepath.Join(root, ".git", "objects", flowmapFile), "service: ghost\n")
	writeFileT(t, filepath.Join(root, "node_modules", "pkg", flowmapFile), "service: ghost\n")
	writeFileT(t, filepath.Join(root, ".verdi", "data", "cache", flowmapFile), "service: ghost\n")
	// testdata/ is this module's own fixture directory (PLAN.md §2: "the
	// hermetic fixture"); phase 4 added it to skipDirNames so self-hosting
	// this repo's own store does not discover testdata/svcfix and
	// testdata/corpus's fixture .flowmap.yaml files as real services.
	writeFileT(t, filepath.Join(root, "testdata", "fixturesvc", flowmapFile), "service: ghost\n")

	got, err := DiscoverServices(root)
	if err != nil {
		t.Fatalf("DiscoverServices: %v", err)
	}
	if len(got) != 2 {
		names := make([]string, len(got))
		for i, s := range got {
			names[i] = s.Name
		}
		t.Fatalf("got %d services %v, want 2 (svcfix, barefix)", len(got), names)
	}

	byName := map[string]Service{}
	for _, s := range got {
		byName[s.Name] = s
	}

	svcfix, ok := byName["svcfix"]
	if !ok {
		t.Fatalf("svcfix not discovered; got %+v", got)
	}
	if len(svcfix.Obligations) != 1 || svcfix.Obligations[0] != "audit-before-publish" {
		t.Fatalf("svcfix.Obligations = %v, want [audit-before-publish]", svcfix.Obligations)
	}
	if svcfix.BoundaryContractPath == "" {
		t.Fatal("svcfix.BoundaryContractPath not set")
	}
	if svcfix.BindingsPath == "" || svcfix.Bindings == nil {
		t.Fatalf("svcfix bindings not discovered/decoded: path=%q bindings=%v", svcfix.BindingsPath, svcfix.Bindings)
	}
	if svcfix.Bindings.Spec != "spec/stale-decline" {
		t.Fatalf("svcfix.Bindings.Spec = %q, want spec/stale-decline", svcfix.Bindings.Spec)
	}
	if svcfix.OpenAPIPath == "" {
		t.Fatal("svcfix.OpenAPIPath not set")
	}

	barefix, ok := byName["barefix"]
	if !ok {
		t.Fatalf("barefix not discovered; got %+v", got)
	}
	if barefix.Name != "barefix" {
		t.Fatalf("barefix.Name = %q, want dir-name default %q", barefix.Name, "barefix")
	}
	if barefix.BoundaryContractPath != "" || barefix.BindingsPath != "" || barefix.OpenAPIPath != "" {
		t.Fatalf("barefix has unexpected companion files: %+v", barefix)
	}
}

func TestDiscoverServices_NoServices(t *testing.T) {
	root := t.TempDir()
	writeFileT(t, filepath.Join(root, ".verdi", "verdi.yaml"), "schema: verdi.layout/v1\n")

	got, err := DiscoverServices(root)
	if err != nil {
		t.Fatalf("DiscoverServices: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d services, want 0", len(got))
	}
}

func TestDiscoverServices_Negative(t *testing.T) {
	t.Run("dialect violation in .flowmap.yaml", func(t *testing.T) {
		root := t.TempDir()
		writeFileT(t, filepath.Join(root, "badsvc", flowmapFile), "service: &s svcfix\nother: *s\n")
		if _, err := DiscoverServices(root); err == nil {
			t.Fatal("DiscoverServices: want error on dialect violation, got nil")
		}
	})

	t.Run("malformed bindings sidecar", func(t *testing.T) {
		root := t.TempDir()
		writeFileT(t, filepath.Join(root, "badsvc", flowmapFile), svcFlowmapYAML)
		writeFileT(t, filepath.Join(root, "badsvc", bindingsFile), "schema: verdi.bindings/v0\nspec: spec/x\nbindings: []\n")
		if _, err := DiscoverServices(root); err == nil {
			t.Fatal("DiscoverServices: want error on malformed bindings sidecar, got nil")
		}
	})

	t.Run("nonexistent root", func(t *testing.T) {
		if _, err := DiscoverServices(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
			t.Fatal("DiscoverServices(nonexistent root): want error, got nil")
		}
	})
}

func TestFilterImpacted(t *testing.T) {
	services := []Service{{Name: "loansvc"}, {Name: "notification-svc"}, {Name: "other"}}

	got := FilterImpacted(services, []string{"loansvc", "notification-svc"})
	if len(got) != 2 || got[0].Name != "loansvc" || got[1].Name != "notification-svc" {
		t.Fatalf("FilterImpacted = %+v, want [loansvc notification-svc]", got)
	}

	if got := FilterImpacted(services, nil); len(got) != 0 {
		t.Fatalf("FilterImpacted(nil impacts) = %+v, want empty", got)
	}
	if got := FilterImpacted(services, []string{"nope"}); len(got) != 0 {
		t.Fatalf("FilterImpacted(unmatched impact) = %+v, want empty", got)
	}
}
