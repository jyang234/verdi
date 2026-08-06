package execworkspace

// Contract item 2: a programmatic totality matrix proving spec/
// execution-workspace §Workspace naming's own totality clause — "every
// reachable combination of {unit path, .request, .request.staging,
// .released, .lock, registry entry} reaches exactly one outcome" — holds
// for gc.GC across the full state space, never panicking, never leaving a
// unit undecided, and never mutating anything outside the unit under
// scrutiny.
//
// Fixtures reuse this package's own established helpers
// (newReconcileTestRepo/adminWorktreesDir/hashAdminDir/scanAdminDir/
// canonicalPath from reconcile_test.go/reconcile.go, writeDeadPIDLock/
// mustPathKind from gc_test.go, buildTestRepo from materialize_test.go) —
// no copy-pasted fixture logic.
//
// STRATEGY (documented per the contract): the six dimensions are hand-
// crafted directly on disk rather than driven through real `git worktree
// add`/`git worktree prune` calls for every combination. GC's own
// classification (decideUnit, gc.go) reads only raw lstat kinds plus
// gitx.StatusDirty; GitReconciler's own registry scan (reconcile.go,
// resolveAdminEntry/scanAdminDir) reads only the raw bytes of a
// `gitdir`/`gitdir.reconciling` record — neither validates that git itself
// considers an entry legitimate. So a "registered" unit path is
// constructed by writing that record directly, and a "real directory that
// looks like a git worktree" is constructed as a directory holding a
// syntactically worktree-shaped but non-functional `.git` file (its own
// linked-worktree-ness is exercised for real, end-to-end, by
// lifecycle_test.go's Materializer + real GitReconciler run — this matrix
// is proving TOTALITY of the decision, not git plumbing). This keeps every
// combination's cost to a handful of filesystem writes plus GC's own two
// unavoidable subprocess calls (gitx.CommonDir always, gitx.StatusDirty
// only when the combination reaches rank 3), letting the whole matrix run
// SERIALLY, sharing one git repository (repo.Dir) as gc's repoRoot for
// every combination (a fresh git repository per combination was measured
// unnecessary and would dominate runtime). Serial execution is deliberate,
// not merely default: distinct combinations share ONE physical
// $GIT_COMMON_DIR/worktrees/ administrative directory (since they share
// repo.Dir), so a registry entry one combination plants is visible to
// EVERY concurrent GC scan against that same repoRoot — running
// combinations in parallel would let one combination's hand-planted
// registry entry pollute another's scan. Each combination that plants an
// entry removes it again before the next combination runs, keeping the
// shared administrative directory's size bounded across the whole matrix.
import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- dimension 1: unit path ---

type mxUnitPath int

const (
	mxUnitAbsent mxUnitPath = iota
	// mxUnitDirWorktree is a real directory holding a syntactically
	// worktree-shaped (but non-functional, resolving nowhere real) `.git`
	// FILE — a linked worktree's `.git` is always a file, never a
	// directory, so LstatType still reports PathDir at the unit path
	// itself; only gitx.StatusDirty (which shells into it) can tell it
	// apart from a functioning worktree, and does, failing closed.
	mxUnitDirWorktree
	mxUnitDirPlain
	mxUnitRegularFile
	mxUnitSymlink
)

var mxUnitPathLabels = [...]string{"absent", "dir-worktree-shaped", "dir-plain", "regular-file", "symlink"}

func (d mxUnitPath) String() string { return mxUnitPathLabels[d] }

var allMxUnitPaths = []mxUnitPath{mxUnitAbsent, mxUnitDirWorktree, mxUnitDirPlain, mxUnitRegularFile, mxUnitSymlink}

func mxConstructUnitPath(t *testing.T, path, tmpBase string, d mxUnitPath) (ok bool, skipReason string) {
	t.Helper()
	switch d {
	case mxUnitAbsent:
		return true, ""
	case mxUnitDirWorktree:
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir unit path (dir-worktree-shaped): %v", err)
		}
		if err := os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: /nonexistent-admin-entry-for-matrix-fixture/.git\n"), 0o644); err != nil {
			t.Fatalf("planting worktree-shaped .git file: %v", err)
		}
		return true, ""
	case mxUnitDirPlain:
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir unit path (dir-plain): %v", err)
		}
		return true, ""
	case mxUnitRegularFile:
		if err := os.WriteFile(path, []byte("regular file at unit path"), 0o644); err != nil {
			t.Fatalf("planting regular file at unit path: %v", err)
		}
		return true, ""
	case mxUnitSymlink:
		target := filepath.Join(tmpBase, "unit-symlink-target")
		if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
			t.Fatalf("writing symlink target: %v", err)
		}
		if err := os.Symlink(target, path); err != nil {
			return false, fmt.Sprintf("symlink unsupported in this environment: %v", err)
		}
		return true, ""
	}
	return false, "unreachable dimension value"
}

