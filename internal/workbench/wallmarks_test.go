package workbench

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
)

// marksConcern is one Focus next concern: its id and the declared object
// its family's object segment names (SI-345), or "".
func marksConcern(id, object string) readinesspilot.Concern {
	return readinesspilot.Concern{ID: id, Object: object}
}

// marksSnapshot is a readiness snapshot of spec/wall on design/wall at
// HEAD abc, whose Focus next is concerns.
func marksSnapshot(concerns ...readinesspilot.Concern) *readinesspilot.Snapshot {
	return &readinesspilot.Snapshot{TargetRef: "spec/wall", Branch: "design/wall", Head: "abc", Attention: concerns}
}

// marksWall is a wall of spec/wall on design/wall at HEAD abc, carrying
// object cards ac-1, ac-2, oq-1 and the stubs stubs.
func marksWall(readiness *readinesspilot.Snapshot, loadErr error, stubs ...string) wallMarksInput {
	return wallMarksInput{
		Ref: "spec/wall", Branch: "design/wall", Head: "abc",
		ObjectIDs: []string{"ac-1", "ac-2", "oq-1"}, StubSlugs: stubs,
		Readiness: readiness, LoadErr: loadErr,
	}
}

// TestDeriveWallMarks is the marks' facts (spec/wall-canvas-v2 ac-1, dc-1;
// ledger SI-350 (1)–(2), SI-360 (1), (4)): per object card, the Focus next
// concerns whose Object names it, in Focus next order; per stub card, the
// stub-unreconciled concern journey's own slug rule names it by; each with
// its chip word; and, when the input is unreadable, one reason and no
// mark at all.
func TestDeriveWallMarks(t *testing.T) {
	coverage := marksConcern("success/coverage/ac-2", "ac-2")
	floor := marksConcern("success/blocker/outcome-floor/ac-2", "ac-2")
	question := marksConcern("shape/question/oq-1", "oq-1")
	claimed := marksConcern("review/blocker/question-claimed/oq-1", "oq-1")
	digitStub := marksConcern("review/blocker/stub-unreconciled/s-2fa-x", "")
	letterStub := marksConcern("review/blocker/stub-unreconciled/notice-retraction", "")
	readable := func(objects map[string][]wallMark, stubs map[string]wallMark) wallMarks {
		return wallMarks{Objects: objects, Stubs: stubs}
	}
	tests := []struct {
		name string
		in   wallMarksInput
		want wallMarks
	}{
		{
			name: "object concerns mark their cards in Focus next order, and only coverage reads no stub",
			in:   marksWall(marksSnapshot(coverage, question, floor, claimed), nil),
			want: readable(map[string][]wallMark{
				"ac-2": {{Concern: coverage.ID, Chip: "no stub"}, {Concern: floor.ID, Chip: "unresolved"}},
				"oq-1": {{Concern: question.ID, Chip: "unresolved"}, {Concern: claimed.ID, Chip: "unresolved"}},
			}, nil),
		},
		{
			name: "a letter-led stub is named by its own slug",
			in:   marksWall(marksSnapshot(letterStub), nil, "notice-retraction"),
			want: readable(nil, map[string]wallMark{"notice-retraction": {Concern: letterStub.ID, Chip: "unresolved"}}),
		},
		{
			name: "a digit-led stub is named through journey's slug rule",
			in:   marksWall(marksSnapshot(digitStub, coverage), nil, "2fa-x", "notice-retraction"),
			want: readable(
				map[string][]wallMark{"ac-2": {{Concern: coverage.ID, Chip: "no stub"}}},
				map[string]wallMark{"2fa-x": {Concern: digitStub.ID, Chip: "unresolved"}},
			),
		},
		{
			name: "a concern spelling a digit-led stub's raw slug names no card",
			in:   marksWall(marksSnapshot(marksConcern("review/blocker/stub-unreconciled/2fa-x", "")), nil, "2fa-x"),
			want: readable(nil, nil),
		},
		{
			name: "a concern whose object has no card on the wall marks nothing",
			in:   marksWall(marksSnapshot(marksConcern("success/coverage/ac-9", "ac-9")), nil),
			want: readable(nil, nil),
		},
		{
			name: "a concern with no object marks nothing",
			in:   marksWall(marksSnapshot(marksConcern("context/verdict", ""), marksConcern("review/role/merge/approve", "")), nil, "notice-retraction"),
			want: readable(nil, nil),
		},
		{
			name: "a stub of another family's segment is not a stub concern",
			in:   marksWall(marksSnapshot(marksConcern("review/blocker/question-claimed/notice-retraction", "")), nil, "notice-retraction"),
			want: readable(nil, nil),
		},
		{
			name: "an empty Focus next is readable and marks nothing",
			in:   marksWall(marksSnapshot(), nil, "2fa-x"),
			want: readable(nil, nil),
		},
		{
			name: "a failed load is unreadable",
			in:   marksWall(nil, errors.New("boom"), "2fa-x"),
			want: wallMarks{Unavailable: "the readiness load failed: boom"},
		},
		{
			name: "no derived readiness is unreadable",
			in:   marksWall(nil, nil),
			want: wallMarks{Unavailable: marksUnwired},
		},
		{
			name: "a snapshot of another branch is unreadable and marks nothing",
			in: func() wallMarksInput {
				snap := marksSnapshot(coverage, digitStub)
				snap.Branch = "main"
				return marksWall(snap, nil, "2fa-x")
			}(),
			want: wallMarks{Unavailable: "the readiness snapshot describes spec/wall on branch main at HEAD abc, and this wall shows spec/wall on branch design/wall at HEAD abc"},
		},
		{
			name: "a snapshot at another head is unreadable and marks nothing",
			in: func() wallMarksInput {
				snap := marksSnapshot(coverage)
				snap.Head = "def"
				return marksWall(snap, nil)
			}(),
			want: wallMarks{Unavailable: "the readiness snapshot describes spec/wall on branch design/wall at HEAD def, and this wall shows spec/wall on branch design/wall at HEAD abc"},
		},
		{
			name: "a wall whose head is unresolved is unreadable",
			in: func() wallMarksInput {
				in := marksWall(marksSnapshot(coverage), nil)
				in.Head = ""
				return in
			}(),
			want: wallMarks{Unavailable: "the readiness snapshot describes spec/wall on branch design/wall at HEAD abc, and this wall shows spec/wall on branch design/wall at HEAD (unresolved)"},
		},
		{
			name: "a snapshot of another spec is unreadable",
			in: func() wallMarksInput {
				snap := marksSnapshot(coverage)
				snap.TargetRef = "spec/other"
				return marksWall(snap, nil)
			}(),
			want: wallMarks{Unavailable: "the readiness snapshot describes spec/other on branch design/wall at HEAD abc, and this wall shows spec/wall on branch design/wall at HEAD abc"},
		},
		{
			name: "two stubs sharing one concern id are unreadable",
			in:   marksWall(marksSnapshot(coverage, digitStub), nil, "s-2fa-x", "2fa-x"),
			want: wallMarks{Unavailable: "stubs 2fa-x and s-2fa-x share the readiness concern review/blocker/stub-unreconciled/s-2fa-x, so its mark cannot name one card"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deriveWallMarks(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("deriveWallMarks =\n  %#v\nwant\n  %#v", got, tt.want)
			}
		})
	}
}

