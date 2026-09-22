// Real two-process integration tests for `verdi serve` + `verdi mcp`
// (PLAN.md Phase 9 exit criteria): builds the actual verdi binary and
// exercises it as real OS processes — never a mocked transport — proving
// D3's single-writer guarantee, I-12's lock takeover after SIGKILL, and
// the S4-binding shim-shutdown fix end to end.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/workbench"
)

type readinessSnapshotBuilderFunc func(context.Context, string, string) (string, *readinessload.PredecodedRequest, error)

func (f readinessSnapshotBuilderFunc) Build(ctx context.Context, root, requestPath string) (string, *readinessload.PredecodedRequest, error) {
	return f(ctx, root, requestPath)
}

func TestServeContextRequestFlagGrammar(t *testing.T) {
	t.Run("one request in either flag position preserves HTTP behavior", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			args []string
		}{
			{name: "context first", args: []string{"--context-request", "request.json", "--http", "127.0.0.1:4101"}},
			{name: "context last", args: []string{"--http", "127.0.0.1:4101", "--context-request", "request.json"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var calls []string
				deps := serveCommandDeps{
					findRoot: func(string) (string, error) {
						calls = append(calls, "find-root")
						return "/store", nil
					},
					readiness: readinessSnapshotBuilderFunc(func(_ context.Context, root, requestPath string) (string, *readinessload.PredecodedRequest, error) {
						calls = append(calls, "build:"+root+":"+requestPath)
						return "spec/startup-target", &readinessload.PredecodedRequest{}, nil
					}),
					run: func(root, httpAddr string, loader readinessload.Loader, defaultSpec string, _, _ io.Writer) int {
						calls = append(calls, "run:"+root+":"+httpAddr)
						if loader.Root != root || defaultSpec != "spec/startup-target" {
							t.Fatalf("run loader=%+v defaultSpec=%q, want root %q and the built default spec", loader, defaultSpec, root)
						}
						return 0
					},
				}
				var stdout, stderr bytes.Buffer
				if code := cmdServeWithDeps(tc.args, &stdout, &stderr, deps); code != 0 {
					t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
				}
				want := []string{"find-root", "build:/store:request.json", "run:/store:127.0.0.1:4101"}
				if !reflect.DeepEqual(calls, want) {
					t.Fatalf("calls = %q, want %q", calls, want)
				}
			})
		}
	})

	t.Run("legacy no-flag and repeated HTTP retain exact selection", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			args     []string
			wantHTTP string
		}{
			{name: "default", wantHTTP: defaultWorkbenchAddr},
			{name: "explicit", args: []string{"--http", "127.0.0.1:0"}, wantHTTP: "127.0.0.1:0"},
			{name: "last repeated value wins", args: []string{"--http", "127.0.0.1:4100", "--http", "127.0.0.1:4102"}, wantHTTP: "127.0.0.1:4102"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				deps := serveCommandDeps{
					findRoot: func(string) (string, error) { return "/store", nil },
					readiness: readinessSnapshotBuilderFunc(func(context.Context, string, string) (string, *readinessload.PredecodedRequest, error) {
						t.Fatal("readiness builder called without --context-request")
						return "", nil, nil
					}),
					run: func(_ string, gotHTTP string, loader readinessload.Loader, defaultSpec string, _, _ io.Writer) int {
						if gotHTTP != tc.wantHTTP || defaultSpec != "" {
							t.Fatalf("run(http=%q, defaultSpec=%q), want http=%q and no default spec", gotHTTP, defaultSpec, tc.wantHTTP)
						}
						if loader.Root != "/store" {
							t.Fatalf("run loader.Root = %q, want /store — a working loader is always wired even with no --context-request", loader.Root)
						}
						return 0
					},
				}
				var stdout, stderr bytes.Buffer
				if code := cmdServeWithDeps(tc.args, &stdout, &stderr, deps); code != 0 {
					t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
				}
			})
		}
	})

	t.Run("closed invalid grammar has no root or server effect", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			args []string
		}{
			{name: "missing context value", args: []string{"--context-request"}},
			{name: "empty context value", args: []string{"--context-request", ""}},
			{name: "context followed by flag", args: []string{"--context-request", "--http", "127.0.0.1:0"}},
			{name: "stdin context", args: []string{"--context-request", "-"}},
			{name: "duplicate context", args: []string{"--context-request", "a", "--context-request", "b"}},
			{name: "unknown flag", args: []string{"--unknown"}},
			{name: "positional", args: []string{"extra"}},
			{name: "missing HTTP value", args: []string{"--http"}},
			{name: "empty HTTP value", args: []string{"--http", ""}},
			{name: "HTTP followed by context flag", args: []string{"--http", "--context-request"}},
			{name: "HTTP followed by unknown flag", args: []string{"--http", "--unknown"}},
			{name: "HTTP followed by repeated HTTP flag", args: []string{"--http", "--http"}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				called := false
				deps := serveCommandDeps{
					findRoot: func(string) (string, error) { called = true; return "", errors.New("must not run") },
					readiness: readinessSnapshotBuilderFunc(func(context.Context, string, string) (string, *readinessload.PredecodedRequest, error) {
						called = true
						return "", nil, errors.New("must not run")
					}),
					run: func(string, string, readinessload.Loader, string, io.Writer, io.Writer) int { called = true; return 0 },
				}
				var stdout, stderr bytes.Buffer
				if code := cmdServeWithDeps(tc.args, &stdout, &stderr, deps); code != 2 {
					t.Fatalf("exit = %d, want 2", code)
				}
				if called || stdout.Len() != 0 || stderr.Len() == 0 {
					t.Fatalf("called=%v stdout=%q stderr=%q, want parser-only diagnostic", called, stdout.String(), stderr.String())
				}
			})
		}
	})
}

