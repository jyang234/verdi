// verdi waive <story-ref> <ac-id> --rationale <text> [--expires
// YYYY-MM-DD] [--reaffirm] (spec/verb-surfaces ac-1/ac-2,
// spec/creation-surfaces#ac-5, guide 8.4, ledger L-N9): the loud,
// recorded, audited exception the guide names — "you never skip the gate
// silently — you waive loudly." Resolves the (story, AC) pair through the
// same classifyPair seam verdi attest and verdi obligation author already
// share. Without --reaffirm it is create-only: refuses (exit 1, naming
// --reaffirm) when a waiver already sits at the convention path. With
// --reaffirm it requires one already there (refusing, exit 1, naming the
// plain create form, when none exists) and rewrites it in place — a fresh
// committed record each invocation, its body's reaffirmation log
// accumulating one new dated entry per call (internal/evidence's
// RenderWaiver/RenderWaiverReaffirm, spec/verb-surfaces' own disclosed
// reading of why this never mints a reaffirmations/ file — see the story
// spec).
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go/attest.go
// convention, so dispatch.go's diff for wiring this verb in stays small.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

const waiveUsage = "usage: verdi waive <story-ref> <ac-id> --rationale <text> [--expires YYYY-MM-DD] [--reaffirm]"

// cmdWaive is `verdi waive`'s entry point, invoked by dispatch.go.
func cmdWaive(args []string, stdout, stderr io.Writer) int {
	storyRefArg, acID, rationale, expires, reaffirm, err := parseWaiveArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, "waive:", err)
		fmt.Fprintln(stderr, waiveUsage)
		return 2
	}
	if rationale == "" {
		fmt.Fprintln(stderr, "waive: --rationale is required — a waiver is a loud, recorded exception, never a silent one")
		fmt.Fprintln(stderr, waiveUsage)
		return 2
	}
	if expires != "" {
		if _, perr := time.Parse("2006-01-02", expires); perr != nil {
			fmt.Fprintf(stderr, "waive: --expires %q is not a YYYY-MM-DD date\n", expires)
			return 2
		}
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "waive:", err)
		return 2
	}
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "waive:", err)
		return 2
	}

	return runWaive(context.Background(), root, storyRefArg, acID, reaffirm, rationale, expires, cfg.Model, time.Now().UTC(), stdout, stderr)
}

// parseWaiveArgs pulls "--rationale"/"--expires" (each as a separate value
// token or "=value") and the boolean "--reaffirm" out of args in whatever
// position they appear, returning the two required positionals
// (story-ref, ac-id) in order — the same hand-rolled, flag-in-any-position
// parse design.go's own extractFlags uses and documents why
// (flag.FlagSet cannot parse a positional-then-flags invocation shape
// without also accepting every flag-first permutation).
func parseWaiveArgs(args []string) (storyRefArg, acID, rationale, expires string, reaffirm bool, err error) {
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--reaffirm":
			reaffirm = true
		case a == "--rationale" || a == "-rationale":
			if rationale != "" {
				return "", "", "", "", false, fmt.Errorf("--rationale given more than once")
			}
			if i+1 >= len(args) {
				return "", "", "", "", false, fmt.Errorf("--rationale requires a value")
			}
			rationale = args[i+1]
			i++
		case strings.HasPrefix(a, "--rationale=") || strings.HasPrefix(a, "-rationale="):
			if rationale != "" {
				return "", "", "", "", false, fmt.Errorf("--rationale given more than once")
			}
			_, rationale, _ = strings.Cut(a, "=")
		case a == "--expires" || a == "-expires":
			if expires != "" {
				return "", "", "", "", false, fmt.Errorf("--expires given more than once")
			}
			if i+1 >= len(args) {
				return "", "", "", "", false, fmt.Errorf("--expires requires a value")
			}
			expires = args[i+1]
			i++
		case strings.HasPrefix(a, "--expires=") || strings.HasPrefix(a, "-expires="):
			if expires != "" {
				return "", "", "", "", false, fmt.Errorf("--expires given more than once")
			}
			_, expires, _ = strings.Cut(a, "=")
		case strings.HasPrefix(a, "-"):
			return "", "", "", "", false, fmt.Errorf("unknown flag %q", a)
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) != 2 {
		return "", "", "", "", false, fmt.Errorf("expected exactly <story-ref> <ac-id>, got %d positional argument(s)", len(positional))
	}
	return positional[0], positional[1], rationale, expires, reaffirm, nil
}

