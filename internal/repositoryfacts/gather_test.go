package repositoryfacts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

// =========================================================================
// Step 1: byte-compatible shared fact shape (canonical encoding parity +
// known/value invariants).
// =========================================================================

// TestFacts_CanonicalEncodingMatchesJourneyWireShape pins Facts's exact
// wire shape against internal/journey's pre-extraction RepositoryFacts
// canonical output, captured byte-for-byte from
// internal/journey/testdata/canonical-record.json's own "repository"
// section (a committed golden fixture this task must not change). Any
// json-tag rename, field addition/removal, or key-ordering change that
// silently diverged the shared leaf from journey's existing wire would
// fail this test AND journey's own golden fixture test.
func TestFacts_CanonicalEncodingMatchesJourneyWireShape(t *testing.T) {
	f := Facts{
		RemoteOrigin:  StringFact{Known: true, Value: "example.invalid/repo"},
		Branch:        StringFact{Known: true, Value: "main"},
		Head:          StringFact{Known: true, Value: "abc123"},
		DefaultBranch: DefaultBranchFact{Known: true, Name: "main", Ref: "origin/main", Head: "abc123"},
		Relationship:  RelationshipEqual,
		Dirty:         BoolFact{Known: true, Value: false},
		Staged:        BoolFact{Known: true, Value: false},
		Worktree:      WorktreeFact{Managed: true, Name: "glg-journey-projection"},
		Source:        SourceHead,
	}

	got, err := canonjson.Marshal(f)
	if err != nil {
		t.Fatalf("canonjson.Marshal(Facts): %v", err)
	}

	// Verbatim substring of internal/journey/testdata/canonical-record.json's
	// "repository" value (do not hand-edit independently of that fixture).
	want := `{"branch":{"known":true,"value":"main"},"default_branch":{"head":"abc123","known":true,"name":"main","ref":"origin/main"},"dirty":{"known":true,"value":false},"head":{"known":true,"value":"abc123"},"relationship":"equal","remote_origin":{"known":true,"value":"example.invalid/repo"},"source":"head","staged":{"known":true,"value":false},"worktree":{"managed":true,"name":"glg-journey-projection"}}` + "\n"

	if string(got) != want {
		t.Errorf("canonical Facts encoding drifted from journey's wire shape:\ngot:  %s\nwant: %s", got, want)
	}
}

// TestFacts_CanonicalEncodingUnknownEverywhere proves the all-unknown
// shape (every fact Known == false, Worktree unmanaged) canonically
// encodes with every value field at its zero value — the shape
// gatherRepositoryFacts always produced for a checkout nothing could be
// established from, never a guessed non-zero value alongside Known ==
// false.
func TestFacts_CanonicalEncodingUnknownEverywhere(t *testing.T) {
	f := Facts{Relationship: RelationshipUnknown, Source: SourceRemoteRef}
	got, err := canonjson.Marshal(f)
	if err != nil {
		t.Fatalf("canonjson.Marshal(Facts): %v", err)
	}
	want := `{"branch":{"known":false,"value":""},"default_branch":{"head":"","known":false,"name":"","ref":""},"dirty":{"known":false,"value":false},"head":{"known":false,"value":""},"relationship":"unknown","remote_origin":{"known":false,"value":""},"source":"remote-ref","staged":{"known":false,"value":false},"worktree":{"managed":false,"name":""}}` + "\n"
	if string(got) != want {
		t.Errorf("canonical Facts encoding (all-unknown) = %s, want %s", got, want)
	}
}

func TestStringFact_Validate(t *testing.T) {
	tests := []struct {
		name    string
		f       StringFact
		wantErr bool
	}{
		{"known with value", StringFact{Known: true, Value: "x"}, false},
		{"unknown with empty value", StringFact{Known: false, Value: ""}, false},
		{"known with empty value", StringFact{Known: true, Value: ""}, true},
		{"unknown with nonempty value", StringFact{Known: false, Value: "x"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.f.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate(): want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate(): %v", err)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "known") {
				t.Errorf("Validate() error = %q, want it to mention known", err)
			}
		})
	}
}

