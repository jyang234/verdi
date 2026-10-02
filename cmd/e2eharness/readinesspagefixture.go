package main

// readinessPageFixture (spec/readiness-page-v2; lane F4-data G5): the
// readiness page's own isolated fixtures, served like the all-proven
// cockpit (readinessallproven.go) — a SEPARATE, in-process workbench
// instance over the REAL workbench.NewHandlerWith wiring, its loader
// answering five fixed snapshots by ref through the production
// Deps.ReadinessLoader seam. Each snapshot is readinesspilot.Derive's own
// output over a fixed input, so its guidance, objects, and human-review
// rows are the derivation's, never fixture text:
//
//   - spec/readiness-page-step-2: the current step is Define success (a
//     violated current success blocker, and a criterion no stub covers);
//   - spec/readiness-page-step-3: the current step is Check constraints (a
//     blocked-unproven policy-conflict verdict);
//   - spec/readiness-page-step-4: the current step is Get approval (a
//     violated current review blocker);
//   - spec/readiness-page-solo-author: a solo profile whose only principal
//     is the author, so the broader-role duty (countersign on close) stays
//     visible, labeled human review, and unsatisfied (SI-339 (6));
//   - spec/readiness-page-three-states: a proven, a violated-with-witness,
//     and a disclosed-unproven item (ac-3).
//
// The inputs read two committed fixtures through the module root — the
// journey canonical record and the policy-conflict report the
// readinesspilot package's own tests derive over — and adjust them as
// that package's tests do. Loopback only; started lazily on the control
// server's GET /readiness-page-fixture and reused thereafter, so a spec
// that warms it in its own beforeAll passes alone (BL-98). No default
// spec: every page names its fixture with ?spec=<name>. Test-only.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/workbench"
)

// The fixture specs, each named on the fixture's /readiness?spec=<name>.
const (
	readinessPageStep2       = "readiness-page-step-2"
	readinessPageStep3       = "readiness-page-step-3"
	readinessPageStep4       = "readiness-page-step-4"
	readinessPageSoloAuthor  = "readiness-page-solo-author"
	readinessPageThreeStates = "readiness-page-three-states"
)

// readinessPageLoader answers each fixture snapshot by its ref; any other
// ref is a loader error, which the route discloses as a 503.
type readinessPageLoader struct {
	snaps map[string]readinesspilot.Snapshot
}

func (l readinessPageLoader) Load(_ context.Context, ref string) (readinesspilot.Snapshot, error) {
	snap, ok := l.snaps[ref]
	if !ok {
		return readinesspilot.Snapshot{}, fmt.Errorf("readiness-page fixture: no fixture snapshot for %s", ref)
	}
	return snap, nil
}

// readinessPageFixture lazily starts its isolated server and remembers its
// bound URL — the same start-once cache shape as the all-proven fixture.
type readinessPageFixture struct {
	moduleRoot string

	mu  sync.Mutex
	url string
}

func newReadinessPageFixture(moduleRoot string) *readinessPageFixture {
	return &readinessPageFixture{moduleRoot: moduleRoot}
}

// handler answers GET with the fixture's base URL as a plain-text body,
// starting the isolated server on the first call.
func (f *readinessPageFixture) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	url, err := f.ensureStarted(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(url))
}

// ensureStarted derives the fixture snapshots, binds a loopback listener,
// and serves the real workbench handler with the snapshots behind
// Deps.ReadinessLoader — exactly the production seam. ctx bounds the bind
// only; the server lives until the harness exits. A failed start caches
// nothing, so the next call retries.
func (f *readinessPageFixture) ensureStarted(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}
	snaps, err := readinessPageSnapshots(f.moduleRoot)
	if err != nil {
		return "", fmt.Errorf("readiness-page fixture: %w", err)
	}
	// An empty scratch root: the fixture serves only /readiness and its
	// assets; the root exists so the real handler wiring has a store path.
	root, err := os.MkdirTemp("", "verdi-e2e-readiness-page-*")
	if err != nil {
		return "", fmt.Errorf("readiness-page fixture: %w", err)
	}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("readiness-page fixture: %w", err)
	}
	srv := &http.Server{Handler: workbench.NewHandlerWith(root, workbench.Deps{ReadinessLoader: readinessPageLoader{snaps: snaps}})}
	go func() { _ = srv.Serve(ln) }()
	f.url = "http://" + ln.Addr().String() + "/"
	return f.url, nil
}