// --- dimension 2: .request ---

type mxRequest int

const (
	mxReqAbsent mxRequest = iota
	mxReqValidWitness
	mxReqGarbageBytes
	mxReqSymlink
)

var mxRequestLabels = [...]string{"absent", "valid-witness", "garbage-bytes", "symlink"}

func (d mxRequest) String() string { return mxRequestLabels[d] }

var allMxRequests = []mxRequest{mxReqAbsent, mxReqValidWitness, mxReqGarbageBytes, mxReqSymlink}

func mxConstructRequest(t *testing.T, path, tmpBase string, d mxRequest) (ok bool, skipReason string) {
	t.Helper()
	switch d {
	case mxReqAbsent:
		return true, ""
	case mxReqValidWitness:
		id, err := NewExactIdentity("matrix-fixture-run", strings.Repeat("a", 40))
		if err != nil {
			t.Fatalf("NewExactIdentity (witness fixture): %v", err)
		}
		data, eerr := EncodeSidecar(id)
		if eerr != nil {
			t.Fatalf("EncodeSidecar (witness fixture): %v", eerr)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("writing valid witness: %v", err)
		}
		return true, ""
	case mxReqGarbageBytes:
		if err := os.WriteFile(path, []byte("not json garbage \x00\xff\xfe"), 0o644); err != nil {
			t.Fatalf("writing garbage .request: %v", err)
		}
		return true, ""
	case mxReqSymlink:
		target := filepath.Join(tmpBase, "request-symlink-target")
		if err := os.WriteFile(target, []byte("t"), 0o644); err != nil {
			t.Fatalf("writing symlink target: %v", err)
		}
		if err := os.Symlink(target, path); err != nil {
			return false, fmt.Sprintf("symlink unsupported in this environment: %v", err)
		}
		return true, ""
	}
	return false, "unreachable dimension value"
}

// --- dimension 3: .request.staging ---

type mxStaging int

const (
	mxStagingAbsent mxStaging = iota
	mxStagingRegularResidue
)

var mxStagingLabels = [...]string{"absent", "regular-residue"}

func (d mxStaging) String() string { return mxStagingLabels[d] }

var allMxStagings = []mxStaging{mxStagingAbsent, mxStagingRegularResidue}

func mxConstructStaging(t *testing.T, path string, d mxStaging) {
	t.Helper()
	switch d {
	case mxStagingAbsent:
	case mxStagingRegularResidue:
		if err := os.WriteFile(path, []byte("stale staging residue"), 0o644); err != nil {
			t.Fatalf("writing staging residue: %v", err)
		}
	}
}

// --- dimension 4: .released ---

type mxReleased int

const (
	mxRelAbsent mxReleased = iota
	mxRelEmptyRegular
	mxRelNonEmptyRegular
	mxRelDirectory
	mxRelSymlink
)

var mxReleasedLabels = [...]string{"absent", "empty-regular", "nonempty-regular", "directory", "symlink"}

func (d mxReleased) String() string { return mxReleasedLabels[d] }

var allMxReleaseds = []mxReleased{mxRelAbsent, mxRelEmptyRegular, mxRelNonEmptyRegular, mxRelDirectory, mxRelSymlink}

func mxConstructReleased(t *testing.T, path, tmpBase string, d mxReleased) (ok bool, skipReason string) {
	t.Helper()
	switch d {
	case mxRelAbsent:
		return true, ""
	case mxRelEmptyRegular:
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("writing empty .released: %v", err)
		}
		return true, ""
	case mxRelNonEmptyRegular:
		if err := os.WriteFile(path, []byte("content ignored — existence is the record"), 0o644); err != nil {
			t.Fatalf("writing nonempty .released: %v", err)
		}
		return true, ""
	case mxRelDirectory:
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir .released: %v", err)
		}
		return true, ""
	case mxRelSymlink:
		target := filepath.Join(tmpBase, "released-symlink-target")
		if err := os.WriteFile(target, []byte("t"), 0o644); err != nil {
			t.Fatalf("writing symlink target: %v", err)
		}
		if err := os.Symlink(target, path); err != nil {
			return false, fmt.Sprintf("symlink unsupported in this environment: %v", err)
		}
		return true, ""
	}
	return false, "unreachable dimension value"
}

// --- dimension 5: .lock ---

type mxLock int

const (
	mxLockAbsent mxLock = iota
	mxLockStaleDeadPID
	mxLockGarbageBody
)

var mxLockLabels = [...]string{"absent", "stale-dead-pid", "garbage-body"}

func (d mxLock) String() string { return mxLockLabels[d] }

var allMxLocks = []mxLock{mxLockAbsent, mxLockStaleDeadPID, mxLockGarbageBody}