func TestBoolFact_Validate(t *testing.T) {
	tests := []struct {
		name    string
		f       BoolFact
		wantErr bool
	}{
		{"known true", BoolFact{Known: true, Value: true}, false},
		{"known false", BoolFact{Known: true, Value: false}, false},
		{"unknown false", BoolFact{Known: false, Value: false}, false},
		{"unknown true", BoolFact{Known: false, Value: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.f.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate(): want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate(): %v", err)
			}
		})
	}
}

func TestDefaultBranchFact_Validate(t *testing.T) {
	tests := []struct {
		name    string
		f       DefaultBranchFact
		wantErr bool
	}{
		{"known complete", DefaultBranchFact{Known: true, Name: "main", Ref: "origin/main", Head: "abc"}, false},
		{"unknown empty", DefaultBranchFact{Known: false}, false},
		{"known missing name", DefaultBranchFact{Known: true, Name: "", Ref: "r", Head: "h"}, true},
		{"known missing ref", DefaultBranchFact{Known: true, Name: "n", Ref: "", Head: "h"}, true},
		{"known missing head", DefaultBranchFact{Known: true, Name: "n", Ref: "r", Head: ""}, true},
		{"unknown with name set", DefaultBranchFact{Known: false, Name: "main"}, true},
		{"unknown with ref set", DefaultBranchFact{Known: false, Ref: "origin/main"}, true},
		{"unknown with head set", DefaultBranchFact{Known: false, Head: "abc"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.f.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate(): want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate(): %v", err)
			}
		})
	}
}

func TestWorktreeFactType_Validate(t *testing.T) {
	tests := []struct {
		name    string
		f       WorktreeFact
		wantErr bool
	}{
		{"managed with name", WorktreeFact{Managed: true, Name: "x"}, false},
		{"unmanaged without name", WorktreeFact{Managed: false, Name: ""}, false},
		{"managed without name", WorktreeFact{Managed: true, Name: ""}, true},
		{"unmanaged with name", WorktreeFact{Managed: false, Name: "x"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.f.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate(): want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate(): %v", err)
			}
		})
	}
}

func TestSource_Validate(t *testing.T) {
	for _, s := range []Source{SourceHead, SourceWorkingTree, SourceRemoteRef, SourceReceiptBound} {
		t.Run(string(s), func(t *testing.T) {
			if err := s.Validate(); err != nil {
				t.Fatalf("Validate(%q): %v", s, err)
			}
		})
	}
	t.Run("unknown source", func(t *testing.T) {
		if err := Source("cache").Validate(); err == nil {
			t.Fatal("Validate(): want error for unknown source")
		}
	})
}

func TestDisclosureCode_Validate(t *testing.T) {
	for code := range validDisclosureCode {
		t.Run(string(code), func(t *testing.T) {
			if err := code.Validate(); err != nil {
				t.Fatalf("Validate(%q): %v", code, err)
			}
		})
	}
	t.Run("unknown code", func(t *testing.T) {
		if err := DisclosureCode("bogus-code").Validate(); err == nil {
			t.Fatal("Validate(): want error for unknown disclosure code")
		}
	})
}

// validFacts returns a fully-known, valid Facts fixture happy-path tests
// mutate from.
func validFacts() Facts {
	return Facts{
		RemoteOrigin:  StringFact{Known: true, Value: "example.invalid/repo"},
		Branch:        StringFact{Known: true, Value: "main"},
		Head:          StringFact{Known: true, Value: "abc123"},
		DefaultBranch: DefaultBranchFact{Known: true, Name: "main", Ref: "origin/main", Head: "abc123"},
		Relationship:  RelationshipEqual,
		Dirty:         BoolFact{Known: true, Value: false},
		Staged:        BoolFact{Known: true, Value: false},
		Worktree:      WorktreeFact{Managed: false},
		Source:        SourceHead,
	}
}

