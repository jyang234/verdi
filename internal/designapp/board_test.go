package designapp

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
)

func TestGetBoard(t *testing.T) {
	root := newTestStore(t, "draft-write")

	t.Run("happy path returns the board projection with identity", func(t *testing.T) {
		svc := NewService()
		result, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetBoard: %v", err)
		}
		if result.BoardProjection == nil {
			t.Fatal("GetBoard: nil board projection")
		}
		if result.Identity.Checkout != root || result.Identity.Spec != "spec/sample" || result.Identity.Branch != "design/sample" {
			t.Fatalf("GetBoard identity = %+v", result.Identity)
		}
	})

	t.Run("invalid ref is input-invalid", func(t *testing.T) {
		svc := NewService()
		_, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "not-a-ref"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "input-invalid" {
			t.Fatalf("GetBoard(invalid ref) = %+v, want verdict input-invalid", err)
		}
	})

	t.Run("pinned ref is input-invalid", func(t *testing.T) {
		svc := NewService()
		_, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "spec/sample@0123456789abcdef0123456789abcdef01234567"})
		if err == nil || err.Classification != ClassificationVerdict {
			t.Fatalf("GetBoard(pinned ref) = %+v, want verdict", err)
		}
	})

	t.Run("missing spec is not-found", func(t *testing.T) {
		svc := NewService()
		_, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "spec/does-not-exist"})
		if err == nil || err.Classification != ClassificationVerdict || err.Code != "board-not-found" {
			t.Fatalf("GetBoard(missing spec) = %+v, want verdict board-not-found", err)
		}
	})

	t.Run("nil board loader is operational", func(t *testing.T) {
		svc := NewService()
		svc.Board = nil
		_, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationOperational {
			t.Fatalf("GetBoard(nil board loader) = %+v, want operational", err)
		}
	})

	t.Run("identity resolution failure is operational", func(t *testing.T) {
		svc := NewService()
		svc.Identity = failingIdentityReader{}
		_, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "spec/sample"})
		if err == nil || err.Classification != ClassificationOperational {
			t.Fatalf("GetBoard(bad identity) = %+v, want operational", err)
		}
	})
}

// failingIdentityReader is the shared negative-path fake every operation's
// identity-resolution-failure test uses.
type failingIdentityReader struct{}

func (failingIdentityReader) CheckoutRoot(context.Context, string) (string, error) {
	return "", errIdentityUnavailable
}
func (failingIdentityReader) CurrentBranch(context.Context, string) (string, error) { return "", nil }
func (failingIdentityReader) Head(context.Context, string) (string, error)          { return "", nil }

var errIdentityUnavailable = &identityUnavailableError{}

type identityUnavailableError struct{}

func (*identityUnavailableError) Error() string { return "identity unavailable (test fake)" }

var _ draftmutation.IdentityReader = failingIdentityReader{}
