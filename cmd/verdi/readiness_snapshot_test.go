package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/store"
)

func TestReadinessSnapshotRequestBoundary(t *testing.T) {
	t.Run("reads one canonical request exactly once", func(t *testing.T) {
		repo, requestPath, requestBytes, _, _ := readinessSnapshotRepo(t, "feature")
		reads := 0
		builder := localReadinessSnapshotBuilder{
			readFile: func(path string) ([]byte, error) {
				if path == requestPath {
					reads++
				}
				return os.ReadFile(path)
			},
			providerFactory: readinessSnapshotProviderFactory(t, repo.Dir, policyconflict.VerdictPass, nil),
		}

		snapshot, err := builder.Build(context.Background(), repo.Dir, requestPath)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if reads != 1 {
			t.Fatalf("request reads = %d, want exactly 1", reads)
		}
		if snapshot.RequestDigest != testReadinessDigest(requestBytes) {
			t.Fatalf("RequestDigest = %q, want digest of exact canonical request bytes %q", snapshot.RequestDigest, testReadinessDigest(requestBytes))
		}
	})

	t.Run("refuses dot-dot and symlink paths before reading", func(t *testing.T) {
		root := t.TempDir()
		requestBytes := contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil)
		realPath := filepath.Join(root, "request.json")
		if err := os.WriteFile(realPath, requestBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(root, "request-link.json")
		if err := os.Symlink(realPath, linkPath); err != nil {
			t.Fatal(err)
		}

		for _, tc := range []struct {
			name string
			path string
			want string
		}{
			{name: "dot-dot", path: root + string(filepath.Separator) + "nested" + string(filepath.Separator) + ".." + string(filepath.Separator) + "request.json", want: `".."`},
			{name: "symlink", path: linkPath, want: "symlink"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				reads := 0
				builder := localReadinessSnapshotBuilder{readFile: func(string) ([]byte, error) {
					reads++
					return nil, errors.New("must not read")
				}}
				_, err := builder.Build(context.Background(), root, tc.path)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("Build error = %v, want %q refusal", err, tc.want)
				}
				if reads != 0 {
					t.Fatalf("read calls = %d after path refusal, want 0", reads)
				}
			})
		}
	})

	t.Run("requires design phase before repository projection", func(t *testing.T) {
		root := t.TempDir()
		requestPath := filepath.Join(root, "build-request.json")
		if err := os.WriteFile(requestPath, contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseBuild, nil), 0o644); err != nil {
			t.Fatal(err)
		}
		called := false
		builder := localReadinessSnapshotBuilder{projectJourney: func(context.Context, *store.Config, string) (journey.Record, error) {
			called = true
			return journey.Record{}, nil
		}}
		_, err := builder.Build(context.Background(), root, requestPath)
		if err == nil || !strings.Contains(err.Error(), `phase "design"`) {
			t.Fatalf("Build error = %v, want design-phase refusal", err)
		}
		if called {
			t.Fatal("journey projection called for a non-design request")
		}
	})
}

func TestReadinessSnapshotExpectedIdentity(t *testing.T) {
	t.Run("binds absent expected to current design branch and HEAD", func(t *testing.T) {
		repo, requestPath, _, targetRef, _ := readinessSnapshotRepo(t, "feature")
		var captured *policyconflict.AcceptanceCandidate
		factory := func(root string, request policyconflict.Request) (policyconflict.VerdictProvider, error) {
			candidate := request.Target.AcceptanceCandidate
			if candidate != nil {
				copy := *candidate
				captured = &copy
			}
			return contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
				return readinessSnapshotReport(t, root, request, policyconflict.VerdictPass, nil), nil
			}), nil
		}

		snapshot, err := (localReadinessSnapshotBuilder{providerFactory: factory}).Build(context.Background(), repo.Dir, requestPath)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if captured == nil {
			t.Fatal("provider did not receive an acceptance candidate")
		}
		if captured.Expected.Branch != "design/feature-alpha" || captured.Expected.Head != repo.Head {
			t.Fatalf("candidate expected = %+v, want current design branch/HEAD", captured.Expected)
		}
		if captured.Spec != targetRef || snapshot.Branch != captured.Expected.Branch || snapshot.Head != captured.Expected.Head {
			t.Fatalf("candidate/snapshot identity drift: candidate=%+v snapshot=%+v", captured, snapshot)
		}
	})

	t.Run("refuses supplied expected mismatch before evaluation", func(t *testing.T) {
		repo, _, _, targetRef, _ := readinessSnapshotRepo(t, "feature")
		requestPath := contextLifecycleRequestFile(t, repo.Dir, "mismatched-request.json", targetRef, contextcompile.PhaseDesign, &contextcompile.Expected{
			Branch: "design/other",
			Head:   strings.Repeat("a", 40),
		})
		called := false
		builder := localReadinessSnapshotBuilder{providerFactory: func(string, policyconflict.Request) (policyconflict.VerdictProvider, error) {
			called = true
			return nil, errors.New("must not construct provider")
		}}
		_, err := builder.Build(context.Background(), repo.Dir, requestPath)
		if err == nil || !strings.Contains(err.Error(), "expected repository") {
			t.Fatalf("Build error = %v, want expected identity mismatch", err)
		}
		if called {
			t.Fatal("provider constructed after expected identity mismatch")
		}
	})
}

