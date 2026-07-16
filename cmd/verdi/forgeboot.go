// Store/forge bootstrap helpers (spec/file-topics ac-1): the manifest
// loader, current ref/commit resolver, and forge adapter constructors that
// eight verb files (align.go, audit.go, buildstart.go, close.go, dex.go,
// design.go, feature.go, gate.go/gate_threads.go, rollup.go, sync.go)
// consume — moved verbatim out of sync.go, which had become their de facto
// (and undocumented) home against dispatch.go's own per-verb-file charter.
// This file owns exactly this topic: nothing else.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/forge"
	forgegithub "github.com/jyang234/verdi/internal/forge/github"
	forgegitlab "github.com/jyang234/verdi/internal/forge/gitlab"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// loadManifest reads and strict-decodes root's verdi.yaml.
func loadManifest(root string) (*store.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(root, ".verdi", "verdi.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading verdi.yaml: %w", err)
	}
	m, err := store.DecodeManifest(data)
	if err != nil {
		return nil, fmt.Errorf("decoding verdi.yaml: %w", err)
	}
	return m, nil
}

// resolveRefCommit determines the current ref and commit sync operates
// on. Ref resolution prefers forge-provided CI environment variables
// (GitLab's CI_COMMIT_REF_NAME, GitHub's GITHUB_HEAD_REF for a PR run or
// GITHUB_REF_NAME for a push) over `git symbolic-ref`, since CI checkouts
// are usually detached HEAD, where symbolic-ref fails.
func resolveRefCommit(ctx context.Context, root string) (ref, commit string, err error) {
	commit, err = gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		return "", "", fmt.Errorf("resolving current commit: %w", err)
	}

	for _, envVar := range []string{"CI_COMMIT_REF_NAME", "GITHUB_HEAD_REF", "GITHUB_REF_NAME"} {
		if v := os.Getenv(envVar); v != "" {
			return v, commit, nil
		}
	}
	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		return "", "", fmt.Errorf("resolving current ref: %w", err)
	}
	// CurrentBranch returns ("", nil) for a detached HEAD (a normal git
	// state lint tolerates — I-14); sync, unlike lint, cannot proceed
	// without a ref name to slug, so absence is an operational error here.
	if branch == "" {
		return "", "", fmt.Errorf("resolving current ref: detached HEAD and no CI ref env var set (CI_COMMIT_REF_NAME / GITHUB_REF_NAME)")
	}
	return branch, commit, nil
}

// buildForge constructs the real adapter for kind ("gitlab" or "github"),
// reading connection secrets from CI-provided environment variables (never
// verdi.yaml — 01 §Store manifest: "secrets come from env/CI vars"). The
// github REPO IDENTIFIER (owner/repo), unlike the token, is not a secret and
// falls back to the origin remote URL when GitHub Actions' env vars are
// absent (D6-14; githubOwnerRepo) — so a local `verdi sync`/`close`/`gate`
// no longer needs GITHUB_REPOSITORY[_OWNER] exported by hand. remoteURL is
// the `origin` remote (best-effort; "" when none) both callers already read.
//
// spec/sync-local-flow ac-1/dc-2: this is the ONE shared construction seam
// every buildForge caller reaches (eight verb files; sync.go is the only
// direct, ungated one — the other seven pre-gate through
// forgeCredentialsPresent/forgeBestEffort, gate_threads.go). When the
// github identifier cannot be resolved from either source,
// githubOwnerRepo's own error propagates here and out to the caller as an
// operational refusal — never a live adapter built around two empty
// strings.
func buildForge(kind, remoteURL string) (forge.Forge, error) {
	switch kind {
	case "gitlab":
		return forgegitlab.New(forgegitlab.Config{
			BaseURL:   os.Getenv("CI_API_V4_URL"),
			ProjectID: os.Getenv("CI_PROJECT_ID"),
			Token:     os.Getenv("CI_JOB_TOKEN"),
		}), nil
	case "github":
		owner, repo, err := githubOwnerRepo(remoteURL)
		if err != nil {
			return nil, err
		}
		return forgegithub.New(forgegithub.Config{
			Owner: owner,
			Repo:  repo,
			Token: os.Getenv("GITHUB_TOKEN"),
		}), nil
	default:
		return nil, fmt.Errorf("unknown forge kind %q", kind)
	}
}

// githubOwnerRepo resolves the GitHub (owner, repo) the adapter needs,
// preferring GitHub Actions' own GITHUB_REPOSITORY_OWNER / GITHUB_REPOSITORY
// env vars (authoritative inside CI) and falling back per-field to parsing
// the origin remote URL (D6-14) for a local run where those vars are unset.
// The env wins where it is set, so a partial CI environment is never
// overridden.
//
// spec/sync-local-flow ac-1/dc-2: when NEITHER source identifies both
// fields, this is the one shared construction seam where the refusal
// lands — never a silently-returned empty pair. err names every source
// tried (both env vars and the origin remote URL, or its absence) so the
// caller's operational refusal (buildForge, then cmdSync) is legible
// rather than a confusing downstream network failure against an empty
// owner/repo.
func githubOwnerRepo(remoteURL string) (owner, repo string, err error) {
	envOwner := os.Getenv("GITHUB_REPOSITORY_OWNER")
	envRepository := os.Getenv("GITHUB_REPOSITORY")
	owner, repo = envOwner, githubRepoName()
	if owner != "" && repo != "" {
		return owner, repo, nil
	}
	if o, r, ok := forgegithub.OwnerRepoFromURL(remoteURL); ok {
		if owner == "" {
			owner = o
		}
		if repo == "" {
			repo = r
		}
	}
	if owner != "" && repo != "" {
		return owner, repo, nil
	}
	return "", "", fmt.Errorf(
		"cannot identify the GitHub repository: GITHUB_REPOSITORY_OWNER=%q, GITHUB_REPOSITORY=%q, and the git origin remote (%q) does not resolve one either — set both env vars (inside CI) or configure a github.com origin remote (for a local checkout)",
		envOwner, envRepository, remoteURL,
	)
}

// githubRepoName extracts the repo name from GITHUB_REPOSITORY
// ("owner/repo"), GitHub Actions' own combined env var. "" when unset — the
// local case githubOwnerRepo then resolves from the origin URL.
func githubRepoName() string {
	full := os.Getenv("GITHUB_REPOSITORY")
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '/' {
			return full[i+1:]
		}
	}
	return full
}
