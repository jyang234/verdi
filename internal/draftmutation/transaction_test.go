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
	"sync"
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

func TestTransactionSyncsDirectoryEntryParentsBeforeJournal(t *testing.T) {
	tests := []struct {
		name              string
		removeData        bool
		wantBeforeJournal []string
		failSteps         []string
	}{
		{
			name: "transaction directories",
			wantBeforeJournal: []string{
				"directory-parent-fsync:.verdi",
				"directory-parent-fsync:.verdi/data",
				"directory-parent-fsync:.verdi",
				"directory-parent-fsync:.verdi/data",
				"directory-parent-fsync:.verdi/data/draft-mutation",
				"directory-parent-fsync:.verdi/data/draft-mutation/sample",
			},
			failSteps: []string{
				"directory-parent-fsync:.verdi/data/draft-mutation",
				"directory-parent-fsync:.verdi/data/draft-mutation/sample",
			},
		},
		{
			name:       "missing data ancestor",
			removeData: true,
			wantBeforeJournal: []string{
				"directory-parent-fsync:.verdi",
				"directory-parent-fsync:.verdi/data",
				"directory-parent-fsync:.verdi",
				"directory-parent-fsync:.verdi/data",
				"directory-parent-fsync:.verdi/data/draft-mutation",
				"directory-parent-fsync:.verdi/data/draft-mutation/sample",
			},
			failSteps: []string{
				"directory-parent-fsync:.verdi/data",
				"directory-parent-fsync:.verdi/data/draft-mutation",
				"directory-parent-fsync:.verdi/data/draft-mutation/sample",
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
			if len(steps) < len(tt.wantBeforeJournal)+1 || !reflect.DeepEqual(steps[:len(tt.wantBeforeJournal)], tt.wantBeforeJournal) || steps[len(tt.wantBeforeJournal)] != StepJournalWrite {
				t.Fatalf("durability order = %v, want %v before %q", steps, tt.wantBeforeJournal, StepJournalWrite)
			}

			for _, failStep := range tt.failSteps {
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

func TestTransactionRetriesVisibleDirectoryParentSyncBeforeJournal(t *testing.T) {
	root := transactionRoot(t)
	parent := filepath.Join(root, ".verdi", "data")
	created := filepath.Join(parent, "draft-mutation")
	journalPath := store.DraftMutationJournalPath(root, transactionSpecName)
	stop := errors.New("injected directory sync failure")
	syncAttempts := 0
	retrySyncBeforeJournal := false
	coordinator := Coordinator{DirectorySync: func(directory *os.File) error {
		if directory.Name() != parent {
			return directory.Sync()
		}
		syncAttempts++
		if syncAttempts == 1 {
			return stop
		}
		if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("journal exists before retried parent sync: %v", err)
		}
		retrySyncBeforeJournal = true
		return directory.Sync()
	}}

	if err := runTransaction(t, root, coordinator); !errors.Is(err, stop) {
		t.Fatalf("first transaction error = %v, want directory sync failure", err)
	}
	info, err := os.Lstat(created)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("created directory is not visibly retained: info=%v err=%v", info, err)
	}
	if _, err := os.Stat(journalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal became reachable after failed parent sync: %v", err)
	}

	if err := runTransaction(t, root, coordinator); err != nil {
		t.Fatalf("retry transaction: %v", err)
	}
	if syncAttempts != 2 || !retrySyncBeforeJournal {
		t.Fatalf("parent sync attempts = %d, before journal = %t; want two attempts with retry before journal", syncAttempts, retrySyncBeforeJournal)
	}
	assertCompleteOldOrNew(t, root)
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
	// SI-177: this process already owns the exact outer lock (acquired
	// directly, above, exactly as `verdi serve`'s lifetime lock would be),
	// so WithWriterLock now PROVES that ownership via the registry and
	// reuses the outer exclusion rather than failing — the dedicated,
	// thorough proof of this lives in
	// TestWithWriterLockReusesCurrentProcessHolder; this call only checks
	// that reuse does not disturb the surrounding contention/takeover
	// scenario this test otherwise covers.
	if err := WithWriterLock(context.Background(), root, Coordinator{}, func(*LockedWriter) error { return nil }); err != nil {
		t.Fatalf("WithWriterLock reuse of our own outer lock = %v, want nil", err)
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

// TestWithWriterLockReusesCurrentProcessHolder is Task 1B's required
// semantic RED (design §6.1.2 item 4, ledger SI-177): a process that
// already owns the checkout's outer writer lock (acquired directly here,
// exactly as `verdi serve`'s lifetime lock is) calls WithWriterLock on the
// SAME checkout. Base: filelock.Acquire's ordinary path returns ErrHeld
// with nothing further, so WithWriterLock fails and the callback never
// runs. GREEN: WithWriterLock proves ownership via
// filelock.HeldByCurrentProcess, runs the callback under the reused outer
// exclusion, and leaves the exact same outer lock file present and
// registry-proven owned afterward — it is never released or replaced by
// the inner reuse.
func TestWithWriterLockReusesCurrentProcessHolder(t *testing.T) {
	root := transactionRoot(t)
	lockPath := store.WriterLockPath(root)
	lock, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatalf("Acquire (simulating serve's lifetime lock): %v", err)
	}
	defer func() { _ = filelock.Release(lock, lockPath) }()

	before, err := os.Stat(lockPath)
	if err != nil {
		t.Fatal(err)
	}

	callbackRan := false
	err = WithWriterLock(context.Background(), root, Coordinator{}, func(*LockedWriter) error {
		callbackRan = true
		held, herr := filelock.HeldByCurrentProcess(lockPath)
		if herr != nil || !held {
			t.Fatalf("HeldByCurrentProcess during reuse = %t, %v, want true, nil", held, herr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithWriterLock(reentrant reuse) = %v, want nil (the outer lock is already ours)", err)
	}
	if !callbackRan {
		t.Fatal("WithWriterLock(reentrant reuse) did not run the callback")
	}

	after, statErr := os.Stat(lockPath)
	if statErr != nil {
		t.Fatalf("outer lock is gone after reuse: %v", statErr)
	}
	if !os.SameFile(before, after) {
		t.Fatal("outer lock file identity changed across reuse — it was released and recreated rather than reused")
	}
	held, herr := filelock.HeldByCurrentProcess(lockPath)
	if herr != nil || !held {
		t.Fatalf("HeldByCurrentProcess after reuse returns = %t, %v, want true, nil (the outer lock must remain owned)", held, herr)
	}
}

// TestWithWriterLockSerializesConcurrentReuseUnderOneOuterLock proves the
// per-checkout in-process transaction mutex: two concurrent WithWriterLock
// calls reusing the SAME already-held outer lock never run their
// callbacks at the same time, and complete in a deterministic,
// test-controlled order.
func TestWithWriterLockSerializesConcurrentReuseUnderOneOuterLock(t *testing.T) {
	root := transactionRoot(t)
	lockPath := store.WriterLockPath(root)
	lock, err := filelock.Acquire(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = filelock.Release(lock, lockPath) }()

	var mu sync.Mutex
	var order []string
	record := func(event string) {
		mu.Lock()
		order = append(order, event)
		mu.Unlock()
	}

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	first := make(chan error, 1)
	go func() {
		first <- WithWriterLock(context.Background(), root, Coordinator{}, func(*LockedWriter) error {
			record("first-enter")
			close(firstEntered)
			<-releaseFirst
			record("first-exit")
			return nil
		})
	}()
	select {
	case <-firstEntered:
	case err := <-first:
		t.Fatalf("first WithWriterLock returned before entering its callback: %v (want it to reuse the already-held outer lock)", err)
	case <-time.After(5 * time.Second):
		t.Fatal("first WithWriterLock never entered its callback")
	}

	secondEntered := make(chan struct{})
	second := make(chan error, 1)
	go func() {
		second <- WithWriterLock(context.Background(), root, Coordinator{}, func(*LockedWriter) error {
			record("second-enter")
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second WithWriterLock entered its callback while the first still held the mutex — callbacks overlapped")
	case <-time.After(150 * time.Millisecond):
	}

	close(releaseFirst)
	if err := <-first; err != nil {
		t.Fatalf("first WithWriterLock: %v", err)
	}
	select {
	case <-secondEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("second WithWriterLock never entered its callback after the first released the mutex")
	}
	if err := <-second; err != nil {
		t.Fatalf("second WithWriterLock: %v", err)
	}

	mu.Lock()
	got := append([]string(nil), order...)
	mu.Unlock()
	want := []string{"first-enter", "first-exit", "second-enter"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("callback order = %v, want %v (no overlap, deterministic order)", got, want)
	}

	if held, herr := filelock.HeldByCurrentProcess(lockPath); herr != nil || !held {
		t.Fatalf("HeldByCurrentProcess after both reuses = %t, %v, want true, nil", held, herr)
	}
}

// TestWithWriterLockDifferentCheckoutsAreNotGloballySerialized proves the
// mutex is scoped per validated checkout writer-lock path: two DIFFERENT
// checkouts, each already locked by this process, run their reused
// WithWriterLock callbacks CONCURRENTLY rather than blocking on each
// other.
func TestWithWriterLockDifferentCheckoutsAreNotGloballySerialized(t *testing.T) {
	rootA := transactionRoot(t)
	rootB := transactionRoot(t)
	lockPathA, lockPathB := store.WriterLockPath(rootA), store.WriterLockPath(rootB)

	lockA, err := filelock.Acquire(lockPathA)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = filelock.Release(lockA, lockPathA) }()
	lockB, err := filelock.Acquire(lockPathB)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = filelock.Release(lockB, lockPathB) }()

	aEntered := make(chan struct{})
	bEntered := make(chan struct{})
	release := make(chan struct{})
	errA := make(chan error, 1)
	errB := make(chan error, 1)

	go func() {
		errA <- WithWriterLock(context.Background(), rootA, Coordinator{}, func(*LockedWriter) error {
			close(aEntered)
			<-release
			return nil
		})
	}()
	go func() {
		errB <- WithWriterLock(context.Background(), rootB, Coordinator{}, func(*LockedWriter) error {
			close(bEntered)
			<-release
			return nil
		})
	}()

	waitingFor := map[string]bool{"a": true, "b": true}
	timeout := time.After(5 * time.Second)
	for len(waitingFor) > 0 {
		select {
		case <-aEntered:
			delete(waitingFor, "a")
			aEntered = nil
		case <-bEntered:
			delete(waitingFor, "b")
			bEntered = nil
		case <-timeout:
			t.Fatalf("different checkouts appear globally serialized: still waiting for %v to enter its callback", waitingFor)
		}
	}
	close(release)
	if err := <-errA; err != nil {
		t.Fatalf("checkout A: %v", err)
	}
	if err := <-errB; err != nil {
		t.Fatalf("checkout B: %v", err)
	}
}

// TestWithWriterLockForgedLockWithoutRegistryEntryRefuses is SI-177's
// forged-lock RED family: a lock file naming this process's own real
// pid/start bytes, written directly to disk (never through this
// process's own successful filelock.Acquire), carries no registry entry
// — HeldByCurrentProcess must report false, and WithWriterLock must stay
// refused exactly as it would for any other live foreign holder, with
// zero journal/spec/provenance effects and the forged lock file left
// completely untouched (never treated as stale, since it does name a
// genuinely live pid).
func TestWithWriterLockForgedLockWithoutRegistryEntryRefuses(t *testing.T) {
	root := transactionRoot(t)
	lockPath := store.WriterLockPath(root)
	forged, err := json.Marshal(filelock.Info{PID: os.Getpid(), Start: time.Now().Unix()})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, forged, 0o644); err != nil {
		t.Fatalf("seeding forged lock file: %v", err)
	}

	err = runTransaction(t, root, Coordinator{})
	if err == nil || !strings.Contains(err.Error(), "held") {
		t.Fatalf("WithWriterLock(forged lock, no registry entry) error = %v, want a held refusal", err)
	}

	if _, statErr := os.Stat(store.DraftMutationDir(root, transactionSpecName)); !os.IsNotExist(statErr) {
		t.Fatalf("journal/staging effects leaked from a refused forged-lock transaction: stat err=%v", statErr)
	}
	if spec, rerr := os.ReadFile(store.SpecPath(root, store.ZoneActive, transactionSpecName)); rerr != nil || string(spec) != string(oldSpecBytes) {
		t.Fatalf("spec bytes mutated despite refused forged-lock transaction: %q, %v", spec, rerr)
	}
	if provenance, rerr := os.ReadFile(store.DesignProvenancePath(root, store.ZoneActive, transactionSpecName)); rerr != nil || string(provenance) != string(oldProvenanceBytes) {
		t.Fatalf("provenance mutated despite refused forged-lock transaction: %q, %v", provenance, rerr)
	}
	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Fatalf("forged lock file was removed (a genuinely live pid must never be treated as stale): %v", statErr)
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