func TestServeContextRequestBuildsBeforeEveryServerEffect(t *testing.T) {
	root := t.TempDir()
	lockPath := store.WriterLockPath(root)
	var calls []string
	builds := 0
	builder := readinessSnapshotBuilderFunc(func(context.Context, string, string) (string, *readinessload.PredecodedRequest, error) {
		builds++
		calls = append(calls, "builder-start")
		if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
			t.Fatal(err)
		}
		lock, err := filelock.Acquire(lockPath)
		if err != nil {
			t.Fatalf("builder transient lock: %v", err)
		}
		if err := filelock.Release(lock, lockPath); err != nil {
			t.Fatalf("builder transient lock release: %v", err)
		}
		calls = append(calls, "builder-lock-released", "builder-complete")
		return "spec/startup-target", &readinessload.PredecodedRequest{}, nil
	})
	deps := serveCommandDeps{
		findRoot:  func(string) (string, error) { return root, nil },
		readiness: builder,
		run: func(string, string, readinessload.Loader, string, io.Writer, io.Writer) int {
			lock, err := filelock.Acquire(lockPath)
			if err != nil {
				t.Fatalf("server could not acquire writer lock after builder returned: %v", err)
			}
			if err := filelock.Release(lock, lockPath); err != nil {
				t.Fatalf("server lock release: %v", err)
			}
			calls = append(calls,
				"data-directory", "writer-lock", "unix-listen", "pointer-file",
				"forge-wiring", "tcp-listen", "handler-construction", "mcp-service",
			)
			return 0
		},
	}
	var stdout, stderr bytes.Buffer
	if code := cmdServeWithDeps([]string{"--context-request", "request.json"}, &stdout, &stderr, deps); code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
	}
	want := []string{
		"builder-start", "builder-lock-released", "builder-complete",
		"data-directory", "writer-lock", "unix-listen", "pointer-file",
		"forge-wiring", "tcp-listen", "handler-construction", "mcp-service",
	}
	if builds != 1 || !reflect.DeepEqual(calls, want) {
		t.Fatalf("builds=%d calls=%q, want one build then %q", builds, calls, want)
	}
}

func TestServeContextRequestBuilderFailureStopsBeforeServerEffects(t *testing.T) {
	var serverEffects int
	deps := serveCommandDeps{
		findRoot: func(string) (string, error) { return t.TempDir(), nil },
		readiness: readinessSnapshotBuilderFunc(func(context.Context, string, string) (string, *readinessload.PredecodedRequest, error) {
			return "", nil, errors.New("snapshot unavailable")
		}),
		run: func(string, string, readinessload.Loader, string, io.Writer, io.Writer) int {
			serverEffects++
			return 0
		},
	}
	var stdout, stderr bytes.Buffer
	if code := cmdServeWithDeps([]string{"--context-request", "request.json"}, &stdout, &stderr, deps); code != 2 {
		t.Fatalf("exit = %d, want operational 2", code)
	}
	if serverEffects != 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "snapshot unavailable") {
		t.Fatalf("serverEffects=%d stdout=%q stderr=%q", serverEffects, stdout.String(), stderr.String())
	}
}

// TestServeReadinessRouteRederivesLiveOnEveryRequest is spec/readiness-
// recovery ac-2/ac-4: the served /readiness route re-derives fresh
// through Deps.ReadinessLoader on EVERY request — the opposite property
// from what this test proved before Task 3 (its old name was
// TestServeContextRequestSnapshotRemainsImmutableAcrossRequests): under
// the pre-Task-3 design, one startup snapshot was handed into
// workbench.Deps{Readiness: ...} and replayed byte-identically forever;
// ac-2's whole point is that a source mutation between two requests IS
// now picked up live, without restarting verdi serve. The startup
// warm-up itself still runs exactly once regardless
// (TestServeContextRequestBuildsBeforeEveryServerEffect already proves
// that in isolation) — this test's own subject is the SERVED route's
// live re-derivation. Drives internal/readinessload.Load entirely
// in-process, through Options.ConflictProvider (fix round 1, Important
// 2's hermetic seam) — no shell script, no judge process; that recipe is
// reserved for TestReadinessLoadBuilderHandsOffTheCacheOnlyLoaderAndDefaultSpec
// below (whose own subject IS the JudgeRun pre-run) and
// TestServeContextRequestReadinessReachesGetDocumentOverSocket (the real
// binary end to end).
func TestServeReadinessRouteRederivesLiveOnEveryRequest(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	requestPath := writeContextRequestFile(t, repo.Dir, "readiness-request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))
	specPath := store.ActiveSpecPath(repo.Dir, "feature-alpha")
	specSource, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	const mutatedTitle = "Feature Alpha, retitled between two requests"

	builds := 0
	builder := readinessSnapshotBuilderFunc(func(ctx context.Context, root, gotRequestPath string) (string, *readinessload.PredecodedRequest, error) {
		builds++
		targetSpec, predecoded, err := readinessload.ContextRequestSpec(root, gotRequestPath)
		if err != nil {
			return "", nil, err
		}
		_, err = readinessload.Load(ctx, root, targetSpec, readinessload.Options{
			ContextRequestPath: gotRequestPath, PredecodedRequest: predecoded, BoardHref: workbench.BranchBoardHref,
			Judge: readinessload.JudgeRun, ConflictProvider: readinessLoadPassProviderFunc(t),
		})
		if err != nil {
			return "", nil, err
		}
		return targetSpec, predecoded, nil
	})
	deps := serveCommandDeps{
		findRoot:  func(string) (string, error) { return repo.Dir, nil },
		readiness: builder,
		run: func(root, _ string, loader readinessload.Loader, defaultSpec string, _, _ io.Writer) int {
			if loader.Root != root || defaultSpec == "" {
				t.Fatalf("run loader=%+v defaultSpec=%q, want root %q and a non-empty default spec", loader, defaultSpec, root)
			}
			// The production loader cmdServeWithDeps built carries no
			// ConflictProvider (that seam is test-only); overlay the same
			// hermetic pass-provider the warm-up used onto this LOCAL copy
			// so the route's own re-derivation needs no real judge either
			// — everything else (Root, ContextRequestPath, BoardHref) is
			// exactly what production wired.
			loader.Opts.ConflictProvider = readinessLoadPassProviderFunc(t)
			handler := workbench.NewHandlerWith(root, workbench.Deps{ReadinessLoader: loader, ReadinessDefaultSpec: defaultSpec})
			first := httptest.NewRecorder()
			handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/readiness", nil))
			if first.Code != http.StatusOK {
				t.Fatalf("first readiness status = %d, body=%q", first.Code, first.Body.String())
			}
			if !strings.Contains(first.Body.String(), repo.Head) || !strings.Contains(first.Body.String(), "for this request") {
				t.Fatalf("first body misses derivation HEAD or stamp: %q", first.Body.String())
			}
			// A VALID mutation, never a corruption: the spec keeps
			// deriving, so the second response is a 200 carrying the NEW
			// title. A route that merely failed every request after a
			// write would satisfy "the two responses differ" without
			// re-deriving anything (fix round 1, M5), which is why the
			// positive value is asserted here rather than a failure.
			mutated := strings.Replace(string(specSource), `title: "Feature Alpha"`, `title: "`+mutatedTitle+`"`, 1)
			if mutated == string(specSource) {
				t.Fatal("fixture drift: the spec's title line no longer matches, so nothing was mutated")
			}
			if err := os.WriteFile(specPath, []byte(mutated), 0o644); err != nil {
				t.Fatal(err)
			}
			second := httptest.NewRecorder()
			handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/readiness", nil))
			if second.Code != http.StatusOK {
				t.Fatalf("second readiness request after a valid source mutation must re-derive successfully, got status=%d body=%q", second.Code, second.Body.String())
			}
			if !strings.Contains(second.Body.String(), mutatedTitle) {
				t.Fatalf("second readiness response does not carry the title written between the two requests — the route is not re-deriving live (ac-2 regression):\n%s", second.Body.String())
			}
			if strings.Contains(first.Body.String(), mutatedTitle) {
				t.Fatalf("the FIRST response already carried the post-mutation title, so the assertion above proves nothing:\n%s", first.Body.String())
			}
			return 0
		},
	}
	var stdout, stderr bytes.Buffer
	if code := cmdServeWithDeps([]string{"--http", "127.0.0.1:0", "--context-request", requestPath}, &stdout, &stderr, deps); code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
	}
	if builds != 1 {
		t.Fatalf("readiness warm-up builds = %d after two HTTP requests, want exactly 1", builds)
	}
}

