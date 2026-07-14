package refindex

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// fakeGitRunner is the in-process GitRunner double dc-2 requires: every
// ComputeIndex behavior must be provable against a fake with no real git
// process at all, in addition to the hermetic fixturegit exercise
// (refindex_test.go). Each method delegates to an optional func field; a
// nil field panics if called, so a test that forgets to wire a dependency
// fails loudly rather than silently returning a zero value.
type fakeGitRunner struct {
	defaultBranchFn func(ctx context.Context, dir string) (string, error)
	localDesignFn   func(ctx context.Context, dir string) ([]string, error)
	remoteDesignFn  func(ctx context.Context, dir string) ([]string, error)
	showFn          func(ctx context.Context, dir, ref, path string) ([]byte, error)
	listTreeFn      func(ctx context.Context, dir, ref, path string) ([]string, error)
	isAncestorFn    func(ctx context.Context, dir, ancestor, ref string) (bool, error)
}

func (f *fakeGitRunner) DefaultBranch(ctx context.Context, dir string) (string, error) {
	return f.defaultBranchFn(ctx, dir)
}

func (f *fakeGitRunner) LocalDesignBranches(ctx context.Context, dir string) ([]string, error) {
	return f.localDesignFn(ctx, dir)
}

func (f *fakeGitRunner) RemoteDesignBranches(ctx context.Context, dir string) ([]string, error) {
	return f.remoteDesignFn(ctx, dir)
}

func (f *fakeGitRunner) Show(ctx context.Context, dir, ref, path string) ([]byte, error) {
	return f.showFn(ctx, dir, ref, path)
}

func (f *fakeGitRunner) ListTree(ctx context.Context, dir, ref, path string) ([]string, error) {
	return f.listTreeFn(ctx, dir, ref, path)
}

func (f *fakeGitRunner) IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error) {
	return f.isAncestorFn(ctx, dir, ancestor, ref)
}

var _ GitRunner = (*fakeGitRunner)(nil)

// noDesignBranches is a convenience for tests exercising only the
// default-branch walk.
func noDesignBranches() (func(context.Context, string) ([]string, error), func(context.Context, string) ([]string, error)) {
	empty := func(context.Context, string) ([]string, error) { return nil, nil }
	return empty, empty
}

const fakeComponentSpec = `---
id: spec/fake
kind: spec
class: component
title: "Fake"
status: active
owners: [platform-team]
---
# Fake
`

// TestComputeIndex_Fake_DefaultBranchUnconfigured proves an unconfigured
// default branch (gitx.DefaultBranch's own "", nil contract) is treated
// honestly as "nothing to walk", not a fabricated entry or an error.
func TestComputeIndex_Fake_DefaultBranchUnconfigured(t *testing.T) {
	local, remote := noDesignBranches()
	f := &fakeGitRunner{
		defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
		localDesignFn:   local,
		remoteDesignFn:  remote,
	}
	got, err := ComputeIndex(context.Background(), "/fake/root", f)
	if err != nil {
		t.Fatalf("ComputeIndex: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ComputeIndex with unconfigured default branch = %v, want empty", got)
	}
}

// TestComputeIndex_Fake_PropagatesOperationalErrors proves every git-runner
// failure propagates as a real Go error (ac-1: "a ref that fails to resolve
// at all ... propagates as a real Go error rather than a silently-skipped
// entry") rather than being swallowed into an empty or partial result.
func TestComputeIndex_Fake_PropagatesOperationalErrors(t *testing.T) {
	sentinel := errors.New("boom")
	local, remote := noDesignBranches()

	t.Run("DefaultBranch fails", func(t *testing.T) {
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "", sentinel },
			localDesignFn:   local,
			remoteDesignFn:  remote,
		}
		if _, err := ComputeIndex(context.Background(), "/fake", f); !errors.Is(err, sentinel) {
			t.Fatalf("ComputeIndex: want error wrapping %v, got %v", sentinel, err)
		}
	})

	t.Run("ListTree fails during default-branch walk", func(t *testing.T) {
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "main", nil },
			localDesignFn:   local,
			remoteDesignFn:  remote,
			listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
				return nil, sentinel
			},
		}
		if _, err := ComputeIndex(context.Background(), "/fake", f); !errors.Is(err, sentinel) {
			t.Fatalf("ComputeIndex: want error wrapping %v, got %v", sentinel, err)
		}
	})

	t.Run("LocalDesignBranches fails", func(t *testing.T) {
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
			localDesignFn: func(context.Context, string) ([]string, error) {
				return nil, sentinel
			},
			remoteDesignFn: remote,
		}
		if _, err := ComputeIndex(context.Background(), "/fake", f); !errors.Is(err, sentinel) {
			t.Fatalf("ComputeIndex: want error wrapping %v, got %v", sentinel, err)
		}
	})

	t.Run("RemoteDesignBranches fails", func(t *testing.T) {
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
			localDesignFn:   local,
			remoteDesignFn: func(context.Context, string) ([]string, error) {
				return nil, sentinel
			},
		}
		if _, err := ComputeIndex(context.Background(), "/fake", f); !errors.Is(err, sentinel) {
			t.Fatalf("ComputeIndex: want error wrapping %v, got %v", sentinel, err)
		}
	})

	t.Run("IsAncestor fails", func(t *testing.T) {
		f := &fakeGitRunner{
			defaultBranchFn: func(context.Context, string) (string, error) { return "main", nil },
			localDesignFn: func(context.Context, string) ([]string, error) {
				return []string{"design/foo"}, nil
			},
			remoteDesignFn: remote,
			listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
				if ref == "main" {
					return nil, nil // empty default-branch tree
				}
				return []string{path}, nil // design branch has its spec.md
			},
			isAncestorFn: func(context.Context, string, string, string) (bool, error) {
				return false, sentinel
			},
		}
		if _, err := ComputeIndex(context.Background(), "/fake", f); !errors.Is(err, sentinel) {
			t.Fatalf("ComputeIndex: want error wrapping %v, got %v", sentinel, err)
		}
	})
}