func mxConstructLock(t *testing.T, path string, d mxLock, deadPIDBytes []byte) {
	t.Helper()
	switch d {
	case mxLockAbsent:
	case mxLockStaleDeadPID:
		if err := os.WriteFile(path, deadPIDBytes, 0o644); err != nil {
			t.Fatalf("writing stale dead-pid lock: %v", err)
		}
	case mxLockGarbageBody:
		if err := os.WriteFile(path, []byte("not-json-garbage"), 0o644); err != nil {
			t.Fatalf("writing garbage lock body: %v", err)
		}
	}
}

// --- dimension 6: registry entry ---

type mxRegistry int

const (
	mxRegNone mxRegistry = iota
	mxRegValid
	mxRegStaleDirGone
	mxRegCrashedAside
)

var mxRegistryLabels = [...]string{"none", "valid", "stale-dir-gone", "crashed-aside"}

func (d mxRegistry) String() string { return mxRegistryLabels[d] }

var allMxRegistries = []mxRegistry{mxRegNone, mxRegValid, mxRegStaleDirGone, mxRegCrashedAside}

// mxPlantRegistry hand-crafts an administrative entry directly (see the
// file-level STRATEGY comment): for mxRegValid/mxRegStaleDirGone it writes
// an intact `gitdir` record (the two labels name the SAME construction —
// an intact registration naming this unit path — and are therefore
// constructible against EVERY unit-path value, including the ones where
// the registration turns out to name a plain directory, a regular file, a
// symlink, or nothing at all; those are precisely the mismatches gc must
// still decide); for mxRegCrashedAside it writes only the
// `gitdir.reconciling` aside record, exactly the shape step 3's atomic
// claim leaves behind after a simulated crash.
func mxPlantRegistry(t *testing.T, adminDir, canonicalUnitPath, entryName string, d mxRegistry) (planted bool) {
	t.Helper()
	if d == mxRegNone {
		return false
	}
	entryDir := filepath.Join(adminDir, entryName)
	if err := os.MkdirAll(entryDir, 0o755); err != nil {
		t.Fatalf("mkdir admin entry %s: %v", entryDir, err)
	}
	content := []byte(canonicalUnitPath + "/.git\n")
	recordName := "gitdir"
	if d == mxRegCrashedAside {
		recordName = "gitdir.reconciling"
	}
	if err := os.WriteFile(filepath.Join(entryDir, recordName), content, 0o644); err != nil {
		t.Fatalf("writing admin entry record %s: %v", recordName, err)
	}
	return true
}

// mxJointSkipReason names the only UNCONSTRUCTIBLE pairing this matrix
// documents — the single vacuous all-absent/none combination — and it is
// checked BEFORE every combination is built. Its one branch is the ENTIRE
// joint skip set: a joint skip for any other reason is a bug, caught by
// TestGC_TotalityMatrix's own hard-fail check against
// wantMxSkipped/mxKnownSkipReasonPrefixes below.
//
// It deliberately does NOT constrain registry=valid or
// registry=stale-dir-gone to a single unit-path value. Both are built by
// the same mxPlantRegistry intact-`gitdir` construction, which succeeds
// against every unit-path value; treating the other pairings as
// unconstructible would have excluded 960 genuinely constructible
// combinations (in particular the 360 "intact registration x {dir-plain,
// regular-file, symlink} unit path" mismatches, which are exactly the
// registration/filesystem disagreements gc must still decide) from the
// totality proof.
func mxJointSkipReason(up mxUnitPath, req mxRequest, st mxStaging, rel mxReleased, lk mxLock, reg mxRegistry) string {
	if up == mxUnitAbsent && req == mxReqAbsent && st == mxStagingAbsent && rel == mxRelAbsent && lk == mxLockAbsent && reg == mxRegNone {
		return "vacuous: every dimension is absent/none simultaneously — nothing on disk or in the registry names this id, so it never enters gc's scan set at all; not a reachable per-unit decision state"
	}
	return ""
}

// mxKnownSkipReasonPrefixes is the closed set of JOINT skip-reason
// PREFIXES this matrix will accept without hard-failing — one per branch
// of mxJointSkipReason. A joint skip reason matching none of these means
// an UNDOCUMENTED skip appeared and the test must fail loudly rather than
// silently absorb it.
var mxKnownSkipReasonPrefixes = []string{
	"vacuous: every dimension is absent/none simultaneously",
}

// mxKnownEnvSkipPrefixes is the separate closed set of ENVIRONMENT skip
// reasons a dimension constructor may return (as opposed to a documented
// unconstructible pairing). These are counted apart from joint skips and,
// on every platform where symlinks are expected to work, are themselves a
// hard failure — an environment skip silently removes a combination from
// the proof, which is exactly what this matrix must never tolerate.
var mxKnownEnvSkipPrefixes = []string{
	"symlink unsupported in this environment",
}