// TestServeReadinessSurvivesTheRequestFileVanishingAfterStartup is
// R-RR1-17: the per-request loader `verdi serve` threads into the board,
// the Document tab and MCP carries the PredecodedRequest bundle the
// startup warm-up already validated, so it never re-reads the request
// file. A request file deleted (or edited, or moved) mid-run must not
// turn the request's own spec — nor, through the same loader, any other
// spec — into a loader error for as long as that server runs. The file is
// removed INSIDE the fake run, i.e. after startup completed, and the
// per-request Load that follows must still derive; the RequestDigest
// assertion proves it derived through the request (the startup-validated
// bytes), not merely that the error went away. Hermetic per co-1 and
// R-RR1-14: the conflict verdict comes from Options.ConflictProvider
// in-process — no judge process, no network.
func TestServeReadinessSurvivesTheRequestFileVanishingAfterStartup(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	requestData := contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil)
	requestPath := writeContextRequestFile(t, repo.Dir, "readiness-request.json", requestData)
	requestSum := sha256.Sum256(requestData)
	wantDigest := "sha256:" + hex.EncodeToString(requestSum[:])

	builder := readinessSnapshotBuilderFunc(func(ctx context.Context, root, gotRequestPath string) (string, *readinessload.PredecodedRequest, error) {
		targetSpec, predecoded, err := readinessload.ContextRequestSpec(root, gotRequestPath)
		if err != nil {
			return "", nil, err
		}
		if _, err := readinessload.Load(ctx, root, targetSpec, readinessload.Options{
			ContextRequestPath: gotRequestPath, PredecodedRequest: predecoded, BoardHref: workbench.BranchBoardHref,
			Judge: readinessload.JudgeRun, ConflictProvider: readinessLoadPassProviderFunc(t),
		}); err != nil {
			return "", nil, err
		}
		return targetSpec, predecoded, nil
	})
	deps := serveCommandDeps{
		findRoot:  func(string) (string, error) { return repo.Dir, nil },
		readiness: builder,
		run: func(_, _ string, loader readinessload.Loader, defaultSpec string, _, _ io.Writer) int {
			if loader.Opts.ContextRequestPath != requestPath {
				t.Fatalf("loader ContextRequestPath = %q, want %q — Load's contextFallback vector still names it", loader.Opts.ContextRequestPath, requestPath)
			}
			// The mid-run event this ruling exists for.
			if err := os.Remove(requestPath); err != nil {
				t.Fatalf("removing the request file after startup: %v", err)
			}
			loader.Opts.ConflictProvider = readinessLoadPassProviderFunc(t)
			snap, err := loader.Load(context.Background(), defaultSpec)
			if err != nil {
				t.Fatalf("per-request Load after the request file was removed must still derive, got: %v", err)
			}
			if snap.RequestDigest != wantDigest {
				t.Fatalf("RequestDigest = %q, want the startup-validated request's own %q — the derivation must travel the request path, not fall back to no request", snap.RequestDigest, wantDigest)
			}
			return 0
		},
	}
	var stdout, stderr bytes.Buffer
	if code := cmdServeWithDeps([]string{"--http", "127.0.0.1:0", "--context-request", requestPath}, &stdout, &stderr, deps); code != 0 {
		t.Fatalf("exit = %d, stderr=%q", code, stderr.String())
	}
}

// readinessLoadPassProviderFunc builds a readinessload.ConflictProviderFunc
// (fix round 1, Important 2's seam) whose Evaluate always returns a
// self-consistent VerdictPass report re-targeted at the caller's own
// request — an in-process, no-subprocess fake, mirroring the real
// internal/policyconflict/testdata/report.json fixture cmd/verdi's deleted
// readiness_snapshot_test.go once re-targeted the same way
// (readinessSnapshotReport) and internal/readinessload's own load_test.go
// still does (fixtureReport) — never a hand-authored duplicate of the fixed
// report shape.
func readinessLoadPassProviderFunc(t *testing.T) readinessload.ConflictProviderFunc {
	t.Helper()
	return func(_ context.Context, root string, request policyconflict.Request) (policyconflict.VerdictProvider, error) {
		return contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			return readinessLoadPassReport(t, root, request), nil
		}), nil
	}
}

