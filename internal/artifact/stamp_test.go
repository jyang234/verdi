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

// TestStampProvenance_Happy proves StampProvenance (L-M5, spec/model-digest)
// writes exactly Model and leaves every other field untouched.
func TestStampProvenance_Happy(t *testing.T) {
	p := &Provenance{
		Generator: "verdi-align",
		Version:   "v0",
		Inputs:    []string{"spec/foo@" + hex64[:7]},
		Digest:    "sha256:" + hex64,
		Integrity: "sha256:" + hex64,
	}
	before := *p
	modelDigest := "sha256:" + strings.Repeat("cd", 32)

	StampProvenance(p, modelDigest)

	if p.Model != modelDigest {
		t.Fatalf("StampProvenance: p.Model = %q, want %q", p.Model, modelDigest)
	}
	// Every other field is untouched.
	after := *p
	after.Model = "" // isolate the one field this seam is allowed to change
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("StampProvenance mutated a field other than Model: before %+v, after (Model cleared) %+v", before, after)
	}
}

// TestStampProvenance_Deterministic proves two stamps of identical p/digest
// values produce byte-identical results (ac-1's "identical across repeated
// runs" extended to this seam directly).
func TestStampProvenance_Deterministic(t *testing.T) {
	modelDigest := "sha256:" + strings.Repeat("ef", 32)
	build := func() Provenance {
		p := &Provenance{
			Generator: "verdi-align",
			Version:   "v0",
			Inputs:    []string{"spec/foo@" + hex64[:7]},
			Digest:    "sha256:" + hex64,
		}
		StampProvenance(p, modelDigest)
		return *p
	}
	first, second := build(), build()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("StampProvenance not deterministic: first %+v, second %+v", first, second)
	}
}

func TestStampProvenance_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("StampProvenance(nil, \"sha256:...\"): want panic, got none")
		}
	}()
	StampProvenance(nil, "sha256:"+hex64)
}

// TestStampProvenance_EmptyDigestPanics is the seam's other fail-closed
// edge (Outcome: "panics on an empty modelDigest rather than silently
// minting an artifact with an absent model claim from a call site that
// should always have a real one") — mirrors NewFrozen's own empty-argument
// panic convention above.
func TestStampProvenance_EmptyDigestPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("StampProvenance(p, \"\"): want panic, got none")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "modelDigest must not be empty") {
			t.Fatalf("StampProvenance(p, \"\"): panic = %v, want substring %q", r, "modelDigest must not be empty")
		}
	}()
	StampProvenance(&Provenance{
		Generator: "verdi-align", Version: "v0", Inputs: []string{"spec/foo@" + hex64[:7]}, Digest: "sha256:" + hex64,
	}, "")
}
