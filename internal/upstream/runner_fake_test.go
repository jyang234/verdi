package upstream

import (
	"context"
	"errors"
	"testing"
)

func TestFakeRunner_HappyPath(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "version", Result{Stdout: []byte("flowmap v1\n"), ExitCode: 0})

	res, err := f.Run(context.Background(), Request{Bin: "flowmap", Subcommand: "version"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(res.Stdout) != "flowmap v1\n" {
		t.Errorf("Stdout = %q, want %q", res.Stdout, "flowmap v1\n")
	}
	if len(f.Calls) != 1 || f.Calls[0].Subcommand != "version" {
		t.Errorf("Calls = %+v, want one recorded 'version' call", f.Calls)
	}
}

// TestFakeRunner_Sequenced proves repeated calls to the same key drain a
// queue in order, then keep serving the last response.
func TestFakeRunner_Sequenced(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "graph", Result{Stdout: []byte("base"), ExitCode: 0})
	f.Enqueue("flowmap", "graph", Result{Stdout: []byte("branch"), ExitCode: 0})

	ctx := context.Background()
	req := Request{Bin: "flowmap", Subcommand: "graph"}

	res1, _ := f.Run(ctx, req)
	res2, _ := f.Run(ctx, req)
	res3, _ := f.Run(ctx, req)

	if string(res1.Stdout) != "base" {
		t.Errorf("call 1 Stdout = %q, want %q", res1.Stdout, "base")
	}
	if string(res2.Stdout) != "branch" {
		t.Errorf("call 2 Stdout = %q, want %q", res2.Stdout, "branch")
	}
	if string(res3.Stdout) != "branch" {
		t.Errorf("call 3 (queue drained) Stdout = %q, want sticky %q", res3.Stdout, "branch")
	}
}

func TestFakeRunner_EnqueueError(t *testing.T) {
	f := NewFakeRunner()
	wantErr := errors.New("boom")
	f.EnqueueError("groundwork", "review", wantErr)

	_, err := f.Run(context.Background(), Request{Bin: "groundwork", Subcommand: "review"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run error = %v, want %v", err, wantErr)
	}
}

func TestFakeRunner_Negative_NoResponseRegistered(t *testing.T) {
	f := NewFakeRunner()
	_, err := f.Run(context.Background(), Request{Bin: "flowmap", Subcommand: "graph"})
	if err == nil {
		t.Fatal("Run with no registered response: want error, got nil")
	}
}

func TestFakeRunner_Negative_CancelledContext(t *testing.T) {
	f := NewFakeRunner()
	f.Enqueue("flowmap", "graph", Result{ExitCode: 0})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Run(ctx, Request{Bin: "flowmap", Subcommand: "graph"}); err == nil {
		t.Fatal("Run with cancelled context: want error, got nil")
	}
}
