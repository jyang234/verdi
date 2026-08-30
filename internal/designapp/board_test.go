package designapp

import (
	"context"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/workbench"
)

func TestGetBoard(t *testing.T) {
	root := newTestStore(t, "draft-write")

	t.Run("happy path returns the board projection with identity", func(t *testing.T) {
		svc := NewService()
		result, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "spec/sample"})
		if err != nil {
			t.Fatalf("GetBoard: %v", err)
		}
		if result.Schema != BoardResultSchema {
			t.Fatalf("Schema = %q, want %q", result.Schema, BoardResultSchema)
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

// retainingBoardLoader is a BoardLoader that loads once and then hands the
// SAME retained projection back on every call — the shape a caching or
// otherwise stateful port adapter has, and the only shape that can prove
// GetBoard hands out a copy rather than the port's own retained state
// (Task 1 contract: results are "deep-copy safe"). The production
// workbench loader rereads from disk each call, which would mask an
// aliasing defect here rather than disprove it.
type retainingBoardLoader struct {
	inner    BoardLoader
	retained *workbench.BoardProjection
	notice   string
	calls    int
}

func (l *retainingBoardLoader) LoadProjection(ctx context.Context, root, name string) (*workbench.BoardProjection, string, error) {
	l.calls++
	if l.retained == nil {
		proj, notice, err := l.inner.LoadProjection(ctx, root, name)
		if err != nil {
			return nil, "", err
		}
		l.retained, l.notice = proj, notice
	}
	return l.retained, l.notice, nil
}

// TestGetBoardDoesNotAliasTheLoadersState proves a caller cannot reach
// through the returned projection and mutate the port's retained state:
// every nested collection the caller can touch must be its own copy.
func TestGetBoardDoesNotAliasTheLoadersState(t *testing.T) {
	root := newTestStore(t, "draft-write")
	loader := &retainingBoardLoader{inner: workbenchBoardLoader{}}
	svc := NewService()
	svc.Board = loader

	first, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "spec/sample"})
	if err != nil {
		t.Fatalf("GetBoard: %v", err)
	}
	if first.BoardProjection == loader.retained {
		t.Fatal("GetBoard returned the loader's own retained projection pointer")
	}

	// The fixture must actually carry the nested collections under test,
	// or an "unaffected" assertion would prove nothing.
	if len(first.Cards) == 0 || len(first.StubViews) == 0 || len(first.ACCoverage) == 0 ||
		len(first.StubViews[0].AcceptanceCriteria) == 0 {
		t.Fatalf("fixture projection lacks the nested collections under test: cards=%d stubs=%d coverage=%d",
			len(first.Cards), len(first.StubViews), len(first.ACCoverage))
	}
	wantCardText := first.Cards[0].Text
	wantStubAC := first.StubViews[0].AcceptanceCriteria[0]
	wantCoverage := first.ACCoverage["ac-1"]
	wantTitle := first.Title

	// A hostile (or merely careless) caller mutates every reachable level.
	first.Title = "TAMPERED"
	first.Cards[0].Text = "TAMPERED"
	first.StubViews[0].AcceptanceCriteria[0] = "TAMPERED"
	first.ACCoverage["ac-1"] = 9999
	first.Notices = append(first.Notices, "TAMPERED")

	second, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "spec/sample"})
	if err != nil {
		t.Fatalf("GetBoard (second): %v", err)
	}
	if loader.calls != 2 {
		t.Fatalf("loader.calls = %d, want 2", loader.calls)
	}
	if second.Title != wantTitle {
		t.Fatalf("Title = %q, want %q — the caller reached the port's retained state", second.Title, wantTitle)
	}
	if second.Cards[0].Text != wantCardText {
		t.Fatalf("Cards[0].Text = %q, want %q — the cards slice is aliased", second.Cards[0].Text, wantCardText)
	}
	if second.StubViews[0].AcceptanceCriteria[0] != wantStubAC {
		t.Fatalf("StubViews[0].AcceptanceCriteria[0] = %q, want %q — a nested slice is aliased",
			second.StubViews[0].AcceptanceCriteria[0], wantStubAC)
	}
	if second.ACCoverage["ac-1"] != wantCoverage {
		t.Fatalf("ACCoverage[ac-1] = %d, want %d — the coverage map is aliased", second.ACCoverage["ac-1"], wantCoverage)
	}
	for _, notice := range second.Notices {
		if notice == "TAMPERED" {
			t.Fatalf("Notices = %+v — the notices slice is aliased", second.Notices)
		}
	}
	// The port's own retained value is the ground truth: prove it directly.
	if loader.retained.Title != wantTitle || loader.retained.Cards[0].Text != wantCardText {
		t.Fatalf("the loader's retained projection was mutated: title=%q card=%q",
			loader.retained.Title, loader.retained.Cards[0].Text)
	}
}

// TestBoardProjectionCloneCoverage is a ratchet, not a behavior test:
// cloneBoardProjection must copy every field of a type this package does
// not own. A field added to (or removed from) workbench's projection —
// or to one of the nested element types carrying its own collections —
// fails here, forcing the clone to be reviewed rather than silently
// leaving a new slice or map aliased.
func TestBoardProjectionCloneCoverage(t *testing.T) {
	fieldNames := func(typ reflect.Type) []string {
		names := make([]string, 0, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			names = append(names, typ.Field(i).Name)
		}
		return names
	}
	var proj workbench.BoardProjection
	for _, tc := range []struct {
		name string
		typ  reflect.Type
		want []string
	}{
		{
			name: "BoardProjection",
			typ:  reflect.TypeOf(proj),
			want: []string{
				"Spec", "Title", "Mode", "Status", "Class", "StoryRef", "Spike",
				"ClassLabel", "StatusLabel", "Problem", "Outcome",
				"ProblemBodyHTML", "OutcomeBodyHTML",
				"Cards", "RefCards", "Edges", "Stickies", "Tray",
				"StubViews", "ACCoverage", "OQClaims", "CreateFields",
				"Notices", "CaseFileBadges", "CaseFileDisclosures", "words",
			},
		},
		{
			name: "card element",
			typ:  reflect.TypeOf(proj.Cards).Elem(),
			want: []string{"ID", "Kind", "Text", "X", "Y", "Anchored", "Obligations", "Badges"},
		},
		{
			name: "stub element",
			typ:  reflect.TypeOf(proj.StubViews).Elem(),
			want: []string{
				"Slug", "Spike", "Resolves", "AcceptanceCriteria", "X", "Y",
				"Badges", "StoryLinks", "InstantiatedNotice",
			},
		},
		{
			name: "badge element",
			typ:  reflect.TypeOf(proj.CaseFileBadges).Elem(),
			want: []string{"Source", "Label", "Target", "Inputs", "Records", "Disclosures", "Provenance"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fieldNames(tc.typ)
			if len(got) != len(tc.want) {
				t.Fatalf("%s fields = %v, want %v (cloneBoardProjection must be reviewed)", tc.name, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("%s field %d = %q, want %q (cloneBoardProjection must be reviewed)", tc.name, i, got[i], tc.want[i])
				}
			}
		})
	}
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