func TestFacts_Validate(t *testing.T) {
	if err := validFacts().Validate(); err != nil {
		t.Fatalf("Validate() on well-formed fixture: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Facts)
	}{
		{"remote origin invalid", func(f *Facts) { f.RemoteOrigin = StringFact{Known: true, Value: ""} }},
		{"branch invalid", func(f *Facts) { f.Branch = StringFact{Known: false, Value: "x"} }},
		{"head invalid", func(f *Facts) { f.Head = StringFact{Known: true, Value: ""} }},
		{"default branch invalid", func(f *Facts) { f.DefaultBranch = DefaultBranchFact{Known: true} }},
		{"unknown relationship", func(f *Facts) { f.Relationship = "parallel" }},
		{"dirty invalid", func(f *Facts) { f.Dirty = BoolFact{Known: false, Value: true} }},
		{"staged invalid", func(f *Facts) { f.Staged = BoolFact{Known: false, Value: true} }},
		{"worktree invalid", func(f *Facts) { f.Worktree = WorktreeFact{Managed: true} }},
		{"unknown source", func(f *Facts) { f.Source = "cache" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := validFacts()
			tt.mutate(&f)
			if err := f.Validate(); err == nil {
				t.Fatal("Validate(): want error")
			}
		})
	}
}

func TestSnapshot_Validate(t *testing.T) {
	tests := []struct {
		name    string
		snap    Snapshot
		wantErr bool
	}{
		{"valid, no disclosures", Snapshot{Facts: validFacts(), Disclosures: []DisclosureCode{}}, false},
		{
			"valid, sorted unique disclosures",
			Snapshot{Facts: validFacts(), Disclosures: []DisclosureCode{DisclosureDirtyUnknown, DisclosureStagedUnknown}},
			false,
		},
		{"nil disclosures", Snapshot{Facts: validFacts(), Disclosures: nil}, true},
		{
			"unsorted disclosures",
			Snapshot{Facts: validFacts(), Disclosures: []DisclosureCode{DisclosureStagedUnknown, DisclosureDirtyUnknown}},
			true,
		},
		{
			"duplicate disclosures",
			Snapshot{Facts: validFacts(), Disclosures: []DisclosureCode{DisclosureDirtyUnknown, DisclosureDirtyUnknown}},
			true,
		},
		{
			"unknown disclosure code",
			Snapshot{Facts: validFacts(), Disclosures: []DisclosureCode{"bogus"}},
			true,
		},
		{
			"invalid facts propagates",
			Snapshot{Facts: Facts{Relationship: "parallel", Source: SourceHead}, Disclosures: []DisclosureCode{}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.snap.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate(): want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate(): %v", err)
			}
		})
	}
}

// =========================================================================
// Step 2: gathering behind the shared consumer-owned port.
// =========================================================================

// fakeGitReader is the in-process GitReader double every Gather test
// proves its behavior against, with no real git process at all —
// mirroring internal/journey/fake_test.go's identically named convention.
// A nil func field panics if called, so a test that forgets to wire a
// dependency fails loudly.
type fakeGitReader struct {
	revParseFn      func(ctx context.Context, dir, rev string) (string, error)
	currentBranchFn func(ctx context.Context, dir string) (string, error)
	remoteURLFn     func(ctx context.Context, dir, name string) (string, error)
	statusDirtyFn   func(ctx context.Context, dir string) (bool, error)
	stagedPathsFn   func(ctx context.Context, dir string) ([]string, error)
	showFn          func(ctx context.Context, dir, ref, path string) ([]byte, error)
	isAncestorFn    func(ctx context.Context, dir, ancestor, ref string) (bool, error)
}

func (f *fakeGitReader) RevParse(ctx context.Context, dir, rev string) (string, error) {
	return f.revParseFn(ctx, dir, rev)
}

func (f *fakeGitReader) CurrentBranch(ctx context.Context, dir string) (string, error) {
	return f.currentBranchFn(ctx, dir)
}

func (f *fakeGitReader) RemoteURL(ctx context.Context, dir, name string) (string, error) {
	return f.remoteURLFn(ctx, dir, name)
}

func (f *fakeGitReader) StatusDirty(ctx context.Context, dir string) (bool, error) {
	return f.statusDirtyFn(ctx, dir)
}

func (f *fakeGitReader) StagedPaths(ctx context.Context, dir string) ([]string, error) {
	return f.stagedPathsFn(ctx, dir)
}

func (f *fakeGitReader) Show(ctx context.Context, dir, ref, path string) ([]byte, error) {
	return f.showFn(ctx, dir, ref, path)
}

func (f *fakeGitReader) IsAncestor(ctx context.Context, dir, ancestor, ref string) (bool, error) {
	return f.isAncestorFn(ctx, dir, ancestor, ref)
}

var _ GitReader = (*fakeGitReader)(nil)

