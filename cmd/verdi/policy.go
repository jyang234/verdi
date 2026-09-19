// verdi policy adopt --starter [--profile solo|team] [--owner <handle>]
// (spec/spec-documents ac-10, dc-6; SI-204): renders a starter
// constitution, one profile, one policy, and the consumers inventory
// through internal/policyadopt, cuts policy/adopt from the resolved
// default branch (design start's own dc-7 chain, R-W4-3), writes exactly
// those four paths, stages exactly them, and commits. Kept in its own
// file per the harness.go convention.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyadopt"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/store"
)

// vocab:identity — CLI usage/flag grammar (identity)
const policyUsage = "usage: verdi policy adopt --starter [--profile solo|team] [--owner <kebab-handle>]"

// policyAdoptWrittenState is the state clause every refusal AFTER a
// complete write appends. A refusal that leaves the operator on a branch
// they did not start on must say so — the same disclosure duty close.go's
// own AddPaths/CreateCommit pair carries — and naming the clause once
// keeps its two sites from drifting apart.
const policyAdoptWrittenState = "; the checkout is on policy/adopt with the four files written but not committed"

// policyAdoptAddPaths and policyAdoptCommit are this verb's two post-write
// git write ops as package-level seams, so a test can force the exact
// AddPaths/CreateCommit failure whose disclosure the verb owes the
// operator. The house pattern verbatim (close.go's closeAddPaths/
// closeCreateCommit, accept.go's accept* pair, spec/obligation-seam ac-3):
// a real `git add`/`git commit` cannot be made to fail deterministically
// in a clean hermetic fixture repo. Production is gitx's own; tests
// override and restore.
var (
	policyAdoptAddPaths = gitx.AddPaths
	policyAdoptCommit   = gitx.CreateCommit
)

// policyAdoptOptions is parsePolicyAdoptFlags' complete, validated operand
// set for `verdi policy adopt --starter`.
type policyAdoptOptions struct {
	profile governanceprincipal.Class
	owner   string
}

func cmdPolicy(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "adopt" {
		if len(args) > 0 {
			fmt.Fprintf(stderr, "policy: unknown subcommand %q\n", args[0])
		}
		fmt.Fprintln(stderr, policyUsage)
		return 2
	}
	opts, err := parsePolicyAdoptFlags(args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "policy adopt: %v\n%s\n", err, policyUsage)
		return 2
	}
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "policy adopt:", err)
		return 2
	}
	return runPolicyAdopt(context.Background(), root, opts, stdout, stderr)
}

// parsePolicyAdoptFlags hand-parses args (mirroring this package's
// established loop-based style, disposition_record.go's own
// parseDispositionRecordArgs): --starter (required, at most once),
// --profile <v>|--profile=<v> (at most once; "solo" default; only
// solo/team are legal — --starter is the only supported adoption form,
// R-W4-9), --owner <v>|--owner=<v> (at most once; kebab-case grammar,
// policyartifact.IsOwnerHandle; default "local-operator" for solo; the
// team profile has no honest default and requires it explicitly). No
// positionals.
func parsePolicyAdoptFlags(args []string) (policyAdoptOptions, error) {
	var starterSeen, profileSeen, ownerSeen bool
	profileValue := "solo"
	var owner string

	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value, inline := a, "", false
		if k, v, ok := strings.Cut(a, "="); ok && (k == "--profile" || k == "--owner") {
			name, value, inline = k, v, true
		}
		switch name {
		case "--starter":
			if starterSeen {
				return policyAdoptOptions{}, fmt.Errorf("--starter given twice")
			}
			starterSeen = true
		case "--profile":
			if profileSeen {
				return policyAdoptOptions{}, fmt.Errorf("--profile given twice")
			}
			profileSeen = true
			if !inline {
				var nerr error
				i, value, nerr = nextFlagValue(args, i)
				if nerr != nil {
					return policyAdoptOptions{}, nerr
				}
			}
			if value == "" {
				return policyAdoptOptions{}, fmt.Errorf("--profile requires a value")
			}
			profileValue = value
		case "--owner":
			if ownerSeen {
				return policyAdoptOptions{}, fmt.Errorf("--owner given twice")
			}
			ownerSeen = true
			if !inline {
				var nerr error
				i, value, nerr = nextFlagValue(args, i)
				if nerr != nil {
					return policyAdoptOptions{}, nerr
				}
			}
			if value == "" {
				return policyAdoptOptions{}, fmt.Errorf("--owner requires a value")
			}
			owner = value
		default:
			return policyAdoptOptions{}, fmt.Errorf("unexpected argument %q", a)
		}
	}

	if !starterSeen {
		return policyAdoptOptions{}, fmt.Errorf("--starter is required (it is the only supported adoption form)")
	}

	var profile governanceprincipal.Class
	switch profileValue {
	case "solo":
		profile = governanceprincipal.ClassSolo
	case "team":
		profile = governanceprincipal.ClassTeam
	default:
		return policyAdoptOptions{}, fmt.Errorf("--profile %q is invalid; legal values are solo, team", profileValue)
	}

	if owner == "" {
		if profile == governanceprincipal.ClassTeam {
			return policyAdoptOptions{}, fmt.Errorf("the team profile needs --owner <kebab-handle>: a team has no honest default owner")
		}
		owner = "local-operator"
	}
	if !policyartifact.IsOwnerHandle(owner) {
		return policyAdoptOptions{}, fmt.Errorf("--owner %q is not a kebab-case owner handle", owner)
	}

	return policyAdoptOptions{profile: profile, owner: owner}, nil
}

