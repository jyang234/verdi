// actorlocal.go resolves the one opt-in local-operator actor claim the
// conflict-service factory (context_conflict.go) feeds into every
// lifecycle-gate evaluation (`context conflict`, `build start`, `gate`,
// `close` all share the one factory) — 2026-09-05 local-operator
// disposition design §2.1-§2.2, ledger SI-183.
//
// resolveLocalActors runs only when the resolved governance profile itself
// declares an identity_trust_sources entry of kind local-operator: every
// other profile gets exactly nil (today's behavior, unchanged — no
// hand-built PrincipalResolution is ever constructed here or anywhere else;
// governanceprincipal.Resolver alone mints the value ServiceDeps.Actors
// carries). When the profile does declare exactly one such source, this
// file ALWAYS resolves the one claim, whatever the outcome: an absent
// local identity resolves unproven through the real port rather than being
// silently omitted, so "a local-operator resolution was attempted" is a
// single, uniform fact policyconflict's disclosure seam
// (localOperatorDisclosures) can read directly off the Actors slice.
//
// Identity source: the store's own configured Git identity — user.email,
// falling back to user.name — read through gitx.ConfigValue scoped
// `--local` to the store root itself, exactly as the design specifies
// ("the store's Git identity"). Two safety properties beyond
// gitx.ConfigValue's own absent/broken split:
//
//   - The store root must be a Git repository top level IN ITS OWN RIGHT
//     (verifyGitTopLevel). A `.verdi/` store need not be a Git top level at
//     all (internal/store.FindRoot walks up to the nearest .verdi/
//     verdi.yaml, entirely independent of Git); `--local` scope reads
//     whatever repository Git DISCOVERS from that directory, which — for a
//     store nested inside a larger checkout with no .git of its own — is
//     the ENCLOSING repository, not the store's own declaration. Refusing
//     operationally here keeps that enclosing identity from ever being
//     read on the store's behalf.
//   - extensions.worktreeConfig per-worktree identities are NOT consulted:
//     gitx.ConfigValue reads `--local` scope only. A linked worktree that
//     enables that extension and sets a WORKTREE-scoped user.email via
//     `git config --worktree` will not be seen here; only the shared
//     .git/config the worktree's repository as a whole carries is read.
//     This is a known, disclosed narrowing, not a defect this file's scope
//     corrects.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// localOperatorAbsentSubject is the placeholder PrincipalClaim.Subject used
// when the store configures neither user.email nor user.name.
// PrincipalClaim.Validate requires a nonempty subject even for a claim that
// is about to resolve unproven, but the value itself is inert: the paired
// TrustFact reports Available: false, and governanceprincipal.Resolver's
// !fact.Available branch resolves ResolutionUnproven without ever
// comparing the claim's subject to anything (role mappings included), so
// no real profile declaration can ever turn this placeholder into a false
// authenticated principal.
const localOperatorAbsentSubject = "(no local git identity configured)"

// resolveLocalActors resolves the store's local-operator actor claim
// against profile. It returns nil, nil when profile declares no
// local-operator trust source (today's exact behavior). A profile
// declaring more than one is refused operationally: the design and this
// wiring both assume exactly one such source per profile, and silently
// picking one to trust would be a favorable default on an authorization
// input the design never sanctions (disclosed judgment call — no profile
// in this task's fixtures or the ratified design declares two).
func resolveLocalActors(ctx context.Context, root string, profile governanceprincipal.Profile) ([]governanceprincipal.PrincipalResolution, error) {
	var sources []governanceprincipal.TrustSource
	for _, source := range profile.IdentityTrustSources {
		if source.Kind == governanceprincipal.TrustSourceLocalOperator {
			sources = append(sources, source)
		}
	}
	if len(sources) == 0 {
		return nil, nil
	}
	if len(sources) > 1 {
		ids := make([]string, len(sources))
		for i, s := range sources {
			ids[i] = s.ID
		}
		return nil, fmt.Errorf("cmd/verdi: profile %q declares %d local-operator trust sources (%s), want at most one", profile.ID, len(sources), strings.Join(ids, ", "))
	}
	source := sources[0]

	if err := verifyGitTopLevel(ctx, root); err != nil {
		return nil, err
	}

	identity, available, err := readLocalGitIdentity(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("cmd/verdi: reading local-operator identity: %w", err)
	}
	subject := localOperatorAbsentSubject
	if available {
		subject = identity
	}

	resolver := governanceprincipal.NewResolver(localOperatorTrustFactReader{root: root})
	resolution, err := resolver.Resolve(ctx, profile, governanceprincipal.PrincipalClaim{
		TrustSource: source.ID,
		Subject:     subject,
	})
	if err != nil {
		return nil, fmt.Errorf("cmd/verdi: resolving local-operator actor: %w", err)
	}
	return []governanceprincipal.PrincipalResolution{resolution}, nil
}