// baseGitReader returns a fake wired with harmless defaults for every
// method Gather calls, so a table-driven test only needs to override the
// one or two methods it actually exercises.
func baseGitReader() *fakeGitReader {
	return &fakeGitReader{
		remoteURLFn:     func(context.Context, string, string) (string, error) { return "", gitx.ErrNoSuchRemote },
		currentBranchFn: func(context.Context, string) (string, error) { return "main", nil },
		revParseFn:      func(context.Context, string, string) (string, error) { return "sha", nil },
		statusDirtyFn:   func(context.Context, string) (bool, error) { return false, nil },
		stagedPathsFn:   func(context.Context, string) ([]string, error) { return nil, nil },
		showFn:          func(context.Context, string, string, string) ([]byte, error) { return nil, errors.New("not found") },
		isAncestorFn:    func(context.Context, string, string, string) (bool, error) { return false, nil },
	}
}

func alwaysUnresolvedDefaultBranch(context.Context, string) (specstate.Branch, bool) {
	return specstate.Branch{}, false
}

func containsCode(codes []DisclosureCode, want DisclosureCode) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

func TestGatherer_ZeroValue_FailsClosed(t *testing.T) {
	var g Gatherer
	_, err := g.Gather(context.Background(), GatherInput{Root: "/tmp/x"})
	if err == nil {
		t.Fatal("Gather() on zero-value Gatherer: want error")
	}
}

func TestGather_RemoteOrigin(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL func(context.Context, string, string) (string, error)
		wantKnown bool
		wantValue string
		wantCode  DisclosureCode
	}{
		{
			// Never the raw origin URL: scheme, userinfo, and the ".git"
			// suffix are gone (gitx.CanonicalRemoteIdentity).
			name:      "known, scp-like spelling",
			remoteURL: func(context.Context, string, string) (string, error) { return "git@example.com:x/y.git", nil },
			wantKnown: true,
			wantValue: "example.com/x/y",
		},
		{
			// Same repository, https spelling: the SAME identity, so two
			// checkouts of one repository never disagree.
			name:      "known, https spelling of the same repository",
			remoteURL: func(context.Context, string, string) (string, error) { return "https://Example.com/x/y.git", nil },
			wantKnown: true,
			wantValue: "example.com/x/y",
		},
		{
			name: "credential-bearing url is reduced to an identity",
			remoteURL: func(context.Context, string, string) (string, error) {
				return "https://user:s3cr3t-TOKEN@example.com/x/y.git", nil
			},
			wantKnown: true,
			wantValue: "example.com/x/y",
		},
		{
			name:      "uncanonicalizable url is unknown and disclosed",
			remoteURL: func(context.Context, string, string) (string, error) { return "file:///srv/git/y.git", nil },
			wantKnown: false,
			wantCode:  DisclosureRemoteOriginUncanonicalizable,
		},
		{
			name: "no such remote",
			remoteURL: func(context.Context, string, string) (string, error) {
				return "", gitx.ErrNoSuchRemote
			},
			wantKnown: false,
			wantCode:  DisclosureRemoteOriginNotConfigured,
		},
		{
			name: "other error",
			remoteURL: func(context.Context, string, string) (string, error) {
				return "", errors.New("boom")
			},
			wantKnown: false,
			wantCode:  DisclosureRemoteOriginReadFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := baseGitReader()
			git.remoteURLFn = tt.remoteURL
			g := newGatherer(git, alwaysUnresolvedDefaultBranch)

			snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("x"), TargetFoundOnDisk: true})
			if err != nil {
				t.Fatalf("Gather: %v", err)
			}
			if snap.Facts.RemoteOrigin.Known != tt.wantKnown || snap.Facts.RemoteOrigin.Value != tt.wantValue {
				t.Fatalf("RemoteOrigin = %+v", snap.Facts.RemoteOrigin)
			}
			if tt.wantCode != "" && !containsCode(snap.Disclosures, tt.wantCode) {
				t.Fatalf("Disclosures = %v, want to contain %q", snap.Disclosures, tt.wantCode)
			}
			if err := snap.Validate(); err != nil {
				t.Fatalf("Snapshot.Validate(): %v", err)
			}
		})
	}
}

