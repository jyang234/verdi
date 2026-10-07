package refindex

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
)

// fakeDraftSpec is a valid draft whose title differs from its ref, so a
// title invented from the ref can never pass for the decoded one.
const fakeDraftSpec = `---
id: spec/alpha
kind: spec
class: component
title: "Alpha draft, decoded"
status: draft
owners: [platform-team]
---
# Alpha
`

// fakeUndecodableSpec splits but fails the strict decode: its frontmatter
// carries a field no spec declares.
const fakeUndecodableSpec = `---
id: spec/broken
kind: spec
class: component
title: "Broken"
status: draft
owners: [platform-team]
not_a_spec_field: true
---
# Broken
`

// draftTitleGitRunner is a fake with one default-branch component spec,
// a local draft (design/alpha), a remote-only draft (design/beta), and a
// branch with no draft spec (design/gamma-empty); each draft's content is
// drafts[ref].
func draftTitleGitRunner(drafts map[string]string) *fakeGitRunner {
	return &fakeGitRunner{
		defaultBranchFn: func(context.Context, string) (string, error) { return "main", nil },
		localDesignFn: func(context.Context, string) ([]string, error) {
			return []string{"design/alpha", "design/gamma-empty"}, nil
		},
		remoteDesignFn: func(context.Context, string) ([]string, error) { return []string{"design/beta"}, nil },
		listTreeFn: func(_ context.Context, _, ref, path string) ([]string, error) {
			switch {
			case ref == "main" && path == ".verdi/specs/active":
				return []string{".verdi/specs/active/fake/spec.md"}, nil
			case ref == "main":
				return nil, nil
			case ref == "design/gamma-empty":
				return nil, nil
			default:
				return []string{path}, nil
			}
		},
		showFn: func(_ context.Context, _, ref, _ string) ([]byte, error) {
			if ref == "main" {
				return []byte(fakeComponentSpec), nil
			}
			return []byte(drafts[ref]), nil
		},
		isAncestorFn: func(context.Context, string, string, string) (bool, error) { return false, nil },
	}
}

// TestComputeIndex_DraftTitles (spec/index-v2 ac-2; SI-366 (1)): an
// ordinary design-branch draft carries the title decoded from the spec
// content the walk already reads — local and remote-tracking alike — with
// no Show call beyond the one per spec the walk always made. A branch with
// no draft spec, and every default-branch entry, carry no title (the
// directory reads a default entry's title from the working tree). A draft
// that fails to decode fails the walk as before: no entry, so no title is
// ever invented from the ref.
func TestComputeIndex_DraftTitles(t *testing.T) {
	wantShows := []string{
		"main:.verdi/specs/active/fake/spec.md",
		"design/alpha:.verdi/specs/active/alpha/spec.md",
		"origin/design/beta:.verdi/specs/active/beta/spec.md",
	}
	tests := []struct {
		name   string
		drafts map[string]string
		// wantTitles maps each entry's ref to its Title; nil means the walk
		// must fail.
		wantTitles map[string]string
		wantErr    string
	}{
		{
			name:   "ordinary drafts carry their decoded titles; the empty branch and the default entry carry none",
			drafts: map[string]string{"design/alpha": fakeDraftSpec, "origin/design/beta": fakeStatuslessStorySpec},
			wantTitles: map[string]string{
				"spec/alpha":       "Alpha draft, decoded",
				"spec/beta":        "Fake Story",
				"spec/gamma-empty": "",
				"spec/fake":        "",
			},
		},
		{
			name:    "an undecodable draft fails the walk; no title is invented from its ref",
			drafts:  map[string]string{"design/alpha": fakeUndecodableSpec, "origin/design/beta": fakeStatuslessStorySpec},
			wantErr: "not_a_spec_field",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := draftTitleGitRunner(tt.drafts)
			got, err := ComputeIndex(context.Background(), "/fake", f, proposedResolver())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("ComputeIndex error = %v, want one naming %q", err, tt.wantErr)
				}
				if got != nil {
					t.Fatalf("ComputeIndex = %+v alongside its error, want no entries", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ComputeIndex: %v", err)
			}
			if len(got) != len(tt.wantTitles) {
				t.Fatalf("ComputeIndex = %v, want exactly %d entries", refs(got), len(tt.wantTitles))
			}
			for ref, want := range tt.wantTitles {
				if e := entryByRef(t, got, ref); e.Title != want {
					t.Errorf("%s: Title = %q, want %q", ref, e.Title, want)
				}
			}
			if !reflect.DeepEqual(f.showCalls, wantShows) {
				t.Fatalf("Show calls = %q, want exactly %q (one read per spec; the title adds none)", f.showCalls, wantShows)
			}
		})
	}
}

// TestComputeIndex_DraftTitlesFixturegit is TestComputeIndex_DraftTitles
// against real git (fixturegit): a local draft and a remote-only draft
// carry their committed titles; the default-branch entry carries none.
func TestComputeIndex_DraftTitlesFixturegit(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/specs/active/root-spec/spec.md": componentSpecMD("root-spec", "active")},
		Message: "seed default branch",
	}})
	setDefaultBranchSymref(t, repo.Dir, "main")

	checkoutNewBranch(t, repo.Dir, "design/local-draft")
	writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/local-draft/spec.md": titled(componentSpecMD("local-draft", "draft"), "local-draft", "Local draft, committed")}, "local draft")
	checkoutExisting(t, repo.Dir, "main")

	checkoutNewBranch(t, repo.Dir, "design/remote-draft-tmp")
	remoteSHA := writeAndCommit(t, repo.Dir, map[string]string{".verdi/specs/active/remote-draft/spec.md": titled(componentSpecMD("remote-draft", "draft"), "remote-draft", "Remote draft, committed")}, "remote draft")
	checkoutExisting(t, repo.Dir, "main")
	createRemoteDesignRef(t, repo.Dir, "remote-draft", remoteSHA)
	deleteLocalBranch(t, repo.Dir, "design/remote-draft-tmp")

	got, err := ComputeIndex(context.Background(), repo.Dir, NewGitRunner(), NewStateResolver())
	if err != nil {
		t.Fatalf("ComputeIndex: %v", err)
	}
	for ref, want := range map[string]string{
		"spec/local-draft":  "Local draft, committed",
		"spec/remote-draft": "Remote draft, committed",
		"spec/root-spec":    "",
	} {
		if e := entryByRef(t, got, ref); e.Title != want {
			t.Errorf("%s: Title = %q, want %q", ref, e.Title, want)
		}
	}
}

// titled replaces spec's title line (componentSpecMD's title is its name)
// with title, so a decoded title is distinguishable from any ref fragment.
func titled(spec, name, title string) string {
	return strings.Replace(spec, `title: "`+name+`"`, `title: "`+title+`"`, 1)
}