func mxHasKnownPrefix(reason string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(reason, prefix) {
			return true
		}
	}
	return false
}

func mxIsKnownSkipReason(reason string) bool {
	return mxHasKnownPrefix(reason, mxKnownSkipReasonPrefixes)
}

func mxIsKnownEnvSkipReason(reason string) bool {
	return mxHasKnownPrefix(reason, mxKnownEnvSkipPrefixes)
}

// wantMxTotal/wantMxSkipped are this matrix's hand-derived coverage
// arithmetic, asserted (not merely logged) so a change to either the
// dimension sizes or the joint-skip rule is caught rather than silently
// drifting: total = 5(unit path) * 4(.request) * 2(.request.staging) *
// 5(.released) * 3(.lock) * 4(registry) = 2400. skipped = 1 — the single
// vacuous all-absent/none combination, the only combination nothing on
// disk or in the registry names. Every other combination is
// constructible, so asserted = 2400 - 1 = 2399.
const (
	wantMxTotal   = 2400
	wantMxSkipped = 1
)

// mxComboStatus is runMxCombo's tri-state report. runMxCombo itself
// returns only mxComboAsserted or mxComboEnvSkipped; mxComboFailed is
// derived by the caller from t.Run's own result, because a t.Fatalf inside
// the subtest Goexits before runMxCombo can return anything at all.
type mxComboStatus int

const (
	mxComboAsserted mxComboStatus = iota
	mxComboEnvSkipped
	mxComboFailed
)

func TestGC_TotalityMatrix(t *testing.T) {
	ctx := context.Background()
	repo := newReconcileTestRepo(t)
	adminDir := adminWorktreesDir(t, repo.Dir)

	deadPIDPath := filepath.Join(t.TempDir(), "dead-pid-lock-template")
	writeDeadPIDLock(t, deadPIDPath)
	deadPIDBytes, err := os.ReadFile(deadPIDPath)
	if err != nil {
		t.Fatalf("reading dead-pid lock template: %v", err)
	}

	total, skipped, asserted, envSkipped, failed := 0, 0, 0, 0, 0
	var skipLog []string
	var envSkipLog []string

	comboIndex := 0
	for _, up := range allMxUnitPaths {
		for _, req := range allMxRequests {
			for _, st := range allMxStagings {
				for _, rel := range allMxReleaseds {
					for _, lk := range allMxLocks {
						for _, reg := range allMxRegistries {
							total++
							comboIndex++
							label := fmt.Sprintf("c%04d/up=%s,req=%s,st=%s,rel=%s,lk=%s,reg=%s", comboIndex, up, req, st, rel, lk, reg)

							if reason := mxJointSkipReason(up, req, st, rel, lk, reg); reason != "" {
								if !mxIsKnownSkipReason(reason) {
									t.Fatalf("%s: undocumented skip reason %q — a new, unaccounted-for skip appeared", label, reason)
								}
								skipped++
								skipLog = append(skipLog, label+": "+reason)
								continue
							}

							combo := mxCombo{up: up, req: req, st: st, rel: rel, lk: lk, reg: reg}
							status, envReason := mxComboAsserted, ""
							passed := t.Run(label, func(t *testing.T) {
								status, envReason = runMxCombo(t, ctx, repo.Dir, adminDir, comboIndex, combo, deadPIDBytes)
							})
							if !passed {
								status = mxComboFailed
							}
							switch status {
							case mxComboEnvSkipped:
								if !mxIsKnownEnvSkipReason(envReason) {
									t.Fatalf("%s: undocumented environment skip reason %q — a new, unaccounted-for skip appeared", label, envReason)
								}
								envSkipped++
								envSkipLog = append(envSkipLog, label+": "+envReason)
							case mxComboFailed:
								failed++
							default:
								asserted++
							}
						}
					}
				}
			}
		}
	}

	t.Logf("totality matrix coverage: total=%d skipped=%d asserted=%d environment-skipped=%d failed=%d", total, skipped, asserted, envSkipped, failed)
	for _, s := range skipLog {
		t.Logf("skipped (unconstructible): %s", s)
	}
	for _, s := range envSkipLog {
		t.Logf("skipped (environment): %s", s)
	}
	if total != wantMxTotal {
		t.Fatalf("total combinations = %d, want %d (dimension sizes changed — update the arithmetic comment)", total, wantMxTotal)
	}
	if skipped != wantMxSkipped {
		t.Fatalf("skipped combinations = %d, want exactly %d (only the single documented vacuous combination)", skipped, wantMxSkipped)
	}
	if envSkipped > 0 && runtime.GOOS != "windows" {
		t.Fatalf("environment-skipped=%d on GOOS=%s, want 0: symlink construction is expected to work on every platform this suite runs on, and an environment skip silently removes a combination from the totality proof: %v", envSkipped, runtime.GOOS, envSkipLog)
	}
	if failed != 0 {
		t.Fatalf("failed combinations = %d, want 0 (see the failing subtests above)", failed)
	}
	if asserted+envSkipped+failed != total-skipped {
		t.Fatalf("coverage arithmetic mismatch: asserted=%d + environment-skipped=%d + failed=%d, want total-skipped=%d", asserted, envSkipped, failed, total-skipped)
	}
}

