package dex

import (
	"context"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
	"github.com/OWNER/verdi/internal/gitx"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		kind   string
		frozen bool
		want   temporalClass
	}{
		{"spec", true, classFrozen},
		{"adr", true, classFrozen},
		{"external", false, classLivingGated},
		{"spec", false, classAuthoredLiving},
		{"diagram", false, classAuthoredLiving},
	}
	for _, tc := range cases {
		if got := classify(tc.kind, tc.frozen); got != tc.want {
			t.Errorf("classify(%q, %v) = %v, want %v", tc.kind, tc.frozen, got, tc.want)
		}
	}
}

func TestLivingGatedBanner(t *testing.T) {
	got := livingGatedBanner(buildStamp{SHA: "c5e360a9ee5e9eb6089e54b772fa16959ada4662", Date: "2024-01-01"})
	want := "main @ c5e360a · 2024-01-01"
	if got != want {
		t.Fatalf("livingGatedBanner = %q, want %q", got, want)
	}
}

func TestFrozenBanner(t *testing.T) {
	got := frozenBanner("2026-05-14", "c5e360a9ee5e9eb6089e54b772fa16959ada4662")
	want := "point-in-time record · frozen 2026-05-14 @ c5e360a"
	if got != want {
		t.Fatalf("frozenBanner = %q, want %q", got, want)
	}
}

func TestAuthoredLivingBanner(t *testing.T) {
	got := authoredLivingBanner(gitx.Commit{SHA: "706a74e31c85917a62314e99c72d1ddd8a7ac261", Date: "2026-06-20T00:00:00+00:00"})
	if !strings.Contains(got, "last-modified") || !strings.Contains(got, "2026-06-20") {
		t.Fatalf("authoredLivingBanner = %q, want it to mention last-modified and 2026-06-20", got)
	}
}

func TestResolveBuildStamp_Happy(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
	})

	stamp, err := resolveBuildStamp(context.Background(), repo.Dir, "HEAD")
	if err != nil {
		t.Fatalf("resolveBuildStamp: %v", err)
	}
	if stamp.SHA != repo.Head {
		t.Errorf("SHA = %q, want %q", stamp.SHA, repo.Head)
	}
	// fixturegit pins every commit's date to 2024-01-01.
	if stamp.Date != "2024-01-01" {
		t.Errorf("Date = %q, want 2024-01-01", stamp.Date)
	}
}

func TestResolveBuildStamp_Negative_UnknownCommit(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"a.txt": "hello\n"}, Message: "add a"},
	})
	if _, err := resolveBuildStamp(context.Background(), repo.Dir, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); err == nil {
		t.Fatal("resolveBuildStamp: expected an error for an unknown commit, got nil")
	}
}

func TestDateOnly(t *testing.T) {
	if got := dateOnly("2026-05-14T12:34:56+00:00"); got != "2026-05-14" {
		t.Errorf("dateOnly = %q, want 2026-05-14", got)
	}
	if got := dateOnly("2026-05-14"); got != "2026-05-14" {
		t.Errorf("dateOnly (no T) = %q, want 2026-05-14", got)
	}
}

func TestShortSHA(t *testing.T) {
	if got := shortSHA("c5e360a9ee5e9eb6089e54b772fa16959ada4662"); got != "c5e360a" {
		t.Errorf("shortSHA = %q, want c5e360a", got)
	}
	if got := shortSHA("abc"); got != "abc" {
		t.Errorf("shortSHA(short) = %q, want abc", got)
	}
}