// TestWallMarkReasons: the unavailable notice's fixed reasons — a /b/
// wall whose branch is not the serving root's (SI-360 (3)) and the
// remote-only sealed render (SI-352 (1)) — name their branch or ref, and
// unavailableMarks carries one reason and no mark.
func TestWallMarkReasons(t *testing.T) {
	for _, tc := range []struct {
		name, got, want string
	}{
		{"a /b/ wall", marksBranchWall("design/two-a"), "this wall serves branch design/two-a from its own working tree, and readiness derives only for the serving checkout's branch, so the two cannot describe one commit"},
		{"a sealed render", marksSealed("origin/design/x"), "this wall is a read-only render of remote-tracking ref origin/design/x's committed content, for which no readiness is derived"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("reason =\n  %q\nwant\n  %q", tc.got, tc.want)
			}
			marks := unavailableMarks(tc.got)
			if marks.Unavailable != tc.want || marks.Objects != nil || marks.Stubs != nil {
				t.Fatalf("unavailableMarks = %#v", marks)
			}
		})
	}
	if marks := unavailableMarks(""); marks.Unavailable == "" {
		t.Fatal("an empty reason made the marks readable: unavailable must always say why")
	}
}

// TestWallMarks_Wire pins the marks' JSON shape, which the wall's
// snapshot serves: objects and stubs keyed by card, each mark its concern
// and chip word, or the one unavailable reason alone.
func TestWallMarks_Wire(t *testing.T) {
	for _, tc := range []struct {
		name  string
		marks wallMarks
		want  string
	}{
		{
			name: "readable",
			marks: wallMarks{
				Objects: map[string][]wallMark{"oq-1": {{Concern: "shape/question/oq-1", Chip: "unresolved"}}, "ac-2": {{Concern: "success/coverage/ac-2", Chip: "no stub"}}},
				Stubs:   map[string]wallMark{"2fa-x": {Concern: "review/blocker/stub-unreconciled/s-2fa-x", Chip: "unresolved"}},
			},
			want: `{"objects":{"ac-2":[{"concern":"success/coverage/ac-2","chip":"no stub"}],"oq-1":[{"concern":"shape/question/oq-1","chip":"unresolved"}]},"stubs":{"2fa-x":{"concern":"review/blocker/stub-unreconciled/s-2fa-x","chip":"unresolved"}}}`,
		},
		{name: "readable, nothing named", marks: wallMarks{}, want: `{}`},
		{name: "unavailable", marks: unavailableMarks("why"), want: `{"unavailable":"why"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.marks)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("wire =\n  %s\nwant\n  %s", got, tc.want)
			}
		})
	}
}