// runWaive is the testable core: given an already-resolved root, the
// parsed arguments, the resolved display model (nil-safe: bare-id
// fallback), and the invocation's own single wall-clock read (attest.go's
// stamp-at-the-boundary convention — never re-read downstream, so one
// invocation's frozen stamp and any lapsed-prior-expiry disclosure agree),
// run the whole create-or-reaffirm ritual and return the exit code
// (CLAUDE.md: 0 clean / 1 verdict / 2 operational).
func runWaive(ctx context.Context, root, storyRefArg, acID string, reaffirm bool, rationale, expires string, mdl *model.Model, now time.Time, stdout, stderr io.Writer) int {
	spec, refusal, opErr := classifyPair(root, storyRefArg, acID, mdl)
	if opErr != nil {
		fmt.Fprintln(stderr, "waive:", opErr)
		return 2
	}
	if refusal != "" {
		fmt.Fprintln(stderr, "waive:", refusal)
		return 1
	}

	storySlug := store.RefSlug(spec.Story)
	path := store.WaiverPath(root, storySlug, acID)
	displayPath := store.WaiverPath("", storySlug, acID)

	existingBytes, readErr := os.ReadFile(path)
	exists := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		fmt.Fprintln(stderr, "waive:", readErr)
		return 2
	}

	if reaffirm && !exists {
		fmt.Fprintf(stderr, "waive: --reaffirm: no waiver exists yet at %s — run verdi waive (without --reaffirm) first\n", displayPath)
		return 1
	}
	if !reaffirm && exists {
		fmt.Fprintf(stderr, "waive: a waiver already exists at %s — use --reaffirm to extend it with a fresh rationale\n", displayPath)
		return 1
	}

	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "waive:", err)
		return 2
	}

	in := evidence.WaiverInput{
		StorySlug:   storySlug,
		ACID:        acID,
		StoryRefArg: storyRefArg,
		VerifiesRef: spec.ID,
		Owners:      spec.Owners,
		Reason:      rationale,
		Expiry:      expires,
		Frozen:      artifact.NewFrozen(now.Format("2006-01-02"), head),
	}

	var priorExpiry string
	var priorLapsed bool
	var content string
	if reaffirm {
		existingFM, existingBody, splitErr := artifact.SplitFrontmatter(existingBytes)
		if splitErr != nil {
			fmt.Fprintf(stderr, "waive: existing waiver at %s does not decode: %v\n", displayPath, splitErr)
			return 2
		}
		// Best-effort: a prior file that splits but does not itself decode
		// as a valid waiver (hand-edited into a bad state) still gets a
		// reaffirmation — RenderWaiverReaffirm tolerates any prior body —
		// it simply has nothing to disclose about a lapsed prior expiry.
		if priorFM, decErr := artifact.DecodeWaiver(existingFM); decErr == nil {
			priorExpiry = priorFM.Expiry
			priorLapsed = evidence.WaiverLapsed(priorExpiry, now)
		}
		content = evidence.RenderWaiverReaffirm(string(existingBody), in)
	} else {
		content = evidence.RenderWaiver(in)
	}

	// Self-validate the exact bytes before ever touching disk (CLAUDE.md:
	// never fake success) — the same pre-write posture attest.go/
	// acceptobligation.go/obligation.go all wear.
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		fmt.Fprintln(stderr, "waive: internal error: rendered waiver failed self-validation:", err)
		return 2
	}
	if _, err := artifact.DecodeWaiver(fm); err != nil {
		fmt.Fprintln(stderr, "waive: internal error: rendered waiver failed self-validation:", err)
		return 2
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintln(stderr, "waive:", err)
		return 2
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fmt.Fprintln(stderr, "waive:", err)
		return 2
	}

	if reaffirm {
		fmt.Fprintf(stdout, "waive: reaffirmed %s — commit and merge to keep it active\n", displayPath)
	} else {
		fmt.Fprintf(stdout, "waive: created %s — commit and merge to activate\n", displayPath)
	}
	fmt.Fprintf(stdout, "waive: fold for %s will show waived (not evidenced) until discharged or expired\n", acID)
	if expires == "" {
		fmt.Fprintln(stdout, "waive: no --expires given — this waiver does not lapse on its own")
	} else {
		fmt.Fprintf(stdout, "waive: expires %s\n", expires)
	}
	if reaffirm && priorExpiry != "" && priorLapsed {
		fmt.Fprintf(stdout, "waive: the prior recorded expiry (%s) had already lapsed — this reaffirmation replaces it with a fresh one\n", priorExpiry)
	}
	return 0
}
