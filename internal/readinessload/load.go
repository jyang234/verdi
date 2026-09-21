package readinessload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/store"
)

// noContextRequestWitness is R-RR1-5's fixed witness sentence for a
// derivation with no supplied --context-request.
const noContextRequestWitness = "no context request supplied for this derivation"

// staleConflictReportWitness is R-RR1-18's fixed witness sentence for a
// policy-conflict report whose target content digest does not match the
// spec bytes on disk. That report cannot speak for these bytes, which is
// the SAME epistemic state as a cache miss — so the derivation discloses
// it as unproven rather than failing operationally (CLAUDE.md's
// three-valued honesty: disclosed-as-unproven over an operational
// failure).
//
// It deliberately does NOT say "cached" (re-review M2): the digest check
// runs over every provider result, including a JudgeRun warm-up whose own
// read of the spec raced an edit, so a freshly computed report reaches
// this sentence too and an operator told the CACHE is stale would look in
// the wrong place. The epistemic claim and the destination hold either
// way. It also carries no path, digest or control character:
// readinesspilot rejects control characters, and the digests, while
// deterministic, tell an operator nothing they can act on. The
// context-conflict verb named here is the destination that refreshes the
// report, and it is exactly the verb the request-bound contextFallback
// vector already points at.
const staleConflictReportWitness = "the policy-conflict report was computed for different spec bytes than the ones on disk; re-run the context-conflict verb to refresh it"

// staleExpectedRepositoryWitness is R-RRF-3's (SI-214) fixed witness
// sentence for a per-request derivation whose --context-request carries an
// optional `expected` claim that no longer matches the checkout. It is the
// SI-208 shape one level earlier: nothing was evaluated for these bytes, so
// the honest answer is disclosed-as-unproven, not an operational failure
// that blanks every unrelated area of the page (CLAUDE.md's three-valued
// honesty). It names both repositories because that pair is exactly what an
// operator needs to decide whether to re-run the context-conflict verb or
// to move the checkout back.
//
// Both repositories are rendered with %q: `expected.branch` is
// caller-authored request data that contextcompile only requires to be
// non-empty, so an unquoted branch could carry a control character and
// readinesspilot rejects a control-bearing witness — which would turn this
// disclosure straight back into the operational failure the ruling
// removes. %q escapes every such rune, so the sentence is display-safe by
// construction for any request the decoder accepts. It carries no path and
// no digest (the SI-208 reasoning): the request-bound contextFallback
// vector already names the file, and a digest tells an operator nothing
// they can act on.
func staleExpectedRepositoryWitness(expected, computed contextcompile.Expected) string {
	return fmt.Sprintf(
		"the context request expected repository %q; the repository now reads %q: the policy-conflict evaluation was not derived for these bytes",
		expected.Branch+"@"+expected.Head, computed.Branch+"@"+computed.Head,
	)
}

// noContextRequestCLI is R-RR1-5's fixed context/verdict destination when
// no request was supplied: an instructive vector naming the verb and flag a
// caller would use to supply one, not a runnable command against any real
// file (there is none).
var noContextRequestCLI = []string{"verdi", "context", "conflict", "--request", "<path>"}

// Load derives readiness for ref (an unpinned whole spec ref) at the
// checkout's HEAD, on whatever branch is currently checked out — any active
// feature or story spec, any branch (spec/readiness-recovery ac-2). Two
// derivations for the same ref at the same HEAD are byte-identical: every
// source read is deterministic and Derive itself is pure.
func Load(ctx context.Context, root, ref string, opts Options) (readinesspilot.Snapshot, error) {
	return loader{}.load(ctx, root, ref, opts)
}

// Loader is the consumer-side port every surface (workbench, MCP, cmd/verdi
// itself) defines for itself; this is the production value.
type Loader struct {
	Root string
	Opts Options
}

