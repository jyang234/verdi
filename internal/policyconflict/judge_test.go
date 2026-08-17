// Task 7 Step 1 RED matrix for JudgeAdapter (authority design §6, ledger
// SI-96): hermetic fake JudgeRunner covering start failure, nonzero exit,
// timeout, cancellation, malformed output, and success. Test names match
// -run 'TestJudgeAdapter'.
package policyconflict

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/contextcompile"
)

// fakeJudgeRunner is a hermetic, deterministic JudgeRunner: every call is
// recorded, and fn decides the response — no real process, no network.
type fakeJudgeRunner struct {
	fn        func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error)
	calls     int
	lastArgv  []string
	lastStdin []byte
}

func (f *fakeJudgeRunner) Run(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
	f.calls++
	f.lastArgv = append([]string(nil), argv...)
	f.lastStdin = append([]byte(nil), stdin...)
	return f.fn(ctx, argv, stdin)
}

func noConflictResultBytes(t *testing.T) []byte {
	t.Helper()
	out, err := EncodeJudgeResult(JudgeResult{Schema: JudgeResultSchema, Recommendation: RecommendationNoConflict, Findings: []JudgeFinding{}})
	if err != nil {
		t.Fatalf("EncodeJudgeResult: %v", err)
	}
	return out
}

func baseAdapter(runner JudgeRunner) JudgeAdapter {
	return JudgeAdapter{
		Role:    "primary",
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Model:   "judge-model-v1",
		Argv:    []string{"judge-bin", "--stdin"},
		Root:    "/store",
		Runner:  runner,
	}
}

func TestJudgeAdapter_Success(t *testing.T) {
	want := noConflictResultBytes(t)
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		return want, 0, nil
	}}
	a := baseAdapter(runner)
	prompt := []byte("prompt bytes")
	input := []byte(`{"claims":[]}`)

	got, err := a.Judge(context.Background(), prompt, input)
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner.calls = %d, want 1", runner.calls)
	}
	if got.Role != JudgePrimary {
		t.Errorf("Role = %q, want %q", got.Role, JudgePrimary)
	}
	if got.Adapter != a.Adapter {
		t.Errorf("Adapter = %+v, want %+v", got.Adapter, a.Adapter)
	}
	if got.Model != a.Model {
		t.Errorf("Model = %q, want %q (never taken from output)", got.Model, a.Model)
	}
	if got.PromptDigest != rawContentDigest(prompt) {
		t.Errorf("PromptDigest = %q, want digest of the exact prompt bytes sent", got.PromptDigest)
	}
	if got.InputDigest != rawContentDigest(input) {
		t.Errorf("InputDigest = %q, want digest of the exact input bytes sent", got.InputDigest)
	}
	if got.RawResult != string(want) {
		t.Errorf("RawResult = %q, want the exact process stdout", got.RawResult)
	}
	if got.RawDigest != rawContentDigest(want) {
		t.Errorf("RawDigest = %q, want digest of the exact raw stdout", got.RawDigest)
	}
	if got.Result.Recommendation != RecommendationNoConflict {
		t.Errorf("Result.Recommendation = %q, want %q", got.Result.Recommendation, RecommendationNoConflict)
	}
}

func TestJudgeAdapter_RejectsNULArgvBeforeRun(t *testing.T) {
	runner := &fakeJudgeRunner{fn: func(context.Context, []string, []byte) ([]byte, int, error) {
		t.Fatal("runner must not receive an argv value that cannot reach exec")
		return nil, 0, nil
	}}
	a := baseAdapter(runner)
	a.Argv = []string{"judge-bin", "a\x00b"}

	if _, err := a.Judge(context.Background(), []byte("prompt"), []byte(`{}`)); err == nil {
		t.Fatal("Judge accepted an argv element containing NUL")
	}
	if runner.calls != 0 {
		t.Fatalf("runner.calls = %d, want 0", runner.calls)
	}
}

func TestJudgeAdapter_StartFailure(t *testing.T) {
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		return nil, 0, errors.New("exec: \"judge-bin\": executable file not found in $PATH")
	}}
	a := baseAdapter(runner)
	_, err := a.Judge(context.Background(), []byte("p"), []byte("{}"))
	if err == nil {
		t.Fatal("expected an error for a start failure")
	}
	if !errors.Is(err, ErrJudgeOperational) {
		t.Fatalf("error = %v, want it to wrap ErrJudgeOperational", err)
	}
}

func TestJudgeAdapter_NonzeroExit(t *testing.T) {
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		return []byte("stderr-ish output"), 1, nil
	}}
	a := baseAdapter(runner)
	_, err := a.Judge(context.Background(), []byte("p"), []byte("{}"))
	if err == nil {
		t.Fatal("expected an error for a nonzero exit")
	}
	if !errors.Is(err, ErrJudgeOperational) {
		t.Fatalf("error = %v, want it to wrap ErrJudgeOperational", err)
	}
}

func TestJudgeAdapter_MalformedOutput(t *testing.T) {
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		return []byte("not json"), 0, nil
	}}
	a := baseAdapter(runner)
	_, err := a.Judge(context.Background(), []byte("p"), []byte("{}"))
	if err == nil {
		t.Fatal("expected an error for malformed judge output")
	}
	if !errors.Is(err, ErrJudgeOperational) {
		t.Fatalf("error = %v, want it to wrap ErrJudgeOperational", err)
	}
}

