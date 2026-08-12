package contextcompile

import (
	"context"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/repositoryfacts"
	"github.com/jyang234/verdi/internal/specstate"
)

// ProjectionFile is one adapter-selected file the pure renderer produced,
// paired with its exact bytes and content digest.
type ProjectionFile struct {
	Path    string
	Content []byte
	Digest  string
}

// Result is the compiled context: the decoded manifest and data items plus
// their exact wire bytes, and the rendered projection files.
//
// ManagedProjectionPaths is an in-memory-only field (never part of the
// wire manifest — EncodeManifest/DecodeManifest never touch Result): the
// sorted, de-duplicated set of every managed instruction-projection path
// declared by ANY adapter in the resolved constitution, not only the one
// this request named. A caller enforcing "never overwrite a managed
// projection file" (e.g. `verdi context compile --out`) must guard against
// EVERY adapter's managed path, since a two-adapter constitution's other
// adapter file is exactly as reserved as the requested adapter's own.
type Result struct {
	Manifest               Manifest
	ManifestBytes          []byte
	DataItems              []DataItem
	DataItemBytes          [][]byte
	ProjectionFiles        []ProjectionFile
	ManagedProjectionPaths []string
}

// RepositoryFactsGatherer computes the shared repository-identity Snapshot
// (branch, HEAD, default branch, their relationship, dirty/staged posture,
// managed-worktree identity) for the checkout at root. Compile's stage 3
// calls this before any spec target has resolved (that is stage 4), so the
// gather runs with no evaluated target.
type RepositoryFactsGatherer interface {
	Gather(ctx context.Context, in repositoryfacts.GatherInput) (repositoryfacts.Snapshot, error)
}

// ProjectionVerifier verifies the on-disk managed instruction projection
// against a freshly recomputed rendering of the resolved constitution.
type ProjectionVerifier interface {
	Verify(root string) (*instructionprojection.Report, error)
}

// Compiler compiles a context request into a Result over a fixed bundle of
// trusted, read-only ports. The zero value fails closed: construct with
// NewCompiler (production) before calling Compile.
type Compiler struct {
	constructed bool

	git        GitReader
	states     StateResolver
	authority  AuthorityLoader
	actors     ActorResolver // nil in v1 production: the CLI supplies no principal-resolution port.
	repoFacts  RepositoryFactsGatherer
	projection ProjectionVerifier
}

// NewCompiler returns a Compiler wired to the real, production trusted
// ports: gitx-backed Git reads, specstate.Projector-backed specification
// state resolution, policyauthority-backed authority loading, a
// repositoryfacts.Gatherer, and the shared instructionprojection.Verify
// entry point. ActorResolver stays nil in v1: the built-binary CLI supplies
// no principal-resolution port, so actor posture is always explicitly
// unproven (authority design §2, §4).
func NewCompiler() Compiler {
	return newCompilerWithPorts(
		gitxGitReader{},
		specstate.NewProjector(),
		defaultAuthorityLoader{},
		nil,
		repositoryfacts.NewGatherer(),
		defaultProjectionVerifier{},
	)
}

// newCompilerWithPorts is the package-private construction seam this
// package's own tests use to inject fakes for every trusted port.
func newCompilerWithPorts(
	git GitReader,
	states StateResolver,
	authority AuthorityLoader,
	actors ActorResolver,
	repoFacts RepositoryFactsGatherer,
	projection ProjectionVerifier,
) Compiler {
	return Compiler{
		constructed: true,
		git:         git,
		states:      states,
		authority:   authority,
		actors:      actors,
		repoFacts:   repoFacts,
		projection:  projection,
	}
}

// gitxGitReader adapts the package-level internal/gitx functions to the
// GitReader port (mirrors internal/journey's and internal/repositoryfacts's
// own gitxReader adapter: exec the pinned upstream plumbing, never a second
// copy of it).
type gitxGitReader struct{}

func (gitxGitReader) Show(ctx context.Context, root, ref, path string) ([]byte, error) {
	return gitx.Show(ctx, root, ref, path)
}

func (gitxGitReader) LsTreeEntries(ctx context.Context, root, ref string) ([]gitx.TreeEntry, error) {
	return gitx.LsTreeEntries(ctx, root, ref)
}

func (gitxGitReader) WorktreeChangedPaths(ctx context.Context, root string) ([]string, error) {
	return gitx.WorktreeChangedPaths(ctx, root)
}

// defaultProjectionVerifier adapts instructionprojection.Verify — which
// loads and resolves the constitution store itself — to the
// ProjectionVerifier port.
type defaultProjectionVerifier struct{}

func (defaultProjectionVerifier) Verify(root string) (*instructionprojection.Report, error) {
	return instructionprojection.Verify(root)
}