// TestGather_RemoteOriginNeverCarriesCredentials is the negative twin of
// the table above, stated as the security property itself: for a
// credential-bearing origin URL — canonicalizable or not — neither the
// fact value nor any disclosure code may contain the secret, the
// userinfo, or the raw URL.
func TestGather_RemoteOriginNeverCarriesCredentials(t *testing.T) {
	const secret = "s3cr3t-TOKEN"
	urls := []string{
		"https://user:" + secret + "@example.com/x/y.git",
		"ssh://user:" + secret + "@example.com:2222/x/y.git",
		"file://user:" + secret + "@example.com/x/y.git",
	}
	for _, raw := range urls {
		t.Run(raw, func(t *testing.T) {
			git := baseGitReader()
			git.remoteURLFn = func(context.Context, string, string) (string, error) { return raw, nil }
			g := newGatherer(git, alwaysUnresolvedDefaultBranch)

			snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("x"), TargetFoundOnDisk: true})
			if err != nil {
				t.Fatalf("Gather: %v", err)
			}
			if strings.Contains(snap.Facts.RemoteOrigin.Value, secret) || strings.Contains(snap.Facts.RemoteOrigin.Value, "user@") {
				t.Fatalf("credential material leaked into the fact: %q", snap.Facts.RemoteOrigin.Value)
			}
			for _, code := range snap.Disclosures {
				if strings.Contains(string(code), secret) {
					t.Fatalf("credential material leaked into a disclosure code: %q", code)
				}
			}
		})
	}
}

func TestGather_Branch(t *testing.T) {
	tests := []struct {
		name          string
		currentBranch func(context.Context, string) (string, error)
		wantKnown     bool
		wantValue     string
		wantCode      DisclosureCode
	}{
		{"known", func(context.Context, string) (string, error) { return "main", nil }, true, "main", ""},
		{"detached HEAD", func(context.Context, string) (string, error) { return "", nil }, false, "", DisclosureBranchDetached},
		{"error", func(context.Context, string) (string, error) { return "", errors.New("boom") }, false, "", DisclosureBranchUnresolved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := baseGitReader()
			git.currentBranchFn = tt.currentBranch
			g := newGatherer(git, alwaysUnresolvedDefaultBranch)

			snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("x"), TargetFoundOnDisk: true})
			if err != nil {
				t.Fatalf("Gather: %v", err)
			}
			if snap.Facts.Branch.Known != tt.wantKnown || snap.Facts.Branch.Value != tt.wantValue {
				t.Fatalf("Branch = %+v", snap.Facts.Branch)
			}
			if tt.wantCode != "" && !containsCode(snap.Disclosures, tt.wantCode) {
				t.Fatalf("Disclosures = %v, want to contain %q", snap.Disclosures, tt.wantCode)
			}
		})
	}
}

func TestGather_Head(t *testing.T) {
	tests := []struct {
		name      string
		revParse  func(context.Context, string, string) (string, error)
		wantKnown bool
		wantCode  DisclosureCode
	}{
		{"known", func(context.Context, string, string) (string, error) { return "headsha", nil }, true, ""},
		{"error", func(context.Context, string, string) (string, error) { return "", errors.New("boom") }, false, DisclosureHeadUnresolved},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := baseGitReader()
			git.revParseFn = tt.revParse
			g := newGatherer(git, alwaysUnresolvedDefaultBranch)

			snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("x"), TargetFoundOnDisk: true})
			if err != nil {
				t.Fatalf("Gather: %v", err)
			}
			if snap.Facts.Head.Known != tt.wantKnown {
				t.Fatalf("Head = %+v, want known=%v", snap.Facts.Head, tt.wantKnown)
			}
			if tt.wantCode != "" && !containsCode(snap.Disclosures, tt.wantCode) {
				t.Fatalf("Disclosures = %v, want to contain %q", snap.Disclosures, tt.wantCode)
			}
		})
	}
}

