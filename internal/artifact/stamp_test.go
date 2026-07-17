package artifact

import (
	"reflect"
	"strings"
	"testing"
)

func TestNewFrozen_Happy(t *testing.T) {
	f := NewFrozen("2026-07-16", "3e91ab2")
	want := Frozen{At: "2026-07-16", Commit: "3e91ab2"}
	if f != want {
		t.Fatalf("NewFrozen(...) = %+v, want %+v", f, want)
	}
}

// TestNewFrozen_RejectsEmptyArgs is NewFrozen's negative path (L-M4): the
// constructor's signature returns a bare Frozen with no error channel, so
// "rejects" can only mean panics — the same fail-closed posture CLAUDE.md's
// "never fake success" applies everywhere else in this module, aimed here at
// a caller bug (an uninitialized stamp) rather than a runtime condition.
func TestNewFrozen_RejectsEmptyArgs(t *testing.T) {
	cases := []struct {
		name          string
		at, commit    string
		wantSubstring string
	}{
		{name: "empty at", at: "", commit: "3e91ab2", wantSubstring: "at must not be empty"},
		{name: "empty commit", at: "2026-07-16", commit: "", wantSubstring: "commit must not be empty"},
		{name: "both empty", at: "", commit: "", wantSubstring: "at must not be empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("NewFrozen(%q, %q): want panic, got none", c.at, c.commit)
				}
				msg, ok := r.(string)
				if !ok || !strings.Contains(msg, c.wantSubstring) {
					t.Fatalf("NewFrozen(%q, %q): panic = %v, want substring %q", c.at, c.commit, r, c.wantSubstring)
				}
			}()
			NewFrozen(c.at, c.commit)
		})
	}
}

// TestStampProvenance_Happy documents today's contract (L-M5 has not landed
// yet): Provenance carries no Model field, so modelDigest is unused and
// StampProvenance must leave every existing field exactly as it found it —
// a real assertion, not a placeholder, so a future accidental mutation
// fails this test rather than silently changing behavior.
func TestStampProvenance_Happy(t *testing.T) {
	p := &Provenance{
		Generator: "verdi-align",
		Version:   "v0",
		Inputs:    []string{"spec/foo@" + hex64[:7]},
		Digest:    "sha256:" + hex64,
		Integrity: "sha256:" + hex64,
	}
	before := *p
	StampProvenance(p, "")
	if !reflect.DeepEqual(*p, before) {
		t.Fatalf("StampProvenance(p, \"\") mutated p: before %+v, after %+v (modelDigest is unused until L-M5)", before, *p)
	}

	// A non-empty modelDigest is equally inert today — the field it would
	// write to (Provenance.Model) does not exist until L-M5.
	StampProvenance(p, "sha256:"+hex64)
	if !reflect.DeepEqual(*p, before) {
		t.Fatalf("StampProvenance(p, sha256:...) mutated p: before %+v, after %+v (modelDigest is unused until L-M5)", before, *p)
	}
}

func TestStampProvenance_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("StampProvenance(nil, \"\"): want panic, got none")
		}
	}()
	StampProvenance(nil, "")
}