// Compile compiles the given request rooted at root through the ordered
// pipeline (Wave-3 plan Task 7 step 3):
//
//  1. validate the request;
//  2. resolve policy authority and the exact requested adapter;
//  3. gather repository facts and compare the optional caller expectation;
//  4. resolve the accepted target and its semantic dependencies (feature
//     fragments, bound obligations, declared context);
//  5. verify the full existing managed instruction projection against disk;
//  6. discover the candidate universe with the exact store/declared-context
//     lifts;
//  7. evaluate applicability and select applicable authority operands and
//     policy ids;
//  8. render the pure, phase-filtered projection for exactly those ids;
//  9. classify every candidate exactly once and build data payloads;
//  10. project sealed actors, or the explicit unproven-absence posture;
//  11. compute revisions, every manifest row, the disclosure union, and
//     canonical bytes;
//  12. return the fresh, in-memory Result.
//
// Construction note on 6/7/8's actual call order: stage 6's own
// UniverseInput needs two facts that are only available once stage 7 has
// run — which store-authority paths are lifted (authority design §5:
// "applicable policies/overlays/exemptions", a fact stage 7's applicability
// selection alone determines) and which policy/overlay/exemption bytes back
// each lifted candidate's later classification material. Stage 8's
// projection PATHS, by contrast, are already fixed by the resolved
// adapter's own Managed list (known since stage 2) even though their
// CONTENT is only produced by stage 8's render; that half of stage 8 (the
// paths) is therefore computed alongside stage 6, and the render call
// itself follows once stage 7's selection exists. Compile therefore issues
// stage 7's authority-operand selection before calling BuildUniverse, then
// renders (stage 8) immediately after — the documented stage NUMBERS above
// name each step's authority-design role and refusal precedence, not a
// claim that Go statement order is 6-then-7-then-8.
func (c Compiler) Compile(ctx context.Context, root string, request Request) (Result, error) {
	if !c.constructed {
		return Result{}, fmt.Errorf("contextcompile: zero-value Compiler cannot compile; use NewCompiler")
	}
	if root == "" {
		return Result{}, fmt.Errorf("contextcompile: root must not be empty")
	}

	// Stage 1: validate the request. A phase outside a nonempty
	// scope.phases surfaces as *PhaseScopeRefusal, unchanged.
	if err := request.Validate(); err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 1 validate request: %w", err)
	}

	// Stage 2: resolve policy authority and the exact requested adapter.
	authority, err := ResolvePolicyAuthority(c.authority, root, request.Adapter)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 2 resolve policy authority: %w", err)
	}

	// Stage 3: gather repository facts and compare the optional caller
	// expectation. No spec target has resolved yet (that is stage 4), so
	// Gather runs with no evaluated target.
	snapshot, err := c.repoFacts.Gather(ctx, repositoryfacts.GatherInput{Root: root})
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 3 gather repository facts: %w", err)
	}
	if err := ResolveExpectedRepository(request.Expected, snapshot.Facts); err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 3 compare expected repository: %w", err)
	}
	if !snapshot.Facts.Head.Known {
		return Result{}, fmt.Errorf("contextcompile: stage 3 repository HEAD is unknown; cannot resolve stage 4's spec target")
	}
	head := snapshot.Facts.Head.Value

	// Stage 4: resolve the accepted target and its semantic dependencies.
	target, err := ResolveAcceptedSpec(ctx, c.git, c.states, root, head, request.Spec)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 4 resolve accepted spec: %w", err)
	}

	var fragments []FeatureFragment
	switch target.Spec.Class {
	case artifact.ClassFeature:
		// A feature is not itself governed by parent features; a feature
		// target carries no parent-feature-fragment material. It is,
		// however, not an authoritative BUILD target (capsule.go's own
		// validateCapsuleTarget rule, ported here since Compile assembles
		// the manifest directly rather than through ComposeCapsule).
		if request.Phase == PhaseBuild {
			return Result{}, &DeclaredScopeRefusal{
				Phase: request.Phase, Ref: target.Ref,
				// vocab:identity — "feature" names the fixed artifact spec class this refusal targets, not display prose
				Reason: "feature specifications are not authoritative build targets",
			}
		}
		fragments = []FeatureFragment{}
	case artifact.ClassStory:
		fragments, err = ResolveFeatureFragments(ctx, c.git, c.states, root, head, target)
		if err != nil {
			// vocab:identity — "feature" names the ResolveFeatureFragments stage this error wraps, the fixed artifact class
			return Result{}, fmt.Errorf("contextcompile: stage 4 resolve feature fragments: %w", err)
		}
	default:
		// Every remaining closed spec class (v1: component) is a
		// state-valid accepted target that NO phase's capsule may consume
		// as its authoritative target — the same "wrong target class"
		// condition the feature/build case above refuses (Wave-3 plan Task
		// 7 Step 2 lists "wrong target class" among the typed refusals, and
		// authority design §6 fixes each phase's admissible target class).
		// It is therefore the same typed exit-1 family, never an untyped
		// exit-2 operational error.
		return Result{}, &DeclaredScopeRefusal{
			Phase: request.Phase, Ref: target.Ref,
			Reason: fmt.Sprintf("target class %q is not an authoritative context-compile target", target.Spec.Class),
		}
	}

	obligations, err := ResolveBoundObligations(ctx, c.git, root, head, target)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 4 resolve bound obligations: %w", err)
	}
	declared, err := ResolveDeclaredContext(ctx, c.git, root, head, target, fragments)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 4 resolve declared context: %w", err)
	}

	// Stage 5: verify the full existing managed instruction projection
	// against disk.
	report, err := c.projection.Verify(root)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 5 verify instruction projection: %w", err)
	}
	if !report.Clean() {
		driftedPaths, driftReasons, err := driftWitness(report)
		if err != nil {
			return Result{}, fmt.Errorf("contextcompile: stage 5 verify instruction projection: %w", err)
		}
		return Result{}, &ProjectionDriftRefusal{Paths: driftedPaths, Reasons: driftReasons}
	}

	// Stage 5b: compute the full managed-projection path set across EVERY
	// adapter the resolved constitution declares — not only the requested
	// one — for Result.ManagedProjectionPaths (see its doc comment). Uses
	// the SAME resolved authority.Store.Constitution.Adapters stage 2
	// already loaded, so this never re-loads authority a second time.
	managedProjectionPaths, err := instructionprojection.ManagedPaths(authority.Store.Constitution.Adapters)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 5b compute managed projection paths: %w", err)
	}

	// Stage 7 (evaluated ahead of the stage-6 BuildUniverse call — see the
	// construction note above): evaluate applicability and select the
	// applicable authority operands and policy ids.
	operandCandidates, err := authorityOperandCandidates(authority)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 7 list authority operand candidates: %w", err)
	}
	selection, err := selectAuthorityOperands(operandCandidates, request, target)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 7 select authority operands: %w", err)
	}

	environment := ""
	if len(request.Scope.Environments) == 1 {
		environment = request.Scope.Environments[0]
	}

	// Stage 6: discover the candidate universe with the exact
	// store/declared-context lifts.
	entries, err := c.git.LsTreeEntries(ctx, root, head)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 6 list HEAD tree: %w", err)
	}
	worktreePaths, err := c.git.WorktreeChangedPaths(ctx, root)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 6 list worktree changed paths: %w", err)
	}

	authorityArtifacts, err := resolvedAuthorityArtifacts(authority)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 6 resolve authority artifacts: %w", err)
	}
	storeLifts, err := buildStoreLifts(target, fragments, obligations, selection.Operands, authorityArtifacts)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 6 build store-authority lifts: %w", err)
	}
	// The EFFECTIVE declared-context set, computed exactly once here and
	// used for the universe lifts, the classification materials and the
	// manifest's pinned-ref index alike, so no stage can disagree with
	// another about which declared refs this compile actually carries.
	effectiveDeclared := suppressStoreOwnedDeclaredContext(declared, storeLifts)
	declaredByLogicalRef, err := indexDeclaredContextItems(effectiveDeclared)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 6 index declared context: %w", err)
	}

	candidates, err := BuildUniverse(UniverseInput{
		Head:               head,
		Tree:               entries,
		WorktreePaths:      worktreePaths,
		LiftedStorePaths:   storeLifts,
		LiftedContextPaths: effectiveDeclared.Lift,
		ProjectionPaths:    append([]string(nil), authority.Adapter.Managed...),
		Adapter:            authority.Adapter,
	})
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 6 build candidate universe: %w", err)
	}

	// Stage 8: render the pure, phase-filtered projection for exactly the
	// stage-7 selected policy ids.
	projectionFiles, err := renderSelectedProjection(authority, selection.Selection)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 8 render selected projection: %w", err)
	}

	// Stage 9: classify every candidate exactly once and build data
	// payloads.
	// The one governance catalog every stored profile is decoded against
	// (policyauthority.Load's own rule), needed to re-derive the selected
	// profile's adopted digest from its exact HEAD bytes.
	catalog, err := authority.Store.Constitution.GovernanceCatalog()
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 9 resolve governance catalog: %w", err)
	}
	materials, err := buildClassificationMaterials(ctx, c.git, root, head, target, fragments, obligations, effectiveDeclared, selection, authorityArtifacts, catalog, projectionFiles)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 9 build classification materials: %w", err)
	}
	classification, err := Classify(ctx, c.git, root, head, ClassificationInput{
		Candidates:   candidates,
		Materials:    materials,
		Phase:        request.Phase,
		Environment:  environment,
		RequestScope: request.Scope,
		TargetRef:    target.Ref,
		Adapter:      AdapterRef{ID: authority.Adapter.ID, Version: authority.Adapter.Version},
	})
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 9 classify candidates: %w", err)
	}

	// Stage 10: project sealed actors, or the explicit unproven-absence
	// posture.
	actorsSection, err := projectActors(ctx, c.actors)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 10 project actors: %w", err)
	}

	// Stage 11: compute revisions, every manifest row, the disclosure
	// union, and canonical bytes.
	manifest, err := c.assembleManifest(ctx, root, head, snapshot, request, authority, selection, target, fragments, obligations, declaredByLogicalRef, classification, actorsSection)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 11 assemble manifest: %w", err)
	}
	manifestBytes, err := EncodeManifest(manifest)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 11 encode manifest: %w", err)
	}
	finalManifest, err := DecodeManifest(manifestBytes)
	if err != nil {
		return Result{}, fmt.Errorf("contextcompile: stage 11 decode canonical manifest: %w", err)
	}

	// Stage 12: return the fresh, in-memory Result.
	resultProjectionFiles := make([]ProjectionFile, 0, len(classification.ProjectionPayloads))
	for _, p := range classification.ProjectionPayloads {
		resultProjectionFiles = append(resultProjectionFiles, ProjectionFile{
			Path: p.Path, Content: append([]byte(nil), p.Content...), Digest: p.Digest,
		})
	}
	dataItemBytes := make([][]byte, len(classification.DataItemBytes))
	for i, b := range classification.DataItemBytes {
		dataItemBytes[i] = append([]byte(nil), b...)
	}

	return Result{
		Manifest:               finalManifest,
		ManifestBytes:          append([]byte(nil), manifestBytes...),
		DataItems:              append([]DataItem(nil), classification.DataItems...),
		DataItemBytes:          dataItemBytes,
		ProjectionFiles:        resultProjectionFiles,
		ManagedProjectionPaths: append([]string(nil), managedProjectionPaths...),
	}, nil
}