// TestComputeIndex_Fake_SourceBoth_SingleEntry proves a branch present in
// both LocalDesignBranches and RemoteDesignBranches folds into exactly one
// SourceBoth entry (ac-2), against the in-process fake — no real git
// process, and no local/remote loop drift, since ComputeIndex reads both
// through mergeDesignSources's one shared path.
func TestComputeIndex_Fake_SourceBoth_SingleEntry(t *testing.T) {
	f := &fakeGitRunner{
		defaultBranchFn: func(context.Context, string) (string, error) { return "", nil },
		localDesignFn: func(context.Context, string) ([]string, error) {
			return []string{"design/both"}, nil
		},
		remoteDesignFn: func(context.Context, string) ([]string, error) {
			return []string{"design/both"}, nil
		},
		listTreeFn: func(ctx context.Context, dir, ref, path string) ([]string, error) {
			return []string{path}, nil
		},
		showFn: func(ctx context.Context, dir, ref, path string) ([]byte, error) {
			return []byte(fakeComponentSpec), nil
		},
		isAncestorFn: func(context.Context, string, string, string) (bool, error) {
			return false, nil
		},
	}
	got, err := ComputeIndex(context.Background(), "/fake", f)
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("ComputeIndex = %d entries, want exactly 1: %+v", len(got), got)
	}
	if got[0].Source != SourceBoth {
		t.Fatalf("Source = %q, want %q", got[0].Source, SourceBoth)
	}
}

// forbiddenMethodName matches a method name shaped like a checkout/switch/
// generic-escape-hatch capability — ac-5's static guarantee must hold at
// the GitRunner interface's method set itself, not merely "the current
// implementation happens not to call such a method".
var forbiddenMethodName = regexp.MustCompile(`(?i)checkout|switch|^run$`)

// TestGitRunner_MethodSet_ExposesNoCheckoutCapability reads the GitRunner
// interface's method set via reflection and asserts none of its methods is
// named or shaped like a checkout/switch/generic-run escape hatch — the
// interface itself makes a HEAD-moving call impossible to express, which is
// ac-5's static claim (spec/ref-index ac-5, obligation ac-5--static.md).
func TestGitRunner_MethodSet_ExposesNoCheckoutCapability(t *testing.T) {
	typ := reflect.TypeOf((*GitRunner)(nil)).Elem()
	if typ.NumMethod() == 0 {
		t.Fatal("GitRunner declares no methods at all — did reflection resolve the wrong type?")
	}
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i)
		if forbiddenMethodName.MatchString(m.Name) {
			t.Fatalf("GitRunner exposes method %q, which looks like a checkout/switch/generic-run capability — ac-5 requires the port's method set to make this structurally impossible", m.Name)
		}
		// A generic exec.Command-shaped escape hatch (variadic string args
		// returning ([]byte, error) or error) would also defeat ac-5's
		// guarantee even under an innocuous name; none of this interface's
		// methods take a bare variadic string arg list.
		sig := m.Type
		lastIn := sig.NumIn() - 1
		if lastIn >= 0 && sig.IsVariadic() && sig.In(lastIn).Elem().Kind() == reflect.String {
			t.Fatalf("GitRunner method %q takes a variadic string arg list — a generic git-subcommand escape hatch ac-5 forbids", m.Name)
		}
	}
}

// TestGitRunner_MethodNames documents the exact method set this test
// pins against drift, so a future addition is a deliberate, reviewed edit
// to this list rather than a silent expansion of the port's capabilities.
func TestGitRunner_MethodNames(t *testing.T) {
	typ := reflect.TypeOf((*GitRunner)(nil)).Elem()
	var names []string
	for i := 0; i < typ.NumMethod(); i++ {
		names = append(names, typ.Method(i).Name)
	}
	want := "DefaultBranch,IsAncestor,ListTree,LocalDesignBranches,RemoteDesignBranches,Show"
	got := strings.Join(sortedCopy(names), ",")
	if got != want {
		t.Fatalf("GitRunner method set = %q, want %q", got, want)
	}
}

func sortedCopy(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