func TestGather_DefaultBranchAndRelationship(t *testing.T) {
	tests := []struct {
		name         string
		resolveDB    DefaultBranchResolver
		revParse     func(context.Context, string, string) (string, error)
		isAncestor   func(context.Context, string, string, string) (bool, error)
		wantDBKnown  bool
		wantRelation string
	}{
		{
			name:         "unresolved default branch",
			resolveDB:    alwaysUnresolvedDefaultBranch,
			revParse:     func(_ context.Context, _, rev string) (string, error) { return "headsha", nil },
			wantDBKnown:  false,
			wantRelation: RelationshipUnknown,
		},
		{
			name: "equal",
			resolveDB: func(context.Context, string) (specstate.Branch, bool) {
				return specstate.Branch{Name: "main", Ref: "origin/main"}, true
			},
			revParse: func(_ context.Context, _, rev string) (string, error) {
				return "samesha", nil
			},
			wantDBKnown:  true,
			wantRelation: RelationshipEqual,
		},
		{
			name: "ahead",
			resolveDB: func(context.Context, string) (specstate.Branch, bool) {
				return specstate.Branch{Name: "main", Ref: "origin/main"}, true
			},
			revParse: func(_ context.Context, _, rev string) (string, error) {
				if rev == "HEAD" {
					return "headsha", nil
				}
				return "defaultsha", nil
			},
			isAncestor: func(_ context.Context, _, ancestor, ref string) (bool, error) {
				return ancestor == "defaultsha" && ref == "headsha", nil
			},
			wantDBKnown:  true,
			wantRelation: RelationshipAhead,
		},
		{
			name: "behind",
			resolveDB: func(context.Context, string) (specstate.Branch, bool) {
				return specstate.Branch{Name: "main", Ref: "origin/main"}, true
			},
			revParse: func(_ context.Context, _, rev string) (string, error) {
				if rev == "HEAD" {
					return "headsha", nil
				}
				return "defaultsha", nil
			},
			isAncestor: func(_ context.Context, _, ancestor, ref string) (bool, error) {
				return ancestor == "headsha" && ref == "defaultsha", nil
			},
			wantDBKnown:  true,
			wantRelation: RelationshipBehind,
		},
		{
			name: "diverged",
			resolveDB: func(context.Context, string) (specstate.Branch, bool) {
				return specstate.Branch{Name: "main", Ref: "origin/main"}, true
			},
			revParse: func(_ context.Context, _, rev string) (string, error) {
				if rev == "HEAD" {
					return "headsha", nil
				}
				return "defaultsha", nil
			},
			isAncestor:   func(context.Context, string, string, string) (bool, error) { return false, nil },
			wantDBKnown:  true,
			wantRelation: RelationshipDiverged,
		},
		{
			name: "ancestry error yields unknown relationship",
			resolveDB: func(context.Context, string) (specstate.Branch, bool) {
				return specstate.Branch{Name: "main", Ref: "origin/main"}, true
			},
			revParse: func(_ context.Context, _, rev string) (string, error) {
				if rev == "HEAD" {
					return "headsha", nil
				}
				return "defaultsha", nil
			},
			isAncestor:   func(context.Context, string, string, string) (bool, error) { return false, errors.New("boom") },
			wantDBKnown:  true,
			wantRelation: RelationshipUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := baseGitReader()
			git.revParseFn = tt.revParse
			if tt.isAncestor != nil {
				git.isAncestorFn = tt.isAncestor
			}
			g := newGatherer(git, tt.resolveDB)

			snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("x"), TargetFoundOnDisk: true})
			if err != nil {
				t.Fatalf("Gather: %v", err)
			}
			if snap.Facts.DefaultBranch.Known != tt.wantDBKnown {
				t.Fatalf("DefaultBranch.Known = %v, want %v (%+v)", snap.Facts.DefaultBranch.Known, tt.wantDBKnown, snap.Facts.DefaultBranch)
			}
			if snap.Facts.Relationship != tt.wantRelation {
				t.Fatalf("Relationship = %q, want %q", snap.Facts.Relationship, tt.wantRelation)
			}
		})
	}
}

// TestGather_DefaultBranchRevParseFails proves the default branch NAME
// resolving but RevParse of its own ref failing is a distinct, disclosed
// failure — never silently the same "unknown" as no default branch
// resolving at all (which this leaf deliberately never discloses; see
// DisclosureDefaultBranchRefUnresolved's doc comment).
func TestGather_DefaultBranchRevParseFails(t *testing.T) {
	git := baseGitReader()
	git.revParseFn = func(_ context.Context, _, rev string) (string, error) {
		if rev == "HEAD" {
			return "headsha", nil
		}
		return "", errors.New("boom")
	}
	resolveDB := func(context.Context, string) (specstate.Branch, bool) {
		return specstate.Branch{Name: "main", Ref: "origin/main"}, true
	}
	g := newGatherer(git, resolveDB)

	snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("x"), TargetFoundOnDisk: true})
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if snap.Facts.DefaultBranch.Known {
		t.Fatalf("DefaultBranch = %+v, want unknown", snap.Facts.DefaultBranch)
	}
	if !containsCode(snap.Disclosures, DisclosureDefaultBranchRefUnresolved) {
		t.Fatalf("Disclosures = %v, want %q", snap.Disclosures, DisclosureDefaultBranchRefUnresolved)
	}
}