// driftWitness returns the sorted, de-duplicated paths AND closed
// instructionprojection.Reason codes report's findings named, for
// ProjectionDriftRefusal (authority design §10: the drift refusal carries a
// closed projection reason, not a bare path list).
//
// A finding legally names no path — the discovery walk and the orphan-
// manifest scan both produce directory- or manifest-level findings — so an
// empty Paths result is not itself a defect. A report that is not clean yet
// witnesses NEITHER a path nor a reason code is malformed port output: it
// fails closed as an operational error rather than becoming an exit-1
// refusal carrying no witness at all.
func driftWitness(report *instructionprojection.Report) ([]string, []string, error) {
	paths := uniqueSorted(len(report.Findings), func(yield func(string)) {
		for _, f := range report.Findings {
			yield(f.Path)
		}
	})
	reasons := uniqueSorted(len(report.Findings), func(yield func(string)) {
		for _, f := range report.Findings {
			yield(string(f.Code))
		}
	})
	if len(paths) == 0 && len(reasons) == 0 {
		// vocab:identity — "closed" names the closed-enum instructionprojection.Reason code property, not display prose
		return nil, nil, fmt.Errorf("instruction-projection report is not clean but names neither a path nor a closed reason code for any of its %d findings", len(report.Findings))
	}
	return paths, reasons, nil
}

// uniqueSorted collects every non-empty value emit yields into one sorted,
// de-duplicated slice.
func uniqueSorted(capacity int, emit func(yield func(string))) []string {
	seen := make(map[string]bool, capacity)
	out := make([]string, 0, capacity)
	emit(func(value string) {
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	})
	sort.Strings(out)
	return out
}

