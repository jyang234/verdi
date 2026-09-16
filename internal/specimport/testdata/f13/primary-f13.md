### Task F13: Implement the bounded gatekeeper state machine

**Files:**
- Create: `internal/gatekeeper/state.go`
- Create: `internal/gatekeeper/event.go`
- Create: `internal/gatekeeper/transition.go`
- Create: `internal/gatekeeper/transition_test.go`
- Create: `internal/gatekeeper/review.go`
- Create: `internal/gatekeeper/review_test.go`
- Create: `internal/gatekeeper/invalidation_test.go`
- Modify: `internal/board/reduce.go`

**Interfaces:**
- Consumes: committed candidate, align, review, adjudication, receipt, and G2 facts.
- Produces: pure next-state decisions and required effects.

- [ ] **Step 1: Define the closed state catalog**

```go
const (
	Planned State = "Planned"
	Ready State = "Ready"
	BuildStart State = "BuildStart"
	Implementing State = "Implementing"
	Aligning State = "Aligning"
	AlignDisposition State = "AlignDisposition"
	ReviewR0 State = "ReviewR0"
	AuthorAdjudication State = "AuthorAdjudication"
	CorrectionR1 State = "CorrectionR1"
	ClosureCheckR2 State = "ClosureCheckR2"
	Countersigning State = "Countersigning"
	MergeReady State = "MergeReady"
	Landing State = "Landing"
	ClosePrepared State = "ClosePrepared"
	ClosurePublished State = "ClosurePublished"
	AwaitingUAT State = "AwaitingUAT"
	Done State = "Done"
	Blocked State = "Blocked"
	NeedsDecision State = "NeedsDecision"
)
```

- [ ] **Step 2: Write model tests and observe RED**

```bash
go test ./internal/gatekeeper ./internal/board -run 'Test(Transition|ReviewBound|Invalidation)'
```

Enumerate every allowed edge and representative forbidden edges. Prove R0
cannot repeat, R1 count cannot exceed one, R1 candidate re-enters Aligning,
R2 cannot produce another automatic correction, any tree change invalidates
bound results, all stories Done moves the feature to AwaitingUAT, two
consecutive operational exits from Align route only that flight to G2, provider
summaries never satisfy a gate, and there is no `Failed` state.

- [ ] **Step 3: Implement review finding validation**

A blocking finding requires nonempty binding-authority cite, reachable-state
witness, concrete incorrect result, and threat-model fit. The author lane
adjudicates each finding. Conflicting blocking findings route to G2.

- [ ] **Step 4: Run gates and commit**

```bash
go test -race ./internal/gatekeeper ./internal/board
make verify
git add internal/gatekeeper internal/board
git commit -m "Enforce bounded flight review"
```

