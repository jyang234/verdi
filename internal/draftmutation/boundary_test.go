package draftmutation_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/store"
)

const draftMutationImport = "github.com/jyang234/verdi/internal/draftmutation"

func TestLaterWorkbenchAndMCPAdaptersDoNotImportDraftMutation(t *testing.T) {
	repositoryRoot := filepath.Join("..", "..")
	targets := []string{
		filepath.Join(repositoryRoot, "internal", "workbench"),
		filepath.Join(repositoryRoot, "internal", "mcpserve"),
		filepath.Join(repositoryRoot, "cmd", "verdi", "mcp.go"),
		filepath.Join(repositoryRoot, "cmd", "verdi", "serve.go"),
	}
	parsed := 0
	for _, target := range targets {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatal(err)
		}
		paths := []string{target}
		if info.IsDir() {
			paths = nil
			err = filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
					paths = append(paths, path)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		for _, path := range paths {
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			parsed++
			for _, imported := range file.Imports {
				value, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					t.Fatalf("unquoting import in %s: %v", path, err)
				}
				if value == draftMutationImport {
					t.Fatalf("later adapter %s imports draftmutation before its delivery unit", path)
				}
			}
		}
	}
	if parsed == 0 {
		t.Fatal("adapter import witness parsed no production Go files")
	}
}

func TestDesignProvenanceExclusionIsConsumableWithoutArtifactClassification(t *testing.T) {
	identity, err := designprovenance.ResolveIdentity("spec/sample", store.ZoneActive)
	if err != nil {
		t.Fatal(err)
	}
	if identity.ExclusionReason != designprovenance.ExclusionReason {
		t.Fatalf("external exclusion identity = %+v", identity)
	}
	if designprovenance.ExclusionReason != "design-provenance-sidecar" {
		t.Fatalf("ExclusionReason = %q", designprovenance.ExclusionReason)
	}
	if kind, ok := artifact.ClassifyPath("specs/active/sample/design-provenance.jsonl"); ok || kind != "" {
		t.Fatalf("ClassifyPath classified non-authoritative sidecar as %q, ok=%v", kind, ok)
	}
	if kind, ok := artifact.ClassifyPath("specs/active/sample/spec.md"); !ok || kind != "spec" {
		t.Fatalf("ClassifyPath existing spec behavior changed to %q, ok=%v", kind, ok)
	}
}