// readinessPageSnapshots derives every fixture snapshot, keyed by ref.
func readinessPageSnapshots(moduleRoot string) (map[string]readinesspilot.Snapshot, error) {
	builds := []func(readinessPageSources) (readinesspilot.Input, error){
		readinessPageStep2Input,
		readinessPageStep3Input,
		readinessPageStep4Input,
		readinessPageSoloAuthorInput,
		readinessPageThreeStatesInput,
	}
	src, err := loadReadinessPageSources(moduleRoot)
	if err != nil {
		return nil, err
	}
	snaps := make(map[string]readinesspilot.Snapshot, len(builds))
	for _, build := range builds {
		in, err := build(src)
		if err != nil {
			return nil, err
		}
		snap, err := readinesspilot.Derive(in)
		if err != nil {
			return nil, fmt.Errorf("deriving %s: %w", in.Target.Ref, err)
		}
		snaps[snap.TargetRef] = snap
	}
	return snaps, nil
}

// readinessPageSources are the two committed fixtures every input starts
// from: the journey canonical record and the policy-conflict report.
type readinessPageSources struct {
	record journey.Record
	report policyconflict.Report
}

func loadReadinessPageSources(moduleRoot string) (readinessPageSources, error) {
	data, err := os.ReadFile(filepath.Join(moduleRoot, "internal", "journey", "testdata", "canonical-record.json"))
	if err != nil {
		return readinessPageSources{}, fmt.Errorf("reading the journey fixture: %w", err)
	}
	record, err := journey.Decode(data)
	if err != nil {
		return readinessPageSources{}, fmt.Errorf("decoding the journey fixture: %w", err)
	}
	data, err = os.ReadFile(filepath.Join(moduleRoot, "internal", "policyconflict", "testdata", "report.json"))
	if err != nil {
		return readinessPageSources{}, fmt.Errorf("reading the policy-conflict fixture: %w", err)
	}
	report, err := policyconflict.DecodeReport(data)
	if err != nil {
		return readinessPageSources{}, fmt.Errorf("decoding the policy-conflict fixture: %w", err)
	}
	return readinessPageSources{record: record, report: report}, nil
}

// passReport is the fixture report with a passing verdict: every
// mechanical row proven and no semantic row, as readinesspilot's own tests
// build it.
func (s readinessPageSources) passReport() (policyconflict.Report, error) {
	report := s.report
	report.Digest = ""
	report.Semantic = []policyconflict.SemanticEvaluation{}
	report.Verdict = policyconflict.VerdictPass
	if err := report.Validate(); err != nil {
		return policyconflict.Report{}, fmt.Errorf("the passing policy-conflict report: %w", err)
	}
	return report, nil
}

// unprovenReport is the fixture report as committed: a blocked-unproven
// verdict over one proven mechanical row and one unproven semantic row.
func (s readinessPageSources) unprovenReport() (policyconflict.Report, error) {
	report := s.report
	report.Digest = ""
	if err := report.Validate(); err != nil {
		return policyconflict.Report{}, fmt.Errorf("the blocked-unproven policy-conflict report: %w", err)
	}
	return report, nil
}

