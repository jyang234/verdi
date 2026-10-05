package pollwitness

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/workbench"
)

// TestPollWitness_NoIndexWrite is Wave 6 §5.3's "no mutation" over the
// wall's poll (BL-105; ledger SI-343 (3), SI-352): after a tracked spec's
// mtime moves with its bytes unchanged — the stat drift a plain `git
// status` refreshes and writes back into .git/index — the wall's snapshot
// poll, the readiness page, the Document tab, a readiness load and the
// wall badges leave the served checkout's index byte for byte as it was.
func TestPollWitness_NoIndexWrite(t *testing.T) {
	repo := buildE2EStore(t)
	const name = "stale-decline"
	spec := filepath.Join(repo.Dir, ".verdi", "specs", "active", name, "spec.md")
	stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(spec, stale, stale); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(repo.Dir, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}

	h := workbench.NewHandlerWith(repo.Dir, workbench.Deps{ReadinessLoader: newLoader(repo.Dir)})
	for _, path := range []string{"/board/spec/" + name + "/snapshot", "/readiness?spec=" + name, "/board/spec/" + name + "/document?format=md"} {
		get(t, h, path)
	}
	if _, err := newLoader(repo.Dir).Load(context.Background(), "spec/"+name); err != nil {
		t.Fatalf("readiness load: %v", err)
	}
	badges(t, repo.Dir, name)

	after, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("serving the wall rewrote .git/index: a read path took git's optional index lock")
	}
}
