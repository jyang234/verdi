// Package stubinstantiate is the shared core for creating a story (or
// spike) spec from a feature's declared stub, via pure git plumbing that
// never touches the calling checkout's HEAD, working tree, or index
// (spec/scoping-canvas ac-6) — the mechanism both the board's
// stub-instantiate action (internal/workbench/boardspecapi.go) and
// `verdi design start --from-stub` (cmd/verdi/design.go, spec/
// cli-creation ac-3, ledger L-N7) call, so the two surfaces can never
// drift (the ADJ-65 asymmetry closed at the mechanism, not merely at the
// surface). Extracted verbatim out of internal/workbench's own
// actionStubInstantiate/sealedFeatureWallGuard/commitScaffoldBranch —
// every message and guard below is unchanged from what that function did
// before the extraction (the board's own existing handler tests pass
// unmodified, the behavior-preserving proof).
package stubinstantiate

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// PlaceholderStoryRef is the `story:` tracker scalar an instantiated
// story spec carries when the instantiating surface has no real tracker
// ref of its own to give it (ac-6: "bound to its stub by slug, with no
// new provenance record") — moved verbatim from workbench's own
// stubInstantiatePlaceholderStoryRef.
const PlaceholderStoryRef = "todo:REPLACE-ME"

// SealedFeatureWallGuard is the shared guard every stub- or form-driven
// scaffold-a-story action enforces against the FEATURE spec it scaffolds
// from (spec/scoping-canvas ac-6; spec/creation-form ac-2 inherits it
// verbatim): class feature, status accepted-pending-build — "the owner's
// rule: implementations build accepted specs only." action names the
// refusing action in the message; the spoken class/state words are
// display and resolve through mdl (nil is safe, falling back to bare
// ids, model.DisplayClass/DisplayState's own contract); the COMPARISONS
// stay on bare ids. Moved verbatim from workbench's own unexported
// sealedFeatureWallGuard.
func SealedFeatureWallGuard(class artifact.SpecClass, status, action string, mdl *model.Model) error {
	if class != artifact.ClassFeature {
		return fmt.Errorf("%s is only available on %s-class wall; this wall is class %s", action, model.Indefinite(mdl.DisplayClass("feature")), mdl.DisplayClass(string(class)))
	}
	if status != "accepted-pending-build" {
		// "accepted" in the owner's-rule parenthetical is that rule's own
		// word, not a lifecycle state id.
		return fmt.Errorf("%s is only available on %s spec (implementations build accepted specs only); this wall's status is %s",
			action, model.Indefinite(mdl.DisplayState("feature", "accepted-pending-build")), mdl.DisplayState("feature", status))
	}
	return nil
}

// CommitScaffoldBranch lands content as .verdi/specs/active/<slug>/spec.md
// in exactly one commit on a fresh design/<slug> branch, entirely via git
// plumbing — the calling checkout's HEAD, working tree, and real index are
// never touched (spec/scoping-canvas ac-6's mechanism, shared by
// stub-instantiate and the board's creation form). Fails closed if the
// branch already exists (gitx.UpdateRef's create-only atomicity — a
// caller's own RevParse pre-check only makes the common refusal legible).
// Moved verbatim from workbench's own unexported commitScaffoldBranch.
func CommitScaffoldBranch(ctx context.Context, root, slug, content, msg string) error {
	baseCommit, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		return err
	}
	blobSHA, err := gitx.WriteBlob(ctx, root, []byte(content))
	if err != nil {
		return err
	}
	path := store.ActiveSpecRelPath(slug)
	tree, err := gitx.BuildTreeWithFile(ctx, root, baseCommit+"^{tree}", path, blobSHA)
	if err != nil {
		return err
	}
	commit, err := gitx.CommitTree(ctx, root, tree, baseCommit, msg)
	if err != nil {
		return err
	}
	return gitx.UpdateRef(ctx, root, "refs/heads/design/"+slug, commit)
}

// buildLinks maps stub's own declared edges to the scaffold's
// document-level links against featureName's spec: a spike stub's
// resolves ids become resolves edges, a plain stub's acceptance_criteria
// ids become implements edges — moved verbatim from workbench's own
// inline construction in actionStubInstantiate.
func buildLinks(featureName string, stub artifact.Stub) []designscaffold.StoryLink {
	var links []designscaffold.StoryLink
	if stub.Spike {
		for _, oq := range stub.Resolves {
			links = append(links, designscaffold.StoryLink{Type: artifact.LinkResolves, Ref: "spec/" + featureName + "#" + oq})
		}
	} else {
		for _, ac := range stub.AcceptanceCriteria {
			links = append(links, designscaffold.StoryLink{Type: artifact.LinkImplements, Ref: "spec/" + featureName + "#" + ac})
		}
	}
	return links
}

// findStub returns the declared stub named slug from stubs, or false when
// none matches.
func findStub(stubs []artifact.Stub, slug string) (artifact.Stub, bool) {
	for _, s := range stubs {
		if s.Slug == slug {
			return s, true
		}
	}
	return artifact.Stub{}, false
}

