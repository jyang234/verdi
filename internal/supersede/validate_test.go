package supersede

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jyang234/verdi/internal/store"
)

// TestValidateSuccessorName_ReExportWiring proves this package's thin
// re-export of internal/specname.ValidateSuccessorName wires correctly —
// happy path, and one refusal per the reason constants this package itself
// re-exports (invalid name, active exists, archived exists) — so a caller
// still reading supersede.ValidateSuccessorName/supersede.NameError/
// supersede.ReasonXxx (designsupersede.go; the board's actionRevise) gets
// the exact same behavior as a direct internal/specname caller. The
// exhaustive table (every refusal shape, including the base-ref check) is
// internal/specname/validate_test.go's own; this file does not duplicate
// it.
func TestValidateSuccessorName_ReExportWiring(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		root := t.TempDir()
		ref, err := ValidateSuccessorName(context.Background(), root, "lockbox-v2", "")
		if err != nil {
			t.Fatalf("ValidateSuccessorName = %v, want no error", err)
		}
		if ref.String() != "spec/lockbox-v2" {
			t.Fatalf("ref = %q, want spec/lockbox-v2", ref.String())
		}
	})

	t.Run("invalid name (fragment, UAT-030)", func(t *testing.T) {
		root := t.TempDir()
		_, err := ValidateSuccessorName(context.Background(), root, "foo#dc-1", "")
		var nerr *NameError
		if !errors.As(err, &nerr) || nerr.Reason != ReasonInvalidName {
			t.Fatalf("ValidateSuccessorName = %v, want a *NameError with reason %q", err, ReasonInvalidName)
		}
	})

	t.Run("active zone exists", func(t *testing.T) {
		root := t.TempDir()
		dir := store.ActiveSpecDir(root, "lockbox-v2")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := ValidateSuccessorName(context.Background(), root, "lockbox-v2", "")
		var nerr *NameError
		if !errors.As(err, &nerr) || nerr.Reason != ReasonSuccessorExists {
			t.Fatalf("ValidateSuccessorName = %v, want a *NameError with reason %q", err, ReasonSuccessorExists)
		}
		if nerr.Path != dir {
			t.Fatalf("Path = %q, want %q", nerr.Path, dir)
		}
	})

	t.Run("archive zone exists (UAT-032)", func(t *testing.T) {
		root := t.TempDir()
		dir := store.ArchiveSpecDir(root, "retired")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err := ValidateSuccessorName(context.Background(), root, "retired", "")
		var nerr *NameError
		if !errors.As(err, &nerr) || nerr.Reason != ReasonArchivedExists {
			t.Fatalf("ValidateSuccessorName = %v, want a *NameError with reason %q", err, ReasonArchivedExists)
		}
	})
}
