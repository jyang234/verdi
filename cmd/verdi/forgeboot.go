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
	"strings"

	"github.com/jyang234/verdi/internal/forge"
	forgegithub "github.com/jyang234/verdi/internal/forge/github"
	forgegitlab "github.com/jyang234/verdi/internal/forge/gitlab"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// loadManifest reads and strict-decodes root's verdi.yaml. A thin
// delegate to store.Open (L-M3's config bottleneck): the read+decode body
// that used to live here now lives there verbatim, so this function's
// error text is unchanged. Kept as its own function since the ~10 verb
// call sites still want the bare *store.Manifest return shape; widening
// them to consume *store.Config directly is a separate, later task.
func loadManifest(root string) (*store.Manifest, error) {
	st, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	return st.Manifest, nil
}

// resolveModelDigest resolves root's operating model digest
// (model.Model.Digest(), spec/model-digest ledger L-M5) via store.Open —
// Config.Model is never nil (model-schema's own guarantee), so an absent
// .verdi/model.yaml resolves to the embedded canonical exactly as it does
// everywhere else. This is the shared path every verb that threads a
// ModelDigest into an align.Input/DecisionConflictInput/DiagramSweepInput
// or a commitdesign.Input uses when it doesn't already hold a
// *store.Config for some other reason (cmd/verdi/align.go inlines the
// equivalent two calls itself, since it also needs the rest of the
// resolved Config for its manifest-derived deps; board.go, close.go, and
// closefeature.go call this instead).
func resolveModelDigest(root string) (string, error) {
	st, err := store.Open(root)
	if err != nil {
		return "", err
	}
	digest, err := st.Model.Digest()
	if err != nil {
		return "", fmt.Errorf("computing model digest: %w", err)
	}
	return digest, nil
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
// remoteErr carries a genuine origin-remote READ failure (nil when the origin
// resolved or was merely absent — the caller clears gitx.ErrNoSuchRemote to
// nil); githubOwnerRepo surfaces it operationally ONLY when it actually falls
// back to the origin to identify the repo (ADJ-64), never when the CI env
// already identifies it and never for gitlab.
func buildForge(kind, remoteURL string, remoteErr error) (forge.Forge, error) {
	return buildForgeWithIdentifier(kind, remoteURL, remoteErr, true)
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
	// nil remoteErr: --produce never dials, never consults the origin for
	// identification, so an unreadable origin is irrelevant here (co-3
	// byte-identity, ADJ-43).
	return buildForgeWithIdentifier(kind, remoteURL, nil, false)
}

// buildForgeWithIdentifier is the shared constructor. requireIdentifier
// gates the identifier refusal for BOTH forge kinds: true propagates the
// unresolved-identifier error at the dialing seam — githubOwnerRepo's
// (owner, repo) for github, gitlabProjectID's project id for gitlab; false
// tolerates it, building the adapter with the empty identifier the
// CIContext-only (--produce/--produce-runtime) paths never dial. github
// resolves its identifier from the CI env or, failing that, the origin URL;
// gitlab resolves its from CI_PROJECT_ID alone (env, never URL-derived —
// dc-3). ADJ-69 made gitlab symmetric with github: it too refuses an empty
// project id here rather than dialing gitlab.com/api/v4/projects//... with
// an empty :id and failing downstream as a confusing "unexpected status".
func buildForgeWithIdentifier(kind, remoteURL string, remoteErr error, requireIdentifier bool) (forge.Forge, error) {
	switch kind {
	case "gitlab":
		// gitlab identification is CI_PROJECT_ID (env-only, never URL-derived
		// — dc-3), so an unreadable origin (remoteErr) never affects it. When
		// requireIdentifier and CI_PROJECT_ID is unset, refuse here rather than
		// building an adapter that would dial with an empty :id (ADJ-69); the
		// non-dialing --produce/--produce-runtime paths (requireIdentifier
		// false) tolerate the empty id byte-identically, since they never dial
		// (ADJ-43).
		projectID, err := gitlabProjectID()
		if err != nil && requireIdentifier {
			return nil, err
		}
		return forgegitlab.New(forgegitlab.Config{
			BaseURL:   os.Getenv("CI_API_V4_URL"),
			ProjectID: projectID,
			Token:     os.Getenv("CI_JOB_TOKEN"),
		}), nil
	case "github":
		owner, repo, err := githubOwnerRepo(remoteURL, remoteErr)
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

// gitlabProjectID resolves the GitLab project identifier the adapter dials
// by. Unlike github's (owner, repo), gitlab's identity is env-only — the
// numeric or URL-encoded CI_PROJECT_ID GitLab CI injects, never URL-derived
// (dc-3) — so there is exactly one source to consult (this takes no
// remoteURL). It returns the empty id alongside an error naming that source
// when CI_PROJECT_ID is unset, so a caller that will DIAL the forge
// (requireIdentifier) refuses legibly at the construction seam rather than
// egressing to gitlab.com/api/v4/projects//... with an empty :id and failing
// as a confusing downstream "unexpected status" (ADJ-69, symmetric with
// githubOwnerRepo's ac-1 refusal).
func gitlabProjectID() (string, error) {
	projectID := os.Getenv("CI_PROJECT_ID")
	if projectID == "" {
		return "", fmt.Errorf(
			"cannot identify the GitLab project: CI_PROJECT_ID is unset, and gitlab identifies a project only by that env var, never the origin URL (dc-3) — set CI_PROJECT_ID to the numeric project id (GitLab CI injects it automatically inside a pipeline; export it by hand for a local run)",
		)
	}
	return projectID, nil
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
func githubOwnerRepo(remoteURL string, remoteErr error) (owner, repo string, err error) {
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
		// The CI env fully identifies the repo — the origin is never consulted,
		// so an unreadable origin (remoteErr) is irrelevant and the env still
		// wins byte-identically to today (ac-1, dc-3), even for a broken origin.
		return owner, repo, nil
	}
	// Env insufficient → the origin URL is the fallback identification source
	// (D6-14), so a genuine failure READING it is material ONLY here, where the
	// origin is actually needed (absence was cleared to nil by the caller).
	// Surface it operationally, naming the read failure rather than
	// mis-reporting the resulting empty URL as an absent origin (ADJ-64) — the
	// original honesty defect was that githubOwnerRepo saw an unreadable origin
	// as the same empty string an absent one produces.
	if remoteErr != nil {
		return "", "", fmt.Errorf(
			"cannot identify the GitHub repository: GITHUB_REPOSITORY_OWNER=%q, GITHUB_REPOSITORY=%q, and the git origin remote could not be read (%v) — set GITHUB_REPOSITORY=owner/repo (inside CI) or fix the github.com origin remote (for a local checkout)",
			envOwner, envRepository, remoteErr,
		)
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
