package workbench

// Task 1 of the extensibility-phase1 plan (audit CLEANUP-BEFORE #1):
// spliceSpec used a hand-rolled CreateTemp->Write->Close->Rename sequence
// with no fsync before the rename — the same crash-durability gap
// atomicfile.Write already closed for boardio/boardlayout/boarddiagram (see
// those packages' own doc comments). These tests pin the fixed behavior: no
// temp debris ever lands beside spec.md, the edited content lands exactly as
// requested, and the source no longer hand-rolls the primitive.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpliceSpecUsesAtomicWrite proves spliceSpec's write leaves no temp
// sibling behind and lands exactly the requested content, across several
// distinct card edits.
func TestSpliceSpecUsesAtomicWrite(t *testing.T) {
	tests := []struct {
		name string
		id   string
		text string
	}{
		{"edit an acceptance criterion", "ac-1", "a declined applicant sees the current reason [atomic-write-check]"},
		{"edit a constraint", "co-1", "notices never name internal model scores [atomic-write-check]"},
		{"edit a decision", "dc-2", "reuse the notification channel [atomic-write-check]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := newBoardFixture(t)
			h := NewHandler(root)

			body := `{"id":"` + tc.id + `","text":"` + tc.text + `"}`
			rec := postBoardAPI(t, h, boardFixtureName, "edit-text", body)
			if rec.Code != http.StatusOK {
				t.Fatalf("edit-text(%s) = %d, want 200\n%s", tc.id, rec.Code, rec.Body.String())
			}

			specDir := filepath.Join(root, ".verdi", "specs", "active", boardFixtureName)
			entries, err := os.ReadDir(specDir)
			if err != nil {
				t.Fatalf("ReadDir(%s): %v", specDir, err)
			}
			for _, e := range entries {
				if strings.Contains(e.Name(), ".tmp") {
					t.Fatalf("leftover temp file %s", e.Name())
				}
			}

			data, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
			if err != nil {
				t.Fatalf("reading spec.md: %v", err)
			}
			if !strings.Contains(string(data), tc.text) {
				t.Fatalf("spec.md does not contain the edited text %q:\n%s", tc.text, data)
			}
		})
	}
}

// TestBoardSpecAPI_AtomicWrite_NoDirectCreateTemp is a source-text witness
// (the pattern evidenceslotstatic_test.go already uses in this package):
// boardspecapi.go must route its spec write through atomicfile.Write, not a
// private CreateTemp->Rename sequence — a second hand-rolled copy that had
// drifted out of fsync parity with atomicfile's own, exactly as the audit
// found (CLEANUP-BEFORE #1).
func TestBoardSpecAPI_AtomicWrite_NoDirectCreateTemp(t *testing.T) {
	data, err := os.ReadFile("boardspecapi.go")
	if err != nil {
		t.Fatalf("reading boardspecapi.go: %v", err)
	}
	if strings.Contains(string(data), "os.CreateTemp") {
		t.Error("boardspecapi.go calls os.CreateTemp directly — spliceSpec must route through internal/atomicfile.Write instead (CLEANUP-BEFORE #1)")
	}
}