type mxCombo struct {
	up  mxUnitPath
	req mxRequest
	st  mxStaging
	rel mxReleased
	lk  mxLock
	reg mxRegistry
}

// runMxCombo builds ONE combination's fixture (a fresh store root, so the
// only thing shared across combinations is repoDir/adminDir), runs
// GC over it, and asserts totality: exactly one GCResult for the target
// unit OR a scan-level disclosure naming it (never both, never neither),
// membership in the closed GCOutcome enum, no error from GC itself, and —
// via the walk+hash guard reused from gc_test.go's hashAdminDir — that a
// canary sibling unit present in every fixture is left byte-identical.
//
// It reports (mxComboAsserted, "") when the combination was really built
// and really asserted, or (mxComboEnvSkipped, reason) when a dimension
// constructor could not build its fixture in THIS environment. It never
// returns mxComboFailed: an assertion failure goes through t.Fatalf, which
// Goexits, and the caller derives that state from t.Run's own result. The
// environment-skip return exists so such a skip is counted separately
// instead of being silently absorbed into the asserted count (which is
// what an unchecked t.Skipf did).
func runMxCombo(t *testing.T, ctx context.Context, repoDir, adminDir string, comboIndex int, c mxCombo, deadPIDBytes []byte) (mxComboStatus, string) {
	t.Helper()
	storeRoot := t.TempDir()
	if err := os.MkdirAll(ExecutionRoot(storeRoot), 0o755); err != nil {
		t.Fatalf("mkdir execution root: %v", err)
	}

	targetID := fmt.Sprintf("mx%04d--0123456789ab", comboIndex)
	canaryID := "canary--0123456789ab"

	canaryPath := UnitPath(storeRoot, canaryID)
	if err := os.MkdirAll(canaryPath, 0o755); err != nil {
		t.Fatalf("mkdir canary unit: %v", err)
	}
	canaryHashBefore := hashAdminDir(t, canaryPath)

	tmpBase := t.TempDir()
	unitPath := UnitPath(storeRoot, targetID)
	if ok, reason := mxConstructUnitPath(t, unitPath, tmpBase, c.up); !ok {
		t.Logf("unit-path construction unavailable: %s", reason)
		return mxComboEnvSkipped, reason
	}
	if ok, reason := mxConstructRequest(t, RequestPath(storeRoot, targetID), tmpBase, c.req); !ok {
		t.Logf("request construction unavailable: %s", reason)
		return mxComboEnvSkipped, reason
	}
	mxConstructStaging(t, RequestStagingPath(storeRoot, targetID), c.st)
	if ok, reason := mxConstructReleased(t, ReleasedPath(storeRoot, targetID), tmpBase, c.rel); !ok {
		t.Logf("released construction unavailable: %s", reason)
		return mxComboEnvSkipped, reason
	}
	mxConstructLock(t, LockPath(storeRoot, targetID), c.lk, deadPIDBytes)

	canonicalUnit := canonicalPath(unitPath)
	entryName := fmt.Sprintf("mx-entry-%04d", comboIndex)
	planted := mxPlantRegistry(t, adminDir, canonicalUnit, entryName, c.reg)
	if planted {
		defer func() {
			if rerr := os.RemoveAll(filepath.Join(adminDir, entryName)); rerr != nil {
				t.Fatalf("cleaning up planted admin entry %s: %v", entryName, rerr)
			}
		}()
	}

	results, disclosures, err := GC(ctx, storeRoot, repoDir)
	if err != nil {
		t.Fatalf("GC returned a pre-sweep error for a per-unit combination (AD-10: per-unit conditions must never fail GC itself): %v", err)
	}

	resultCount := 0
	var gotOutcome GCOutcome
	var gotDetail string
	for _, r := range results {
		if r.WorkspaceID == targetID {
			resultCount++
			gotOutcome = r.Outcome
			gotDetail = r.Detail
		}
	}
	disclosureCount := 0
	for _, d := range disclosures {
		if strings.Contains(d, targetID) {
			disclosureCount++
		}
	}
	if resultCount+disclosureCount != 1 {
		t.Fatalf("target unit produced result_count=%d disclosure_count=%d, want exactly one of {GCResult, scan-level disclosure} (never both, never neither); outcome=%v detail=%q disclosures=%v",
			resultCount, disclosureCount, gotOutcome, gotDetail, disclosures)
	}
	if resultCount == 1 && (gotOutcome <= GCOutcomeUnknown || gotOutcome >= numGCOutcomes) {
		t.Fatalf("outcome %v (%d) falls outside the closed GCOutcome enum", gotOutcome, int(gotOutcome))
	}

	canaryHashAfter := hashAdminDir(t, canaryPath)
	if canaryHashBefore != canaryHashAfter {
		t.Fatalf("canary sibling unit %s mutated by a gc call over this unrelated combination", canaryID)
	}
	return mxComboAsserted, ""
}

