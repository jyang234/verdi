package draftmutation

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/store"
)

const transactionSpecName = "sample"

var (
	oldSpecBytes       = []byte("old spec bytes\n")
	newSpecBytes       = []byte("new spec bytes\n")
	oldProvenanceBytes = []byte("old provenance\n")
	newProvenanceBytes = []byte("old provenance\nnew provenance\n")
)

func transactionRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(store.SpecDir(root, store.ZoneActive, transactionSpecName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".verdi", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.SpecPath(root, store.ZoneActive, transactionSpecName), oldSpecBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.DesignProvenancePath(root, store.ZoneActive, transactionSpecName), oldProvenanceBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func runTransaction(t *testing.T, root string, coordinator Coordinator) error {
	t.Helper()
	return WithWriterLock(context.Background(), root, coordinator, func(writer *LockedWriter) error {
		if err := writer.Recover(transactionSpecName); err != nil {
			return err
		}
		return writer.Commit(Transaction{
			Spec: "spec/" + transactionSpecName, OldSpec: oldSpecBytes, NewSpec: newSpecBytes,
			OldProvenance: oldProvenanceBytes, NewProvenance: newProvenanceBytes,
		})
	})
}

func TestTransactionFaultMatrixNeverSpecAheadAndRecoversOldOrNew(t *testing.T) {
	probeRoot := transactionRoot(t)
	var steps []string
	if err := runTransaction(t, probeRoot, Coordinator{After: func(step string) error {
		steps = append(steps, step)
		return nil
	}}); err != nil {
		t.Fatalf("probe transaction: %v", err)
	}
	if len(steps) < 10 {
		t.Fatalf("durability steps = %v, want every write/fsync/rename/cleanup boundary", steps)
	}

	for _, failStep := range steps {
		t.Run(failStep, func(t *testing.T) {
			root := transactionRoot(t)
			failed := false
			err := runTransaction(t, root, Coordinator{After: func(step string) error {
				if step == failStep && !failed {
					failed = true
					return errors.New("injected crash")
				}
				return nil
			}})
			if err == nil || !strings.Contains(err.Error(), "injected crash") {
				t.Fatalf("transaction error = %v", err)
			}
			assertNeverSpecAhead(t, root)
			if err := WithWriterLock(context.Background(), root, Coordinator{}, func(writer *LockedWriter) error {
				return writer.Recover(transactionSpecName)
			}); err != nil {
				t.Fatalf("Recover: %v", err)
			}
			assertCompleteOldOrNew(t, root)
			if _, err := os.Stat(store.DraftMutationDir(root, transactionSpecName)); !os.IsNotExist(err) {
				t.Fatalf("transaction root remains after recovery: %v", err)
			}
		})
	}
}

func TestTransactionSyncsEachNewDirectoryEntryBeforeJournal(t *testing.T) {
	tests := []struct {
		name        string
		removeData  bool
		wantCreates []string
	}{
		{
			name: "transaction directories",
			wantCreates: []string{
				"created-directory-parent-fsync:.verdi/data/draft-mutation",
				"created-directory-parent-fsync:.verdi/data/draft-mutation/sample",
			},
		},
		{
			name:       "missing data ancestor",
			removeData: true,
			wantCreates: []string{
				"created-directory-parent-fsync:.verdi/data",
				"created-directory-parent-fsync:.verdi/data/draft-mutation",
				"created-directory-parent-fsync:.verdi/data/draft-mutation/sample",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newRoot := func(t *testing.T) string {
				t.Helper()
				root := transactionRoot(t)
				if tt.removeData {
					if err := os.Remove(filepath.Join(root, ".verdi", "data")); err != nil {
						t.Fatal(err)
					}
				}
				return root
			}
			root := newRoot(t)
			var steps []string
			if err := runTransaction(t, root, Coordinator{After: func(step string) error {
				steps = append(steps, step)
				return nil
			}}); err != nil {
				t.Fatalf("probe transaction: %v", err)
			}
			if len(steps) < len(tt.wantCreates)+1 || !reflect.DeepEqual(steps[:len(tt.wantCreates)], tt.wantCreates) || steps[len(tt.wantCreates)] != StepJournalWrite {
				t.Fatalf("durability order = %v, want %v before %q", steps, tt.wantCreates, StepJournalWrite)
			}

			for _, failStep := range tt.wantCreates {
				t.Run(failStep, func(t *testing.T) {
					root := newRoot(t)
					stop := errors.New("injected directory crash")
					err := runTransaction(t, root, Coordinator{After: func(step string) error {
						if step == failStep {
							return stop
						}
						return nil
					}})
					if !errors.Is(err, stop) {
						t.Fatalf("transaction error = %v, want injected crash", err)
					}
					if _, err := os.Stat(store.DraftMutationJournalPath(root, transactionSpecName)); !os.IsNotExist(err) {
						t.Fatalf("journal became reachable before directory durability: %v", err)
					}
					if err := runTransaction(t, root, Coordinator{}); err != nil {
						t.Fatalf("retry after directory crash: %v", err)
					}
					assertCompleteOldOrNew(t, root)
				})
			}
		})
	}
}

func assertNeverSpecAhead(t *testing.T, root string) {
	t.Helper()
	spec, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, transactionSpecName))
	provenance, _ := os.ReadFile(store.DesignProvenancePath(root, store.ZoneActive, transactionSpecName))
	if string(spec) == string(newSpecBytes) && string(provenance) != string(newProvenanceBytes) {
		t.Fatalf("reachable spec-ahead state: spec=%q provenance=%q", spec, provenance)
	}
}