// TestGather_NoDefaultBranchNeverDisclosed proves the "no default branch
// resolves at all" case is silent at this leaf (Known == false, zero
// disclosures) — a deliberate, ported behavior: that gap is a
// lifecycle/blocker-level concern a caller derives from
// Facts.DefaultBranch.Known itself, outside this package's scope.
func TestGather_NoDefaultBranchNeverDisclosed(t *testing.T) {
	// Every OTHER fact must succeed so the only possible disclosure
	// source left is the default-branch resolution this test isolates.
	git := baseGitReader()
	git.remoteURLFn = func(context.Context, string, string) (string, error) { return "https://example.com/x/y.git", nil }
	git.showFn = func(context.Context, string, string, string) ([]byte, error) { return []byte("x"), nil }
	g := newGatherer(git, alwaysUnresolvedDefaultBranch)

	snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("x"), TargetFoundOnDisk: true})
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if snap.Facts.DefaultBranch.Known {
		t.Fatalf("DefaultBranch = %+v, want unknown", snap.Facts.DefaultBranch)
	}
	if len(snap.Disclosures) != 0 {
		t.Fatalf("Disclosures = %v, want none for an unresolved default branch", snap.Disclosures)
	}
}

func TestGather_DirtyStaged(t *testing.T) {
	git := baseGitReader()
	git.statusDirtyFn = func(context.Context, string) (bool, error) { return true, nil }
	git.stagedPathsFn = func(context.Context, string) ([]string, error) { return []string{"a", "b"}, nil }
	g := newGatherer(git, alwaysUnresolvedDefaultBranch)

	snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("x"), TargetFoundOnDisk: true})
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if !snap.Facts.Dirty.Known || !snap.Facts.Dirty.Value {
		t.Fatalf("Dirty = %+v", snap.Facts.Dirty)
	}
	if !snap.Facts.Staged.Known || !snap.Facts.Staged.Value {
		t.Fatalf("Staged = %+v", snap.Facts.Staged)
	}

	git2 := baseGitReader()
	git2.statusDirtyFn = func(context.Context, string) (bool, error) { return false, errors.New("boom") }
	git2.stagedPathsFn = func(context.Context, string) ([]string, error) { return nil, errors.New("boom") }
	g2 := newGatherer(git2, alwaysUnresolvedDefaultBranch)
	snap2, err := g2.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("x"), TargetFoundOnDisk: true})
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if snap2.Facts.Dirty.Known || snap2.Facts.Staged.Known {
		t.Fatalf("Dirty/Staged should be unknown on error: %+v %+v", snap2.Facts.Dirty, snap2.Facts.Staged)
	}
	if !containsCode(snap2.Disclosures, DisclosureDirtyUnknown) || !containsCode(snap2.Disclosures, DisclosureStagedUnknown) {
		t.Fatalf("Disclosures = %v, want dirty-unknown and staged-unknown", snap2.Disclosures)
	}
}