// buildStoreLifts builds the exact store-authority lift map authority
// design §5's store-authority row enumerates: "Resolved constitution,
// profile, applicable policies/overlays/exemptions, accepted spec, parent
// feature fragments, and obligations".
//
// One path may legitimately lift to only ONE ref, so a second entry
// claiming an already-claimed path is not a silent overwrite here: it is an
// inconsistent authority resolution (two distinct refs both claiming to own
// one tracked file's bytes) and fails closed. Re-declaring the identical
// (path, ref) pair is harmless and accepted, since the resulting lift map is
// unchanged.
func buildStoreLifts(
	target ResolvedSpec,
	fragments []FeatureFragment,
	obligations []BoundObligation,
	operands []PolicyOperand,
	authorityArtifacts []storeAuthorityArtifact,
) (map[string]string, error) {
	lifts := make(map[string]string, 1+len(fragments)+len(obligations)+len(operands)+len(authorityArtifacts))
	add := func(what, path, ref string) error {
		if path == "" || ref == "" {
			return fmt.Errorf("%s lifts empty path %q or empty ref %q", what, path, ref)
		}
		if prior, claimed := lifts[path]; claimed && prior != ref {
			return fmt.Errorf("%s and %s both lift path %q; a tracked path has exactly one store-authority owner", prior, ref, path)
		}
		lifts[path] = ref
		return nil
	}

	if err := add("accepted spec", target.Path, target.Ref); err != nil {
		return nil, err
	}
	for _, f := range fragments {
		// vocab:identity — "feature" names the parent FeatureFragment's artifact class in this store-lift owner label
		if err := add("parent feature fragment", f.Feature.Path, f.Feature.Ref); err != nil {
			return nil, err
		}
	}
	for _, o := range obligations {
		if err := add("obligation", o.Path, o.Ref); err != nil {
			return nil, err
		}
	}
	for _, op := range operands {
		if err := add("policy operand", op.Path, op.ID); err != nil {
			return nil, err
		}
	}
	for _, a := range authorityArtifacts {
		if err := add("resolved authority artifact", a.Path, a.Ref); err != nil {
			return nil, err
		}
	}
	return lifts, nil
}

// suppressStoreOwnedDeclaredContext returns the EFFECTIVE declared-context
// result after authority design §5's source precedence — SI-92: "source
// precedence remains store-authority > declared-context > head-tree, so an
// overlapping store-authority path suppresses the declared lift".
//
// Suppression is classification, not failure: a governing feature may
// legally pin an artifact whose store path this same compile already owns
// as authority (the commonest case being one parent feature pinning
// another, which is also a parent of the target story). That path's bytes
// are still in the capsule — once, under store-authority — so the declared
// lift AND its classification material are dropped together here, before
// either the universe or the materials are built. Dropping only the lift
// (universe.go's own resolveLifts already did that) would leave a material
// naming a candidate the universe deliberately never created, which
// Classify rejects as absent from the universe.
//
// Suppression is per path, exactly as universe.go's precedence rule is: an
// uncontested declared pin of a different path survives untouched.
func suppressStoreOwnedDeclaredContext(declared DeclaredContextResult, storeLifts map[string]string) DeclaredContextResult {
	effective := DeclaredContextResult{
		Items: make([]DeclaredContextItem, 0, len(declared.Items)),
		Lift:  make(map[string]string, len(declared.Lift)),
	}
	for _, item := range declared.Items {
		if _, storeOwned := storeLifts[item.Path]; storeOwned {
			continue
		}
		effective.Items = append(effective.Items, item)
	}
	for path, logicalRef := range declared.Lift {
		if _, storeOwned := storeLifts[path]; storeOwned {
			continue
		}
		effective.Lift[path] = logicalRef
	}
	return effective
}

// indexDeclaredContextItems builds the explicit one-to-one
// logicalRef->DeclaredContextItem index this service uses to preserve each
// declared-context-ref's complete pinned identity (authority design §5,
// §8.1; SI-92). ResolveDeclaredContext's own DeclaredContextResult.Items
// already carries at most one item per LogicalRef (it fails closed on a
// collapse itself), but this service never trusts that upstream guarantee
// blindly: it re-derives and re-checks the same invariant from the
// concrete Items/Lift values it was actually handed.
//
// The Items<->Lift correspondence is checked in BOTH directions, because
// the universe is built from Lift while the classification materials are
// built from Items: every Lift value must name a resolved Item (else the
// universe would carry a candidate no material can classify), and every
// Item's own Path must be lifted to that Item's own LogicalRef (else a
// material would name a candidate the universe never created, or two
// sources would claim one path). Either direction failing is a
// disagreement between this compile's own already-computed values, so it
// fails closed rather than being reconciled.
func indexDeclaredContextItems(result DeclaredContextResult) (map[string]DeclaredContextItem, error) {
	byLogicalRef := make(map[string]DeclaredContextItem, len(result.Items))
	for _, item := range result.Items {
		if _, dup := byLogicalRef[item.LogicalRef]; dup {
			return nil, fmt.Errorf("declared context: duplicate logical ref %q among resolved items", item.LogicalRef)
		}
		byLogicalRef[item.LogicalRef] = item
	}
	for path, logicalRef := range result.Lift {
		if _, ok := byLogicalRef[logicalRef]; !ok {
			return nil, fmt.Errorf("declared context: lift for path %q names logical ref %q with no resolved item", path, logicalRef)
		}
	}
	for _, item := range result.Items {
		lifted, ok := result.Lift[item.Path]
		if !ok {
			return nil, fmt.Errorf("declared context: item %q resolved at path %q, which no lift claims", item.LogicalRef, item.Path)
		}
		if lifted != item.LogicalRef {
			return nil, fmt.Errorf("declared context: path %q is lifted to logical ref %q but its resolved item names %q", item.Path, lifted, item.LogicalRef)
		}
	}
	return byLogicalRef, nil
}

