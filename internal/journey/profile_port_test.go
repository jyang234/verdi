package journey

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/policyauthority"
)

const fixtureSelectedProfileDigest = "sha256:e57f7183f68956ba34e4208655e34656a26b7090f9bc8ba275a141e215af46a3"

func TestProfileLoader_Load_Adopted(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "policyauthority", "testdata", "store")
	got, err := NewProfileLoader().Load(context.Background(), root)
	if err != nil {
		t.Fatalf("ProfileLoader.Load: %v", err)
	}
	if got.ID != "solo-default" {
		t.Fatalf("ProfileSelection.ID = %q, want solo-default", got.ID)
	}
	if got.Digest != fixtureSelectedProfileDigest {
		t.Fatalf("ProfileSelection.Digest = %q, want %q", got.Digest, fixtureSelectedProfileDigest)
	}
}

func TestProfileLoader_Load_NonAdoptedAndInvalidStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		arrange        func(*testing.T, string)
		wantNotAdopted bool
		wantErr        string
	}{
		{
			name:           "genuine absence",
			arrange:        func(*testing.T, string) {},
			wantNotAdopted: true,
		},
		{
			name: "incomplete adoption",
			arrange: func(t *testing.T, root string) {
				t.Helper()
				mustMkdirAll(t, filepath.Join(root, ".verdi", "policy"))
			},
			wantErr: "incomplete adoption",
		},
		{
			name: "malformed authority",
			arrange: func(t *testing.T, root string) {
				t.Helper()
				mustWriteProfilePortFile(t, filepath.Join(root, ".verdi", "policy", "constitution.md"), "---\nschema: [not valid here]\n")
			},
			wantErr: "decoding constitution.md",
		},
		{
			name: "unavailable authority path",
			arrange: func(t *testing.T, root string) {
				t.Helper()
				mustWriteProfilePortFile(t, filepath.Join(root, ".verdi", "policy"), "not a directory")
			},
			wantErr: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			tt.arrange(t, root)

			_, err := NewProfileLoader().Load(context.Background(), root)
			if tt.wantNotAdopted {
				if !errors.Is(err, policyauthority.ErrNotAdopted) {
					t.Fatalf("ProfileLoader.Load error = %v, want errors.Is(err, policyauthority.ErrNotAdopted)", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ProfileLoader.Load error = %v, want containing %q", err, tt.wantErr)
			}
			if errors.Is(err, policyauthority.ErrNotAdopted) {
				t.Fatalf("ProfileLoader.Load error = %v, invalid authority must not be classified as genuine absence", err)
			}
		})
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%s): %v", path, err)
	}
}

func mustWriteProfilePortFile(t *testing.T, path, content string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%s): %v", path, err)
	}
}