// Result is what a successful Instantiate produced.
type Result struct {
	Branch  string // "design/<slug>"
	Content string // the rendered, self-validated spec.md content committed at Branch
}

// Instantiate scaffolds slug's declared stub — one of featureName's own,
// per stubs — as a story (or spike) spec on a fresh design/<slug> branch,
// built entirely via git plumbing (CommitScaffoldBranch) so the calling
// checkout's HEAD, working tree, and real index are never touched (spec/
// scoping-canvas ac-6). featureClass/featureStatus gate the sealed-wall
// guard (SealedFeatureWallGuard: class feature, status
// accepted-pending-build); mdl resolves that guard's disclosed prose
// through the store's display vocabulary (nil is safe). Fails closed if
// slug is empty, undeclared among stubs, or its design/<slug> branch
// already exists, and before ever touching the object database if the
// resolved template renders content whose own declared class disagrees
// with the story class this always requests (designscaffold.CheckClass,
// the K1 guard).
//
// This is the ONE shared core both the board's stub-instantiate action
// (internal/workbench) and `verdi design start --from-stub` (cmd/verdi)
// call (spec/cli-creation ac-3, ledger L-N7) — extracted out of
// internal/workbench/boardspecapi.go's actionStubInstantiate, behavior-
// preserving: every message, guard, and render step here is byte-
// identical to what that function did before the extraction.
func Instantiate(ctx context.Context, root, featureName string, featureClass artifact.SpecClass, featureStatus string, stubs []artifact.Stub, slug string, mdl *model.Model) (Result, error) {
	if slug == "" {
		return Result{}, fmt.Errorf("stub-instantiate requires a stub slug (id)")
	}
	if err := SealedFeatureWallGuard(featureClass, featureStatus, "stub-instantiate", mdl); err != nil {
		return Result{}, err
	}
	stub, ok := findStub(stubs, slug)
	if !ok {
		return Result{}, fmt.Errorf("no stub %q declared on this spec", slug)
	}

	links := buildLinks(featureName, stub)

	// A plain-language pre-check on the branch (callers surface this
	// message verbatim): UpdateRef inside CommitScaffoldBranch stays the
	// atomic create-only guard — this only makes the common refusal
	// legible, it does not replace the fail-closed write.
	if _, err := gitx.RevParse(ctx, root, "refs/heads/design/"+slug); err == nil {
		return Result{}, fmt.Errorf("branch design/%s already exists — this stub was already instantiated (or the name is taken); check that branch out instead", slug)
	}

	// The story class's own template — the store's own resolved model
	// (spec/scaffold-templates ac-1 cont.), with a store override at
	// .verdi/templates/<name>.md winning over the embedded canonical
	// default when present.
	cfg, err := store.Open(root)
	if err != nil {
		return Result{}, fmt.Errorf("stubinstantiate: resolving store config: %w", err)
	}
	class, ok := cfg.Model.Classes[string(artifact.ClassStory)]
	if !ok {
		return Result{}, fmt.Errorf("stubinstantiate: internal error: resolved model has no %q class", artifact.ClassStory)
	}
	tmpl, err := designscaffold.LoadTemplate(root, class.Template)
	if err != nil {
		return Result{}, fmt.Errorf("stubinstantiate: %w", err)
	}
	content, err := designscaffold.Story(tmpl, "spec/"+slug, PlaceholderStoryRef, designscaffold.HumanizeName(slug), stub.Spike, links, designscaffold.DefaultProblem, designscaffold.DefaultOutcome)
	if err != nil {
		return Result{}, fmt.Errorf("stubinstantiate: rendering template %s for class %s: %w", class.Template, artifact.ClassStory, err)
	}

	// Self-validate before ever touching the object database (CLAUDE.md:
	// "never fake success" — mirrors design start's own pre-write check).
	fm, _, err := artifact.SplitFrontmatter([]byte(content))
	if err != nil {
		return Result{}, fmt.Errorf("stubinstantiate: internal error: stub-instantiate scaffold failed self-validation: %w", err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return Result{}, fmt.Errorf("stubinstantiate: internal error: stub-instantiate scaffold failed self-validation: %w", err)
	}
	// K1: the decoded scaffold's OWN class must agree with the story
	// class stub-instantiate always requests — class.Template (above) is
	// DATA, so a misconfigured model.yaml (or a hand-edited store
	// override) can bind the story class to a DIFFERENT class's template
	// file and still strict-decode clean, as the other class. Caught
	// here, before any git plumbing runs.
	if err := designscaffold.CheckClass(spec, artifact.ClassStory); err != nil {
		return Result{}, fmt.Errorf("stubinstantiate: internal error: stub-instantiate scaffold failed self-validation: %w", err)
	}

	msg := fmt.Sprintf("stub-instantiate: scaffold spec/%s from stub %q of spec/%s", slug, slug, featureName)
	if err := CommitScaffoldBranch(ctx, root, slug, content, msg); err != nil {
		return Result{}, err
	}
	return Result{Branch: "design/" + slug, Content: content}, nil
}