// readinessLoadPassReport decodes the real, already-cross-validated
// internal/policyconflict/testdata/report.json fixture and re-targets its
// Input.Target/Repository at request's own acceptance candidate, forcing an
// empty Semantic slice and VerdictPass. readinessload.Load cross-checks the
// report's target identity against the resolved journey target/repository
// (load.go's reportIdentity) before it will accept the report at all, so
// this re-targeting is required, not optional.
func readinessLoadPassReport(t *testing.T, root string, request policyconflict.Request) policyconflict.Result {
	t.Helper()
	candidate := request.Target.AcceptanceCandidate
	if request.Target.Kind != policyconflict.TargetAcceptanceCandidate || candidate == nil {
		t.Fatalf("provider request target = %+v, want one acceptance candidate", request.Target)
	}
	ref, err := artifact.ParseRef(candidate.Spec)
	if err != nil {
		t.Fatalf("ParseRef(%q): %v", candidate.Spec, err)
	}
	specBytes, err := os.ReadFile(store.ActiveSpecPath(root, ref.Name))
	if err != nil {
		t.Fatalf("read report target: %v", err)
	}
	fixture, err := os.ReadFile(filepath.Join("..", "..", "internal", "policyconflict", "testdata", "report.json"))
	if err != nil {
		t.Fatalf("read report fixture: %v", err)
	}
	report, err := policyconflict.DecodeReport(fixture)
	if err != nil {
		t.Fatalf("DecodeReport fixture: %v", err)
	}
	report.Digest = ""
	sum := sha256.Sum256(specBytes)
	report.Input.Target = policyconflict.TargetIdentity{
		Kind: policyconflict.TargetAcceptanceCandidate,
		Candidate: &policyconflict.CandidateIdentity{
			Ref:           candidate.Spec,
			Path:          store.ActiveSpecRelPath(ref.Name),
			Branch:        candidate.Expected.Branch,
			Head:          candidate.Expected.Head,
			Blob:          strings.Repeat("b", 40),
			ContentDigest: "sha256:" + hex.EncodeToString(sum[:]),
			Scope:         candidate.Scope,
			Adapter:       candidate.Adapter,
			GrantDigest:   "sha256:" + strings.Repeat("d", 64),
		},
	}
	report.Input.Repository.Branch = repositoryfacts.StringFact{Known: true, Value: candidate.Expected.Branch}
	report.Input.Repository.Head = repositoryfacts.StringFact{Known: true, Value: candidate.Expected.Head}
	report.Semantic = []policyconflict.SemanticEvaluation{}
	report.Verdict = policyconflict.VerdictPass
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}
	decoded, err := policyconflict.DecodeReport(encoded)
	if err != nil {
		t.Fatalf("DecodeReport: %v", err)
	}
	return policyconflict.Result{Report: decoded, ReportBytes: encoded}
}

// TestReadinessLoadBuilderHandsOffTheCacheOnlyLoaderAndDefaultSpec proves
// readinessLoadBuilder.Build's own side of the Task-3 hand-off (threaded
// through runServe's own return path — exit obligation: no more
// package-level loader/default-spec hand-off variables): Build's returned
// ref names the request's own target, and after Build's real JudgeRun
// warm-up, a Loader constructed the SAME way cmdServeWithDeps constructs
// its own (root, the request path, BoardHref, Actors — Judge left at its
// zero value, JudgeCacheOnly) independently reaches a real, successful
// derivation for that ref WITHOUT ever launching the judge process a
// second time — proven directly with a counting judge script, not merely
// by comparing two outcomes that could coincidentally agree.
func TestReadinessLoadBuilderHandsOffTheCacheOnlyLoaderAndDefaultSpec(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	counterPath := filepath.Join(t.TempDir(), "judge-calls")
	judge := writeContextConflictJudge(t, "c=$(cat '"+counterPath+"' 2>/dev/null || echo 0); echo $((c+1)) > '"+counterPath+"'; printf '%s\\n' '"+contextConflictNoConflictJudgeResult+"'")
	configureContextConflictJudge(t, repo, judge, 0)
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	requestPath := writeContextRequestFile(t, repo.Dir, "readiness-request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	defaultSpec, predecoded, err := (readinessLoadBuilder{}).Build(context.Background(), repo.Dir, requestPath)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if defaultSpec != "spec/feature-alpha" {
		t.Fatalf("defaultSpec = %q, want %q", defaultSpec, "spec/feature-alpha")
	}
	// R-RR1-17: Build hands back the exact bytes it validated, so the
	// per-request loader below never re-reads the request file.
	if predecoded == nil || len(predecoded.Bytes) == 0 || predecoded.Request.Spec != defaultSpec {
		t.Fatalf("Build predecoded = %+v, want the startup-validated bundle for %q", predecoded, defaultSpec)
	}
	callsAfterWarm, err := os.ReadFile(counterPath)
	if err != nil || strings.TrimSpace(string(callsAfterWarm)) != "1" {
		t.Fatalf("judge calls after the warm-up = %q (err %v), want exactly 1", callsAfterWarm, err)
	}

	// The SAME loader construction cmdServeWithDeps itself performs when a
	// --context-request was supplied (serve.go).
	loader := readinessload.Loader{Root: repo.Dir, Opts: readinessload.Options{
		ContextRequestPath: requestPath, BoardHref: workbench.BranchBoardHref, Actors: resolveConflictActors,
		PredecodedRequest: predecoded,
	}}
	if loader.Opts.Judge != "" {
		t.Fatalf("hand-off loader Judge = %q, want the zero value (normalizes to JudgeCacheOnly)", loader.Opts.Judge)
	}
	cached, err := loader.Load(context.Background(), defaultSpec)
	if err != nil {
		t.Fatalf("hand-off loader.Load: %v", err)
	}
	if cached.Head != repo.Head || cached.TargetRef != defaultSpec {
		t.Fatalf("hand-off loader snapshot = %+v, want Head %q and TargetRef %q", cached, repo.Head, defaultSpec)
	}
	callsAfterCacheOnly, err := os.ReadFile(counterPath)
	if err != nil || strings.TrimSpace(string(callsAfterCacheOnly)) != "1" {
		t.Fatalf("judge calls after the cache-only hand-off load = %q (err %v), want STILL exactly 1 (no second launch)", callsAfterCacheOnly, err)
	}
}