func TestGather_Source(t *testing.T) {
	tests := []struct {
		name        string
		foundOnDisk bool
		showFn      func(context.Context, string, string, string) ([]byte, error)
		content     []byte
		want        Source
	}{
		{
			name:        "matches HEAD",
			foundOnDisk: true,
			showFn:      func(context.Context, string, string, string) ([]byte, error) { return []byte("same"), nil },
			content:     []byte("same"),
			want:        SourceHead,
		},
		{
			name:        "differs from HEAD",
			foundOnDisk: true,
			showFn:      func(context.Context, string, string, string) ([]byte, error) { return []byte("old"), nil },
			content:     []byte("new"),
			want:        SourceWorkingTree,
		},
		{
			name:        "absent at HEAD",
			foundOnDisk: true,
			showFn:      func(context.Context, string, string, string) ([]byte, error) { return nil, errors.New("not found") },
			content:     []byte("new"),
			want:        SourceWorkingTree,
		},
		{
			name:        "remote-ref fallback",
			foundOnDisk: false,
			showFn:      func(context.Context, string, string, string) ([]byte, error) { panic("should not be called") },
			content:     []byte("remote"),
			want:        SourceRemoteRef,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := baseGitReader()
			git.showFn = tt.showFn
			g := newGatherer(git, alwaysUnresolvedDefaultBranch)

			snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: tt.content, TargetFoundOnDisk: tt.foundOnDisk})
			if err != nil {
				t.Fatalf("Gather: %v", err)
			}
			if snap.Facts.Source != tt.want {
				t.Fatalf("Source = %q, want %q", snap.Facts.Source, tt.want)
			}
		})
	}
}

func TestWorktreeFact(t *testing.T) {
	tests := []struct {
		name        string
		root        string
		wantManaged bool
		wantName    string
	}{
		{"unmanaged plain checkout", "/home/user/code/verdi", false, ""},
		{"managed worktree", "/home/user/code/verdi/.verdi/data/worktrees/my-design", true, "my-design"},
		{"managed worktree nested path", "/home/user/code/verdi/.verdi/data/worktrees/my-design/sub", true, "my-design"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := worktreeFact(tt.root)
			if got.Managed != tt.wantManaged || got.Name != tt.wantName {
				t.Fatalf("worktreeFact(%q) = %+v, want managed=%v name=%q", tt.root, got, tt.wantManaged, tt.wantName)
			}
		})
	}
}

// TestGather_HappyPath_FullyKnown proves an entirely successful gather
// yields zero disclosures and a Snapshot that validates.
func TestGather_HappyPath_FullyKnown(t *testing.T) {
	git := baseGitReader()
	git.remoteURLFn = func(context.Context, string, string) (string, error) { return "https://example.com/x/y.git", nil }
	git.showFn = func(context.Context, string, string, string) ([]byte, error) { return []byte("same"), nil }
	resolveDB := func(context.Context, string) (specstate.Branch, bool) {
		return specstate.Branch{Name: "main", Ref: "origin/main"}, true
	}
	g := newGatherer(git, resolveDB)

	snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("same"), TargetFoundOnDisk: true})
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if len(snap.Disclosures) != 0 {
		t.Fatalf("Disclosures = %v, want none", snap.Disclosures)
	}
	if snap.Facts.Source != SourceHead {
		t.Fatalf("Source = %q, want head", snap.Facts.Source)
	}
	if err := snap.Validate(); err != nil {
		t.Fatalf("Snapshot.Validate(): %v", err)
	}
}

// TestGather_DisclosuresSortedAndDeduped proves a gather that hits
// multiple independent failure causes returns them sorted ascending —
// Snapshot.Validate enforces this, but this test additionally proves
// Gather itself, not just a well-formed literal, produces that order.
func TestGather_DisclosuresSortedAndDeduped(t *testing.T) {
	git := baseGitReader()
	git.remoteURLFn = func(context.Context, string, string) (string, error) { return "", errors.New("boom") }
	git.currentBranchFn = func(context.Context, string) (string, error) { return "", nil }
	git.revParseFn = func(context.Context, string, string) (string, error) { return "", errors.New("boom") }
	git.statusDirtyFn = func(context.Context, string) (bool, error) { return false, errors.New("boom") }
	git.stagedPathsFn = func(context.Context, string) ([]string, error) { return nil, errors.New("boom") }
	g := newGatherer(git, alwaysUnresolvedDefaultBranch)

	snap, err := g.Gather(context.Background(), GatherInput{Root: t.TempDir(), TargetPath: "rel/spec.md", TargetContent: []byte("x"), TargetFoundOnDisk: true})
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	if err := snap.Validate(); err != nil {
		t.Fatalf("Snapshot.Validate(): %v (disclosures = %v)", err, snap.Disclosures)
	}
	for i := 1; i < len(snap.Disclosures); i++ {
		if snap.Disclosures[i] <= snap.Disclosures[i-1] {
			t.Fatalf("Disclosures = %v, not strictly ascending", snap.Disclosures)
		}
	}
}
