package supersede

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/store"
)

// TestValidateSuccessorName is the table proof of the successor-side
// precondition both callers of this package share (the CLI today, the
// board's Revise action at W3-C): the successor's name must parse as a
// spec ref, and its store directory must not already exist.
func TestValidateSuccessorName(t *testing.T) {
	t.Run("a fresh, well-formed successor name passes", func(t *testing.T) {
		root := t.TempDir()
		ref, err := ValidateSuccessorName(root, "lockbox-v2")
		if err != nil {
			t.Fatalf("ValidateSuccessorName = %v, want no error", err)
		}
		if ref.String() != "spec/lockbox-v2" {
			t.Fatalf("ref = %q, want spec/lockbox-v2", ref.String())
		}
	})

	t.Run("an existing successor directory refuses, naming the path", func(t *testing.T) {
		root := t.TempDir()
		dir := store.ActiveSpecDir(root, "lockbox-v2")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		_, err := ValidateSuccessorName(root, "lockbox-v2")
		var nerr *NameError
		if !errors.As(err, &nerr) {
			t.Fatalf("ValidateSuccessorName = %v, want a *NameError", err)
		}
		if nerr.Reason != ReasonSuccessorExists {
			t.Fatalf("Reason = %q, want %q", nerr.Reason, ReasonSuccessorExists)
		}
		if nerr.Path != dir {
			t.Fatalf("Path = %q, want %q", nerr.Path, dir)
		}
	})

	t.Run("an existing successor FILE refuses too", func(t *testing.T) {
		root := t.TempDir()
		dir := store.ActiveSpecDir(root, "lockbox-v2")
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(dir, []byte("not a directory"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		_, err := ValidateSuccessorName(root, "lockbox-v2")
		var nerr *NameError
		if !errors.As(err, &nerr) || nerr.Reason != ReasonSuccessorExists {
			t.Fatalf("ValidateSuccessorName = %v, want a *NameError with reason %q", err, ReasonSuccessorExists)
		}
	})

	badNames := []struct {
		name string
		why  string
	}{
		{"", "empty"},
		{"Not_A_Valid_Name", "not kebab-case"},
		{"nested/name", "a path separator"},
		{"UPPER", "uppercase"},
	}
	for _, bad := range badNames {
		t.Run("refuses a name that is "+bad.why, func(t *testing.T) {
			root := t.TempDir()
			_, err := ValidateSuccessorName(root, bad.name)
			var nerr *NameError
			if !errors.As(err, &nerr) {
				t.Fatalf("ValidateSuccessorName(%q) = %v, want a *NameError", bad.name, err)
			}
			if nerr.Reason != ReasonInvalidName {
				t.Fatalf("Reason = %q, want %q", nerr.Reason, ReasonInvalidName)
			}
			if nerr.Name != bad.name {
				t.Fatalf("Name = %q, want %q", nerr.Name, bad.name)
			}
			if errors.Unwrap(nerr) == nil {
				t.Fatal("NameError wraps no parse error; the caller cannot report WHY the name was refused")
			}
		})
	}
}