// readLocalGitIdentity reads the store's own configured Git identity:
// user.email, falling back to user.name when no email is configured.
// available is false only when NEITHER key carries a value in the store's
// own `--local` scope (gitx.ErrConfigUnset for both) — every other
// ConfigValue failure is a genuine operational error, propagated as such
// rather than being conflated with "no identity configured".
func readLocalGitIdentity(ctx context.Context, root string) (identity string, available bool, err error) {
	email, err := gitx.ConfigValue(ctx, root, "user.email")
	if err == nil {
		return email, true, nil
	}
	if !errors.Is(err, gitx.ErrConfigUnset) {
		return "", false, err
	}
	name, err := gitx.ConfigValue(ctx, root, "user.name")
	if err == nil {
		return name, true, nil
	}
	if !errors.Is(err, gitx.ErrConfigUnset) {
		return "", false, err
	}
	return "", false, nil
}

// verifyGitTopLevel refuses when root is not itself a Git repository top
// level: `git -C root rev-parse --show-toplevel` must resolve to exactly
// root, canonicalized the same way on both sides so a symlinked temp
// directory (e.g. macOS's /tmp -> /private/tmp) never produces a false
// mismatch. A store nested inside a larger checkout with no .git of its
// own would otherwise have that ENCLOSING repository's identity read on
// its behalf — refused here rather than silently trusted.
func verifyGitTopLevel(ctx context.Context, root string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--show-toplevel")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cmd/verdi: local-operator identity: store root %q is not a Git repository: %w: %s", root, err, strings.TrimSpace(stderr.String()))
	}
	toplevel, err := canonicalGitDir(strings.TrimSpace(stdout.String()))
	if err != nil {
		return fmt.Errorf("cmd/verdi: local-operator identity: resolving Git top-level path: %w", err)
	}
	wantRoot, err := canonicalGitDir(root)
	if err != nil {
		return fmt.Errorf("cmd/verdi: local-operator identity: resolving store root path: %w", err)
	}
	if toplevel != wantRoot {
		return fmt.Errorf("cmd/verdi: local-operator identity: store root %q is not itself a Git repository top level (found top level %q); refusing to read an enclosing repository's identity on the store's behalf", wantRoot, toplevel)
	}
	return nil
}

// canonicalGitDir resolves path to an absolute, symlink-free, cleaned form
// so two spellings of the same directory always compare equal (mirrors
// policyconflict's own canonicalCheckoutRoot for the identical purpose).
func canonicalGitDir(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

// localOperatorTrustFactReader answers governanceprincipal's
// TrustFactReader port by independently reading the store's own configured
// Git identity — never trusting the caller's already-read claim, mirroring
// how any other adapter (forge, identity-provider) would independently
// observe its own evidence rather than being handed a pre-decided answer.
type localOperatorTrustFactReader struct{ root string }

func (r localOperatorTrustFactReader) ReadTrustFact(ctx context.Context, source governanceprincipal.TrustSource, claim governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
	identity, available, err := readLocalGitIdentity(ctx, r.root)
	if err != nil {
		return governanceprincipal.TrustFact{}, fmt.Errorf("local-operator trust fact: %w", err)
	}
	if !available {
		return governanceprincipal.TrustFact{
			SourceID: source.ID, SourceKind: source.Kind,
			Available: false, Reason: "the store configures neither user.email nor user.name in its own --local Git configuration",
		}, nil
	}
	digest, err := canonjson.Digest(struct {
		SourceID string `json:"source_id"`
		Subject  string `json:"subject"`
	}{source.ID, identity})
	if err != nil {
		return governanceprincipal.TrustFact{}, fmt.Errorf("local-operator trust fact digest: %w", err)
	}
	fact := governanceprincipal.TrustFact{
		SourceID: source.ID, SourceKind: source.Kind,
		EvidenceDigest: digest, Available: true,
	}
	if identity == claim.Subject {
		fact.Valid = true
		fact.Subjects = []string{identity}
	} else {
		fact.Valid = false
		fact.Subjects = []string{}
		fact.Reason = "the store's configured Git identity does not match the claimed subject"
	}
	return fact, nil
}