// nextFlagValue consumes the argument immediately after args[i] as that
// flag's value, returning the advanced index.
func nextFlagValue(args []string, i int) (int, string, error) {
	if i+1 >= len(args) {
		return i, "", fmt.Errorf("%s requires a value", args[i])
	}
	return i + 1, args[i+1], nil
}

// runPolicyAdopt composes the starter store, proves it, cuts policy/adopt
// from the resolved default branch, re-proves the checked-out tree
// (R-W4-3: the pre-checkout Compose above proved the OLD working tree; the
// checkout just switched, and a template override or an already-adopted
// state may exist only on the base branch), writes and commits exactly
// those four paths, and discloses what was granted.
func runPolicyAdopt(ctx context.Context, root string, opts policyAdoptOptions, stdout, stderr io.Writer) int {
	in := policyadopt.Input{Profile: opts.profile, Owner: opts.owner}

	if opts.profile == governanceprincipal.ClassSolo {
		if err := verifyGitTopLevel(ctx, root); err != nil {
			fmt.Fprintln(stderr, "policy adopt:", err)
			return 2
		}
		identity, available, err := readLocalGitIdentity(ctx, root)
		if err != nil {
			fmt.Fprintln(stderr, "policy adopt:", err)
			return 2
		}
		if !available {
			fmt.Fprintln(stderr, "policy adopt: the solo profile binds this checkout's git identity, but neither user.email nor user.name is configured in this repository (git config --local user.email …)")
			return 2
		}
		in.Subject = identity
	}

	if _, err := policyadopt.Compose(root, in); err != nil {
		if errors.Is(err, policyadopt.ErrAlreadyAdopted) {
			fmt.Fprintln(stderr, "policy adopt:", err)
			return 1
		}
		fmt.Fprintln(stderr, "policy adopt:", err)
		return 2
	}

	baseRef, ok := resolveBranchBase(ctx, root, "policy adopt", stdout, stderr)
	if !ok {
		return 2
	}
	if !checkoutNewBranchDisclosed(ctx, root, "policy adopt", "policy/adopt", baseRef, stdout, stderr) {
		return 2
	}

	// R-W4-3: re-prove on the branch actually checked out. A refusal here
	// leaves the checkout on policy/adopt with nothing written — the
	// branch itself carries no commit yet, so it is inert, but it is not
	// silently deleted either (no rollback, disclosed).
	plan, err := policyadopt.Compose(root, in)
	if err != nil {
		if errors.Is(err, policyadopt.ErrAlreadyAdopted) {
			fmt.Fprintln(stderr, "policy adopt: the default branch already carries policy; left the checkout on policy/adopt with nothing written")
			return 1
		}
		fmt.Fprintf(stderr, "policy adopt: %v; the checkout is on policy/adopt with nothing written\n", err)
		return 2
	}

	paths, err := policyadopt.Write(root, plan)
	if err != nil {
		// Write attempts no rollback, so whatever landed is still on the
		// checked-out branch. Name those files: they are the only state
		// this refusal leaves behind, and without their names the
		// operator cannot clean up or continue by hand.
		for _, rel := range paths {
			fmt.Fprintf(stderr, "policy adopt: wrote %s before the failure\n", rel)
		}
		fmt.Fprintf(stderr, "policy adopt: %v; the checkout is on policy/adopt with %d of %d starter files written and nothing committed\n", err, len(paths), len(plan.Files))
		return 2
	}
	for _, f := range plan.Files {
		fmt.Fprintf(stdout, "policy adopt: wrote %s (%s %s)\n", f.RelPath, f.Template.Identity, f.Template.Digest)
	}

	if err := policyAdoptAddPaths(ctx, root, paths...); err != nil {
		fmt.Fprintf(stderr, "policy adopt: %v%s\n", err, policyAdoptWrittenState)
		return 2
	}
	sha, err := policyAdoptCommit(ctx, root, fmt.Sprintf("policy adopt: starter constitution (%s profile)", opts.profile))
	if err != nil {
		fmt.Fprintf(stderr, "policy adopt: %v%s\n", err, policyAdoptWrittenState)
		return 2
	}
	fmt.Fprintf(stdout, "policy adopt: committed %s on policy/adopt\n", shortSHA(sha))

	var clause string
	switch opts.profile {
	case governanceprincipal.ClassSolo:
		// vocab:identity — git branch merge (repository mechanics), not the spec lifecycle transition verb
		clause = "your delegated agents may write drafts on design branches you alone can merge"
	default:
		clause = "delegated agents may propose; humans write, until the team's own review grants more"
	}
	fmt.Fprintf(stdout, "policy adopt: design_assistance mode %s for the %s profile — %s\n", plan.DesignAssistanceMode, opts.profile, clause)
	if opts.profile == governanceprincipal.ClassTeam {
		fmt.Fprintln(stdout, "policy adopt: the team profile maps no subjects yet — add role_mappings to .verdi/policy/profiles/starter-team.md before any approval can be proven")
	}
	fmt.Fprintln(stdout, "policy adopt: the consumers inventory registers no consumers yet — register real consumers before impact review")
	// vocab:identity — TWO deliberate vocabulary words in one plan-mandated sentence. "merge" is the git branch merge the owner performs (repository mechanics), not the spec lifecycle transition verb — that is the hit this marker actually classifies. "accepted" is the lifecycle state word in its own lifecycle sense, spoken to DENY that this verb confers it; the witness skips that hit today under its compound-punctuation rule ("accepted:"), so this half of the rationale classifies it prospectively, for whoever repunctuates the sentence
	fmt.Fprintln(stdout, "policy adopt: nothing here is accepted: acceptance is the owner's merge of policy/adopt to the default branch")

	return 0
}
