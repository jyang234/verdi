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
	"strings"

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
// strings, for any caller that will DIAL the forge by (owner, repo).
func buildForge(kind, remoteURL string) (forge.Forge, error) {
	return buildForgeWithIdentifier(kind, remoteURL, true)
}

// buildForgeForCI builds the forge for invocations that never DIAL the forge
// API — they only read the CI environment (CIContext, a pure env read that
// addresses no repository by owner/repo): `verdi sync --produce` /
// `--produce-runtime`. Unlike buildForge it does NOT apply the ac-1
// identifier refusal, because an unresolved github owner/repo is harmless
// where nothing dials — CIContext never touches those fields (ADJ-43: a
// refusal is only honest where the identifier was needed; --produce with
// --force-local in an env-less, origin-less checkout needs no forge
// identifier and must run exactly as it did before ac-1, restoring co-3
// byte-identity). The adapter is built with whatever origin/env resolve,
// empty owner/repo included; a live FetchEvidenceBundle on such an adapter
// would still fail, but these paths never make one.
func buildForgeForCI(kind, remoteURL string) (forge.Forge, error) {
	return buildForgeWithIdentifier(kind, remoteURL, false)
}

// buildForgeWithIdentifier is the shared constructor. requireIdentifier
// gates only the github identifier refusal: true propagates
// githubOwnerRepo's unresolved-identifier error (the dialing seam); false
// tolerates it, building the adapter with the empty owner/repo the
// CIContext-only paths never use. gitlab takes its identifier from
// CI_PROJECT_ID (env, never URL-derived — dc-3) and has no such refusal
// under either flag.
func buildForgeWithIdentifier(kind, remoteURL string, requireIdentifier bool) (forge.Forge, error) {
	switch kind {
	case "gitlab":
		return forgegitlab.New(forgegitlab.Config{
			BaseURL:   os.Getenv("CI_API_V4_URL"),
			ProjectID: os.Getenv("CI_PROJECT_ID"),
			Token:     os.Getenv("CI_JOB_TOKEN"),
		}), nil
	case "github":
		owner, repo, err := githubOwnerRepo(remoteURL)
		if err != nil && requireIdentifier {
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
	if owner == "" {
		// A bare GITHUB_REPOSITORY (owner/repo) carries its own owner half —
		// authoritative CI env that fully identifies the repository without a
		// separate GITHUB_REPOSITORY_OWNER (ADJ-64: refusing here was a false
		// "cannot identify" disclosure of information the value already
		// carried). Ranked below an explicit GITHUB_REPOSITORY_OWNER
		// (consulted just above) and above the origin URL (below), so the
		// CI-env-wins precedence is unchanged.
		owner = githubRepoOwner()
	}
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
	// Name the origin's ABSENCE explicitly rather than as an empty quoted
	// string (ADJ-64): ac-1 promises to name every source it tried, "the
	// origin remote URL or its absence".
	originDesc := fmt.Sprintf("%q", remoteURL)
	if remoteURL == "" {
		originDesc = "absent (no origin remote configured)"
	}
	return "", "", fmt.Errorf(
		"cannot identify the GitHub repository: GITHUB_REPOSITORY_OWNER=%q, GITHUB_REPOSITORY=%q, and the git origin remote (%s) does not resolve one either — set GITHUB_REPOSITORY=owner/repo (inside CI; GITHUB_REPOSITORY_OWNER, if set, overrides the owner half) or configure a github.com origin remote (for a local checkout)",
		envOwner, envRepository, originDesc,
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

// githubRepoOwner extracts the owner from GITHUB_REPOSITORY ("owner/repo"),
// GitHub Actions' own combined env var — the owner half GITHUB_REPOSITORY
// alone fully identifies (a bare GITHUB_REPOSITORY needs no separate
// GITHUB_REPOSITORY_OWNER, ADJ-64). "" when GITHUB_REPOSITORY is unset or
// carries no "/" owner segment. githubOwnerRepo consults GITHUB_REPOSITORY_OWNER
// first, so an explicit owner env still wins and CI-env precedence is unchanged.
func githubRepoOwner() string {
	full := os.Getenv("GITHUB_REPOSITORY")
	if i := strings.IndexByte(full, '/'); i >= 0 {
		return full[:i]
	}
	return ""
}