// buildClassificationMaterials assembles every CandidateMaterial the stage
// 9 Classify call needs: the accepted target and each parent-feature
// fragment, bound obligation, selected policy operand, declared-context
// item, and rendered projection file. Every store-authority/declared-
// context material's PolicyScope is deliberately universal (authority
// design §6: these operands are not themselves path/phase scoped the way a
// policy artifact is — only a policy/overlay/exemption operand's own
// declared Scope narrows its applicability).
func buildClassificationMaterials(
	ctx context.Context,
	git GitReader,
	root, head string,
	target ResolvedSpec,
	fragments []FeatureFragment,
	obligations []BoundObligation,
	declared DeclaredContextResult,
	selection authoritySelection,
	authorityArtifacts []storeAuthorityArtifact,
	catalog governanceprincipal.Catalog,
	projectionFiles []ProjectionFile,
) ([]CandidateMaterial, error) {
	universal := universalApplicabilityScope()
	materials := make([]CandidateMaterial, 0, 1+len(fragments)+len(obligations)+len(selection.Operands)+len(authorityArtifacts)+len(declared.Items)+len(projectionFiles))

	materials = append(materials, CandidateMaterial{
		Source: SourceStoreAuthority, ID: refID(target.Ref), Kind: IncludedAcceptedSpec,
		PolicyScope: universal, Content: target.Content,
	})
	for _, f := range fragments {
		fragment := f
		materials = append(materials, CandidateMaterial{
			Source: SourceStoreAuthority, ID: refID(f.Feature.Ref), Kind: IncludedParentFeatureFragment,
			PolicyScope: universal, Fragment: &fragment,
		})
	}
	for _, o := range obligations {
		materials = append(materials, CandidateMaterial{
			Source: SourceStoreAuthority, ID: refID(o.Ref), Kind: IncludedObligation,
			PolicyScope: universal, Content: o.Content,
		})
	}
	for _, op := range selection.Operands {
		content, err := git.Show(ctx, root, head, op.Path)
		if err != nil {
			return nil, fmt.Errorf("read HEAD policy operand %s: %w", op.Path, err)
		}
		if err := requireAdoptedAuthorityDigest(op.ID, content, op.Digest, catalog); err != nil {
			return nil, err
		}
		materials = append(materials, CandidateMaterial{
			Source: SourceStoreAuthority, ID: refID(op.ID), Kind: IncludedPolicyArtifact,
			PolicyScope: cloneScope(op.Scope), Content: content,
		})
	}
	// The resolved constitution and selected profile carry no declared
	// Scope of their own — they are not scoped authority operands — so their
	// applicability scope is universal, exactly like the accepted spec and
	// the parent fragments above (authority design §6: only a policy/
	// overlay/exemption's own declared Scope narrows applicability).
	for _, a := range authorityArtifacts {
		content, err := git.Show(ctx, root, head, a.Path)
		if err != nil {
			return nil, fmt.Errorf("read HEAD authority artifact %s: %w", a.Path, err)
		}
		if err := requireAdoptedAuthorityDigest(a.Ref, content, a.Digest, catalog); err != nil {
			return nil, err
		}
		materials = append(materials, CandidateMaterial{
			Source: SourceStoreAuthority, ID: refID(a.Ref), Kind: IncludedPolicyArtifact,
			PolicyScope: universal, Content: content,
		})
	}
	for _, item := range declared.Items {
		materials = append(materials, CandidateMaterial{
			Source: SourceDeclaredContext, ID: refID(item.LogicalRef), Kind: IncludedDeclaredContextRef,
			PolicyScope: universal, Content: item.Content,
		})
	}
	for _, pf := range projectionFiles {
		materials = append(materials, CandidateMaterial{
			Source: SourceProjection, ID: pathID(pf.Path), Kind: IncludedInstructionProjection,
			PolicyScope: universal, Content: pf.Content,
		})
	}
	return materials, nil
}

