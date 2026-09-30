package refindex

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/specstate"
)

func TestSortedUnique(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "sorted and deduplicated", in: []string{"b", "a", "b", "c", "a"}, want: []string{"a", "b", "c"}},
		{name: "empty values dropped", in: []string{"", "a", ""}, want: []string{"a"}},
		{name: "nil in, empty out", in: nil, want: []string{}},
		{name: "only empties, empty out", in: []string{"", ""}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sortedUnique(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("sortedUnique(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestReadDateBatch(t *testing.T) {
	sentinel := errors.New("boom")
	tests := []struct {
		name      string
		revs      []string
		answer    func(context.Context, string, []string) (map[string]string, error)
		wantCalls [][]string
		wantErr   bool
	}{
		{
			name:      "one deduplicated, sorted call",
			revs:      []string{"b", "a", "b"},
			wantCalls: [][]string{{"a", "b"}},
		},
		{
			name:      "no revs: no call at all",
			revs:      []string{"", ""},
			wantCalls: nil,
		},
		{
			name:      "the call's error is kept for disclosure, never dropped",
			revs:      []string{"a"},
			answer:    func(context.Context, string, []string) (map[string]string, error) { return nil, sentinel },
			wantCalls: [][]string{{"a"}},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeGitRunner{commitDatesFn: tt.answer}
			b := readDateBatch(context.Background(), f, "/fake", tt.revs)
			if !reflect.DeepEqual(f.commitDatesCalls, tt.wantCalls) {
				t.Fatalf("calls = %q, want %q", f.commitDatesCalls, tt.wantCalls)
			}
			if (b.err != nil) != tt.wantErr {
				t.Fatalf("batch err = %v, wantErr %v", b.err, tt.wantErr)
			}
		})
	}
}

func TestDateBatch_DateFor(t *testing.T) {
	const good = "2024-06-01T12:00:00+00:00"
	tests := []struct {
		name         string
		batch        dateBatch
		rev          string
		wantDate     string
		wantDisclose string // substring; "" means no disclosure
	}{
		{name: "a readable answer", batch: dateBatch{dates: map[string]string{"r": good}}, rev: "r", wantDate: good},
		{name: "batch error", batch: dateBatch{err: errors.New("git died")}, rev: "r", wantDisclose: "git died"},
		{name: "absent rev", batch: dateBatch{dates: map[string]string{"other": good}}, rev: "r", wantDisclose: "no committer date could be read for r"},
		{name: "nil answer map", batch: dateBatch{}, rev: "r", wantDisclose: "no committer date could be read for r"},
		{name: "empty answer", batch: dateBatch{dates: map[string]string{"r": ""}}, rev: "r", wantDisclose: "no committer date could be read for r"},
		{name: "unparsable answer", batch: dateBatch{dates: map[string]string{"r": "soon"}}, rev: "r", wantDisclose: "is not a committer date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, d := tt.batch.dateFor("spec/x", tt.rev)
			if date != tt.wantDate {
				t.Fatalf("date = %q, want %q", date, tt.wantDate)
			}
			if tt.wantDisclose == "" {
				if d != nil {
					t.Fatalf("disclosure = %+v, want nil", d)
				}
				return
			}
			if d == nil || !strings.Contains(d.Text, tt.wantDisclose) || d.Source != dateUnreadableSource || d.Scope != "spec/x" {
				t.Fatalf("disclosure = %+v, want %s on spec/x carrying %q", d, dateUnreadableSource, tt.wantDisclose)
			}
		})
	}
}

func TestLandingDate(t *testing.T) {
	const good = "2024-06-01T12:00:00+00:00"
	batch := dateBatch{dates: map[string]string{"land": good}}
	tests := []struct {
		name         string
		r            specstate.Result
		wantDate     string
		wantSource   string
		wantDisclose []string
	}{
		{
			name:     "landed: the landing commit's date",
			r:        specstate.Result{State: specstate.AcceptedPendingBuild, Baseline: &specstate.Baseline{LandingCommit: "land"}},
			wantDate: good,
		},
		{
			name:         "unproven, nil baseline: names the unproven state and specstate's witness",
			r:            specstate.Result{State: specstate.Unproven, Disclosures: []string{"w1", "w2"}},
			wantSource:   dateUnprovenSource,
			wantDisclose: []string{"effective lifecycle state is unproven", "(w1; w2)"},
		},
		{
			name:         "a baseline with no landing commit: names the state",
			r:            specstate.Result{State: specstate.Closed, Baseline: &specstate.Baseline{Blob: "b"}},
			wantSource:   dateUnprovenSource,
			wantDisclose: []string{"no landing commit for this entry's closed state"},
		},
		{
			name:         "landed but the batch cannot date it: unreadable",
			r:            specstate.Result{State: specstate.AcceptedPendingBuild, Baseline: &specstate.Baseline{LandingCommit: "gone"}},
			wantSource:   dateUnreadableSource,
			wantDisclose: []string{"gone"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, d := landingDate("spec/x", tt.r, batch)
			if date != tt.wantDate {
				t.Fatalf("date = %q, want %q", date, tt.wantDate)
			}
			if tt.wantSource == "" {
				if d != nil {
					t.Fatalf("disclosure = %+v, want nil", d)
				}
				return
			}
			if d == nil || d.Source != tt.wantSource {
				t.Fatalf("disclosure = %+v, want source %s", d, tt.wantSource)
			}
			for _, want := range tt.wantDisclose {
				if !strings.Contains(d.Text, want) {
					t.Errorf("disclosure text %q lacks %q", d.Text, want)
				}
			}
			if strings.Contains(d.Text, oldBlanketDateText) {
				t.Errorf("disclosure text %q carries the retired blanket claim", d.Text)
			}
		})
	}
}