// --- representative subset: cross-check against Materialize ---

// classifyMaterializeOutcome maps a Materialize call's (Result, error) pair
// into one of the five outcome kinds the contract names, so the
// representative-subset test below can assert "exactly one of" via a
// single comparable string rather than five separate type switches at each
// call site.
func classifyMaterializeOutcome(res Result, err error) string {
	if err == nil {
		switch res.Outcome {
		case OutcomeReused:
			return "reused"
		case OutcomeMaterialized:
			return "fresh-materialize"
		default:
			return fmt.Sprintf("unexpected-success-outcome(%v)", res.Outcome)
		}
	}
	var terminal *ErrReleasedTerminal
	if errors.As(err, &terminal) {
		return "released-terminal"
	}
	var mismatch *ErrIdentityMismatch
	if errors.As(err, &mismatch) {
		return "identity-mismatch"
	}
	var opErr *OperationalError
	if errors.As(err, &opErr) {
		return "operational-error"
	}
	return fmt.Sprintf("unclassified-error(%v)", err)
}

// mxRepresentativeRow is one (unit-path type, released type) pairing, held
// under QUIESCENT .request/.request.staging/.lock/registry (all
// absent/absent/absent/none — no contention, no complicating residue),
// with its hand-derived expectation for BOTH machines and the spec
// citation justifying the pairing. Every unit-path type crosses every
// released type: 5*5 = 25 rows, satisfying "every distinct unit-path type
// x every .released type with quiescent lock/registry".
type mxRepresentativeRow struct {
	up              mxUnitPath
	rel             mxReleased
	wantGC          GCOutcome
	wantMaterialize string
	note            string
	// vacuousForGC marks the ONE row (unit path absent, .released absent)
	// where, under quiescent .request/.request.staging/.lock/registry,
	// NOTHING at all names this id — exactly totality_test.go's own
	// documented vacuous skip. GC's scan set is empty for it (no
	// GCResult, no disclosure); Materialize is unaffected (it addresses
	// the identity's deterministic path directly, never "scans" for it),
	// so only the GC-side assertion special-cases this row.
	vacuousForGC bool
}