// TestJudgeAdapter_Timeout proves an adapter-imposed Timeout that elapses
// before the runner returns is classified as ErrJudgeOperational, mirroring
// align's own proven ctx.Err()==DeadlineExceeded classification (S5).
func TestJudgeAdapter_Timeout(t *testing.T) {
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		<-ctx.Done()
		return nil, 0, ctx.Err()
	}}
	a := baseAdapter(runner)
	a.Timeout = 5 * time.Millisecond
	_, err := a.Judge(context.Background(), []byte("p"), []byte("{}"))
	if err == nil {
		t.Fatal("expected an error for a timed-out judge invocation")
	}
	if !errors.Is(err, ErrJudgeOperational) {
		t.Fatalf("error = %v, want it to wrap ErrJudgeOperational", err)
	}
}

// TestJudgeAdapter_Cancellation proves an outer-context cancellation
// (distinct from an adapter-imposed Timeout — no Timeout is set here) is
// also classified as ErrJudgeOperational.
func TestJudgeAdapter_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		<-ctx.Done()
		return nil, 0, ctx.Err()
	}}
	a := baseAdapter(runner)
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	_, err := a.Judge(ctx, []byte("p"), []byte("{}"))
	if err == nil {
		t.Fatal("expected an error for a canceled judge invocation")
	}
	if !errors.Is(err, ErrJudgeOperational) {
		t.Fatalf("error = %v, want it to wrap ErrJudgeOperational", err)
	}
}

func TestJudgeAdapter_Negative_NilRunner(t *testing.T) {
	a := baseAdapter(nil)
	_, err := a.Judge(context.Background(), []byte("p"), []byte("{}"))
	if !errors.Is(err, ErrJudgeOperational) {
		t.Fatalf("error = %v, want ErrJudgeOperational for a nil runner", err)
	}
}

func TestJudgeAdapter_Negative_EmptyArgv(t *testing.T) {
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		t.Fatal("runner must not be invoked when argv is empty")
		return nil, 0, nil
	}}
	a := baseAdapter(runner)
	a.Argv = nil
	_, err := a.Judge(context.Background(), []byte("p"), []byte("{}"))
	if !errors.Is(err, ErrJudgeOperational) {
		t.Fatalf("error = %v, want ErrJudgeOperational for empty argv", err)
	}
	if runner.calls != 0 {
		t.Fatalf("runner.calls = %d, want 0", runner.calls)
	}
}

func TestJudgeAdapter_Negative_InvalidRole(t *testing.T) {
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		t.Fatal("runner must not be invoked for an invalid role")
		return nil, 0, nil
	}}
	a := baseAdapter(runner)
	a.Role = "bogus"
	_, err := a.Judge(context.Background(), []byte("p"), []byte("{}"))
	if !errors.Is(err, ErrJudgeOperational) {
		t.Fatalf("error = %v, want ErrJudgeOperational for an invalid role", err)
	}
}

func TestJudgeAdapter_Negative_MissingAdapterIdentity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*JudgeAdapter)
	}{
		{"empty adapter id", func(a *JudgeAdapter) { a.Adapter.ID = "" }},
		{"empty adapter version", func(a *JudgeAdapter) { a.Adapter.Version = "" }},
		{"empty model", func(a *JudgeAdapter) { a.Model = "" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
				t.Fatal("runner must not be invoked when adapter identity is incomplete")
				return nil, 0, nil
			}}
			a := baseAdapter(runner)
			tc.mutate(&a)
			_, err := a.Judge(context.Background(), []byte("p"), []byte("{}"))
			if !errors.Is(err, ErrJudgeOperational) {
				t.Fatalf("error = %v, want ErrJudgeOperational", err)
			}
		})
	}
}

// TestJudgeAdapter_ChallengerRole proves the challenger identity round-trips
// exactly like primary — Role is adapter-owned configuration, never
// inferred.
func TestJudgeAdapter_ChallengerRole(t *testing.T) {
	want := noConflictResultBytes(t)
	runner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) {
		return want, 0, nil
	}}
	a := baseAdapter(runner)
	a.Role = "challenger"
	got, err := a.Judge(context.Background(), []byte("p"), []byte("{}"))
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if got.Role != JudgeChallenger {
		t.Fatalf("Role = %q, want %q", got.Role, JudgeChallenger)
	}
}

// TestJudgeAdapter_IdenticalBytesToPrimaryAndChallenger proves primary and
// challenger adapters given the SAME prompt/input bytes compute the SAME
// prompt/input digests independently — the exact independence property
// authority design §6 requires ("Primary and challenger receive the
// identical canonical prompt and normalized semantic input independently").
func TestJudgeAdapter_IdenticalBytesToPrimaryAndChallenger(t *testing.T) {
	want := noConflictResultBytes(t)
	primaryRunner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return want, 0, nil }}
	challengerRunner := &fakeJudgeRunner{fn: func(ctx context.Context, argv []string, stdin []byte) ([]byte, int, error) { return want, 0, nil }}
	primary := baseAdapter(primaryRunner)
	challenger := baseAdapter(challengerRunner)
	challenger.Role = "challenger"

	prompt := []byte("shared prompt")
	input := []byte(`{"claims":[]}`)
	p, err := primary.Judge(context.Background(), prompt, input)
	if err != nil {
		t.Fatalf("primary Judge: %v", err)
	}
	c, err := challenger.Judge(context.Background(), prompt, input)
	if err != nil {
		t.Fatalf("challenger Judge: %v", err)
	}
	if p.PromptDigest != c.PromptDigest {
		t.Errorf("PromptDigest primary=%q challenger=%q, want equal (identical bytes)", p.PromptDigest, c.PromptDigest)
	}
	if p.InputDigest != c.InputDigest {
		t.Errorf("InputDigest primary=%q challenger=%q, want equal (identical bytes)", p.InputDigest, c.InputDigest)
	}
	if p.Role == c.Role {
		t.Errorf("Role primary=%q challenger=%q, want distinct roles despite identical bytes", p.Role, c.Role)
	}
}