// assembleManifest builds the complete stage-11 Manifest value (every
// section except the self digest, which EncodeManifest recomputes).
func (c Compiler) assembleManifest(
	ctx context.Context,
	root, head string,
	snapshot repositoryfacts.Snapshot,
	request Request,
	authority PolicyAuthority,
	selection authoritySelection,
	target ResolvedSpec,
	fragments []FeatureFragment,
	obligations []BoundObligation,
	declaredByLogicalRef map[string]DeclaredContextItem,
	classification ClassificationResult,
	actorsSection ActorsSection,
) (Manifest, error) {
	acceptedSpec := AcceptedSpec{
		Ref: target.Ref, Path: target.Path, Blob: target.Blob, Commit: target.Commit, ContentDigest: target.ContentDigest,
	}

	parentFeatures, err := buildParentFeatureRows(fragments, classification.Included)
	if err != nil {
		return Manifest{}, err
	}
	decisions, err := collectManifestDecisions(target.Ref, target.Spec.Decisions, fragments)
	if err != nil {
		return Manifest{}, err
	}
	obligationRows := buildObligationRows(obligations)

	owners, err := c.resolveOwners(ctx, root, head, target, fragments)
	if err != nil {
		return Manifest{}, err
	}

	repositorySection := buildRepositorySection(snapshot)

	policyEntries, err := buildPolicyEntries(selection, request, target)
	if err != nil {
		return Manifest{}, err
	}
	policySection := PolicySection{
		EffectiveDigest:    authority.EffectiveDigest,
		ConstitutionDigest: authority.Effective.ConstitutionDigest,
		ProfileID:          authority.Effective.ProfileID,
		ProfileDigest:      authority.Effective.ProfileDigest,
		Entries:            policyEntries,
	}

	storedProfile, ok := authority.Store.Profiles[authority.Effective.ProfileID]
	if !ok || storedProfile == nil {
		return Manifest{}, fmt.Errorf("resolved effective policy names profile %q, absent from the loaded store", authority.Effective.ProfileID)
	}
	governanceProfile := GovernanceProfileRef{
		ID: authority.Effective.ProfileID, Class: storedProfile.Profile.Class, Digest: authority.Effective.ProfileDigest,
	}

	includedRows, err := applyDeclaredContextPinnedRefs(classification.Included, declaredByLogicalRef)
	if err != nil {
		return Manifest{}, err
	}

	var requiredInputs []RequiredInput
	var reviewDisclosures []DisclosureCode
	if request.Phase == PhaseReview {
		requiredInputs, reviewDisclosures = reviewRequiredInputs(target.ContentDigest, authority.EffectiveDigest)
	} else {
		requiredInputs = []RequiredInput{}
	}

	evidenceSection := EvidenceSection{
		Authority: EvidenceAuthorityAdvisory,
		// v1 never caches across calls (CLAUDE.md/plan) and computes every
		// fact from the same HEAD it gathers at stage 3 and uses
		// unchanged through every later stage, so freshness is fresh by
		// construction — there is no second, possibly-stale evidence axis
		// in v1 to disclose (authority design §8.2, §10).
		Freshness:       EvidenceFreshnessFresh,
		ConsumedReports: []string{},
		Disclosures:     []DisclosureCode{},
	}

	authorityDigest, err := authorityRevision(authorityRevisionInput{
		EffectivePolicyDigest: authority.EffectiveDigest,
		AcceptedSpec:          acceptedSpec,
		// append's dst starts non-nil ([]FeatureFragment{}, not
		// []FeatureFragment(nil)): appending zero elements onto a nil dst
		// would otherwise return nil unchanged (Go's append is a no-op
		// allocation-wise when there is nothing to append), silently
		// turning an explicit empty fragment set back into a nil one right
		// before authorityRevision's own non-nil gate — defeating that
		// gate rather than satisfying it.
		ParentFragments: append([]FeatureFragment{}, fragments...),
		Decisions:       toAuthorityRevisionDecisions(decisions),
		Obligations:     toAuthorityRevisionObligations(obligationRows),
	})
	if err != nil {
		return Manifest{}, fmt.Errorf("compute authority revision: %w", err)
	}
	revisions, err := contextRevisions(authorityDigest)
	if err != nil {
		return Manifest{}, fmt.Errorf("compute context revisions: %w", err)
	}

	disclosures := unionDisclosures(
		repositorySection.Disclosures, selection.Disclosures, actorsSection.Disclosures,
		evidenceSection.Disclosures, reviewDisclosures,
		disclosuresOf(includedRows), disclosuresOf(classification.Excluded), disclosuresOf(classification.Opaque),
	)

	return Manifest{
		Schema:            ManifestSchema,
		Phase:             request.Phase,
		Adapter:           AdapterRef{ID: authority.Adapter.ID, Version: authority.Adapter.Version},
		Revisions:         revisions,
		AcceptedSpec:      acceptedSpec,
		ParentFeatures:    parentFeatures,
		Decisions:         decisions,
		Obligations:       obligationRows,
		Repository:        repositorySection,
		Policy:            policySection,
		Owners:            owners,
		Scope:             cloneScope(request.Scope),
		GovernanceProfile: governanceProfile,
		Actors:            actorsSection,
		Included:          includedRows,
		// Same non-nil-dst discipline as ParentFragments above: Manifest's
		// mandatory excluded/opaque collections must encode as `[]`, never
		// `null`, even when classification.Excluded/Opaque happens to be
		// empty.
		Excluded:        append([]ExcludedEntry{}, classification.Excluded...),
		Opaque:          append([]OpaqueEntry{}, classification.Opaque...),
		Capabilities:    cloneGrantSet(request.Grants),
		ProjectionFiles: buildProjectionFileRows(classification.ProjectionPayloads),
		RequiredInputs:  requiredInputs,
		Evidence:        evidenceSection,
		Disclosures:     disclosures,
	}, nil
}

// buildParentFeatureRows builds the manifest's sorted parent_features rows.
// PayloadDigest binds each fragment's already-classified data-item wrapper
// (classification.Included's own IncludedEntry.PayloadDigest for that
// fragment's store-authority/parent-feature-fragment candidate) rather than
// recomputing it — the two must always name the identical bytes, so this
// looks the digest up instead of risking a second, possibly-diverging
// computation.
func buildParentFeatureRows(fragments []FeatureFragment, included []IncludedEntry) ([]ParentFeature, error) {
	payloadDigestByRef := make(map[string]string, len(included))
	for _, e := range included {
		if e.Source == SourceStoreAuthority && e.Kind == IncludedParentFeatureFragment && e.Ref != nil {
			payloadDigestByRef[*e.Ref] = e.PayloadDigest
		}
	}
	rows := make([]ParentFeature, 0, len(fragments))
	for _, f := range fragments {
		encoded, err := EncodeFeatureFragment(f)
		if err != nil {
			return nil, err
		}
		payloadDigest, ok := payloadDigestByRef[f.Feature.Ref]
		if !ok {
			return nil, fmt.Errorf("parent_features: no classified payload for %s", f.Feature.Ref)
		}
		rows = append(rows, ParentFeature{
			Ref: f.Feature.Ref, Path: f.Feature.Path, SourceDigest: f.Feature.SourceDigest,
			FragmentDigest: rawContentDigest(encoded), PayloadDigest: payloadDigest,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Ref < rows[j].Ref })
	return rows, nil
}

// buildObligationRows maps every resolved BoundObligation to its manifest
// row, sorted by ref (ResolveBoundObligations already returns its result
// sorted; this re-sorts defensively rather than trusting call-site order).
func buildObligationRows(obligations []BoundObligation) []Obligation {
	rows := make([]Obligation, 0, len(obligations))
	for _, o := range obligations {
		rows = append(rows, Obligation{Ref: o.Ref, Path: o.Path, AC: o.AC, Kind: o.Kind, ContentDigest: o.ContentDigest})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Ref < rows[j].Ref })
	return rows
}

