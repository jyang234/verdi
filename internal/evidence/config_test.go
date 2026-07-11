package evidence

import (
	"os"
	"path/filepath"
	"testing"
)

func writeVerdiYAML(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "verdi.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing verdi.yaml: %v", err)
	}
}

func TestDeviationsStaleThreshold(t *testing.T) {
	t.Run("no verdi.yaml at all: default", func(t *testing.T) {
		got, err := DeviationsStaleThreshold(t.TempDir())
		if err != nil {
			t.Fatalf("DeviationsStaleThreshold: %v", err)
		}
		if got != DefaultDeviationsStaleThreshold {
			t.Fatalf("got %d, want default %d", got, DefaultDeviationsStaleThreshold)
		}
	})

	t.Run("verdi.yaml with no audit block: default", func(t *testing.T) {
		root := t.TempDir()
		writeVerdiYAML(t, root, "schema: verdi.layout/v1\nforge: gitlab\n")
		got, err := DeviationsStaleThreshold(root)
		if err != nil {
			t.Fatalf("DeviationsStaleThreshold: %v", err)
		}
		if got != DefaultDeviationsStaleThreshold {
			t.Fatalf("got %d, want default %d", got, DefaultDeviationsStaleThreshold)
		}
	})

	t.Run("verdi.yaml with audit block but no threshold key: default", func(t *testing.T) {
		root := t.TempDir()
		writeVerdiYAML(t, root, "schema: verdi.layout/v1\naudit:\n  exempts_conflict_threshold: 5\n")
		got, err := DeviationsStaleThreshold(root)
		if err != nil {
			t.Fatalf("DeviationsStaleThreshold: %v", err)
		}
		if got != DefaultDeviationsStaleThreshold {
			t.Fatalf("got %d, want default %d", got, DefaultDeviationsStaleThreshold)
		}
	})

	t.Run("verdi.yaml with an explicit threshold", func(t *testing.T) {
		root := t.TempDir()
		writeVerdiYAML(t, root, "schema: verdi.layout/v1\naudit:\n  deviations_stale_threshold: 7\n")
		got, err := DeviationsStaleThreshold(root)
		if err != nil {
			t.Fatalf("DeviationsStaleThreshold: %v", err)
		}
		if got != 7 {
			t.Fatalf("got %d, want 7", got)
		}
	})

	t.Run("verdi.yaml carrying other unrelated top-level keys still parses", func(t *testing.T) {
		root := t.TempDir()
		writeVerdiYAML(t, root, "schema: verdi.layout/v1\nforge: github\nspike_paths: [design-notes/]\naudit:\n  deviations_stale_threshold: 2\n  exempts_conflict_threshold: 3\n")
		got, err := DeviationsStaleThreshold(root)
		if err != nil {
			t.Fatalf("DeviationsStaleThreshold: %v", err)
		}
		if got != 2 {
			t.Fatalf("got %d, want 2", got)
		}
	})

	t.Run("negative: audit is not a mapping", func(t *testing.T) {
		root := t.TempDir()
		writeVerdiYAML(t, root, "schema: verdi.layout/v1\naudit: not-a-mapping\n")
		if _, err := DeviationsStaleThreshold(root); err == nil {
			t.Fatal("DeviationsStaleThreshold: want error for non-mapping audit block, got nil")
		}
	})

	t.Run("negative: threshold is not an integer", func(t *testing.T) {
		root := t.TempDir()
		writeVerdiYAML(t, root, "schema: verdi.layout/v1\naudit:\n  deviations_stale_threshold: \"three\"\n")
		if _, err := DeviationsStaleThreshold(root); err == nil {
			t.Fatal("DeviationsStaleThreshold: want error for non-integer threshold, got nil")
		}
	})

	t.Run("negative: verdi.yaml is not valid YAML", func(t *testing.T) {
		root := t.TempDir()
		writeVerdiYAML(t, root, "not: [valid: yaml")
		if _, err := DeviationsStaleThreshold(root); err == nil {
			t.Fatal("DeviationsStaleThreshold: want error for unparsable verdi.yaml, got nil")
		}
	})
}