// TestReadinessLoadBuilderRefusesAnAlreadyStaleStartupRequest is R-RRF-3's
// (SI-214) warm-up half. A per-request load now DISCLOSES a stale
// `expected` branch/HEAD claim instead of failing
// (internal/readinessload's own
// TestLoad_ExpectedMismatchPostureIsTheExplicitOption), so the startup
// warm-up became the ONE caller that must still refuse one: a request that
// cannot describe the checkout the server is about to serve is a
// misconfiguration the operator has to see at once, not something a page
// discloses quietly for the life of the process.
// readinessLoadBuilder.Build sets Options.RequireExpectedMatch on its
// warmOpts alone — cmdServeWithDeps' per-request loaderOpts never does —
// and deleting that one line leaves the refusing rows below red while the
// disclosure behaviour stays green.
func TestReadinessLoadBuilderRefusesAnAlreadyStaleStartupRequest(t *testing.T) {
	for _, tc := range []struct {
		name    string
		branch  string // "" keeps the checkout's own branch
		head    string // "" keeps the repository's own HEAD
		wantErr bool
	}{
		{name: "an expected claim describing the checkout warms up"},
		{name: "an already-stale expected HEAD is refused", head: strings.Repeat("a", 40), wantErr: true},
		{name: "an already-stale expected branch is refused", branch: "design/other", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := buildContextCompileRepo(t, map[string]string{
				".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
			})
			judge := writeContextConflictJudge(t, "printf '%s\\n' '"+contextConflictNoConflictJudgeResult+"'")
			configureContextConflictJudge(t, repo, judge, 0)
			checkoutBranch(t, repo.Dir, "design/feature-alpha")
			expected := contextcompile.Expected{Branch: "design/feature-alpha", Head: repo.Head}
			if tc.branch != "" {
				expected.Branch = tc.branch
			}
			if tc.head != "" {
				expected.Head = tc.head
			}
			requestPath := writeContextRequestFile(t, repo.Dir, "readiness-request.json",
				contextRequestBytesWithExpected(t, "spec/feature-alpha", expected))

			defaultSpec, predecoded, err := (readinessLoadBuilder{}).Build(context.Background(), repo.Dir, requestPath)
			if !tc.wantErr {
				if err != nil {
					t.Fatalf("Build: %v", err)
				}
				if defaultSpec != "spec/feature-alpha" || predecoded == nil || predecoded.Request.Expected == nil {
					t.Fatalf("Build = (%q, %+v), want the warmed startup bundle for spec/feature-alpha", defaultSpec, predecoded)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "expected repository") {
				t.Fatalf("Build error = %v, want the warm-up's expected-identity refusal", err)
			}
			// …and `verdi serve` itself exits 2 on it, never entering the
			// run: an operational failure, the exit code every verb owes a
			// misconfiguration (CLAUDE.md 0/1/2).
			var stdout, stderr bytes.Buffer
			deps := serveCommandDeps{
				findRoot:  func(string) (string, error) { return repo.Dir, nil },
				readiness: readinessLoadBuilder{},
				run: func(string, string, readinessload.Loader, string, io.Writer, io.Writer) int {
					t.Error("serve entered its run with an already-stale --context-request")
					return 0
				},
			}
			if code := cmdServeWithDeps([]string{"--http", "127.0.0.1:0", "--context-request", requestPath}, &stdout, &stderr, deps); code != 2 {
				t.Fatalf("serve exit = %d, want 2; stderr=%q", code, stderr.String())
			}
			if !strings.Contains(stderr.String(), "expected repository") {
				t.Fatalf("serve stderr = %q, want the expected-identity refusal", stderr.String())
			}
		})
	}
}

// contextRequestBytesWithExpected is contextRequestBytes plus the optional
// `expected` repository claim R-RRF-3 turns on — built through
// contextcompile's own EncodeRequest seam, never hand-authored JSON.
func contextRequestBytesWithExpected(t *testing.T, spec string, expected contextcompile.Expected) []byte {
	t.Helper()
	data, err := contextcompile.EncodeRequest(contextcompile.Request{
		Schema:   contextcompile.RequestSchema,
		Adapter:  contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Phase:    contextcompile.PhaseDesign,
		Scope:    policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
		Spec:     spec,
		Expected: &expected,
	})
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	return data
}

// TestServeContextRequestReadinessReachesGetDocumentOverSocket is fix
// round 1 F1 (task-4-review.md): runServe's own wiring
// (`srv.Backend.ReadinessLoader = readinessLoader`, serve.go — spec/
// readiness-recovery ac-4) had zero coverage. Every other readiness/
// get_document test either injects a fake `run` into cmdServeWithDeps
// (this file's TestServeContextRequest* trio above, which therefore
// never reach the real runServe body the wiring line lives in) or builds
// mcpserve.Backend{ReadinessLoader: ...} by hand (internal/mcpserve,
// cmd/verdi/document_parity_e2e_test.go) — reviewer mutant M5 (delete
// the wiring line) left the entire ./cmd/verdi/ and
// ./internal/mcpserve/ suites green. This test starts a REAL `verdi
// serve --context-request <fixture>` subprocess — so it goes through the
// genuine, unfaked readinessLoadBuilder{} runServe always uses —
// and drives a REAL get_document call over its live MCP socket, exactly
// mirroring TestServeMutateDraftUsesHeldWriterLock's real-socket-dial
// pattern below. A regression that drops the wiring line makes this test
// fail (verified directly: see the fix-round report).
//
// The readiness fixture recipe (buildContextCompileRepo +
// writeContextConflictJudge + configureContextConflictJudge +
// checkoutBranch) is the SAME hermetic, no-network recipe
// TestReadinessSnapshotPersistenceBoundaryAndD4Cache's "real semantic
// miss" subtest (readiness_snapshot_integration_test.go) already proves,
// in-process, resolves through the real (unfaked) policy-conflict
// provider: a local shell script configured as align.judge_cmd stands in
// for a live judge, so no network or LLM call is ever made.
func TestServeContextRequestReadinessReachesGetDocumentOverSocket(t *testing.T) {
	bin := buildVerdiBinary(t)

	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
	})
	judge := writeContextConflictJudge(t, "printf '%s\\n' '"+contextConflictNoConflictJudgeResult+"'")
	configureContextConflictJudge(t, repo, judge, 0)
	checkoutBranch(t, repo.Dir, "design/feature-alpha")
	requestPath := writeContextRequestFile(t, repo.Dir, "readiness-request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	serveCmd := exec.Command(bin, "serve", "--http", "127.0.0.1:0", "--context-request", requestPath)
	serveCmd.Dir = filepath.FromSlash(repo.Dir)
	serveCmd.Env = commandEnvironment(map[string]string{"CI_DEFAULT_BRANCH": "main"})
	var stdout, stderr syncBuffer
	serveCmd.Stdout, serveCmd.Stderr = &stdout, &stderr
	if err := serveCmd.Start(); err != nil {
		t.Fatalf("starting verdi serve --context-request: %v", err)
	}
	t.Cleanup(func() {
		_ = serveCmd.Process.Signal(syscall.SIGTERM)
		_ = serveCmd.Wait()
	})

	sockPath := waitForPointerFile(t, repo.Dir, 10*time.Second)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dialing serve's MCP socket: %v", err)
	}
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)

	initResp := ndjsonRPC(t, conn, sc, 1, "initialize", map[string]any{"protocolVersion": mcpserve.ProtocolVersion})
	if _, ok := initResp["result"].(map[string]any); !ok {
		t.Fatalf("initialize: no result: %#v", initResp)
	}

	callResp := ndjsonRPC(t, conn, sc, 2, "tools/call", map[string]any{
		"name":      "get_document",
		"arguments": map[string]any{"ref": "spec/feature-alpha"},
	})
	result, ok := callResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call get_document: no result: %#v\nserve stdout:\n%s\nserve stderr:\n%s", callResp, stdout.String(), stderr.String())
	}
	if isErr, _ := result["isError"].(bool); isErr {
		t.Fatalf("get_document over the live serve MCP socket returned an error result: %s", toolCallResultText(t, result))
	}
	var doc struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(toolCallResultText(t, result)), &doc); err != nil {
		t.Fatalf("decoding get_document result: %v", err)
	}
	if !strings.Contains(doc.Markdown, "## Readiness") || strings.Contains(doc.Markdown, "Readiness was not supplied for this render.") {
		t.Fatalf("get_document over the REAL served MCP socket did not carry a populated readiness section — exactly the regression runServe's `srv.Backend.ReadinessLoader = readinessLoader` wiring guards against:\n%s", doc.Markdown)
	}
}

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

