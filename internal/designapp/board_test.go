package designapp

import (
	"context"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
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
				// DomainRefusal and DocumentHref are plain strings: the
				// clone's initial struct assignment value-copies them
				// (reviewed for the I-1 domain-refusal field, Wave 6
				// Task 2 correction; and for the spec-documents Wave 2
				// Document tab link, which is json:"-" and never shared).
				"Spec", "Title", "Mode", "Status", "DocumentHref", "DomainRefusal",
				"Class", "StoryRef", "Spike",
				"ClassLabel", "StatusLabel", "Problem", "Outcome",
				"ProblemBodyHTML", "OutcomeBodyHTML",
				"Cards", "RefCards", "Edges", "Stickies", "Tray",
				"StubViews", "ACCoverage", "OQClaims", "CreateFields", "CreateCoverage",
				"Notices", "CaseFileBadges", "CaseFileDisclosures", "words",
			},
		},
		{
			// The New story dialog's coverage (spec/new-story-dialog-v2):
			// two counts the clone value-copies, and the rows it copies.
			name: "create coverage",
			typ:  fieldType(t, reflect.TypeOf(proj), "CreateCoverage"),
			want: []string{"Criteria", "Uncovered", "Unproven"},
		},
		{
			name: "create coverage criterion element",
			typ:  fieldType(t, fieldType(t, reflect.TypeOf(proj), "CreateCoverage"), "Criteria").Elem(),
			want: []string{"ID", "Stubs", "Stories", "Disclosed", "Uncovered"},
		},
		{
			name: "card element",
			typ:  reflect.TypeOf(proj.Cards).Elem(),
			want: []string{"ID", "Kind", "Text", "X", "Y", "Anchored", "Obligations", "Badges", "Supersessions"},
		},
		{
			// Object (SI-278) is a pointer to a struct carrying nested
			// slices; the clone copies it level by level.
			name: "reference card element",
			typ:  reflect.TypeOf(proj.RefCards).Elem(),
			want: []string{"Ref", "X", "Y", "Pinned", "PinID", "EditorHref", "FeatureHref", "Archived", "UnresolvedNotice", "Object"},
		},
		{
			name: "supersession element",
			typ:  fieldType(t, reflect.TypeOf(proj.Cards).Elem(), "Supersessions").Elem(),
			want: []string{
				"State", "Object", "Edge", "By", "Conflict", "Since", "Closed", "ClosedWitness",
				"Carry", "Revision", "Heads", "Witness", "Carried", "Establisher", "Reason", "Lines",
			},
		},
		{
			name: "supersession line element",
			typ:  fieldType(t, fieldType(t, reflect.TypeOf(proj.Cards).Elem(), "Supersessions").Elem(), "Lines").Elem(),
			want: []string{"Kind", "Text", "Disclosure", "Links", "Trailing"},
		},
		{
			// A link is two strings today; a slice added here would need its
			// own copy in the clone's line loop.
			name: "supersession link element",
			typ:  fieldType(t, fieldType(t, fieldType(t, reflect.TypeOf(proj.Cards).Elem(), "Supersessions").Elem(), "Lines").Elem(), "Links").Elem(),
			want: []string{"Ref", "Href"},
		},
		{
			// The reference card's object: a pointer the clone dereferences
			// and copies, with its Supersession's collections.
			name: "reference card object element",
			typ:  fieldType(t, reflect.TypeOf(proj.RefCards).Elem(), "Object").Elem(),
			want: []string{"Text", "Supersession"},
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

// fieldType is the type of typ's field name, so the nested ratchet rows
// above follow a field if it moves and fail by name if it goes.
func fieldType(t *testing.T, typ reflect.Type, name string) reflect.Type {
	t.Helper()
	f, ok := typ.FieldByName(name)
	if !ok {
		t.Fatalf("%s has no %s field", typ, name)
	}
	return f.Type
}

// TestCloneBoardProjectionCreateCoverage is the mutate-the-clone proof for
// the New story dialog's coverage (spec/new-story-dialog-v2): the rows,
// and each row's Stories and Disclosed, are the clone's own copies, so a
// write through the clone at any level leaves the original unchanged; and
// the clone keeps the nil/empty distinction at both levels.
// TestBoardProjectionCloneCoverage ratchets only the field inventory.
func TestCloneBoardProjectionCreateCoverage(t *testing.T) {
	type row struct {
		id        string
		stubs     int
		stories   []string
		disclosed []string
		uncovered bool
	}
	// build makes a fresh projection from rows, copying each input slice
	// so two builds of one case share no backing array: the second build
	// is the expected value, which a write through an aliased clone must
	// not reach. A nil rows leaves Criteria nil; an empty one, empty.
	build := func(rows []row, uncovered int, unproven bool) *workbench.BoardProjection {
		own := func(in []string) []string {
			if in == nil {
				return nil
			}
			return append(make([]string, 0, len(in)), in...)
		}
		p := &workbench.BoardProjection{}
		if rows != nil {
			p.CreateCoverage.Criteria = rowsLike(p.CreateCoverage.Criteria, len(rows))
		}
		for i, r := range rows {
			c := &p.CreateCoverage.Criteria[i]
			c.ID, c.Stubs, c.Uncovered = r.id, r.stubs, r.uncovered
			c.Stories, c.Disclosed = own(r.stories), own(r.disclosed)
		}
		p.CreateCoverage.Uncovered, p.CreateCoverage.Unproven = uncovered, unproven
		return p
	}

	for _, tc := range []struct {
		name      string
		rows      []row
		uncovered int
		unproven  bool
	}{
		{
			name: "rows with stories and disclosures",
			rows: []row{
				{id: "ac-1", stubs: 1, stories: []string{"spec/story-a", "spec/story-b"}, disclosed: []string{"corpus unreadable"}},
				{id: "ac-2", stubs: 2, stories: []string{"spec/story-c"}, disclosed: []string{"reason one", "reason two"}},
			},
			unproven: true,
		},
		{
			name:      "nil row slices stay nil",
			rows:      []row{{id: "ac-1", uncovered: true}},
			uncovered: 1,
		},
		{
			name: "empty row slices stay empty",
			rows: []row{{id: "ac-1", stubs: 1, stories: []string{}, disclosed: []string{}}},
		},
		{name: "nil criteria stay nil"},
		{name: "empty criteria stay empty", rows: []row{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orig := build(tc.rows, tc.uncovered, tc.unproven)
			want := build(tc.rows, tc.uncovered, tc.unproven)

			clone := cloneBoardProjection(orig)
			got := &clone.CreateCoverage
			if !reflect.DeepEqual(*got, want.CreateCoverage) {
				t.Fatalf("clone's CreateCoverage = %#v, want %#v", *got, want.CreateCoverage)
			}
			// DeepEqual already tells nil from empty; these name the level.
			if (got.Criteria == nil) != (want.CreateCoverage.Criteria == nil) {
				t.Fatalf("clone's Criteria nil = %v, want %v", got.Criteria == nil, want.CreateCoverage.Criteria == nil)
			}
			for i := range got.Criteria {
				w := want.CreateCoverage.Criteria[i]
				if (got.Criteria[i].Stories == nil) != (w.Stories == nil) || (got.Criteria[i].Disclosed == nil) != (w.Disclosed == nil) {
					t.Fatalf("clone's row %d Stories nil = %v, Disclosed nil = %v; want %v, %v", i,
						got.Criteria[i].Stories == nil, got.Criteria[i].Disclosed == nil, w.Stories == nil, w.Disclosed == nil)
				}
			}

			// Write through the clone at every level: each row's fields,
			// each element of its Stories and Disclosed, the rows slice's
			// length, and the two counts.
			for i := range got.Criteria {
				c := &got.Criteria[i]
				c.ID, c.Stubs, c.Uncovered = "TAMPERED", 9999, !c.Uncovered
				for j := range c.Stories {
					c.Stories[j] = "TAMPERED"
				}
				for j := range c.Disclosed {
					c.Disclosed[j] = "TAMPERED"
				}
				c.Stories = append(c.Stories, "TAMPERED")
				c.Disclosed = append(c.Disclosed, "TAMPERED")
			}
			got.Criteria = append(got.Criteria, rowsLike(got.Criteria, 1)...)
			got.Uncovered, got.Unproven = 9999, !got.Unproven

			if !reflect.DeepEqual(orig.CreateCoverage, want.CreateCoverage) {
				t.Fatalf("the original's CreateCoverage changed through the clone:\n got  %#v\n want %#v",
					orig.CreateCoverage, want.CreateCoverage)
			}
		})
	}
}

// rowsLike is n zero rows of rows' slice type. workbench's criterion view
// type is unexported: a test here can set its exported fields but cannot
// name it, so the slice is made by inference from an existing value.
func rowsLike[S ~[]E, E any](_ S, n int) S {
	return make(S, n)
}

// TestGetBoardDoesNotAliasClosedSpecSupersession is the mutate-the-clone
// proof for the SI-278 collections (controller ruling 1): a reference
// card's Object and a decision card's Supersessions, down to a line's
// link slices, are the caller's own copies. The fixture is the accepted
// closed-spec object supersession scenario store, whose successor board
// carries both.
func TestGetBoardDoesNotAliasClosedSpecSupersession(t *testing.T) {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"} {
		t.Setenv(key, "")
	}
	root := scenario.Build(t, "accepted").Dir
	loader := &retainingBoardLoader{inner: workbenchBoardLoader{}}
	svc := NewService()
	svc.Board = loader

	first, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "spec/successor"})
	if err != nil {
		t.Fatalf("GetBoard: %v", err)
	}
	ref, card := -1, -1
	for i := range first.RefCards {
		if first.RefCards[i].Object != nil {
			ref = i
			break
		}
	}
	for i := range first.Cards {
		if len(first.Cards[i].Supersessions) > 0 {
			card = i
			break
		}
	}
	if ref < 0 || card < 0 {
		t.Fatalf("fixture projection lacks the collections under test: ref card with object=%v, card with supersessions=%v", ref >= 0, card >= 0)
	}
	obj := first.RefCards[ref].Object
	if len(obj.Supersession.Lines) < 2 || len(obj.Supersession.Lines[1].Links) == 0 || len(obj.Supersession.Lines[1].Trailing) == 0 ||
		len(first.Cards[card].Supersessions[0].Lines) == 0 || len(first.Cards[card].Supersessions[0].Lines[0].Links) == 0 {
		t.Fatalf("fixture views lack the nested link slices under test: %+v / %+v", obj.Supersession, first.Cards[card].Supersessions)
	}
	wantText := obj.Text
	wantLine := obj.Supersession.Lines[1].Text
	wantHref := obj.Supersession.Lines[1].Links[0].Href
	wantTrailing := obj.Supersession.Lines[1].Trailing[0].Href
	wantEdge := first.Cards[card].Supersessions[0].Lines[0].Text
	wantEdgeHref := first.Cards[card].Supersessions[0].Lines[0].Links[0].Href

	// Mutate every reachable level of both collections.
	obj.Text = "TAMPERED"
	obj.Supersession.Lines[1].Text = "TAMPERED"
	obj.Supersession.Lines[1].Links[0].Href = "TAMPERED"
	obj.Supersession.Lines[1].Trailing[0].Href = "TAMPERED"
	first.Cards[card].Supersessions[0].Lines[0].Text = "TAMPERED"
	first.Cards[card].Supersessions[0].Lines[0].Links[0].Href = "TAMPERED"

	second, err := svc.GetBoard(context.Background(), root, GetBoardRequest{Spec: "spec/successor"})
	if err != nil {
		t.Fatalf("GetBoard (second): %v", err)
	}
	got := second.RefCards[ref].Object
	if got.Text != wantText || got.Supersession.Lines[1].Text != wantLine || got.Supersession.Lines[1].Links[0].Href != wantHref || got.Supersession.Lines[1].Trailing[0].Href != wantTrailing {
		t.Fatalf("the reference card's object is aliased through the clone: %+v", got)
	}
	if s := second.Cards[card].Supersessions[0]; s.Lines[0].Text != wantEdge || s.Lines[0].Links[0].Href != wantEdgeHref {
		t.Fatalf("the decision card's supersessions are aliased through the clone: %+v", s)
	}
	// The port's own retained value is the ground truth.
	kept := loader.retained
	if kept.RefCards[ref].Object.Text != wantText || kept.RefCards[ref].Object.Supersession.Lines[1].Links[0].Href != wantHref ||
		kept.Cards[card].Supersessions[0].Lines[0].Links[0].Href != wantEdgeHref {
		t.Fatalf("the loader's retained projection was mutated: %+v / %+v", kept.RefCards[ref].Object, kept.Cards[card].Supersessions[0])
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
