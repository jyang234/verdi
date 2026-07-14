package diagrambase

import (
	"context"
	"errors"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
)

// baseBody is the pinned base's mermaid body — deliberately with a blank
// line and trailing space so "returns the base bytes exactly" is a
// byte-level claim, not a normalized one.
const baseBody = "graph TD\n  loansvc --> notification-svc\n\n  loansvc --> charge-svc \n"

const baseDiagram = `---
id: diagram/loansvc-topology
kind: diagram
title: "LoanSvc topology (base fixture)"
status: active
owners: [platform-team]
---
` + baseBody

// newBaseRepo commits the base diagram in a hermetic fixturegit repo
// (obligation ac-4--static: fixturegit, no network) and returns the repo
// dir and the pinning commit.
func newBaseRepo(t *testing.T) (dir, commit string) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/diagrams/loansvc-topology.mermaid": baseDiagram},
		Message: "seed base diagram",
	}})
	return repo.Dir, repo.Head
}

func TestCanonicalGraphDigest_DeterministicPureFunction(t *testing.T) {
	first, err := CanonicalGraphDigest([]byte(baseBody))
	if err != nil {
		t.Fatalf("CanonicalGraphDigest: %v", err)
	}
	for i := 0; i < 3; i++ {
		again, err := CanonicalGraphDigest([]byte(baseBody))
		if err != nil {
			t.Fatalf("repeat %d: %v", i, err)
		}
		if again != first {
			t.Fatalf("repeat %d: digest %s != %s", i, again, first)
		}
	}
	// A structurally different body digests differently (the digest is a
	// graph claim, not a constant).
	other, err := CanonicalGraphDigest([]byte("graph TD\n  a --> b\n"))
	if err != nil {
		t.Fatalf("CanonicalGraphDigest(other): %v", err)
	}
	if other == first {
		t.Fatalf("different graphs share digest %s", first)
	}
	if len(first) != len("sha256:")+64 {
		t.Fatalf("digest %q is not sha256:<64-hex>", first)
	}
}

func TestRecover_HappyPath_ReturnsBaseBytesExactly(t *testing.T) {
	dir, commit := newBaseRepo(t)
	digest, err := CanonicalGraphDigest([]byte(baseBody))
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	df := &artifact.DiagramDerivedFrom{Ref: "diagram/loansvc-topology@" + commit, Digest: digest}

	got, err := Recover(context.Background(), dir, df)
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if string(got) != baseBody {
		t.Fatalf("recovered base = %q, want the base body byte-for-byte %q", got, baseBody)
	}
}

func TestRecover_Negative(t *testing.T) {
	dir, commit := newBaseRepo(t)
	goodDigest, err := CanonicalGraphDigest([]byte(baseBody))
	if err != nil {
		t.Fatalf("digest: %v", err)
	}

	cases := []struct {
		name    string
		df      *artifact.DiagramDerivedFrom
		wantErr func(error) bool
	}{
		{
			name: "digest mismatch fails closed with both digests disclosed",
			df:   &artifact.DiagramDerivedFrom{Ref: "diagram/loansvc-topology@" + commit, Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000000"},
			wantErr: func(err error) bool {
				var m *DigestMismatchError
				return errors.As(err, &m) && m.Got == goodDigest && m.Pinned != m.Got
			},
		},
		{
			name: "unresolvable pinned commit",
			df:   &artifact.DiagramDerivedFrom{Ref: "diagram/loansvc-topology@deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", Digest: goodDigest},
			wantErr: func(err error) bool {
				var u *UnavailableError
				return errors.As(err, &u)
			},
		},
		{
			name: "base path absent at the pinned commit",
			df:   &artifact.DiagramDerivedFrom{Ref: "diagram/no-such-diagram@" + commit, Digest: goodDigest},
			wantErr: func(err error) bool {
				var u *UnavailableError
				return errors.As(err, &u)
			},
		},
		{
			name: "unpinned ref refused",
			df:   &artifact.DiagramDerivedFrom{Ref: "diagram/loansvc-topology", Digest: goodDigest},
			wantErr: func(err error) bool {
				var u *UnpinnedRefError
				return errors.As(err, &u)
			},
		},
		{
			name: "missing derived_from refused",
			df:   nil,
			wantErr: func(err error) bool {
				var n *NotDerivedError
				return errors.As(err, &n)
			},
		},
		{
			name: "non-diagram ref refused",
			df:   &artifact.DiagramDerivedFrom{Ref: "spec/loansvc@" + commit, Digest: goodDigest},
			wantErr: func(err error) bool {
				var u *UnavailableError
				return errors.As(err, &u)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Recover(context.Background(), dir, tc.df)
			if err == nil {
				t.Fatal("Recover succeeded, want a typed refusal")
			}
			if !tc.wantErr(err) {
				t.Fatalf("err = %v (%T), want the case's typed error", err, err)
			}
			if got != nil {
				t.Fatalf("Recover returned base bytes %q alongside the refusal; the affordances must have nothing to render or write from on failure", got)
			}
		})
	}
}