// buildVerdiBinary builds the real verdi binary once per test run
// (shared across every test in this file via sync.Once) and returns its
// path.
func buildVerdiBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		// t.TempDir() is per-test and would be removed at test cleanup,
		// which would delete the shared binary out from under later
		// tests in this file — build into a fresh, unmanaged temp dir
		// instead (shared for the whole test binary's run).
		binDir, err := os.MkdirTemp("", "verdi-bin")
		if err != nil {
			buildErr = err
			return
		}
		bin := filepath.Join(binDir, "verdi")
		cmd := exec.Command("go", "build", "-o", bin, ".")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("building verdi binary: %w\n%s", err, out.String())
			return
		}
		builtBin = bin
	})
	if buildErr != nil {
		t.Fatalf("buildVerdiBinary: %v", buildErr)
	}
	return builtBin
}

// newIntegrationStoreRoot builds a minimal, real store root (a real git
// checkout via internal/fixturegit — no golden SHAs pinned or asserted;
// this test only needs A store root that store.FindRoot accepts).
func newIntegrationStoreRoot(t *testing.T) string {
	t.Helper()
	manifest := "schema: verdi.layout/v1\n"
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": manifest, ".verdi/.gitignore": "data/\n"},
		Message: "store root",
	}})
	return repo.Dir
}

// waitForPointerFile polls for root's .verdi/data/serve.path to appear
// and be readable, returning the socket path it names. Fails the test if
// it doesn't appear within timeout.
func waitForPointerFile(t *testing.T, root string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if sockPath, err := mcpserve.ReadPointerFile(root); err == nil {
			return sockPath
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pointer file at %s/.verdi/data/serve.path did not appear within %s", root, timeout)
	return ""
}

// readLockInfo reads and decodes root's writer.lock.
func readLockInfo(t *testing.T, root string) filelock.Info {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "writer.lock"))
	if err != nil {
		t.Fatalf("reading writer.lock: %v", err)
	}
	var info filelock.Info
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("decoding writer.lock: %v", err)
	}
	return info
}

// waitForLockPID polls until root's writer.lock names wantPID, or fails
// the test after timeout. Used instead of waitForPointerFile when a PRIOR
// holder crashed without cleanup: the pointer file's socket path is a
// deterministic function of the checkout root (I-29), so a stale pointer
// file left behind by a crashed holder is indistinguishable, by content
// alone, from a fresh one the new holder just wrote — polling the lock's
// pid is what actually proves a specific process is the current writer.
func waitForLockPID(t *testing.T, root string, wantPID int, timeout time.Duration) filelock.Info {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last filelock.Info
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(root, ".verdi", "data", "writer.lock"))
		if err == nil {
			var info filelock.Info
			if json.Unmarshal(data, &info) == nil {
				last = info
				if info.PID == wantPID {
					return info
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("writer.lock never showed pid %d within %s (last seen: %+v)", wantPID, timeout, last)
	return filelock.Info{}
}

// ndjsonRPC sends one JSON-RPC request over w and reads/decodes one
// response line from sc — the same wire shape internal/mcpserve.wire.go
// speaks, driven here from the OUTSIDE as a real client would.
func ndjsonRPC(t *testing.T, w io.Writer, sc *bufio.Scanner, id int, method string, params any) map[string]any {
	t.Helper()
	req := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		req["params"] = params
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshaling request: %v", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		t.Fatalf("writing request: %v", err)
	}
	if !sc.Scan() {
		t.Fatalf("no response to %s (scan error: %v)", method, sc.Err())
	}
	var resp map[string]any
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response %q: %v", sc.Text(), err)
	}
	return resp
}