var mxRepresentativeRows = []mxRepresentativeRow{
	// unit path ABSENT: gc's rank 0 (siblings-only-or-nothing) mirrors
	// materialize's step 2 exactly — both examine the SAME three sibling
	// forms with the SAME lstat-typed classification (spec §Workspace
	// naming step 2 / §GC slice rank 0 share one prose description).
	{mxUnitAbsent, mxRelAbsent, GCOutcomeUnknown, "fresh-materialize", "vacuous under quiescent request/staging/lock/registry: nothing on disk or in the registry names this id, so it never enters gc's scan set at all (no GCResult, no disclosure) — materialize's step 2 is a no-op then proceeds fresh at step 5, unaffected since it addresses the id's deterministic path directly rather than scanning", true},
	{mxUnitAbsent, mxRelEmptyRegular, ReclaimOrphaned, "fresh-materialize", "a lone regular .released is orphaned metadata on both machines: gc unlinks it under rank 0, materialize unlinks it under step 2, both then proceed", false},
	{mxUnitAbsent, mxRelNonEmptyRegular, ReclaimOrphaned, "fresh-materialize", "content is ignored — existence is the record — so nonempty behaves identically to empty", false},
	{mxUnitAbsent, mxRelDirectory, Partial, "operational-error", "a non-regular object at an orphaned sibling path is 'unexpected object kind' on both machines: gc's rank-0 orphan-sibling loop discloses Partial, materialize's step-2 handleAbsentUnit returns an operational error — the same fail-closed mirror, neither silently deletes through it", false},
	{mxUnitAbsent, mxRelSymlink, Partial, "operational-error", "same mirror as directory: a symlink is never followed, never treated as a plain regular orphan", false},

	// unit path a genuine-shaped (but non-functional) worktree directory:
	// StatusDirty always fails on this fixture (no real git linkage), so
	// gc's rank 3 keeps fail-closed (KeepDirty, unevaluable predicate)
	// whenever the marker is regular; materialize's step 3a is TERMINAL
	// regardless of git state (dirtiness is never even consulted before
	// 3a fires), so both machines refuse to touch/reuse it — the
	// consistency property here is "never destroyed, never silently
	// reused", not identical outcome labels.
	{mxUnitDirWorktree, mxRelAbsent, KeepNotEligible, "fresh-materialize", "marker absent: gc rank 2 (not yet released, no mutation); materialize step 4c rebuilds the (non-functional) residue and re-cuts fresh, since .request's absence proves no consumer ever received it", false},
	{mxUnitDirWorktree, mxRelEmptyRegular, KeepDirty, "released-terminal", "marker regular: materialize's 3a fires unconditionally (terminal, independent of git state); gc's rank 3 cannot evaluate StatusDirty against this non-functional fixture and keeps fail-closed rather than guessing clean — both refuse to mutate/reuse the workspace", false},
	{mxUnitDirWorktree, mxRelNonEmptyRegular, KeepDirty, "released-terminal", "same as empty-regular: content is ignored", false},
	{mxUnitDirWorktree, mxRelDirectory, KeepMalformed, "operational-error", "a non-regular object at the marker path is gc's own keep-malformed rank AND materialize's step 3b operational error — spec's explicit mirror: 'the materialization mirror of gc's keep-malformed rank, so a malformed marker is a decided state on both paths, not a gap'", false},
	{mxUnitDirWorktree, mxRelSymlink, KeepMalformed, "operational-error", "same mirror as directory: symlink at the marker path, never followed", false},

	// unit path a plain (non-worktree) directory: StatusDirty fails just
	// as above (no .git at all — `git status` finds no repository), so
	// the same reasoning applies row-for-row.
	{mxUnitDirPlain, mxRelAbsent, KeepNotEligible, "fresh-materialize", "marker absent: gc rank 2; materialize's 4c RemoveAll's the plain directory (no witness survives it) and re-cuts fresh", false},
	{mxUnitDirPlain, mxRelEmptyRegular, KeepDirty, "released-terminal", "same fail-closed/terminal mirror as the worktree-shaped case: neither StatusDirty's own failure nor materialize's 3a depend on there being real git linkage", false},
	{mxUnitDirPlain, mxRelNonEmptyRegular, KeepDirty, "released-terminal", "same as empty-regular", false},
	{mxUnitDirPlain, mxRelDirectory, KeepMalformed, "operational-error", "keep-malformed / operational-error mirror, as above", false},
	{mxUnitDirPlain, mxRelSymlink, KeepMalformed, "operational-error", "keep-malformed / operational-error mirror, as above", false},

	// unit path a regular file: MALFORMATION IS TESTED BEFORE
	// ELIGIBILITY on both machines, so every .released value here is
	// irrelevant — gc's rank 1 and materialize's own unit-path lstat
	// check both fire before either machine ever looks at the marker
	// path, proving the "malformed unit path" mirror holds identically
	// across all 5 .released values, not just one.
	{mxUnitRegularFile, mxRelAbsent, KeepMalformed, "operational-error", "non-directory object at the unit path: gc rank 1, materialize's own unit-path check ('any object ... not a real directory is an OPERATIONAL ERROR ... the step-3b posture applied one level up')", false},
	{mxUnitRegularFile, mxRelEmptyRegular, KeepMalformed, "operational-error", "malformation dominates before eligibility is ever examined", false},
	{mxUnitRegularFile, mxRelNonEmptyRegular, KeepMalformed, "operational-error", "malformation dominates before eligibility is ever examined", false},
	{mxUnitRegularFile, mxRelDirectory, KeepMalformed, "operational-error", "malformation dominates before eligibility is ever examined", false},
	{mxUnitRegularFile, mxRelSymlink, KeepMalformed, "operational-error", "malformation dominates before eligibility is ever examined", false},

	// unit path a symlink: LstatType never follows it, so it is a
	// non-directory object exactly like the regular-file case above —
	// same mirror, all 5 rows.
	{mxUnitSymlink, mxRelAbsent, KeepMalformed, "operational-error", "symlink at the unit path is never followed and never a directory (BOTH PATHS ARE EXAMINED WITH LSTAT)", false},
	{mxUnitSymlink, mxRelEmptyRegular, KeepMalformed, "operational-error", "malformation dominates before eligibility is ever examined", false},
	{mxUnitSymlink, mxRelNonEmptyRegular, KeepMalformed, "operational-error", "malformation dominates before eligibility is ever examined", false},
	{mxUnitSymlink, mxRelDirectory, KeepMalformed, "operational-error", "malformation dominates before eligibility is ever examined", false},
	{mxUnitSymlink, mxRelSymlink, KeepMalformed, "operational-error", "malformation dominates before eligibility is ever examined", false},
}

