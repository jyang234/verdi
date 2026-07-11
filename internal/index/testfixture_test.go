package index

import (
	"os"
	"path/filepath"
	"testing"
)

// writeIndexFile writes content to root/relPath, creating parent
// directories as needed.
func writeIndexFile(t *testing.T, root, relPath, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

const syntheticADR0001 = `---
id: adr/0001-a
kind: adr
title: "ADR one"
status: proposed
owners: [platform-team]
---
# ADR one

Body text mentioning zephyrtoken, a distinctive search term.
`

const syntheticADR0002 = `---
id: adr/0002-b
kind: adr
title: "ADR two"
status: proposed
owners: [platform-team]
links:
  - { type: supersedes, ref: adr/0001-a }
---
# ADR two

Supersedes ADR one.
`

const syntheticComponentSpec = `---
id: spec/my-spec
kind: spec
class: component
title: "My component spec"
status: active
owners: [platform-team]
links:
  - { type: impacts, ref: svc/svcfix/boundary-contract }
---
# My component spec

References svcfix's boundary.
`

const syntheticFlowmapYAML = `version: 1
service: svcfix
obligations:
  - name: audit-before-publish
    require: "x#Y"
    before: "x#Z"
`

const syntheticBoundaryContractJSON = `{
  "service": "svcfix",
  "schema_version": "flowmap.boundary/v1",
  "entrypoints": { "http": [], "consumers": [] },
  "published": [],
  "consumed": [],
  "external_dependencies": [],
  "blind_spots": []
}
`

const syntheticOpenAPIYAML = `openapi: 3.0.3
info:
  title: x
  version: "1"
paths: {}
`

// buildSyntheticStore creates a small, self-contained store tree (no git
// history needed) with two ADRs (one superseding the other), one component
// spec impacting svcfix's boundary contract, and one discovered service
// (svcfix, with a boundary contract, one obligation, and an OpenAPI doc).
func buildSyntheticStore(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	writeIndexFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
	writeIndexFile(t, root, ".verdi/adr/0001-a.md", syntheticADR0001)
	writeIndexFile(t, root, ".verdi/adr/0002-b.md", syntheticADR0002)
	writeIndexFile(t, root, ".verdi/specs/active/my-spec/spec.md", syntheticComponentSpec)
	// A non-artifact companion file that must NOT be indexed.
	writeIndexFile(t, root, ".verdi/specs/active/my-spec/board.json", `{"schema":"verdi.board/v1","pins":[],"stickies":[],"yarn":[]}`)

	writeIndexFile(t, root, "svcfix/.flowmap.yaml", syntheticFlowmapYAML)
	writeIndexFile(t, root, "svcfix/.flowmap/boundary-contract.json", syntheticBoundaryContractJSON)
	writeIndexFile(t, root, "svcfix/api/openapi.yaml", syntheticOpenAPIYAML)

	return root
}