// TestD3_ConcurrentSecondProcessRoutesThroughSocket is PLAN.md Phase 9's
// exit criterion: "a concurrent second process routes through the socket
// (no second writer — D3 integration test)". Process A (`verdi serve`) is
// the one writer; process B (`verdi mcp`, the shim) is a second, fully
// independent OS process that answers a real MCP handshake by proxying
// through A's socket; a THIRD attempted `verdi serve` (process C),
// started while A is still the writer, is proven to fail rather than
// silently becoming a second writer.
func TestD3_ConcurrentSecondProcessRoutesThroughSocket(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := newIntegrationStoreRoot(t)

	// Process A: the writer.
	serveCmd := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	serveCmd.Dir = root
	if err := serveCmd.Start(); err != nil {
		t.Fatalf("starting verdi serve: %v", err)
	}
	t.Cleanup(func() {
		_ = serveCmd.Process.Signal(syscall.SIGTERM)
		_ = serveCmd.Wait()
	})
	waitForPointerFile(t, root, 10*time.Second)

	// Process C: a second `verdi serve` attempted while A holds the lock
	// — must fail (D3's "one writer" guarantee), not silently start a
	// second writer.
	secondServe := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	secondServe.Dir = root
	var secondErrOut bytes.Buffer
	secondServe.Stderr = &secondErrOut
	if err := secondServe.Run(); err == nil {
		t.Fatal("a second `verdi serve` while the first holds the writer lock succeeded — D3's single-writer guarantee is violated")
	}
	if secondErrOut.Len() == 0 {
		t.Fatal("a second `verdi serve` failed silently with no explanation on stderr")
	}

	// Process B: `verdi mcp`, a second, independent OS process — proxies
	// through process A's socket rather than becoming a writer itself.
	mcpCmd := exec.Command(bin, "mcp")
	mcpCmd.Dir = root
	stdin, err := mcpCmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := mcpCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := mcpCmd.Start(); err != nil {
		t.Fatalf("starting verdi mcp: %v", err)
	}
	// Mirrors serveCmd's own t.Cleanup above: without this, a t.Fatalf
	// anywhere below (e.g. the tools/list count assertion) skips the
	// happy-path stdin.Close()/mcpCmd.Wait() at the end of this function
	// via runtime.Goexit, leaking this child process for the rest of the
	// whole-package test binary's run — under `go test -race ./...` that
	// stranded `verdi mcp` process holds its stdout pipe open until the
	// ENTIRE test binary exits, turning one assertion failure into a
	// ~10-minute hang instead of a fast, clean failure. Safe to run
	// unconditionally: after the happy-path Wait() below already
	// succeeded, signaling an exited process and calling Wait() a second
	// time both just return an (ignored) error.
	t.Cleanup(func() {
		_ = mcpCmd.Process.Signal(syscall.SIGTERM)
		_ = mcpCmd.Wait()
	})

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	resp := ndjsonRPC(t, stdin, sc, 1, "initialize", map[string]any{"protocolVersion": mcpserve.ProtocolVersion})
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("verdi mcp initialize: no result in response: %#v", resp)
	}
	if result["protocolVersion"] != mcpserve.ProtocolVersion {
		t.Fatalf("verdi mcp initialize: protocolVersion = %v, want %s", result["protocolVersion"], mcpserve.ProtocolVersion)
	}

	// tools/list through the shim too — a second full round-trip proving
	// this is a real, working proxy, not a one-shot fluke. The count is the
	// authoritative live inventory: 05 §MCP server's nine tools,
	// `experiment` (CSE Wave 5B, ledger SI-145), Wave 6 Task 1's five new
	// ASD tools (AC-8), Wave 6 Task 3's three new constitution tools
	// (spec/context-integrity-v2 AC-1/AC-2/AC-3), `get_document`
	// (spec-documents Wave 2 Task 3, ac-5's Markdown renderer),
	// `import_preview`/`import_apply` (spec-documents Wave 3 Task 3, ac-9),
	// and `get_recovery` (spec/readiness-recovery-v2 ac-8, ac-10's MCP
	// half, wave 3 Task 3) — the same twenty-two mcpserve/server_test.go
	// and specalign's TestMCPToolInventory pin.
	toolsResp := ndjsonRPC(t, stdin, sc, 2, "tools/list", nil)
	toolsResult, ok := toolsResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("verdi mcp tools/list: no result: %#v", toolsResp)
	}
	tools, _ := toolsResult["tools"].([]any)
	if len(tools) != 22 {
		t.Fatalf("verdi mcp tools/list returned %d tools through the socket, want 22", len(tools))
	}

	// Clean up process B: closing stdin signals EOF on the stdin->socket
	// direction, which the shim's fixed shutdown (I-13/S4) exits on.
	stdin.Close()
	if err := mcpCmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 0 {
			t.Fatalf("verdi mcp exited abnormally after stdin close: %v", err)
		}
	}
}

// TestLockTakeover_AfterSIGKILL is PLAN.md Phase 9's exit criterion:
// "lock takeover after SIGKILL of the holder". Process A is SIGKILLed
// (no clean shutdown, no lock/socket cleanup — exactly a crash);
// process B, started against the same root afterward, must take over the
// lock (I-12) and become the new writer.
func TestLockTakeover_AfterSIGKILL(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := newIntegrationStoreRoot(t)

	a := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	a.Dir = root
	if err := a.Start(); err != nil {
		t.Fatalf("starting verdi serve (A): %v", err)
	}
	waitForPointerFile(t, root, 10*time.Second)
	infoA := readLockInfo(t, root)
	if infoA.PID != a.Process.Pid {
		t.Fatalf("lock pid = %d, want A's pid %d", infoA.PID, a.Process.Pid)
	}

	if err := a.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("SIGKILL A: %v", err)
	}
	_ = a.Wait() // reap; ignore the (expected) killed-signal error

	b := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	b.Dir = root
	if err := b.Start(); err != nil {
		t.Fatalf("starting verdi serve (B): %v", err)
	}
	t.Cleanup(func() {
		_ = b.Process.Signal(syscall.SIGTERM)
		_ = b.Wait()
	})
	infoB := waitForLockPID(t, root, b.Process.Pid, 10*time.Second)
	waitForPointerFile(t, root, 10*time.Second) // B's own socket is up too
	if infoB.PID == infoA.PID {
		t.Fatalf("B somehow reused A's exact pid %d — test is not meaningfully distinguishing the two holders", infoA.PID)
	}
}

// TestShim_ExitsPromptlyWhenServeDies is the S4-binding regression test
// PLAN.md Phase 9 calls for: "shim exits promptly when serve dies (S4's
// hang case, now a regression test)". The shim's stdin is deliberately
// held open (never closed by this test) — an MCP client does not close
// stdin between calls — so the ONLY way the shim can exit is via the
// socket->stdout direction ending when serve dies; a naive
// wait-for-both-directions shutdown would hang here forever.
func TestShim_ExitsPromptlyWhenServeDies(t *testing.T) {
	bin := buildVerdiBinary(t)
	root := newIntegrationStoreRoot(t)

	serveCmd := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	serveCmd.Dir = root
	if err := serveCmd.Start(); err != nil {
		t.Fatalf("starting verdi serve: %v", err)
	}
	waitForPointerFile(t, root, 10*time.Second)

	mcpCmd := exec.Command(bin, "mcp")
	mcpCmd.Dir = root
	stdin, err := mcpCmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe: %v", err)
	}
	stdout, err := mcpCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := mcpCmd.Start(); err != nil {
		t.Fatalf("starting verdi mcp: %v", err)
	}
	// stdin is intentionally never closed by this test — see doc comment.
	defer stdin.Close()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)
	resp := ndjsonRPC(t, stdin, sc, 1, "initialize", map[string]any{"protocolVersion": mcpserve.ProtocolVersion})
	if resp["result"] == nil {
		t.Fatalf("verdi mcp initialize before killing serve: no result: %#v", resp)
	}

	// Kill serve out from under the shim — no clean shutdown, exactly
	// like a crash.
	if err := serveCmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("SIGKILL serve: %v", err)
	}
	_ = serveCmd.Wait()

	waitDone := make(chan error, 1)
	go func() { waitDone <- mcpCmd.Wait() }()

	select {
	case <-waitDone:
		// Exited promptly — the fix. (Exit code is not asserted: on some
		// platforms a half-closed socket read surfaces as a nonzero-but-
		// clean-shutdown exit; promptness is the property under test.)
	case <-time.After(5 * time.Second):
		_ = mcpCmd.Process.Kill() // don't leak the hung process even though the test already failed
		t.Fatal("verdi mcp did not exit within 5s of serve being killed — this is the S4 hang case: a naive wait-for-both-directions shutdown blocks forever on the still-open stdin Read")
	}
}