func TestGC_TotalityMatrix_RepresentativeSubset_MaterializeConsistency(t *testing.T) {
	if len(mxRepresentativeRows) != len(allMxUnitPaths)*len(allMxReleaseds) {
		t.Fatalf("representative row count = %d, want %d (every unit-path type x every .released type)", len(mxRepresentativeRows), len(allMxUnitPaths)*len(allMxReleaseds))
	}

	ctx := context.Background()
	repo := newReconcileTestRepo(t)

	for i, row := range mxRepresentativeRows {
		row := row
		t.Run(fmt.Sprintf("up=%s,rel=%s", row.up, row.rel), func(t *testing.T) {
			// --- GC's own reading, on its own fixture ---
			gcStoreRoot := t.TempDir()
			if err := os.MkdirAll(ExecutionRoot(gcStoreRoot), 0o755); err != nil {
				t.Fatalf("mkdir execution root (gc fixture): %v", err)
			}
			id := fmt.Sprintf("rs%02dgc--0123456789ab", i)
			tmpBase := t.TempDir()
			if ok, reason := mxConstructUnitPath(t, UnitPath(gcStoreRoot, id), tmpBase, row.up); !ok {
				t.Skipf("unit-path construction unavailable: %s", reason)
			}
			if ok, reason := mxConstructReleased(t, ReleasedPath(gcStoreRoot, id), tmpBase, row.rel); !ok {
				t.Skipf("released construction unavailable: %s", reason)
			}
			// .request/.request.staging/.lock/registry: quiescent (absent/
			// absent/absent/none) — nothing further to construct.

			gcResults, _, err := GC(ctx, gcStoreRoot, repo.Dir)
			if err != nil {
				t.Fatalf("GC: %v", err)
			}
			var gotGC GCOutcome
			found := false
			for _, r := range gcResults {
				if r.WorkspaceID == id {
					gotGC = r.Outcome
					found = true
				}
			}
			if row.vacuousForGC {
				if found {
					t.Fatalf("row is documented vacuous (nothing should name %s), but gc produced outcome %v — the vacuous claim no longer holds", id, gotGC)
				}
			} else {
				if !found {
					t.Fatalf("no GCResult for %s (quiescent registry/lock, so the unit must be scan-set-visible via its own filesystem grammar)", id)
				}
				if gotGC != row.wantGC {
					t.Fatalf("GC outcome = %v, want %v (%s)", gotGC, row.wantGC, row.note)
				}
			}

			// --- Materialize's own reading, on a SEPARATE, independently
			// constructed fixture (GC above may itself have mutated its
			// own copy for the mutating outcomes) ---
			mStoreRoot := t.TempDir()
			if err := os.MkdirAll(ExecutionRoot(mStoreRoot), 0o755); err != nil {
				t.Fatalf("mkdir execution root (materialize fixture): %v", err)
			}
			mReconciler := NewGitReconciler(mStoreRoot)
			m, merr := NewMaterializer(mStoreRoot, repo.Dir, mReconciler)
			if merr != nil {
				t.Fatalf("NewMaterializer: %v", merr)
			}
			// The fixture MUST be built at the exact path Materialize will
			// itself address — its own workspace id, computed from reqID,
			// not the "rsNNgc..." id used for gc's own (unrelated)
			// fixture above. Building reqID first and deriving the
			// fixture path from it (rather than a separately chosen id)
			// is what makes this a real "identity-equal request" test.
			reqID, ierr := NewExactIdentity("representative-subset", repo.Head)
			if ierr != nil {
				t.Fatalf("NewExactIdentity: %v", ierr)
			}
			materializeUnitID, widErr := reqID.WorkspaceID()
			if widErr != nil {
				t.Fatalf("WorkspaceID: %v", widErr)
			}

			tmpBase2 := t.TempDir()
			if ok, reason := mxConstructUnitPath(t, UnitPath(mStoreRoot, materializeUnitID), tmpBase2, row.up); !ok {
				t.Skipf("unit-path construction unavailable: %s", reason)
			}
			if ok, reason := mxConstructReleased(t, ReleasedPath(mStoreRoot, materializeUnitID), tmpBase2, row.rel); !ok {
				t.Skipf("released construction unavailable: %s", reason)
			}

			res, materializeErr := m.Materialize(ctx, Request{Identity: reqID})
			gotMaterialize := classifyMaterializeOutcome(res, materializeErr)
			if gotMaterialize != row.wantMaterialize {
				t.Fatalf("Materialize classified as %q (res=%+v err=%v), want %q (%s)", gotMaterialize, res, materializeErr, row.wantMaterialize, row.note)
			}

			// Consistency invariant cited by the contract: gc's own
			// keep-malformed rank and materialize's own operational-error
			// class are the SAME decided state on both paths (never a gap
			// on either side) — assert it holds for exactly the rows where
			// gc decided KeepMalformed.
			if gotGC == KeepMalformed && gotMaterialize != "operational-error" {
				t.Fatalf("mirror rule violated: gc=KeepMalformed but materialize=%q, want operational-error (spec §Exact workspace materialization: 'the materialization mirror of gc's keep-malformed rank')", gotMaterialize)
			}
		})
	}
}