func assertCompleteOldOrNew(t *testing.T, root string) {
	t.Helper()
	spec, _ := os.ReadFile(store.SpecPath(root, store.ZoneActive, transactionSpecName))
	provenance, _ := os.ReadFile(store.DesignProvenancePath(root, store.ZoneActive, transactionSpecName))
	oldPair := string(spec) == string(oldSpecBytes) && string(provenance) == string(oldProvenanceBytes)
	newPair := string(spec) == string(newSpecBytes) && string(provenance) == string(newProvenanceBytes)
	if !oldPair && !newPair {
		t.Fatalf("state is neither complete old nor complete new: spec=%q provenance=%q", spec, provenance)
	}
}

func TestRecoveryMalformedTamperedAndSymlinkStateIsRetained(t *testing.T) {
	t.Run("malformed journal retained", func(t *testing.T) {
		root := transactionRoot(t)
		journal := store.DraftMutationJournalPath(root, transactionSpecName)
		if err := os.MkdirAll(filepath.Dir(journal), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(journal, []byte(`{"schema":"unknown"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		err := WithWriterLock(context.Background(), root, Coordinator{}, func(writer *LockedWriter) error { return writer.Recover(transactionSpecName) })
		if err == nil || !strings.Contains(err.Error(), "journal") {
			t.Fatalf("Recover error = %v", err)
		}
		if _, statErr := os.Stat(journal); statErr != nil {
			t.Fatalf("malformed journal was not retained: %v", statErr)
		}
	})

	t.Run("tampered stage retained", func(t *testing.T) {
		root := transactionRoot(t)
		stop := errors.New("stop with staged files")
		err := runTransaction(t, root, Coordinator{After: func(step string) error {
			if step == StepStageDirectorySync {
				return stop
			}
			return nil
		}})
		if !errors.Is(err, stop) {
			t.Fatalf("transaction error = %v", err)
		}
		if err := os.WriteFile(store.DraftMutationSpecStagePath(root, transactionSpecName), []byte("tampered"), 0o644); err != nil {
			t.Fatal(err)
		}
		err = WithWriterLock(context.Background(), root, Coordinator{}, func(writer *LockedWriter) error { return writer.Recover(transactionSpecName) })
		if err == nil || !strings.Contains(err.Error(), "digest") {
			t.Fatalf("Recover error = %v", err)
		}
		if _, statErr := os.Stat(store.DraftMutationJournalPath(root, transactionSpecName)); statErr != nil {
			t.Fatalf("tampered recovery journal not retained: %v", statErr)
		}
	})

	for _, target := range []string{"transaction root", "transaction parent", "spec", "sidecar", "spec parent", "writer lock"} {
		t.Run("symlink "+target, func(t *testing.T) {
			root := transactionRoot(t)
			outside := t.TempDir()
			switch target {
			case "transaction root":
				if err := os.MkdirAll(filepath.Dir(store.DraftMutationDir(root, transactionSpecName)), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, store.DraftMutationDir(root, transactionSpecName)); err != nil {
					t.Fatal(err)
				}
			case "transaction parent":
				if err := os.Mkdir(filepath.Join(outside, transactionSpecName), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Dir(store.DraftMutationDir(root, transactionSpecName))); err != nil {
					t.Fatal(err)
				}
			case "spec":
				path := store.SpecPath(root, store.ZoneActive, transactionSpecName)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, "spec.md"), path); err != nil {
					t.Fatal(err)
				}
			case "sidecar":
				path := store.DesignProvenancePath(root, store.ZoneActive, transactionSpecName)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, "design-provenance.jsonl"), path); err != nil {
					t.Fatal(err)
				}
			case "writer lock":
				outsideLock := filepath.Join(outside, "writer.lock")
				lockBytes, err := json.Marshal(filelock.Info{PID: 999999999, Start: 1})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(outsideLock, lockBytes, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outsideLock, store.WriterLockPath(root)); err != nil {
					t.Fatal(err)
				}
			case "spec parent":
				specDir := store.SpecDir(root, store.ZoneActive, transactionSpecName)
				if err := os.RemoveAll(specDir); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(outside, "spec.md"), oldSpecBytes, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(outside, "design-provenance.jsonl"), oldProvenanceBytes, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, specDir); err != nil {
					t.Fatal(err)
				}
			}
			if err := runTransaction(t, root, Coordinator{}); err == nil || !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("transaction error = %v", err)
			}
			if target == "transaction parent" {
				if _, err := os.Stat(filepath.Join(outside, transactionSpecName)); err != nil {
					t.Fatalf("recovery traversed and removed outside transaction root: %v", err)
				}
			}
			if target == "writer lock" {
				info, err := os.Lstat(store.WriterLockPath(root))
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("writer lock symlink was not retained: info=%v err=%v", info, err)
				}
			}
		})
	}
}

func TestTransactionFirstProvenanceAndRecoveryCleanupFault(t *testing.T) {
	t.Run("first provenance", func(t *testing.T) {
		root := transactionRoot(t)
		if err := os.Remove(store.DesignProvenancePath(root, store.ZoneActive, transactionSpecName)); err != nil {
			t.Fatal(err)
		}
		err := WithWriterLock(context.Background(), root, Coordinator{}, func(writer *LockedWriter) error {
			return writer.Commit(Transaction{Spec: "spec/sample", OldSpec: oldSpecBytes, NewSpec: newSpecBytes, NewProvenance: []byte("first provenance\n")})
		})
		if err != nil {
			t.Fatalf("Commit: %v", err)
		}
		got, err := os.ReadFile(store.DesignProvenancePath(root, store.ZoneActive, transactionSpecName))
		if err != nil || string(got) != "first provenance\n" {
			t.Fatalf("provenance = %q, %v", got, err)
		}
	})

	t.Run("rollback cleanup fault", func(t *testing.T) {
		root := transactionRoot(t)
		stop := errors.New("stop before provenance stage")
		if err := runTransaction(t, root, Coordinator{After: func(step string) error {
			if step == StepSpecStageSync {
				return stop
			}
			return nil
		}}); !errors.Is(err, stop) {
			t.Fatalf("preparing interrupted transaction: %v", err)
		}
		cleanupStop := errors.New("stop during stage cleanup")
		err := WithWriterLock(context.Background(), root, Coordinator{After: func(step string) error {
			if step == StepCleanupStage {
				return cleanupStop
			}
			return nil
		}}, func(writer *LockedWriter) error { return writer.Recover(transactionSpecName) })
		if !errors.Is(err, cleanupStop) {
			t.Fatalf("cleanup fault = %v", err)
		}
		assertCompleteOldOrNew(t, root)
		if err := WithWriterLock(context.Background(), root, Coordinator{}, func(writer *LockedWriter) error { return writer.Recover(transactionSpecName) }); err != nil {
			t.Fatalf("second recovery: %v", err)
		}
	})
}

func TestConcurrentGlobalLockContentionStaleTakeoverAndNoPerSpecLock(t *testing.T) {
	root := transactionRoot(t)
	lockPath := store.WriterLockPath(root)
	lock, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	err = WithWriterLock(context.Background(), root, Coordinator{}, func(*LockedWriter) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "held") {
		t.Fatalf("WithWriterLock contention error = %v", err)
	}
	if err := filelock.Release(lock, lockPath); err != nil {
		t.Fatal(err)
	}

	stale, _ := json.Marshal(filelock.Info{PID: 99999999, Start: 1})
	if err := os.WriteFile(lockPath, append(stale, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WithWriterLock(context.Background(), root, Coordinator{}, func(*LockedWriter) error { return nil }); err != nil {
		t.Fatalf("stale global-lock takeover: %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("global lock remains after release: %v", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(store.SpecDir(root, store.ZoneActive, transactionSpecName), "*.lock")); len(matches) != 0 {
		t.Fatalf("per-spec locks created: %v", matches)
	}
}

func TestConcurrentProcessHelper(t *testing.T) {
	root := os.Getenv("VERDI_DRAFTMUTATION_HELPER_ROOT")
	if root == "" {
		return
	}
	ready := filepath.Join(root, "ready")
	release := filepath.Join(root, "release")
	err := WithWriterLock(context.Background(), root, Coordinator{}, func(*LockedWriter) error {
		if err := os.WriteFile(ready, []byte("ready"), 0o644); err != nil {
			return err
		}
		for {
			if _, err := os.Stat(release); err == nil {
				return nil
			}
			time.Sleep(5 * time.Millisecond)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentProcessesSerializeOnGlobalLock(t *testing.T) {
	root := transactionRoot(t)
	command := exec.Command(os.Args[0], "-test.run=^TestConcurrentProcessHelper$")
	command.Env = append(os.Environ(), "VERDI_DRAFTMUTATION_HELPER_ROOT="+root)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill() }()
	ready := filepath.Join(root, "ready")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not acquire global lock")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if err := WithWriterLock(context.Background(), root, Coordinator{}, func(*LockedWriter) error { return nil }); err == nil {
		t.Fatal("second process acquired the global writer lock")
	}
	if err := os.WriteFile(filepath.Join(root, "release"), []byte("release"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("child: %v", err)
	}
}