// buildProjectionFileRows maps every classified (included) projection
// payload to its manifest row, sorted by path. classification.
// ProjectionPayloads is already sorted by path (classify.go); this
// re-sorts defensively.
func buildProjectionFileRows(payloads []ProjectionPayload) []ProjectionFileRef {
	rows := make([]ProjectionFileRef, 0, len(payloads))
	for _, p := range payloads {
		rows = append(rows, ProjectionFileRef{Path: p.Path, Digest: p.Digest})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	return rows
}

// resolveOwners returns the sorted, de-duplicated union of the accepted
// target's own declared owners and every governing parent feature's
// declared owners (authority design §8.2: "declared owners, never
// authenticated actors"). SI-88's frozen FeatureFragment wire deliberately
// excludes owners, so each parent's owners are re-read here directly from
// its exact HEAD bytes at fragment.Feature.Path, re-verified (TOCTOU)
// against fragment.Feature.SourceDigest before being trusted — the same
// re-verification discipline semantic_dependencies.go's own
// reverifyGoverningFeature already applies to a governing parent's
// context: list.
func (c Compiler) resolveOwners(ctx context.Context, root, head string, target ResolvedSpec, fragments []FeatureFragment) ([]string, error) {
	seen := make(map[string]bool)
	var owners []string
	add := func(list []string) {
		for _, o := range list {
			if o == "" || seen[o] {
				continue
			}
			seen[o] = true
			owners = append(owners, o)
		}
	}
	add(target.Spec.Owners)

	for _, f := range fragments {
		content, err := c.git.Show(ctx, root, head, f.Feature.Path)
		if err != nil {
			// vocab:identity — "feature" names the governing parent FeatureFragment's artifact class this owners re-read targets
			return nil, fmt.Errorf("owners: read HEAD parent feature %s: %w", f.Feature.Path, err)
		}
		if rawContentDigest(content) != f.Feature.SourceDigest {
			// vocab:identity — "feature" names the governing parent FeatureFragment's artifact class in this TOCTOU diagnostic
			return nil, fmt.Errorf("owners: parent feature %s content at exact HEAD no longer matches its resolved source digest (TOCTOU mismatch)", f.Feature.Ref)
		}
		fmBytes, _, err := artifact.SplitFrontmatter(content)
		if err != nil {
			// vocab:identity — "feature" names the governing parent FeatureFragment's artifact class this frontmatter split targets
			return nil, fmt.Errorf("owners: decode parent feature %s: %w", f.Feature.Ref, err)
		}
		spec, err := artifact.DecodeSpec(fmBytes)
		if err != nil {
			// vocab:identity — "feature" names the governing parent FeatureFragment's artifact class this spec decode targets
			return nil, fmt.Errorf("owners: decode parent feature %s: %w", f.Feature.Ref, err)
		}
		add(spec.Owners)
	}

	sort.Strings(owners)
	if owners == nil {
		owners = []string{}
	}
	return owners, nil
}

// buildRepositorySection maps the ONE stage-3 repository-fact snapshot to
// the manifest's `repository` section, translating repositoryfacts' finer
// per-cause disclosure codes to the coarser closed §8.2 manifest vocabulary
// (SI-85).
//
// It deliberately takes stage 3's already-gathered Snapshot rather than
// re-gathering: authority design §4 computes these facts once, and §8.2's
// `repository` row must disclose the facts THIS compile used — the same
// values stage 3 compared the caller's optional expectation against and
// stage 4 onward resolved every object at. Nothing freezes the checkout
// between stage 3 and stage 11, so a second gather could publish facts no
// other stage ever consumed.
func buildRepositorySection(snapshot repositoryfacts.Snapshot) RepositoryFacts {
	f := snapshot.Facts
	return RepositoryFacts{
		RemoteOrigin:  StringFact{Known: f.RemoteOrigin.Known, Value: f.RemoteOrigin.Value},
		Branch:        StringFact{Known: f.Branch.Known, Value: f.Branch.Value},
		Head:          StringFact{Known: f.Head.Known, Value: f.Head.Value},
		DefaultBranch: DefaultBranchFact{Known: f.DefaultBranch.Known, Name: f.DefaultBranch.Name, Ref: f.DefaultBranch.Ref, Head: f.DefaultBranch.Head},
		Relationship:  f.Relationship,
		Dirty:         BoolFact{Known: f.Dirty.Known, Value: f.Dirty.Value},
		Staged:        BoolFact{Known: f.Staged.Known, Value: f.Staged.Value},
		Worktree:      WorktreeFact{Managed: f.Worktree.Managed, Name: f.Worktree.Name},
		Source:        string(f.Source),
		Disclosures:   mapRepositoryDisclosures(snapshot),
	}
}

// mapRepositoryDisclosures translates repositoryfacts' finer per-cause
// disclosure codes to the coarser closed manifest vocabulary (authority
// design §8.2; SI-85), plus the compiler-only
// DisclosureDefaultRelationshipUnknown code for a computed "unknown"
// relationship that repositoryfacts itself does not separately disclose
// (its own DisclosureHeadUnresolved/DisclosureDefaultBranchRefUnresolved
// codes already cover the two causes it does disclose).
func mapRepositoryDisclosures(snapshot repositoryfacts.Snapshot) []DisclosureCode {
	set := make(map[DisclosureCode]bool, len(snapshot.Disclosures)+1)
	for _, d := range snapshot.Disclosures {
		switch d {
		case repositoryfacts.DisclosureRemoteOriginUncanonicalizable,
			repositoryfacts.DisclosureRemoteOriginNotConfigured,
			repositoryfacts.DisclosureRemoteOriginReadFailed:
			set[DisclosureRepositoryRemoteUnknown] = true
		case repositoryfacts.DisclosureBranchUnresolved, repositoryfacts.DisclosureBranchDetached:
			set[DisclosureRepositoryBranchUnknown] = true
		case repositoryfacts.DisclosureHeadUnresolved:
			set[DisclosureRepositoryHeadUnknown] = true
		case repositoryfacts.DisclosureDefaultBranchRefUnresolved:
			set[DisclosureDefaultBranchUnknown] = true
		case repositoryfacts.DisclosureDirtyUnknown:
			set[DisclosureDirtyStateUnknown] = true
		case repositoryfacts.DisclosureStagedUnknown:
			set[DisclosureStagedStateUnknown] = true
		}
	}
	if snapshot.Facts.Relationship == repositoryfacts.RelationshipUnknown &&
		!set[DisclosureRepositoryHeadUnknown] && !set[DisclosureDefaultBranchUnknown] {
		set[DisclosureDefaultRelationshipUnknown] = true
	}
	out := make([]DisclosureCode, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// buildPolicyEntries recomputes each selected operand's own applicability
// (authority design §8.2: "every applicable authority operand") — the same
// pure ApplicabilityInput selectAuthorityOperands itself evaluated, so the
// two agree by construction — and maps each retained operand to its
// manifest PolicyEntry row, sorted by (kind,id).
func buildPolicyEntries(selection authoritySelection, request Request, target ResolvedSpec) ([]PolicyEntry, error) {
	environment := ""
	if len(request.Scope.Environments) == 1 {
		environment = request.Scope.Environments[0]
	}
	rows := make([]PolicyEntry, 0, len(selection.Operands))
	for _, op := range selection.Operands {
		result, err := EvaluateApplicability(ApplicabilityInput{
			Policy: op.Scope, Request: request.Scope, CandidatePath: target.Path, CandidateRef: target.Ref,
			Phase: request.Phase, Environment: environment,
		})
		if err != nil {
			return nil, fmt.Errorf("policy entry %s applicability: %w", op.ID, err)
		}
		rows = append(rows, PolicyEntry{Kind: op.Kind, ID: op.ID, Digest: op.Digest, Applicability: result.State})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Kind+"\x00"+rows[i].ID < rows[j].Kind+"\x00"+rows[j].ID })
	return rows, nil
}

// applyDeclaredContextPinnedRefs returns a copy of entries with every
// declared-context-ref row's Ref overwritten to carry the COMPLETE PINNED
// ref from its resolved DeclaredContextItem, looked up by the row's
// current (logical) Ref in byLogicalRef — never reconstructed from the
// lift map. The candidate/data-item identity itself (ID, and the
// data-item's own `ref` field) stays the unpinned logical form throughout;
// only this manifest ledger row's `ref` field is widened to the pinned
// identity, since only the manifest is where a reader needs to see exactly
// which commit's bytes were actually included.
func applyDeclaredContextPinnedRefs(entries []IncludedEntry, byLogicalRef map[string]DeclaredContextItem) ([]IncludedEntry, error) {
	out := make([]IncludedEntry, len(entries))
	for i, e := range entries {
		out[i] = e
		// Non-nil dst, matching assembleManifest's own ParentFragments/
		// Excluded/Opaque discipline: e.Disclosures is already a non-nil
		// (possibly empty) slice (classify.go's includedEntry always
		// builds it via append([]DisclosureCode{}, ...)), and copying it
		// onto a nil dst would silently collapse an explicit empty
		// disclosure set back into nil right before EncodeManifest's own
		// non-nil gate on every included row's disclosures.
		out[i].Disclosures = append([]DisclosureCode{}, e.Disclosures...)
		if e.Source != SourceDeclaredContext || e.Kind != IncludedDeclaredContextRef {
			continue
		}
		if e.Ref == nil {
			return nil, fmt.Errorf("declared-context included row %s has no ref", e.ID)
		}
		item, ok := byLogicalRef[*e.Ref]
		if !ok {
			return nil, fmt.Errorf("declared-context included row %s: no resolved item for logical ref %q", e.ID, *e.Ref)
		}
		pinned := item.Ref
		out[i].Ref = &pinned
	}
	return out, nil
}

// toAuthorityRevisionDecisions/toAuthorityRevisionObligations adapt the
// manifest's own DecisionRef/Obligation rows to the narrower operand shape
// authorityRevisionInput admits (ref+digest only — see revisions.go's
// authorityRevisionInput doc comment on why no other field exists there).
func toAuthorityRevisionDecisions(rows []DecisionRef) []authorityRevisionDecision {
	out := make([]authorityRevisionDecision, len(rows))
	for i, r := range rows {
		out[i] = authorityRevisionDecision{Ref: r.Ref, Digest: r.ContentDigest}
	}
	return out
}

func toAuthorityRevisionObligations(rows []Obligation) []authorityRevisionObligation {
	out := make([]authorityRevisionObligation, len(rows))
	for i, r := range rows {
		out[i] = authorityRevisionObligation{Ref: r.Ref, Digest: r.ContentDigest}
	}
	return out
}

// disclosuresOf extracts and flattens every row's own Disclosures field
// from any of the manifest's disclosure-carrying row slices.
func disclosuresOf[T interface{ disclosureCodes() []DisclosureCode }](rows []T) []DisclosureCode {
	var out []DisclosureCode
	for _, r := range rows {
		out = append(out, r.disclosureCodes()...)
	}
	return out
}

func (e IncludedEntry) disclosureCodes() []DisclosureCode { return e.Disclosures }
func (e ExcludedEntry) disclosureCodes() []DisclosureCode { return e.Disclosures }
func (e OpaqueEntry) disclosureCodes() []DisclosureCode   { return e.Disclosures }

// unionDisclosures returns the sorted, de-duplicated union of every
// disclosure code named anywhere in the manifest (authority design §8.2:
// "top-level disclosures = sorted unique union of every disclosure used
// anywhere in the manifest").
func unionDisclosures(sets ...[]DisclosureCode) []DisclosureCode {
	seen := make(map[DisclosureCode]bool)
	for _, set := range sets {
		for _, d := range set {
			seen[d] = true
		}
	}
	out := make([]DisclosureCode, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
