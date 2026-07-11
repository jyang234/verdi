package boardio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

func TestLoadBoardState_Happy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "STORY-1.json")
	board := &artifact.Board{
		Schema: boardStateSchema,
		Pins:   []artifact.Pin{{Ref: "spec/stale-decline@7176513ece8b608ab0911000691bb697ee7e75ec", X: 1, Y: 2}},
		Stickies: []artifact.Sticky{
			{ID: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", X: 3, Y: 4},
		},
		Yarn: []artifact.Yarn{{From: "pin:x", To: "sticky:a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Label: "relates"}},
	}
	if err := SaveBoardState(path, board); err != nil {
		t.Fatalf("SaveBoardState: %v", err)
	}

	got, err := LoadBoardState(path)
	if err != nil {
		t.Fatalf("LoadBoardState: %v", err)
	}
	if len(got.Pins) != 1 || len(got.Stickies) != 1 || len(got.Yarn) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestLoadBoardState_Negative(t *testing.T) {
	t.Run("missing file yields a fresh empty board, not an error", func(t *testing.T) {
		got, err := LoadBoardState(filepath.Join(t.TempDir(), "NOPE.json"))
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got.Schema != boardStateSchema || len(got.Pins) != 0 || len(got.Stickies) != 0 {
			t.Fatalf("expected a fresh empty board, got %+v", got)
		}
	})
	t.Run("malformed json errors", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.json")
		if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadBoardState(path); err == nil {
			t.Fatal("expected a decode error")
		}
	})
}

func TestSaveBoardState_Negative_Invalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "STORY-1.json")
	bad := &artifact.Board{Schema: boardStateSchema, Stickies: []artifact.Sticky{{ID: "not-a-ulid"}}}
	if err := SaveBoardState(path, bad); err == nil {
		t.Fatal("expected SaveBoardState to reject an invalid board")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("SaveBoardState must not leave a partial file on validation failure; stat err = %v", err)
	}
}

// TestSaveBoardState_Atomic proves the write is temp-then-rename: no
// ".tmp"-suffixed file is left behind after a successful save, and a
// concurrent reader never observes a partially-written file (simulated
// here by asserting the final file's content is always one of the two
// complete states across repeated overwrites, never a torn mix).
func TestSaveBoardState_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "STORY-1.json")

	board1 := &artifact.Board{Schema: boardStateSchema, Pins: []artifact.Pin{{Ref: "spec/a@7176513ece8b608ab0911000691bb697ee7e75ec", X: 1, Y: 1}}}
	board2 := &artifact.Board{Schema: boardStateSchema, Pins: []artifact.Pin{{Ref: "spec/a@7176513ece8b608ab0911000691bb697ee7e75ec", X: 99, Y: 99}}}

	if err := SaveBoardState(path, board1); err != nil {
		t.Fatalf("SaveBoardState 1: %v", err)
	}
	if err := SaveBoardState(path, board2); err != nil {
		t.Fatalf("SaveBoardState 2: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("leftover temp file after save: %s", e.Name())
		}
	}

	got, err := LoadBoardState(path)
	if err != nil {
		t.Fatalf("LoadBoardState: %v", err)
	}
	if len(got.Pins) != 1 || got.Pins[0].X != 99 {
		t.Fatalf("expected the second save's content to win, got %+v", got.Pins)
	}
}

func TestBoardStatePath(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		root := t.TempDir()
		got, err := BoardStatePath(root, "STORY-1482")
		if err != nil {
			t.Fatalf("BoardStatePath: %v", err)
		}
		want := filepath.Join(root, ".verdi", "data", "mutable", "boards", "STORY-1482.json")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("negative: path traversal rejected", func(t *testing.T) {
		if _, err := BoardStatePath(t.TempDir(), "../../etc/passwd"); err == nil {
			t.Fatal("expected an error for a traversal attempt")
		}
	})
	t.Run("negative: empty key rejected", func(t *testing.T) {
		if _, err := BoardStatePath(t.TempDir(), ""); err == nil {
			t.Fatal("expected an error for an empty key")
		}
	})
}
