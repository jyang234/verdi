package upstream

import (
	"context"
	"testing"
)

func TestGraph_Happy(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "graph", Result{Stdout: readCanned(t, "graph.json"), ExitCode: 0})

	g, err := RunGraph(context.Background(), f, "testdata/svcfix", "deadbeef")
	if err != nil {
		t.Fatalf("Graph: %v", err)
	}
	if g.Stamp != "deadbeef" {
		t.Errorf("Stamp = %q, want %q", g.Stamp, "deadbeef")
	}

	// Flags-before-positional, per the recorded call.
	if len(f.Calls) != 1 {
		t.Fatalf("Calls = %d, want 1", len(f.Calls))
	}
	argv := f.Calls[0].buildArgv()
	want := []string{"graph", "-stamp", "deadbeef", "testdata/svcfix"}
	for i, w := range want {
		if argv[i] != w {
			t.Fatalf("argv = %v, want %v", argv, want)
		}
	}
}

func TestGraph_Negative_OperationalError(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "graph", Result{Stderr: []byte("bad flag"), ExitCode: 2})

	if _, err := RunGraph(context.Background(), f, "dir", "sha"); err == nil {
		t.Fatal("Graph with exit 2: want error, got nil")
	}
}

func TestGraph_Negative_ExecError(t *testing.T) {
	f := NewFakeRunner()
	f.EnqueueError("flowmap", "graph", context.DeadlineExceeded)

	if _, err := RunGraph(context.Background(), f, "dir", "sha"); err == nil {
		t.Fatal("Graph with exec error: want error, got nil")
	}
}

func TestBoundaryGenerate_Happy(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "boundary", Result{ExitCode: 0})

	if err := BoundaryGenerate(context.Background(), f, "dir"); err != nil {
		t.Fatalf("BoundaryGenerate: %v", err)
	}
}

func TestBoundaryGenerate_Negative(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "boundary", Result{ExitCode: 2, Stderr: []byte("bad dir")})

	if err := BoundaryGenerate(context.Background(), f, "dir"); err == nil {
		t.Fatal("BoundaryGenerate with exit 2: want error, got nil")
	}
}

func TestBoundaryCheck_Happy(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "boundary", Result{ExitCode: 0, Stdout: []byte("boundary contract current: dir\n")})

	if err := BoundaryCheck(context.Background(), f, "dir"); err != nil {
		t.Fatalf("BoundaryCheck: %v", err)
	}
	argv := f.Calls[0].buildArgv()
	if argv[0] != "boundary" || argv[1] != "-check" {
		t.Fatalf("argv = %v, want boundary -check ... (flags before positional)", argv)
	}
}

func TestBoundaryCheck_Negative_Stale(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "boundary", Result{ExitCode: 1, Stderr: []byte("stale")})

	if err := BoundaryCheck(context.Background(), f, "dir"); err == nil {
		t.Fatal("BoundaryCheck stale: want error, got nil")
	}
}

func TestReview_Happy_DecodesOnBlockToo(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("groundwork", "review", Result{Stdout: readCanned(t, "review-block.json"), ExitCode: 1})

	r, err := RunReview(context.Background(), f, "policy.json", "base.json", "branch.json", "deadbeef")
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if !r.Blocking() {
		t.Error("Blocking() = false, want true for a BLOCK verdict")
	}

	argv := f.Calls[0].buildArgv()
	want := []string{"review", "-json", "-expect", "deadbeef", "policy.json", "base.json", "branch.json"}
	if len(argv) != len(want) {
		t.Fatalf("argv = %v, want %v", argv, want)
	}
	for i := range want {
		if argv[i] != want[i] {
			t.Fatalf("argv = %v, want %v", argv, want)
		}
	}
}

func TestReview_Happy_NoExpect(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("groundwork", "review", Result{Stdout: readCanned(t, "review-structurally-clear.json"), ExitCode: 0})

	if _, err := RunReview(context.Background(), f, "policy.json", "base.json", "branch.json", ""); err != nil {
		t.Fatalf("Review: %v", err)
	}
	argv := f.Calls[0].buildArgv()
	for _, tok := range argv {
		if tok == "-expect" {
			t.Fatalf("argv %v contains -expect with an empty expect string, want it omitted", argv)
		}
	}
}

func TestReview_Negative_OperationalError(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("groundwork", "review", Result{ExitCode: 2, Stderr: []byte("bad policy")})

	if _, err := RunReview(context.Background(), f, "p", "b", "br", ""); err == nil {
		t.Fatal("Review with exit 2: want error, got nil")
	}
}

func TestCrossCheckDiff_Agrees(t *testing.T) {
	cases := []struct {
		name         string
		exitCode     int
		wantBreaking bool
	}{
		{"clean agrees", 0, false},
		{"breaking agrees", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewFakeRunner()
			f.Enqueue("groundwork", "diff", Result{ExitCode: tc.exitCode})
			if err := CrossCheckDiff(context.Background(), f, "base.json", "branch.json", tc.wantBreaking); err != nil {
				t.Fatalf("CrossCheckDiff: %v", err)
			}
		})
	}
}

func TestCrossCheckDiff_Disagrees(t *testing.T) {
	cases := []struct {
		name         string
		exitCode     int
		wantBreaking bool
	}{
		{"verdi says breaking, upstream says clean", 0, true},
		{"verdi says clean, upstream says breaking", 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := NewFakeRunner()
			f.Enqueue("groundwork", "diff", Result{ExitCode: tc.exitCode})
			if err := CrossCheckDiff(context.Background(), f, "base.json", "branch.json", tc.wantBreaking); err == nil {
				t.Fatal("CrossCheckDiff disagreement: want error, got nil")
			}
		})
	}
}

func TestCrossCheckDiff_OperationalError(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("groundwork", "diff", Result{ExitCode: 2, Stderr: []byte("usage")})
	if err := CrossCheckDiff(context.Background(), f, "a", "b", false); err == nil {
		t.Fatal("CrossCheckDiff exit 2: want error, got nil")
	}
}

func TestVersion_Happy(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "version", Result{Stdout: []byte("flowmap v0.0.0-20260707202836-cd38b1a56bb7\n"), ExitCode: 0})

	got, err := Version(context.Background(), f, "flowmap")
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if got != "flowmap v0.0.0-20260707202836-cd38b1a56bb7" {
		t.Errorf("Version = %q", got)
	}
}

func TestVersion_Negative(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "version", Result{ExitCode: 2})
	if _, err := Version(context.Background(), f, "flowmap"); err == nil {
		t.Fatal("Version exit 2: want error, got nil")
	}
}

func TestCheckToolPin(t *testing.T) {
	const commit = "cd38b1a56bb7deadbeefdeadbeefdeadbeefdead"
	const tool = "v0.0.0-20260707202836-cd38b1a56bb7"

	if err := CheckToolPin(tool, commit); err != nil {
		t.Fatalf("CheckToolPin(matching): %v", err)
	}

	cases := []struct {
		name   string
		tool   string
		commit string
	}{
		{"mismatched commit", tool, "0000000000000000000000000000000000000000"},
		{"empty tool", "", commit},
		{"empty commit", tool, ""},
		{"not a pseudo-version", "garbage", commit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckToolPin(tc.tool, tc.commit); err == nil {
				t.Fatalf("CheckToolPin(%q, %q): want error, got nil", tc.tool, tc.commit)
			}
		})
	}
}