// toolCallResultText extracts an MCP tool result's first text content item
// — the same shape internal/mcpserve.toolText/toolJSON produce — driven
// here from the OUTSIDE, over the real socket, exactly like ndjsonRPC.
func toolCallResultText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("tool result has no content: %#v", result)
	}
	item, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("tool result content[0] is not an object: %#v", content[0])
	}
	text, ok := item["text"].(string)
	if !ok {
		t.Fatalf("tool result content[0] has no text: %#v", item)
	}
	return text
}

// TestServeMutateDraftUsesHeldWriterLock is Task 1B's second required
// semantic RED (Wave 6 design §6.1.2, ledger SI-69/SI-177): `verdi serve`
// acquires the checkout's writer lock for its entire lifetime (I-12), and
// every draftmutation.Service.Mutate call — including the one this real
// mutate_draft MCP call drives — used to attempt that SAME non-reentrant
// lock a second time and fail operationally before the transaction ever
// began, even though serve is the sole writer and nothing is actually
// contended. SI-177 narrowly supersedes that refusal only for this exact
// caller process's registry-proven outer lock.
//
// Base RED (pre-fix): the typed MCP result is isError with the exact
// {"classification":"operational","code":"io-failure",...} envelope
// (internal/designapp.MutationFailure over draftmutation's CodeIOFailure),
// and zero mutation — the spec/provenance sidecar on disk is untouched.
// GREEN: the canonical clean mutate_draft result, the exact spec and
// provenance mutation landed on disk, and the outer writer lock proven to
// remain the SAME continuously-held open file (never released and
// recreated) across the whole served call.
func TestServeMutateDraftUsesHeldWriterLock(t *testing.T) {
	bin := buildVerdiBinary(t)
	root, head, base := designMutateStore(t)

	serve := exec.Command(bin, "serve", "--http", "127.0.0.1:0")
	serve.Dir = filepath.FromSlash(root)
	serve.Env = commandEnvironment(map[string]string{"CI_DEFAULT_BRANCH": "main"})
	var serveOutput bytes.Buffer
	serve.Stdout, serve.Stderr = &serveOutput, &serveOutput
	if err := serve.Start(); err != nil {
		t.Fatalf("starting verdi serve: %v", err)
	}
	t.Cleanup(func() {
		_ = serve.Process.Signal(syscall.SIGTERM)
		_ = serve.Wait()
	})

	sockPath := waitForPointerFile(t, filepath.FromSlash(root), 10*time.Second)
	lockPath := filepath.Join(filepath.FromSlash(root), ".verdi", "data", "writer.lock")
	lockBefore, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("stating writer lock before the served mutation: %v", err)
	}
	infoBefore := readLockInfo(t, root)
	if infoBefore.PID != serve.Process.Pid {
		t.Fatalf("writer lock pid = %d, want serve's own pid %d", infoBefore.PID, serve.Process.Pid)
	}

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dialing serve's MCP socket: %v", err)
	}
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<24)

	initResp := ndjsonRPC(t, conn, sc, 1, "initialize", map[string]any{"protocolVersion": mcpserve.ProtocolVersion})
	if _, ok := initResp["result"].(map[string]any); !ok {
		t.Fatalf("initialize: no result: %#v", initResp)
	}

	requestBytes := designMutateRequest(t, root, "design/sample", head, base, []map[string]any{
		{"op": "set-problem", "text": "served mutation", "anchor": "#problem"},
	})
	var args map[string]any
	if err := json.Unmarshal(requestBytes, &args); err != nil {
		t.Fatal(err)
	}
	args["harness"] = "codex"
	args["session"] = "session-1"

	callResp := ndjsonRPC(t, conn, sc, 2, "tools/call", map[string]any{"name": "mutate_draft", "arguments": args})
	result, ok := callResp["result"].(map[string]any)
	if !ok {
		t.Fatalf("tools/call mutate_draft: no result: %#v", callResp)
	}
	if isErr, _ := result["isError"].(bool); isErr {
		// Zero-mutation proof for the exact failure this base RED captures:
		// a typed operational/io-failure refusal must never have touched the
		// spec on disk.
		unmutated, readErr := os.ReadFile(store.SpecPath(filepath.FromSlash(root), store.ZoneActive, "sample"))
		t.Fatalf("mutate_draft over the live serve MCP socket returned an error result (want the canonical clean result): %s\nspec on disk unchanged=%t (read err=%v)\nserve output:\n%s",
			toolCallResultText(t, result), readErr == nil && bytes.Equal(unmutated, base), readErr, serveOutput.String())
	}

	mutation := decodeMutationResult(t, toolCallResultText(t, result))
	if len(mutation.Changes) != 1 || mutation.Changes[0].Target != "problem" || mutation.Changes[0].Change != "replaced" {
		t.Fatalf("mutate_draft changes = %+v", mutation.Changes)
	}

	specBytes, err := os.ReadFile(store.SpecPath(filepath.FromSlash(root), store.ZoneActive, "sample"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(specBytes, []byte("served mutation")) {
		t.Fatalf("spec on disk was not mutated by the served mutate_draft call: %s", specBytes)
	}
	logBytes, err := os.ReadFile(store.DesignProvenancePath(filepath.FromSlash(root), store.ZoneActive, "sample"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := designprovenance.DecodeLog(logBytes)
	if err != nil || len(entries) != 1 {
		t.Fatalf("provenance = %+v, %v", entries, err)
	}
	if !entries[0].Attribution.Unauthenticated || entries[0].Harness != "codex" {
		t.Fatalf("served mutation provenance attribution = %+v", entries[0])
	}

	// The outer writer lock must remain the SAME open file identity
	// throughout — proof it was never released and recreated by the
	// mutation's inner reuse of serve's own lifetime lock.
	lockAfter, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("writer lock is gone after the served mutation: %v", err)
	}
	if !os.SameFile(lockBefore, lockAfter) {
		t.Fatal("writer lock file identity changed across the served mutation — it was released and recreated, not held continuously")
	}
	infoAfter := readLockInfo(t, root)
	if infoAfter.PID != serve.Process.Pid {
		t.Fatalf("writer lock pid after mutation = %d, want serve's own pid %d (lock must remain continuously held by serve)", infoAfter.PID, serve.Process.Pid)
	}
}
