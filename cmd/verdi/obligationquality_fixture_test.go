package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// fixtureElaboratedObligationMD renders the smallest authored obligation a
// command fixture can use to prove exact producer/job binding. Code is the
// only invalidator because these fixtures produce evidence for their exact
// evaluation commit.
func fixtureElaboratedObligationMD(specName, acID string, kind artifact.EvidenceKind, producer, job, frozen string) string {
	return "---\nid: obligation/" + specName + "--" + acID + "--" + string(kind) +
		"\nkind: obligation\ntitle: Quality fixture\nowners: [platform-team]\nfor_kind: " + string(kind) +
		"\nquality:\n  state: elaborated\n  claim: fixture claim\n  falsifier: fixture falsifier\n  scope: fixture scope" +
		"\n  producer: { kind: checker, ref: \"" + producer + "\" }" +
		"\n  authoritative_source: { kind: ci-job, ref: \"" + job + "\" }" +
		"\n  freshness:\n    invalidated_by: [code]\n    rule: rerun for the evaluated commit" +
		"\nlinks:\n  - { type: verifies, ref: \"spec/" + specName + "\" }" +
		"\nfrozen: { at: 2024-01-01, commit: " + frozen + " }\n---\n# Quality fixture\n\nAuthored obligation.\n"
}

// markFixtureLegacyObligationsUnresolved upgrades only a temporary fixturegit
// checkout. The repository corpus remains byte-for-byte legacy for the
// compatibility witness, while matrix tests exercise post-adoption semantics
// without pretending their synthetic commit graph contains the adoption SHA.
func markFixtureLegacyObligationsUnresolved(t *testing.T, root string) {
	t.Helper()
	base := filepath.Join(root, ".verdi", "obligations")
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(raw), "\nquality:\n") {
			return nil
		}
		updated := strings.Replace(string(raw), "\nlinks:\n", "\nquality:\n  state: unresolved-design-debt\nlinks:\n", 1)
		if updated == string(raw) {
			return nil
		}
		return os.WriteFile(path, []byte(updated), 0o644)
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("marking fixture obligations unresolved: %v", err)
	}
}