// Load derives readiness for ref through l's own Root/Opts.
func (l Loader) Load(ctx context.Context, ref string) (readinesspilot.Snapshot, error) {
	return Load(ctx, l.Root, ref, l.Opts)
}

// loader is the package's own narrow test seam: its optional function
// fields are hermetic-testing hooks; the useful zero value selects every
// production predecessor (mirrors cmd/verdi's old
// localReadinessSnapshotBuilder, which this type's load method's body is
// lifted from).
type loader struct {
	readFile            func(string) ([]byte, error)
	gatherFacts         func(context.Context, *store.Config, string) (journey.Facts, error)
	projectJourney      func(context.Context, *store.Config, string, journey.Extras) (journey.Record, error)
	newConflictProvider func(context.Context, string, policyconflict.Request, JudgeMode, ActorsResolver) (policyconflict.VerdictProvider, error)
	readAnnotations     func(string) ([]*artifact.Annotation, error)
}

// load captures one complete readiness derivation. All source reads and
// the predecessor-owned policy evaluation finish before the value is
// returned; nothing is retained or persisted (co-2).
func (l loader) load(ctx context.Context, root, ref string, opts Options) (readinesspilot.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: %w", err)
	}
	parsedRef, err := artifact.ParseRef(ref)
	if err != nil || parsedRef.Kind != artifact.KindSpec || parsedRef.Pinned() || parsedRef.Fragment() {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: ref %q is not an unpinned whole spec ref", ref)
	}
	name := parsedRef.Name

	readFile := l.readFile
	if readFile == nil {
		readFile = os.ReadFile
	}

	// The request path is validated, read, and decoded far enough to check
	// phase/spec identity BEFORE store.Open touches the filesystem at all
	// (mirrors cmd/verdi's old adapter: "..\"/symlink refusal before any
	// read" is a hard requirement, not just "before any request read").
	// request/haveRequest carry the decoded request into the second half of
	// conflict handling below, which needs branch/head from gatherFacts
	// first to compute Expected.
	var request contextcompile.Request
	haveRequest := false
	var requestBytes []byte
	switch opts.ContextRequestPath {
	case "":
	case "-":
		return readinesspilot.Snapshot{}, errors.New("readinessload: loading readiness: --context-request does not accept stdin ('-')")
	default:
		if opts.PredecodedRequest != nil {
			// The caller (e.g. serve's startup pre-run, via
			// ContextRequestSpec) already validated the path and read/decoded
			// this exact file once; every check below still applies to its
			// own bytes/value, just without a second read (Minor 6).
			requestBytes = opts.PredecodedRequest.Bytes
			request = opts.PredecodedRequest.Request
		} else {
			validatedPath, err := ValidatedContextRequestPath(root, opts.ContextRequestPath)
			if err != nil {
				return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: %w", err)
			}
			requestBytes, err = readFile(validatedPath)
			if err != nil {
				return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: reading --context-request: %w", err)
			}
			request, err = contextcompile.DecodeRequest(requestBytes)
			if err != nil {
				return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: decoding --context-request: %w", err)
			}
		}
		if request.Phase != contextcompile.PhaseDesign {
			return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: request must use phase %q, got %q", contextcompile.PhaseDesign, request.Phase)
		}
		if request.Spec != ref {
			// R-RR1-15: a supplied request binds to its OWN spec only.
			// `verdi serve --context-request <a request for X>` threads one
			// loader into the board, the Document tab and MCP, so every
			// OTHER spec is derived through this same request-bound loader
			// — and must be derived exactly as if no request had been
			// supplied (the fixed no-request witness and destination below,
			// RequestDigest over zero bytes), never refused. Refusing here
			// made every other spec's readiness a loader error for as long
			// as that server ran; carrying a request-specific witness
			// instead would break ac-4 the other way, since this ref must
			// read byte-identically here and on a CLI that has no request
			// at all (TestLoad_RequestForAnotherSpecDerivesAsIfAbsent
			// states both halves). Dropping the bytes keeps RequestDigest
			// the digest of zero bytes, R-RR1-5's no-request value.
			requestBytes = nil
			break // haveRequest stays false: this derivation carries no request
		}
		haveRequest = true
	}

	cfg, err := store.Open(root)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: opening store: %w", err)
	}

	projector := journey.NewProjector()
	gatherFacts := l.gatherFacts
	if gatherFacts == nil {
		gatherFacts = projector.GatherFacts
	}
	facts, err := gatherFacts(ctx, cfg, ref)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: gathering repository facts: %w", err)
	}
	if !facts.Repository.Branch.Known || !facts.Repository.Head.Known {
		return readinesspilot.Snapshot{}, errors.New("readinessload: loading readiness: repository branch and HEAD must both be proven")
	}
	branch := facts.Repository.Branch.Value
	head := facts.Repository.Head.Value
	// ac-2: the design-branch gate is gone — Load derives readiness for ref
	// on whatever branch is currently checked out. The one gate that
	// remains is "active": an archived spec's Target.Path resolves under a
	// different zone and is refused here exactly as it always was (in
	// practice journey.GatherFacts itself already refuses a spec absent
	// from the active zone before this ever runs; this stays as a
	// defensive invariant against a future GatherFacts that resolves an
	// archived spec by path instead).
	if facts.Target.Path != store.ActiveSpecRelPath(name) {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: target %q is not an active spec", ref)
	}

	var report policyconflict.Report
	conflictUnavailable := ""
	contextFallback := noContextRequestCLI

	computed := contextcompile.Expected{Branch: branch, Head: head}
	// staleExpected: the request carries an optional `expected` claim and it
	// no longer describes this checkout.
	staleExpected := haveRequest && request.Expected != nil && *request.Expected != computed
	switch {
	case !haveRequest:
		conflictUnavailable = noContextRequestWitness
	case staleExpected && opts.RequireExpectedMatch:
		// Only `verdi serve`'s startup warm-up sets the option: a request
		// that is ALREADY stale when the server starts is a
		// misconfiguration the operator must see at once, not a page that
		// quietly discloses it forever.
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: --context-request expected repository %+v does not match computed repository %+v", *request.Expected, computed)
	case staleExpected:
		// R-RRF-3 (SI-214, independent review R3): on an ordinary
		// per-request load the mismatch is the ac-3 ConflictUnavailable
		// posture, not an error. `verdi serve` keeps deriving against the
		// request bundle it validated at startup (R-RR1-17), so one
		// ordinary commit or branch change made this claim stale and
		// refusing here blanked the startup spec's entire readiness page —
		// every area, including the ones the request has nothing to do with
		// — until the server was restarted. Nothing was evaluated for these
		// bytes, so no provider is constructed and no report is read; the
		// context area carries the one witness and every other area
		// derives. `expected` is deliberately NOT rebound to the computed
		// value: silently re-pointing the operator's own claim at whatever
		// the checkout now says would manufacture authority the request
		// never granted. contextFallback becomes the request-bound vector
		// because re-running that verb against this file IS how the
		// operator refreshes it.
		conflictUnavailable = staleExpectedRepositoryWitness(*request.Expected, computed)
		contextFallback = []string{"verdi", "context", "conflict", "--request", opts.ContextRequestPath}
	default:
		request.Expected = &computed

		conflictRequest := policyconflict.Request{
			Schema: policyconflict.RequestSchema,
			Target: policyconflict.Target{
				Kind: policyconflict.TargetAcceptanceCandidate,
				AcceptanceCandidate: &policyconflict.AcceptanceCandidate{
					Adapter: request.Adapter, Expected: computed, Grants: request.Grants, Scope: request.Scope, Spec: request.Spec,
				},
			},
		}
		if err := conflictRequest.Validate(); err != nil {
			return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: constructing conflict request: %w", err)
		}

		var provider policyconflict.VerdictProvider
		var err error
		switch {
		case opts.ConflictProvider != nil:
			// The caller's own seam (Important 2): a hermetic test or a
			// consumer with an already-resolved provider replaces
			// NewConflictProvider entirely — opts.Judge/opts.Actors are the
			// caller's own concern in that case, not this package's.
			provider, err = opts.ConflictProvider(ctx, root, conflictRequest)
		case l.newConflictProvider != nil:
			provider, err = l.newConflictProvider(ctx, root, conflictRequest, opts.Judge, opts.Actors)
		default:
			provider, err = NewConflictProvider(ctx, root, conflictRequest, opts.Judge, opts.Actors)
		}
		if err != nil {
			return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: constructing policy-conflict provider: %w", err)
		}
		if provider == nil {
			return readinesspilot.Snapshot{}, errors.New("readinessload: loading readiness: policy-conflict provider is nil")
		}
		conflictResult, err := provider.Evaluate(ctx, conflictRequest)
		if err != nil {
			return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: evaluating policy conflicts: %w", err)
		}
		if err := reportIdentity(conflictResult.Report, ref, facts.Target.Path, branch, head); err != nil {
			return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: %w", err)
		}
		report = conflictResult.Report
		contextFallback = []string{"verdi", "context", "conflict", "--request", opts.ContextRequestPath}
	}

	specPath := filepath.Join(root, filepath.FromSlash(facts.Target.Path))
	specBytes, err := readFile(specPath)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: reading target spec: %w", err)
	}
	frontmatter, _, err := artifact.SplitFrontmatter(specBytes)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: splitting target spec: %w", err)
	}
	spec, err := artifact.DecodeSpec(frontmatter)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: decoding target spec: %w", err)
	}
	if spec.ID != ref || string(spec.Class) != facts.Target.Class {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: decoded spec identity (%q, %q) does not match resolved target identity (%q, %q)", spec.ID, spec.Class, ref, facts.Target.Class)
	}
	if spec.Class != artifact.ClassFeature && spec.Class != artifact.ClassStory {
		// vocab:identity — operational diagnostic naming the fixed artifact class identities
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: decoded spec class %q is not feature or story", spec.Class)
	}
	if conflictUnavailable == "" {
		candidate := report.Input.Target.Candidate
		if candidate.ContentDigest != readinessDigest(specBytes) {
			// R-RR1-18: a digest MISMATCH is the same epistemic state as a
			// cache MISS — the report was computed over different spec
			// bytes (whether it came from the D4 cache or from a judge run
			// that raced an edit), so it cannot speak for the ones on
			// disk. It used
			// to be a loader error, which meant a single spec edit blanked
			// the entire readiness page (and, through one server-wide
			// loader, kept blanking it for every other spec too). The
			// report is dropped rather than shown: nothing it says was
			// evaluated against these bytes, and readinesspilot refuses a
			// non-zero report alongside an unavailability witness anyway.
			// contextFallback is deliberately left as the request-bound
			// vector — re-running the context-conflict verb IS how the
			// report is refreshed.
			conflictUnavailable = staleConflictReportWitness
			report = policyconflict.Report{}
		}
	}

	provenance, err := provenanceFacts(readFile, root, name, ref, specBytes)
	if err != nil {
		return readinesspilot.Snapshot{}, err
	}
	mutation, err := mutationFacts(root, name)
	if err != nil {
		return readinesspilot.Snapshot{}, err
	}
	provenance.MutationState = mutation.MutationState
	provenance.MutationWitnesses = mutation.MutationWitnesses
	readAnnotations := l.readAnnotations
	if readAnnotations == nil {
		readAnnotations = boardio.ReadAllAnnotations
	}
	board, err := boardFacts(readAnnotations, root, name)
	if err != nil {
		return readinesspilot.Snapshot{}, err
	}

	declared := artifact.DeclaredObjectIDs(spec)
	declaredIDs := make([]string, 0, len(declared))
	for id := range declared {
		declaredIDs = append(declaredIDs, id)
	}
	sort.Strings(declaredIDs)
	openQuestionIDs := make([]string, len(spec.OpenQuestions))
	for i, question := range spec.OpenQuestions {
		openQuestionIDs[i] = question.ID
	}
	sort.Strings(openQuestionIDs)
	claimedQuestions := claimedQuestionsOf(spec.Stubs)

	boardPath := ""
	if opts.BoardHref != nil {
		boardPath = opts.BoardHref(branch, name)
	}

	var conflictPtr *policyconflict.Report
	if conflictUnavailable == "" {
		conflictPtr = &report
	}
	projectJourney := l.projectJourney
	if projectJourney == nil {
		projectJourney = projector.ProjectWith
	}
	record, err := projectJourney(ctx, cfg, ref, journey.Extras{Conflict: conflictPtr})
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: projecting journey: %w", err)
	}
	if record.Target.Ref != ref || record.Target.Path != facts.Target.Path {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: journey target (%q, %q) does not match resolved target (%q, %q)", record.Target.Ref, record.Target.Path, ref, facts.Target.Path)
	}
	if !record.Repository.Branch.Known || record.Repository.Branch.Value != branch || !record.Repository.Head.Known || record.Repository.Head.Value != head {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: journey repository (%q, %q) does not match resolved repository (%q, %q)", record.Repository.Branch.Value, record.Repository.Head.Value, branch, head)
	}

	input := readinesspilot.Input{
		Target: readinesspilot.TargetFacts{
			Ref: ref, Title: spec.Title, Class: string(spec.Class), Branch: branch, Head: head, BoardPath: boardPath,
		},
		Shape: readinesspilot.ShapeFacts{
			ProblemPresent: spec.Problem != nil, OutcomePresent: spec.Outcome != nil,
			DeclaredObjectIDs: declaredIDs, OpenQuestionIDs: openQuestionIDs,
			ClaimedQuestions: claimedQuestions,
		},
		Provenance: provenance,
		Board:      board,
		Journey:    record,
		Conflict:   report,
		Fallbacks: readinesspilot.Fallbacks{
			Shape:   []string{"verdi", "journey", ref},
			Success: []string{"verdi", "journey", ref},
			Context: contextFallback,
			Review:  []string{"verdi", "journey", ref},
		},
		RequestDigest: readinessDigest(requestBytes),
		// spec/vocabulary-surfaces: readinesspilot stays pure and never
		// imports internal/model itself, so this loader resolves the spike
		// pseudo-class's display word once, here, through the store's
		// already-resolved operating model (ledger L-M13a(6)).
		SpikeWord:           cfg.Model.DisplayClass("spike"),
		ConflictUnavailable: conflictUnavailable,
	}
	snapshot, err := readinesspilot.Derive(input)
	if err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: deriving projection: %w", err)
	}
	// R-RR1-6: shape has no registered corrective inspection command. When
	// the caller supplied a board-href function, route every unresolved
	// shape concern to the existing selected-branch board instead of
	// exposing Derive's required CLI fallback outside this package; when it
	// did not, the CLI fallback vector stays (input.Target.BoardPath is
	// already "" either way, so deriveShape's own board-vs-CLI fallback
	// already produced it for every concern this loop would otherwise
	// touch).
	if opts.BoardHref != nil {
		for _, concerns := range [][]readinesspilot.Concern{snapshot.AllConcerns, snapshot.Attention} {
			for i := range concerns {
				if concerns[i].Area == readinesspilot.AreaShape && concerns[i].State != readinesspilot.StateProven {
					concerns[i].Destination = readinesspilot.Destination{BoardPath: input.Target.BoardPath, CLI: []string{}}
				}
			}
		}
	}
	if err := snapshot.Validate(); err != nil {
		return readinesspilot.Snapshot{}, fmt.Errorf("readinessload: loading readiness: routing shape destinations: %w", err)
	}
	return snapshot, nil
}