// readinessPageInput is an all-proven input for spec/<name>: problem and
// outcome present, criteria declared and covered, provenance, mutation,
// and board proven, a passing policy-conflict report, and a journey with
// a known lifecycle, an adopted profile, a safe action, and no blocker.
// Each fixture then raises exactly the concerns it is about.
func readinessPageInput(src readinessPageSources, name, title, class string, criteria []string) (readinesspilot.Input, error) {
	ref := "spec/" + name
	branch := "design/" + name
	sum := sha256.Sum256([]byte(name))
	head := hex.EncodeToString(sum[:20])

	record := src.record
	record.Digest = ""
	record.Target = journey.Target{Ref: ref, Class: class, Path: ".verdi/specs/active/" + name + "/spec.md"}
	record.Lifecycle.Class = class
	record.Lifecycle.State = "proposed"
	record.Lifecycle.Relation = "new"
	record.Lifecycle.Posture = "advisory"
	record.Lifecycle.AcceptedBaseline = nil
	record.Lifecycle.ActiveBranch = journey.StringFact{Known: true, Value: branch}
	record.Lifecycle.Disclosures = []string{}
	record.Blockers.Current = []journey.Blocker{}
	record.Blockers.Eventual = journey.EventualBlockers{Derived: true, Items: []journey.Blocker{}, Disclosures: []string{}, Unavailable: []string{}}
	record.Principals.Required = []journey.RequiredRole{}
	record.Principals.Disclosures = []string{}
	record.Actions.NeededFacts = []string{}

	report, err := src.passReport()
	if err != nil {
		return readinesspilot.Input{}, err
	}
	declared := append([]string{"co-1"}, criteria...)
	empty := sha256.Sum256(nil)
	return readinesspilot.Input{
		Target: readinesspilot.TargetFacts{
			Ref: ref, Title: title, Class: class, Branch: branch, Head: head,
			BoardPath: workbench.BranchBoardHref(branch, name),
		},
		Shape: readinesspilot.ShapeFacts{
			ProblemPresent: true, OutcomePresent: true,
			DeclaredObjectIDs: declared, OpenQuestionIDs: []string{}, ClaimedQuestions: []readinesspilot.ClaimedQuestion{},
		},
		Success: readinesspilot.SuccessFacts{CriterionIDs: append([]string{}, criteria...), UncoveredCriteria: []string{}},
		Provenance: readinesspilot.ProvenanceFacts{
			ChainState: readinesspilot.StateProven, ChainWitnesses: []string{"design-provenance chain classified"},
			// vocab:identity — predecessor store-path identity, not lifecycle display prose
			MutationState: readinesspilot.StateProven, MutationWitnesses: []string{"no draft mutation residue"},
		},
		Board: readinesspilot.BoardFacts{
			State: readinesspilot.StateProven, OpenItems: []readinesspilot.BoardItem{}, Witnesses: []string{"scratch board enumerated"},
		},
		Journey:  record,
		Conflict: report,
		Fallbacks: readinesspilot.Fallbacks{
			Shape:   []string{"verdi", "journey", ref},
			Success: []string{"verdi", "journey", ref},
			Context: []string{"verdi", "context", "conflict", "--request", "readiness-request.json"},
			Review:  []string{"verdi", "journey", ref},
		},
		SpikeWord:     model.Canonical().DisplayClass("spike"),
		RequestDigest: "sha256:" + hex.EncodeToString(empty[:]),
	}, nil
}

// fixtureBlocker builds one journey blocker carrying the canonical
// record's own owner, with its reason's fixed class. clearing and witness
// are formats over the blocked transition (journey's own clearing
// conditions name it the same way), so a transition id is never prose.
func fixtureBlocker(src readinessPageSources, id string, reason journey.ReasonCode, transition, clearing, witness string) (journey.Blocker, error) {
	class, err := reason.Class()
	if err != nil {
		return journey.Blocker{}, err
	}
	if strings.Contains(clearing, "%s") {
		clearing = fmt.Sprintf(clearing, transition)
	}
	return journey.Blocker{
		ID: id, Reason: reason, Class: class, Witnesses: []string{fmt.Sprintf(witness, transition)},
		Owner: src.record.Blockers.Current[0].Owner, ClearingCondition: clearing, Transition: transition,
	}, nil
}

// readinessPageStep2Input: Define success is the current step — the
// canonical record's own current obligation-quality blocker for ac-2, and
// ac-3 no stub covers — while a context verdict and an eventual review
// blocker wait in later steps.
func readinessPageStep2Input(src readinessPageSources) (readinesspilot.Input, error) {
	in, err := readinessPageInput(src, readinessPageStep2, "Decline notice refresh", "feature", []string{"ac-1", "ac-2", "ac-3"})
	if err != nil {
		return in, err
	}
	in.Success.UncoveredCriteria = []string{"ac-3"}
	in.Journey.Blockers.Current = []journey.Blocker{src.record.Blockers.Current[1]}
	in.Journey.Blockers.Eventual.Items = append([]journey.Blocker{}, src.record.Blockers.Eventual.Items...)
	in.Conflict = policyconflict.Report{}
	in.ConflictUnavailable = "no context request supplied for this derivation"
	in.Fallbacks.Context = []string{"verdi", "context", "conflict", "--request", "<path>"}
	return in, in.Journey.Validate()
}

// readinessPageStep3Input: Check constraints is the current step — the
// committed blocked-unproven report — while an eventual governance
// blocker and an unresolved principal role wait in Get approval.
func readinessPageStep3Input(src readinessPageSources) (readinesspilot.Input, error) {
	in, err := readinessPageInput(src, readinessPageStep3, "Decline audit trail", "story", []string{"ac-1"})
	if err != nil {
		return in, err
	}
	report, err := src.unprovenReport()
	if err != nil {
		return in, err
	}
	in.Conflict = report
	resolution, err := fixtureBlocker(src, "principal-resolution-unproven/close", journey.ReasonPrincipalResolutionUnproven, "close",
		"the required principals resolve as authenticated", "principal resolution for %s is unproven")
	if err != nil {
		return in, err
	}
	in.Journey.Blockers.Eventual.Items = []journey.Blocker{resolution}
	in.Journey.Principals.Required = []journey.RequiredRole{{Transition: "close", Obligation: "attestation/countersign", Count: 1, Resolution: "unproven"}}
	in.Journey.Principals.Disclosures = []string{"authenticated principal resolution and profile-contributed requirements remain unproven"}
	return in, in.Journey.Validate()
}