func TestReadinessSnapshotCrossSourceIdentity(t *testing.T) {
	t.Run("journey target mismatch", func(t *testing.T) {
		repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "feature")
		builder := localReadinessSnapshotBuilder{
			projectJourney: func(ctx context.Context, cfg *store.Config, ref string) (journey.Record, error) {
				record, err := journey.NewProjector().Project(ctx, cfg, ref)
				record.Target.Ref = "spec/other"
				return record, err
			},
			providerFactory: readinessSnapshotProviderFactory(t, repo.Dir, policyconflict.VerdictPass, nil),
		}
		_, err := builder.Build(context.Background(), repo.Dir, requestPath)
		if err == nil || !strings.Contains(err.Error(), "journey target") {
			t.Fatalf("Build error = %v, want journey target mismatch", err)
		}
	})

	t.Run("report target mismatch", func(t *testing.T) {
		repo, requestPath, _, _, _ := readinessSnapshotRepo(t, "feature")
		builder := localReadinessSnapshotBuilder{providerFactory: readinessSnapshotProviderFactory(t, repo.Dir, policyconflict.VerdictPass, func(report *policyconflict.Report) {
			report.Input.Target.Candidate.Ref = "spec/other"
		})}
		_, err := builder.Build(context.Background(), repo.Dir, requestPath)
		if err == nil || !strings.Contains(err.Error(), "conflict report target") {
			t.Fatalf("Build error = %v, want report target mismatch", err)
		}
	})

	t.Run("decoded spec target mismatch", func(t *testing.T) {
		repo, requestPath, _, _, specPath := readinessSnapshotRepo(t, "feature")
		builder := localReadinessSnapshotBuilder{
			readFile: func(path string) ([]byte, error) {
				data, err := os.ReadFile(path)
				if err != nil || path != specPath {
					return data, err
				}
				return []byte(strings.Replace(string(data), "id: spec/feature-alpha", "id: spec/feature-other", 1)), nil
			},
			providerFactory: readinessSnapshotProviderFactory(t, repo.Dir, policyconflict.VerdictPass, nil),
		}
		_, err := builder.Build(context.Background(), repo.Dir, requestPath)
		if err == nil || !strings.Contains(err.Error(), "decoded spec") {
			t.Fatalf("Build error = %v, want decoded spec mismatch", err)
		}
	})
}

func readinessSnapshotProviderFactory(t *testing.T, root string, verdict policyconflict.Verdict, mutate func(*policyconflict.Report)) contextConflictProviderFactory {
	t.Helper()
	return func(gotRoot string, request policyconflict.Request) (policyconflict.VerdictProvider, error) {
		if gotRoot != root {
			t.Fatalf("provider root = %q, want %q", gotRoot, root)
		}
		return contextConflictProviderFunc(func(context.Context, policyconflict.Request) (policyconflict.Result, error) {
			return readinessSnapshotReport(t, root, request, verdict, mutate), nil
		}), nil
	}
}

func readinessSnapshotReport(t *testing.T, root string, request policyconflict.Request, verdict policyconflict.Verdict, mutate func(*policyconflict.Report)) policyconflict.Result {
	t.Helper()
	candidate := request.Target.AcceptanceCandidate
	if request.Target.Kind != policyconflict.TargetAcceptanceCandidate || candidate == nil {
		t.Fatalf("provider request target = %+v, want one acceptance candidate", request.Target)
	}
	ref, err := artifact.ParseRef(candidate.Spec)
	if err != nil {
		t.Fatalf("ParseRef(%q): %v", candidate.Spec, err)
	}
	specPath := store.ActiveSpecPath(root, ref.Name)
	specBytes, err := os.ReadFile(specPath)
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
	report.Input.Target = policyconflict.TargetIdentity{
		Kind: policyconflict.TargetAcceptanceCandidate,
		Candidate: &policyconflict.CandidateIdentity{
			Ref:           candidate.Spec,
			Path:          store.ActiveSpecRelPath(ref.Name),
			Branch:        candidate.Expected.Branch,
			Head:          candidate.Expected.Head,
			Blob:          strings.Repeat("b", 40),
			ContentDigest: testReadinessDigest(specBytes),
			Scope:         candidate.Scope,
			Adapter:       candidate.Adapter,
			GrantDigest:   "sha256:" + strings.Repeat("d", 64),
		},
	}
	report.Input.Repository.Branch = repositoryfacts.StringFact{Known: true, Value: candidate.Expected.Branch}
	report.Input.Repository.Head = repositoryfacts.StringFact{Known: true, Value: candidate.Expected.Head}
	switch verdict {
	case policyconflict.VerdictPass:
		report.Semantic = []policyconflict.SemanticEvaluation{}
		report.Verdict = verdict
	case policyconflict.VerdictBlockedUnproven:
		report.Verdict = verdict
	case policyconflict.VerdictBlockedViolated:
		report.Semantic[0].State = policyconflict.ProofViolatedWithWitness
		report.Semantic[0].Reasons = []policyconflict.ReasonCode{policyconflict.ReasonDispositionEffectiveConflict}
		report.Verdict = verdict
	default:
		t.Fatalf("unsupported verdict %q", verdict)
	}
	if mutate != nil {
		mutate(&report)
	}
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

func testReadinessDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func readinessSnapshotConcern(t *testing.T, snapshot readinesspilot.Snapshot, id string) readinesspilot.Concern {
	t.Helper()
	for _, concern := range snapshot.AllConcerns {
		if concern.ID == id {
			return concern
		}
	}
	t.Fatalf("snapshot has no concern %q: %+v", id, snapshot.AllConcerns)
	return readinesspilot.Concern{}
}

func assertReadinessStringsContain(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(value, want) {
			return
		}
	}
	t.Fatalf("%q not found in %q", want, values)
}

func assertReadinessCLI(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CLI = %q, want exact token vector %q", got, want)
	}
}