// readinessPageStep4Input: Get approval is the current step — the
// canonical record's own current forge-facts blocker — with every earlier
// step proven.
func readinessPageStep4Input(src readinessPageSources) (readinesspilot.Input, error) {
	in, err := readinessPageInput(src, readinessPageStep4, "Reversal propagation", "story", []string{"ac-1", "ac-2"})
	if err != nil {
		return in, err
	}
	in.Journey.Blockers.Current = []journey.Blocker{src.record.Blockers.Current[0]}
	in.Journey.Principals.Required = []journey.RequiredRole{{Transition: "close", Obligation: "attestation/countersign", Count: 1, Resolution: "unproven"}}
	in.Journey.Principals.Disclosures = []string{"authenticated principal resolution and profile-contributed requirements remain unproven"}
	return in, in.Journey.Validate()
}

// readinessPageSoloAuthorInput: a solo profile whose only principal is
// the author. The kernel discloses the author filling more than one role
// (the policy-conflict report's solo-principal-collapse disclosure); the
// author's own vouch on merge is judgmental work; and the broader-role
// duty — a countersign on close — together with the unresolved principal
// for merge, stays visible as human review and unsatisfied.
func readinessPageSoloAuthorInput(src readinessPageSources) (readinesspilot.Input, error) {
	in, err := readinessPageInput(src, readinessPageSoloAuthor, "Solo maintainer release", "story", []string{"ac-1"})
	if err != nil {
		return in, err
	}
	in.Conflict.Disclosures = []policyconflict.Disclosure{{
		Code:      policyconflict.DisclosureSoloPrincipalCollapse,
		Witnesses: []string{"one principal fills multiple roles under the solo profile"},
	}}
	if err := in.Conflict.Validate(); err != nil {
		return in, fmt.Errorf("the solo-author policy-conflict report: %w", err)
	}
	resolution, err := fixtureBlocker(src, "principal-resolution-unproven/merge", journey.ReasonPrincipalResolutionUnproven, "merge",
		"the required principals resolve as authenticated", "the author is the only principal resolved for %s")
	if err != nil {
		return in, err
	}
	vouch, err := fixtureBlocker(src, "obligation-author-vouch-unproven/merge/attestation/author-vouch", journey.ReasonObligationAuthorVouchUnproven, "merge",
		"obligation attestation/author-vouch is proven for transition %s", "the author's vouch for %s is absent")
	if err != nil {
		return in, err
	}
	countersign, err := fixtureBlocker(src, "obligation-countersign-unproven/close/attestation/countersign", journey.ReasonObligationCountersignUnproven, "close",
		"obligation attestation/countersign is proven for transition %s", "no principal other than the author can countersign %s")
	if err != nil {
		return in, err
	}
	in.Journey.Blockers.Current = []journey.Blocker{resolution}
	in.Journey.Blockers.Eventual.Items = []journey.Blocker{vouch, countersign}
	in.Journey.Principals.SelectedProfileID = "solo-default"
	in.Journey.Principals.Required = []journey.RequiredRole{
		{Transition: "close", Obligation: "attestation/countersign", Count: 1, Resolution: "unproven"},
		{Transition: "merge", Obligation: "attestation/author-vouch", Count: 1, Resolution: "unproven"},
	}
	in.Journey.Principals.Disclosures = []string{"authenticated principal resolution and profile-contributed requirements remain unproven"}
	return in, in.Journey.Validate()
}

// readinessPageThreeStatesInput: Define the work is the current step, and
// it holds one of each state — the problem statement proven, the outcome
// statement violated with a witness, and an unclaimed open question
// disclosed as unproven.
func readinessPageThreeStatesInput(src readinessPageSources) (readinesspilot.Input, error) {
	in, err := readinessPageInput(src, readinessPageThreeStates, "Decline reason wording", "story", []string{"ac-1"})
	if err != nil {
		return in, err
	}
	in.Shape.OutcomePresent = false
	in.Shape.DeclaredObjectIDs = append(in.Shape.DeclaredObjectIDs, "oq-1")
	in.Shape.OpenQuestionIDs = []string{"oq-1"}
	return in, in.Journey.Validate()
}
