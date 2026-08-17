# Policy Conflict Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Steps use
> checkbox (`- [ ]`) syntax for tracking. When Claude executes this plan, its
> main agent MUST remain on Fable and begin with `/fable-orchestration`; Sonnet
> workers implement tasks, Opus workers repair accepted defects, and the main
> agent adjudicates every finding against authority.

**Goal:** Build the one proof-grade policy conflict verdict used by context
inspection, specification acceptance, build start, build evidence intake, and
closure without weakening pre-constitution repositories.

**Architecture:** Add one `internal/policyconflict` application service over
sealed `internal/contextcompile` operands. Keep artifact decoding and authority
custody in `policyartifact`/`policyauthority`, deterministic proof in focused
scope/mechanical/semantic files, process and immutable-cache behavior behind
consumer-owned ports, and every CLI/lifecycle call behind the single
`VerdictProvider` result. Existing lifecycle verbs receive exact adapter,
grant, environment, and scope operands through one shared strict
context-request loader; they never invent defaults.

**Tech Stack:** Go, `internal/canonjson`, the strict `internal/artifact` decode
wall, `internal/gitx`, `internal/repositoryfacts`, `internal/policyauthority`,
`internal/governanceprincipal`, `internal/store`, `os/exec`, hermetic fixture
Git, and built-binary Go tests.

## Global Constraints

- Binding authority is
  `docs/superpowers/specs/2026-08-12-policy-conflict-gate-authority-design.md`,
  Context Integrity AC-3/DC-3–DC-8/DC-15/DC-17–DC-24/CO-1–CO-6, and
  invention-ledger SI-93–SI-109 as consolidated by the independently reviewed
  exact head carrying this plan.
- Do not edit frozen artifacts or `docs/design/specs/`; do not add a layout
  root, UI, MCP tool, receipt, sealed-execution behavior, forge network call, or
  generic policy language.
- The only public inspection grammar is
  `verdi context conflict --request <path|-> [--out <path>]`; lifecycle verbs
  add only `--context-request <path>` as fixed by SI-102.
- Pre-adoption lifecycle invocations without `--context-request` retain exact
  legacy output and behavior. After adoption, every named integration requires
  the flag and evaluates before its first effect.
- Strict decode rejects unknown/duplicate/null/trailing/noncanonical/invalid-
  UTF-8 input. Canonical JSON uses sorted keys, no HTML escaping, and one
  trailing newline.
- Three-valued honesty is `pass`, `blocked-violated`, or
  `blocked-unproven`; operational failures remain exit 2 and never produce a
  favorable report.
- No wall-clock or randomness enters a report. The only date is injected
  `evaluated_on` in UTC `YYYY-MM-DD`; stable IDs are canonical digests.
- No network in tests. This unit has no browser path and adds no Playwright
  test. Every CLI behavior is exercised through the built Go binary.
- TDD is mandatory: capture the named RED, implement the smallest code, run the
  named GREEN, review the task range, then commit with an imperative subject.
- Do not run Tasks 3–9 in parallel: they build one shared conflict type/proof
  graph. Task 1 and Task 2 are sequential authority foundations; Task 10 starts
  only after Task 9.

**Correction status:** Tasks 3 and 6 were implemented before SI-105–SI-107
closed four residual wire/proof gaps. Their landed runtime therefore still
uses claim-digest-only mechanical identity, requires a nonempty removal set on
every exemption resolution, drops kernel disclosure/role-pair evidence, and
classifies experimental authority as violated. The amended Task 3/6 text below
is the target contract, not a claim that those bytes already conform. Task 6A
is the owned reconciliation tranche and is a hard predecessor of Task 7. Its
independent exact-range review exposed two closure residuals: pair-row IDs
still keyed only by claim digest after SI-105 preserved equal bytes from
different policies, and report-wire validation stopped short of the already
specified digest and enclosing-row membership checks. SI-109 fixes the one
new row-identity choice; the validation correction adds no new semantics.

Task 7's first implementation was independently reviewed before Task 8. That
review found three proof-interface gaps fixed by SI-110–SI-112: unresolved
mechanical rows carried scope without their typed claims and omitted the
higher-order/disjoint residual; cache records asserted rather than recomputed
their complete path-key binding and did not bind raw bytes to the parsed
result; and semantic line identities used a necessary but under-specified
structural bypass around the artifact fragment grammar. The amended Task 3/7
interfaces below are the correction target and a hard predecessor of Task 8.

## File Map

| Responsibility | Files |
|---|---|
| disposition artifact and identity operands | `internal/policyartifact/{claim.go,disposition.go,disposition_test.go,kernel.go,seal.go,fixtures_test.go,testdata/**}` |
| committed authority/store/rendering | `internal/policyauthority/{store.go,resolve.go,*_test.go,testdata/**}`, `internal/store/{paths.go,paths_test.go}`, `internal/humanartifact/{kernel.go,kernel_test.go,policy.go,policy_test.go}`, `internal/designscaffold/templates/policy-disposition.md` |
| request/report/judgment wire | `internal/policyconflict/{schema.go,codec.go,validate.go,schema_test.go,testdata/**}` |
| sealed operands and candidate construction | `internal/contextcompile/{conflict.go,conflict_test.go,compiler.go,port.go}`, `internal/policyconflict/operand.go` |
| scope/mechanical proof | `internal/policyconflict/{scope.go,scope_test.go,mechanical.go,mechanical_test.go,exemption.go,exemption_test.go}` |
| semantic exchange/cache | `internal/policyconflict/{semantic.go,semantic_test.go,judge.go,judge_test.go,cache.go,cache_test.go}`, `internal/atomicfile/{atomicfile.go,atomicfile_test.go}` |
| authority/report/service | `internal/policyconflict/{authority.go,authority_test.go,report.go,report_test.go,service.go,service_test.go,errors.go}` |
| explicit and lifecycle CLI | `cmd/verdi/{context.go,context_test.go,context_e2e_test.go,conflictgate.go,conflictgate_test.go,gate.go,gate_test.go,buildstart.go,buildstart_test.go,close.go,close_test.go,closepreflight_test.go,closeprepare_test.go,dispatch.go,dispatch_test.go}` |

---

### Task 1: Define policy dispositions and principal-relation operands

**Files:**
- Create: `internal/policyartifact/disposition.go`
- Create: `internal/policyartifact/disposition_test.go`
- Modify: `internal/policyartifact/claim.go`
- Modify: `internal/policyartifact/claim_test.go`
- Modify: `internal/policyartifact/kernel.go`
- Modify: `internal/policyartifact/grammar.go`
- Modify: `internal/policyartifact/grammar_test.go`
- Modify: `internal/policyartifact/seal.go`
- Modify: `internal/policyartifact/fixtures_test.go`
- Create: `internal/policyartifact/testdata/store/dispositions/review-no-conflict.md`

**Interfaces:**
- Consumes: `Scope`, `Approval`, `TemplateRecord`, `Claim`, `canonjson.Digest`,
  and the existing strict frontmatter seam.
- Produces the exact artifact boundary below. The private decode document uses
  pointers for every mandatory key; `seal` remains unexported.

~~~go
const SchemaDisposition = "verdi.policy-disposition/v1"
const KindDisposition = "policy-disposition"
const DirDispositions = "dispositions"

type DispositionConclusion string
const (
    DispositionConflict DispositionConclusion = "conflict"
    DispositionNoConflict DispositionConclusion = "no-conflict"
)
type DispositionOrigin string
const (
    DispositionJudgeResult DispositionOrigin = "judge-result"
    DispositionHumanFallback DispositionOrigin = "human-fallback"
)
type SemanticClaimWitness struct {
    ID, Digest, Category, AuthorityDigest string
    Scope Scope
    Values []string
    Bound *int
}
type SemanticExemptionWitness struct { ID, Digest string }
type SemanticWitness struct {
    InputID, TargetDigest string
    Claims []SemanticClaimWitness
    Exemptions []SemanticExemptionWitness
}
type JudgmentProvenance struct { PrimaryDigest, ChallengerDigest string }
type Disposition struct {
    Schema, ID, Kind, Title string
    Owners []string
    Scope Scope
    Witness SemanticWitness
    Conclusion DispositionConclusion
    Origin DispositionOrigin
    Judgment *JudgmentProvenance
    CompensatingControls []string
    Approvals []Approval
    Expiry, ReviewCondition string
    Template *TemplateRecord
    Rationale string
    seal string
}
func DecodeDisposition([]byte) (*Disposition, error)
func (d *Disposition) Digest() (string, error)
~~~

- [ ] **Step 1: Write strict artifact and claim-operand tests**

Add `TestDecodeDisposition_StrictUnion`,
`TestDispositionDigest_SealedAndMutationSafe`, and
`TestClaimPrincipalRelation_ExactlyTwoCanonicalRoles`. The tables must prove a
valid judge-result record, a valid human fallback with controls and a real
date/review bound, then reject unknown/duplicate/null/missing keys, unknown
enums, unsorted/duplicate witnesses, claim text/raw judge output, invalid dates,
blank/multiline controls, missing approvals, and illegal origin-specific
fields. Principal claims require exactly two distinct kebab role IDs, sort them
lexically, and reject bounds or zero/one/three/duplicate roles.
Path-classification cases require exactly
`dispositions/<name>.md -> policy-disposition/<name>` and reject nesting,
wrong extensions, noncanonical names, and any sibling directory.

- [ ] **Step 2: Run the focused RED**

~~~bash
go test ./internal/policyartifact -run 'Test(DecodeDisposition|DispositionDigest|ClaimPrincipalRelation)' -count=1
~~~

Expected: build failure on undefined disposition symbols and failures on the
old empty-values principal rule.

- [ ] **Step 3: Implement the minimum strict artifact**

Use `artifact.DecodeStrict`, `time.Parse("2006-01-02", value)`, explicit enum
switches, unique sorted identity sets, author-ordered controls, and a deep
normalized copy for sealing/digesting. Replace only the two principal
operators' placeholder validation; leave every other claim operator unchanged.

- [ ] **Step 4: Run focused and package race GREEN**

~~~bash
go test -race ./internal/policyartifact -run 'Test(DecodeDisposition|DispositionDigest|ClaimPrincipalRelation)' -count=1
go test -race ./internal/policyartifact -count=1
~~~

Expected: both exit 0.

- [ ] **Step 5: Commit**

~~~bash
git add internal/policyartifact
git commit -m "Define policy disposition artifacts"
~~~

### Task 2: Load, digest, path, and render disposition authority

**Files:**
- Modify: `internal/store/paths.go`
- Modify: `internal/store/paths_test.go`
- Modify: `internal/policyauthority/store.go`
- Modify: `internal/policyauthority/resolve.go`
- Modify: `internal/policyauthority/{store_test.go,load_negative_test.go,resolve_test.go,resolve_negative_test.go}`
- Modify: `internal/policyauthority/testdata/golden-effective-policy.json`
- Modify: `internal/contextcompile/integration_test.go`
- Modify: `internal/contextcompile/testdata/golden/*.json`
- Modify: `internal/humanartifact/{kernel.go,kernel_test.go,policy.go,policy_test.go}`
- Create: `internal/designscaffold/templates/policy-disposition.md`

**Interfaces:**
- Consumes: Task 1 `Disposition`, `DirDispositions`, `DecodeDisposition`.
- Produces:

~~~go
// package store
func PolicyDispositionPath(root, name string) string
func PolicyDispositionRelPath(name string) string

// package policyauthority
type Store struct {
    Root string
    Constitution *policyartifact.Constitution
    Policies map[string]*policyartifact.Policy
    Overlays map[string]*policyartifact.Overlay
    Exemptions map[string]*policyartifact.Exemption
    Dispositions map[string]*policyartifact.Disposition
    Profiles map[string]*policyartifact.StoredProfile
    sealed bool
}

type EffectiveDisposition struct {
    ID, Digest string
    Disposition policyartifact.Disposition
}
// EffectivePolicy gains sorted Dispositions []EffectiveDisposition;
// its existing seal/digest includes them.

// package humanartifact
type DispositionScaffoldData struct {
    Name, Title string
    Owners []string
    InputID, TargetDigest, ClaimID, ClaimDigest string
    Category, AuthorityDigest string
    ApprovalRole, ApprovalPrincipal string
    Expiry, TemplateIdentity, TemplateDigest string
}
func RenderDisposition(Scaffold, DispositionScaffoldData) (string, error)
~~~

- [ ] **Step 1: Write path/load/digest/kernel/render RED tests**

Prove path/ID parity, disposition-directory and file symlink refusal, wrong
kind/name and duplicate ID refusal, mutation-after-load refusal, the
always-present sorted effective-policy `dispositions` field, effective
authority digest change both from the new empty field and when one disposition
byte changes, the resulting context authority-revision/golden ripple, the
complete kernel field inventory, canonical embedded/override scaffold
resolution, and exact render/decode round-trip. The scaffold is a minimal
judge-result skeleton and is not exposed by a creation verb.

- [ ] **Step 2: Run the focused RED**

~~~bash
go test ./internal/store ./internal/policyauthority ./internal/humanartifact ./internal/contextcompile -run 'Test.*(Disposition|KernelFields|Golden_BuildStoryMultiParent)' -count=1
~~~

Expected: undefined path/store/renderer symbols and kernel mismatch.

- [ ] **Step 3: Implement one custody path**

Add `dispositions` to the closed policy directory classifier and loader switch.
Key by `policy-disposition/<name>`, deep-copy into the effective view, sort by
ID, and bind digest into the existing effective-policy seal. Add every
Disposition field to `kernelFieldTable`; render through
`designscaffold.RenderValue`, decode through `DecodeDisposition`, and compare
the fixed kernel to input/defaults. Do not add UI, workbench code, or another
decoder.

- [ ] **Step 4: Update ratchets and run race GREEN**

~~~bash
go test -race ./internal/store ./internal/policyauthority ./internal/humanartifact ./internal/contextcompile -count=1
~~~

Expected: exit 0; the effective-policy golden gains an always-present sorted
`dispositions` field and changes digest even for the empty fixture. Every
context-compiler authority revision and committed golden that binds that digest
changes in the same commit; unrelated manifest fields and data-item bytes do
not drift.

- [ ] **Step 5: Commit**

~~~bash
git add internal/store internal/policyauthority internal/humanartifact internal/contextcompile internal/designscaffold/templates/policy-disposition.md
git commit -m "Load policy disposition authority"
~~~

### Task 3: Define strict conflict request, judgment, and report wires

**Files:**
- Create: `internal/policyconflict/{schema.go,codec.go,validate.go,schema_test.go}`
- Create: `internal/policyconflict/testdata/{request-accepted.json,request-candidate.json,judge-result.json,judgment.json,report.json}`

**Interfaces:**
- Consumes: `contextcompile.Request`, `policyartifact.Claim`,
  `policyartifact.Scope`, `execworkspace.GrantSet`, and Task 1 witness types.
- Produces the closed types/codecs below. `Report` contains exactly one
  `InputIdentity`, sorted `MechanicalEvaluation` and `SemanticEvaluation`
  slices, closed disclosures, verdict, and self-digest. Mechanical rows own
  typed claims, scope proof, domain, pre/post-exemption proof, embedded
  exemption resolutions, state, and reasons. Semantic rows own claim
  identities, unknown-source witnesses, primary/challenger exchanges, embedded
  disposition resolution, state, and reasons. There is no duplicate top-level
  exemption or disposition ledger.

~~~go
const RequestSchema = "verdi.policy-conflict-request/v1"
const ReportSchema = "verdi.policy-conflict-report/v1"
const JudgeResultSchema = "verdi.policy-conflict-judge-result/v1"
const JudgmentSchema = "verdi.policy-conflict-judgment/v1"

type TargetKind string
const (
    TargetAcceptedContext TargetKind = "accepted-context"
    TargetAcceptanceCandidate TargetKind = "acceptance-candidate"
)
type Verdict string
const (
    VerdictPass Verdict = "pass"
    VerdictBlockedViolated Verdict = "blocked-violated"
    VerdictBlockedUnproven Verdict = "blocked-unproven"
)
type ProofState string
const (
    ProofProven ProofState = "proven"
    ProofViolatedWithWitness ProofState = "violated-with-witness"
    ProofUnproven ProofState = "unproven"
)
type Recommendation string
const (
    RecommendationConflict Recommendation = "conflict"
    RecommendationNoConflict Recommendation = "no-conflict"
    RecommendationInconclusive Recommendation = "inconclusive"
)
type ReasonCode string
const (
    ReasonMechanicalSatisfiable ReasonCode = "mechanical-satisfiable"
    ReasonScopeDisjoint ReasonCode = "scope-disjoint"
    ReasonMechanicalConflict ReasonCode = "mechanical-conflict"
    ReasonScopeUnproven ReasonCode = "scope-unproven"
    ReasonHigherOrderScopeUnproven ReasonCode = "higher-order-scope-unproven"
    ReasonPrincipalRelationViolated ReasonCode = "principal-relation-violated"
    ReasonPrincipalRelationUnproven ReasonCode = "principal-relation-unproven"
    ReasonExemptionEffective ReasonCode = "exemption-effective"
    ReasonExemptionIneffective ReasonCode = "exemption-ineffective"
    ReasonJudgeUnavailable ReasonCode = "judge-unavailable"
    ReasonJudgeInconclusive ReasonCode = "judge-inconclusive"
    ReasonChallengerUnavailable ReasonCode = "challenger-unavailable"
    ReasonJudgmentDisagreement ReasonCode = "judgment-disagreement"
    ReasonDispositionRequired ReasonCode = "disposition-required"
    ReasonDispositionEffectiveNoConflict ReasonCode = "disposition-effective-no-conflict"
    ReasonDispositionEffectiveConflict ReasonCode = "disposition-effective-conflict"
    ReasonDispositionIneffective ReasonCode = "disposition-ineffective"
    ReasonProfileExperimental ReasonCode = "profile-experimental"
)
type DisclosureCode = contextcompile.DisclosureCode
const DisclosureSoloPrincipalCollapse contextcompile.DisclosureCode = "solo-principal-collapse"
type AcceptanceCandidate struct {
    Adapter contextcompile.AdapterRef
    Expected contextcompile.Expected
    Grants execworkspace.GrantSet
    Scope policyartifact.Scope
    Spec string
}
type Target struct {
    Kind TargetKind
    AcceptedContext *contextcompile.Request
    AcceptanceCandidate *AcceptanceCandidate
}
type Request struct { Schema string; Target Target }
type Result struct { Report Report; ReportBytes []byte }
type VerdictProvider interface {
    Evaluate(context.Context, Request) (Result, error)
}

type ScopeState string
const (
    ScopeOverlap ScopeState = "overlap"
    ScopeDisjoint ScopeState = "disjoint"
    ScopeUnknown ScopeState = "unknown"
)
type DimensionProof struct {
    Dimension string // phase | environment | path | ref
    State ScopeState
    Left, Right, Intersection, Witnesses []string
}
type ScopeProof struct { State ScopeState; Dimensions []DimensionProof }
type SolverState string
const (
    SolverSatisfiable SolverState = "satisfiable"
    SolverUnsatisfiable SolverState = "unsatisfiable"
    SolverUnproven SolverState = "unproven"
)
type SolverProof struct {
    State SolverState
    Domain string
    Values, Required, Forbidden []string
    Minimum, Maximum *int
    OpenDomain bool
    Witnesses []string
}
type TypedClaimRecord struct {
    PolicyID, PolicyDigest, ClaimDigest string
    Claim policyartifact.Claim
}
type ClaimWitness struct { ID, Digest, Category string }
type MechanicalClaimWitness struct { PolicyID, ClaimID, ClaimDigest string }
type JudgeFinding struct {
    Claims []ClaimWitness
    Categories []string
    Explanation string
}
type JudgeResult struct {
    Schema string
    Recommendation Recommendation
    Findings []JudgeFinding
}
type JudgeRole string
const (
    JudgePrimary JudgeRole = "primary"
    JudgeChallenger JudgeRole = "challenger"
)
type JudgmentExchange struct {
    Role JudgeRole
    Adapter contextcompile.AdapterRef
    Model, CommandDigest, PromptDigest, InputDigest string
    RawResult, RawDigest string
    Result JudgeResult
}
type Judgment struct {
    Schema, TreeHash, InputDigest string
    ProfileID, ProfileDigest, AuthorityDigest string
    Exchange JudgmentExchange
    Digest string
}
type AuthorityResolution struct {
    Match, Freshness, Scope, Bound, Authorization ProofState
}
type ExemptionResolution struct {
    ID, Digest string
    Resolution AuthorityResolution
    RemovedClaims []MechanicalClaimWitness
}
type DispositionResolution struct {
    ID, Digest string
    Conclusion policyartifact.DispositionConclusion
    Resolution AuthorityResolution
}
type MechanicalEvaluation struct {
    ID string
    Family policyartifact.Family
    Subject string
    Claims []TypedClaimRecord
    Scope ScopeProof
    Domain string
    Before SolverProof
    Exemptions []ExemptionResolution
    After SolverProof
    State ProofState
    Reasons []ReasonCode
}
type SemanticEvaluation struct {
    ID, InputID string
    Claims []policyartifact.SemanticClaimWitness
    UnknownMechanicals []UnknownMechanicalWitness
    Primary, Challenger *JudgmentExchange
    Dispositions []DispositionResolution
    State ProofState
    Reasons []ReasonCode
}
type AcceptedIdentity struct { ManifestDigest string }
type CandidateIdentity struct {
    Ref, Path, Branch, Head, Blob, ContentDigest string
    Scope policyartifact.Scope
    Adapter contextcompile.AdapterRef
    GrantDigest string
}
type TargetIdentity struct {
    Kind TargetKind
    Accepted *AcceptedIdentity
    Candidate *CandidateIdentity
}
type PolicyEntryIdentity struct { Kind, ID, Digest string }
type ProfileIdentity struct { ID, Class, Digest string }
type InputIdentity struct {
    Target TargetIdentity
    Repository repositoryfacts.Facts
    ConstitutionDigest, EffectivePolicyDigest string
    PolicyEntries []PolicyEntryIdentity
    Profile ProfileIdentity
    EvaluatedOn string
}
type Disclosure struct { Code DisclosureCode; Witnesses []string }
type Report struct {
    Schema string
    Input InputIdentity
    Mechanical []MechanicalEvaluation
    Semantic []SemanticEvaluation
    Disclosures []Disclosure
    Verdict Verdict
    Digest string
}
func DecodeRequest([]byte) (Request, error)
func EncodeRequest(Request) ([]byte, error)
func DecodeJudgeResult([]byte) (JudgeResult, error)
func EncodeJudgeResult(JudgeResult) ([]byte, error)
func DecodeJudgment([]byte) (Judgment, error)
func EncodeJudgment(Judgment) ([]byte, error)
func DecodeReport([]byte) (Report, error)
func EncodeReport(Report) ([]byte, error)
~~~

`TypedClaimRecord` and `MechanicalClaimWitness` sort and deduplicate by the
composite key `(policy_id, claim_id)`; their `claim_digest` is verified against
the canonical digest of the carried base claim. `removed_claims` is a
mandatory-present array. It is nonempty exactly when all five authority states
are proven and empty (`[]`) otherwise. A proven removal set contains only exact
current witnesses from its enclosing mechanical row. `ClaimWitness` remains
the separate semantic-prose vocabulary and is never used for mechanical
removal.

- [ ] **Step 1: Write strict-codec RED matrices**

For all four schemas cover canonical round-trip, unknown and duplicate fields,
absent/null/empty mandatory members, wrong schema, unknown enum, unsorted or
duplicate sets, trailing data, invalid UTF-8, and noncanonical bytes. Exhaust
both request union arms. Enforce recommendation/findings cardinality and at
least two distinct valid claim witnesses. Mutate every report nested enum,
digest, set, and order and require self-digest verification to fail.

- [ ] **Step 2: Run the focused RED**

~~~bash
go test ./internal/policyconflict -run 'Test(Decode|Encode).*(Request|JudgeResult|Judgment|Report)' -count=1
~~~

Expected: package build failure because the package/codecs do not exist.

- [ ] **Step 3: Implement strict canonical codecs**

Use private pointer-backed wire structs for presence, the shared strict JSON
decoder for unknown/duplicate/trailing checks, exact canonical re-encode byte
equality on decode, explicit enum switches, and deep digestless copies for
self-digests. Never use maps for semantic iteration order.

- [ ] **Step 4: Run focused and package race GREEN**

~~~bash
go test -race ./internal/policyconflict -run 'Test(Decode|Encode).*(Request|JudgeResult|Judgment|Report)' -count=1
go test -race ./internal/policyconflict -count=1
~~~

Expected: both exit 0 and all five golden files equal fresh encodes.

- [ ] **Step 5: Commit**

~~~bash
git add internal/policyconflict
git commit -m "Define policy conflict wire contracts"
~~~

### Task 4: Seal accepted and candidate conflict operands

**Files:**
- Create: `internal/contextcompile/conflict.go`
- Create: `internal/contextcompile/conflict_test.go`
- Modify: `internal/contextcompile/{compiler.go,port.go}`
- Create: `internal/policyconflict/operand.go`
- Create: `internal/policyconflict/operand_test.go`

**Interfaces:**
- Consumes: the existing compiler stages, `PolicyAuthority`,
  `EffectivePolicy`, repository facts, accepted spec/fragment/obligation/
  declared-context resolution, and Task 3 request union.
- Produces sealed, clone-on-read operands:

~~~go
// package contextcompile
type TypedClaim struct {
    PolicyID, PolicyDigest, ClaimDigest string
    Claim policyartifact.Claim
}
type ProseClaim struct {
    ID, Category, Text, TextDigest string
    SourceRef, SourcePath, SourceDigest string
    Scope policyartifact.Scope
    AuthorityDigest, Object, LineIdentity string
}
type ConflictSourceIdentity struct {
    Ref, Path, ContentDigest string
}
type SnapshotIdentity struct {
    TargetKind string // accepted-context | acceptance-candidate
    Repository repositoryfacts.Facts
    ManifestDigest, CandidateDigest string
    EffectivePolicyDigest, ConstitutionDigest string
    ProfileID, ProfileDigest string
    Adapter AdapterRef
    Phase Phase
    Scope policyartifact.Scope
    GrantDigest string
    Sources []ConflictSourceIdentity
}
type CandidateRequest struct {
    Adapter AdapterRef
    Expected Expected
    Grants execworkspace.GrantSet
    Scope policyartifact.Scope
    Spec string
}
type ConflictFacts struct {
    Actors []governanceprincipal.PrincipalResolution
}
type ConflictView struct {
    Snapshot SnapshotIdentity
    EffectivePolicy policyauthority.EffectivePolicy
    TypedClaims []TypedClaim
    ProseClaims []ProseClaim
    Exemptions []policyartifact.Exemption
    Profile governanceprincipal.Profile
    Actors []governanceprincipal.PrincipalResolution
}
type ConflictOperands struct {
    view ConflictView
    seal string
}
func (o *ConflictOperands) View() (ConflictView, error)
func Compiler.CompileConflict(context.Context, string, Request, ConflictFacts) (*ConflictOperands, error)
func ResolveConflictCandidate(context.Context, string, CandidateRequest, ConflictFacts) (*ConflictOperands, error)

// package policyconflict
func ResolveOperands(context.Context, contextcompile.Compiler, string, Request, contextcompile.ConflictFacts) (*contextcompile.ConflictOperands, error)
~~~

The two mutually exclusive identity digest fields are validated by target kind:
accepted requires `ManifestDigest` and empty `CandidateDigest`; candidate is the
reverse. `Sources` is unique and sorted by `(ref,path,content_digest)`. Do not
encode snapshot identity as an untyped map.

- [ ] **Step 1: Write operand sealing and fixture-Git RED tests**

Prove accepted construction reuses one loaded/resolved policy view; candidate
construction reads the exact HEAD-tree regular spec blob, verifies branch,
HEAD, path, class/ref, and declared ID, and never emits an accepted manifest.
Cover a proposed candidate differing from default-branch accepted bytes.
Mutation tests change every returned slice/nested scope/policy field and require
the next `View`/evaluation to fail or remain byte-identical. Cross-snapshot,
hand-built, digest-mismatched, symlink, missing, archive-only, and dirty working-
tree substitution cases fail operationally.

- [ ] **Step 2: Run the focused RED**

~~~bash
go test ./internal/contextcompile ./internal/policyconflict -run 'Test.*(ConflictOperands|ConflictCandidate|ResolveOperands)' -count=1
~~~

Expected: undefined conflict operand APIs.

- [ ] **Step 3: Implement by extracting existing compiler helpers**

Reuse the current compiler's repository/policy/applicability/fragment/decision/
obligation/prose paths. Add a candidate resolver only where accepted-baseline
resolution differs. Compute one private canonical seal over a deep copy and
recheck it before every view. Do not reload policy in `policyconflict`, marshal
manifest summaries back into claims, or call the accepted manifest encoder for
a candidate.

- [ ] **Step 4: Run focused and full package race GREEN**

~~~bash
go test -race ./internal/contextcompile ./internal/policyconflict -run 'Test.*(ConflictOperands|ConflictCandidate|ResolveOperands)' -count=1
go test -race ./internal/contextcompile ./internal/policyconflict -count=1
~~~

Expected: both exit 0; relative to Task 2's intentional authority-digest
ratchet, context manifest/data-item goldens are byte-identical.

- [ ] **Step 5: Commit**

~~~bash
git add internal/contextcompile internal/policyconflict
git commit -m "Seal policy conflict operands"
~~~

### Task 5: Prove four-dimensional scope relations

**Files:**
- Create: `internal/policyconflict/scope.go`
- Create: `internal/policyconflict/scope_test.go`

**Interfaces:**
- Consumes: `policyartifact.Scope` and the exact accepted/candidate artifact
  graph from Task 4.
- Produces:

~~~go
type RefRelationResolver interface {
    Relate(context.Context, string, string) (ScopeState, []string, error)
}
func CompareScopes(context.Context, policyartifact.Scope, policyartifact.Scope, RefRelationResolver) (ScopeProof, error)
func IntersectScopes(context.Context, []policyartifact.Scope, RefRelationResolver) (ScopeProof, error)
~~~

- [ ] **Step 1: Write exhaustive pair/N-way truth-table RED tests**

Cover universal dimensions; equal/intersecting/disjoint phase and environment;
exact/directory/ancestor/sibling/segment-boundary paths; equal whole/fragment,
feature-to-implementer, separate accepted roots, and missing/ambiguous ref
facts. Include the non-transitive witness A overlaps B, B overlaps C, A is
disjoint C, and N-way intersections that pairwise overlap but have empty or
unknown total intersection. Assert fixed dimension and witness ordering.

- [ ] **Step 2: Run the focused RED**

~~~bash
go test ./internal/policyconflict -run 'Test(CompareScopes|IntersectScopes)' -count=1
~~~

Expected: undefined scope proof APIs.

- [ ] **Step 3: Implement the product algebra**

Treat empty sets as universal, paths segment-wise, and refs only through the
provided graph resolver. Any proven-disjoint dimension yields disjoint; all
proven-overlap yields overlap; otherwise unknown. Compute N-way intersections
per dimension once—never by transitive grouping of pair results.

- [ ] **Step 4: Run package race GREEN**

~~~bash
go test -race ./internal/policyconflict -run 'Test(CompareScopes|IntersectScopes)' -count=1
go test -race ./internal/policyconflict -count=1
~~~

Expected: both exit 0.

- [ ] **Step 5: Commit**

~~~bash
git add internal/policyconflict/scope.go internal/policyconflict/scope_test.go
git commit -m "Prove policy scope relations"
~~~

### Task 6: Solve typed mechanical conflicts and exact exemptions

**Files:**
- Create: `internal/policyconflict/{mechanical.go,mechanical_test.go,exemption.go,exemption_test.go}`

**Interfaces:**
- Consumes: Task 4 typed claims/profile/principal facts, Task 5 scope proof,
  `governanceprincipal.Authorize`, and Task 3 mechanical row types.
- Produces:

~~~go
type MechanicalInput struct {
    Claims []contextcompile.TypedClaim
    Profile governanceprincipal.Profile
    Actors []governanceprincipal.PrincipalResolution
    Refs RefRelationResolver
}
type MechanicalResult struct {
    Evaluations []MechanicalEvaluation
    Disclosures []Disclosure
}
func EvaluateMechanical(context.Context, MechanicalInput) (MechanicalResult, error)
func ApplyEffectiveExemptions(MechanicalEvaluation, []ExemptionResolution) (MechanicalEvaluation, error)
~~~

The implementation has four internal solver functions—`solveDiscrete`,
`solveInterval`, `solvePrincipalRelation`, and `solvePathCapability`—returning
one common proof struct. There is no operator-pair dispatch table and no
generic SAT engine.

- [ ] **Step 1: Write operator/domain and multi-claim RED tables**

Cover all eleven operators. For discrete sets prove equals agreement,
allowed-set intersection, required/forbidden union, `not-equals(v)` as
membership exclusion, finite and open-domain witnesses. For intervals cover
multiple min/max and equality. For principal relations cover canonical reversed
role spellings, same+different contradiction, kernel proven/violated/unproven,
and no string comparison. For paths prove read/write union and that absent
execution grants are not conflicts. Reject mixed-domain groups and unregistered
identity transition/roles.

Add exact-scope N-claim, differently-scoped pair, disjoint, unknown, and
unresolved higher-order cases. Structural exemption-recompute tables receive
constructed resolutions, retain the original proof, remove only exact current
witnesses inside covered scope, rerun the same solver, and reject a resolution
whose match, freshness, scope, bound, or authorization state is not proven.
Task 8 owns how those authority states are derived.

- [ ] **Step 2: Run the focused RED**

~~~bash
go test ./internal/policyconflict -run 'Test(EvaluateMechanical|Solve|ApplyExemption)' -count=1
~~~

Expected: undefined mechanical evaluator/solvers.

- [ ] **Step 3: Implement four explicit solvers and post-exemption recompute**

Group by `(family, subject)` and for identity additionally by sorted role pair.
Solve the complete conjunction first, then exact-scope N-way groups and unique
differently-scoped pairs. Apply SI-107's satisfiable-component proof before
feeding the genuine unresolved higher-order scope remainder to an unproven
row. Preserve every typed claim under `(policy_id, claim_id)` and verify its
carried digest against its base claim. `ApplyEffectiveExemptions` accepts only
an all-proven typed resolution, performs exact scoped claim removal through
`MechanicalClaimWitness`, and calls the same solver again; rejected resolutions
carry an empty removal set. It never interprets dates/principals or edits a
proof result in place. Principal-relation evaluation uses SI-106's exact
kernel-finding attribution and returns kernel disclosures beside evaluations.

- [ ] **Step 4: Run focused and package race GREEN**

~~~bash
go test -race ./internal/policyconflict -run 'Test(EvaluateMechanical|Solve|ApplyExemption)' -count=1
go test -race ./internal/policyconflict -count=1
~~~

Expected: both exit 0 with deterministic row/witness order.

- [ ] **Step 5: Commit**

~~~bash
git add internal/policyconflict/mechanical.go internal/policyconflict/mechanical_test.go internal/policyconflict/exemption.go internal/policyconflict/exemption_test.go
git commit -m "Prove typed policy conflicts"
~~~

### Task 6A: Reconcile SI-105–SI-107 runtime wires and kernel evidence

This correction task records the deliberate divergence from the already
landed Task 3/6 runtime. It must complete before Task 7 begins.

**Files:**
- Modify: `internal/policyconflict/{schema.go,schema_test.go,codec.go,validate.go,mechanical.go,mechanical_test.go,exemption.go,exemption_test.go}`
- Modify: `internal/policyconflict/testdata/report.json`
- Modify: `internal/governanceprincipal/{authorize.go,authorize_test.go}`

**Interfaces:**
- Replace mechanical use of semantic `ClaimWitness` with
  `MechanicalClaimWitness { PolicyID, ClaimID, ClaimDigest string }`.
- Preserve and validate every `TypedClaimRecord` under composite
  `(policy_id, claim_id)` identity; equal claim bytes from two policies remain
  two records. Both typed records and removal witnesses sort and deduplicate by
  that composite key, and every digest is recomputed from the carried claim.
- Encode `removed_claims` as mandatory-present: `[]` unless all five authority
  states are proven, and at least one exact current row witness when they are.
- Add `Roles []string` to kernel `Finding`. Every distinctness finding carries
  the exact sorted two-role pair; findings from other rule families carry no
  pair. Add the validated exported query
  `HoldsRole(Profile, PrincipalClaim, string) (bool, error)` and retain one
  private already-validated inner predicate, so consumers do not duplicate
  role-mapping semantics.
- Make `EvaluateMechanical` return
  `MechanicalResult { Evaluations []MechanicalEvaluation; Disclosures []Disclosure }`.
  Only a kernel `solo-role-collapse` disclosure translates to report code
  `solo-principal-collapse`; each principal/role membership becomes the
  lossless witness token `<principal_id>:<role_id>` fixed by SI-108, tokens sort
  and deduplicate, duplicate translations collapse, and any unknown kernel
  disclosure is an operational error. Task 9 remains the sole report-hoisting
  location.
- Map advisory posture caused by an experimental profile to an unproven row
  with reason `profile-experimental`. It is not a mechanical violation and can
  never authorize an authoritative pass.
- Derive each step-2 pair-row component from the canonical digest of its
  composite `(policy_id, claim_id)` identity (SI-109), never from claim digest
  alone. Report validation recomputes every carried claim digest and requires
  every effective `removed_claims` witness to equal one exact current claim in
  its enclosing mechanical row; decoding a self-consistent but forged report
  fails operationally.

- [ ] **Step 1: Write the reconciliation RED matrix**

Cover same-bytes/different-policy preservation, mutated claim digest refusal,
composite ordering/duplicates, rejected `removed_claims: []`, proven empty
removal refusal, exact-current-row membership, exact distinctness role-pair
attribution in the presence of a second rule, exported role lookup parity,
kernel disclosure translation/deduplication/unknown-code refusal, and
experimental unproven classification. Pin the exact SI-108 pair-token grammar,
including malformed components and two principals holding overlapping roles.

- [ ] **Step 2: Run focused RED**

~~~bash
go test ./internal/governanceprincipal ./internal/policyconflict -run 'Test.*(MechanicalClaim|ExemptionResolution|DistinctnessRoles|HoldsRole|MechanicalDisclosure|ProfileExperimental)' -count=1
~~~

Expected: the new wire and kernel assertions fail against the pre-reconciliation
runtime.

- [ ] **Step 3: Implement the minimum reconciliation**

Change only the named wire, validation, kernel evidence, mechanical result,
and translation seams. Keep the four bounded domain solvers and SI-107
component proof unchanged. Do not add a generic SAT engine, operator-pair
dispatch, second role interpreter, or second report disclosure location.

- [ ] **Step 4: Run focused and package race GREEN**

~~~bash
go test -race ./internal/governanceprincipal ./internal/policyconflict -run 'Test.*(MechanicalClaim|ExemptionResolution|DistinctnessRoles|HoldsRole|MechanicalDisclosure|ProfileExperimental)' -count=1
go test -race ./internal/governanceprincipal ./internal/policyconflict -count=1
~~~

Expected: both exit 0 with deterministic composite ordering and exact
three-valued outcomes.

- [ ] **Step 5: Commit**

~~~bash
git add internal/governanceprincipal internal/policyconflict
git commit -m "Correct mechanical evidence transport"
~~~

- [ ] **Step 6: Close the independent-review residuals**

Add RED witnesses for two policies carrying byte-identical contradictory
claims producing distinct, stably ordered pair-row IDs; report decoding with a
re-signed but stale carried claim digest; and a re-signed effective removal
witness that is absent from, or digest-mismatched against, its enclosing row.
Implement only SI-109's composite-identity row component and the existing
wire checks above, refresh the canonical report fixture, rerun both Step 4
commands, and obtain the same independent reviewer's single closure check.

### Task 7: Validate semantic judgments and immutable cache reuse

**Files:**
- Create: `internal/policyconflict/{semantic.go,semantic_test.go,judge.go,judge_test.go,cache.go,cache_test.go}`
- Modify: `internal/atomicfile/{atomicfile.go,atomicfile_test.go}`
- Modify: `internal/store/{paths.go,paths_test.go}`

**Interfaces:**
- Consumes: Task 3 judgment codecs, Task 4 prose claims, Task 5 unknown scope
  witnesses, `store.LayoutVersion`, `store.TreeHash`, and `filelock`/writer-lock
  discipline.
- Produces:

~~~go
type SemanticInput struct {
    Claims []contextcompile.ProseClaim
    UnknownMechanicals []UnknownMechanicalWitness
    Exemptions []policyartifact.SemanticExemptionWitness
    Prompt []byte
}
type UnknownMechanicalWitness struct {
    ID string
    Claims []TypedClaimRecord
    Scope ScopeProof
}
type Judge interface {
    Judge(context.Context, []byte, []byte) (JudgmentExchange, error)
}
type JudgeRunner interface {
    Run(context.Context, []string, []byte) (stdout []byte, exitCode int, err error)
}
type JudgeAdapter struct {
    Role string
    Adapter contextcompile.AdapterRef
    Model string
    Argv []string
    Timeout time.Duration
    Root string
    Runner JudgeRunner
}
type ValidatedExchange struct {
    Exchange JudgmentExchange
    RecordDigest string
}
func BuildSemanticInput(contextcompile.ConflictView, []MechanicalEvaluation) (SemanticInput, error)
func ValidateJudgeResult(SemanticInput, JudgeResult) (ValidatedExchange, error)

// package store
func PolicyConflictCachePath(root, treeHash, inputDigest string) (string, error)
// package atomicfile
func CreateImmutable(path string, data []byte, perm os.FileMode) (created bool, existing []byte, err error)
~~~

`CreateImmutable` is the one narrow shared primitive needed here: same-directory
temp file, content fsync, no-clobber atomic publication, then byte-read of an
already-existing winner. It must not replace `atomicfile.Write` or broaden
writer ownership.

- [ ] **Step 1: Write prose/prompt/judge/cache RED tests**

Ratchet the complete normalized semantic input and fixed prompt bytes. Cover
all source categories, CRLF normalization, SI-112 exact object/line identity,
inherited scope, exclusion of repository data/raw model text, and deterministic
order. Prove that both scope-unknown and higher-order-unproven rows carry their
complete composite typed claims, that a typed-claim change moves the semantic
input digest, and that other mechanical rows are excluded.
Validate conflict/no-conflict/inconclusive cardinality, two distinct known claim
witnesses, unknown/missing/duplicate claims, category mismatch, invalid UTF-8,
noncanonical JSON, and model-identity substitution.

Use a hermetic fake process for start failure, nonzero exit, timeout,
cancellation, malformed output, and success. Cache tests cover hit/miss,
role/argv/model/prompt/input/profile/authority key changes, bare 64-hex filename
segments, strict inner full digests, symlink, corruption, mismatched key,
concurrent identical writers, different-winner collision, persistence failure,
and the rule that failed/invalid attempts are never cached.

- [ ] **Step 2: Run the focused RED**

~~~bash
go test ./internal/policyconflict ./internal/atomicfile ./internal/store -run 'Test(BuildSemanticInput|ValidateJudgeResult|JudgeAdapter|PolicyConflictCache|CreateImmutable)' -count=1
~~~

Expected: undefined semantic/judge/cache/immutable-create APIs.

- [ ] **Step 3: Implement one strict transport and immutable reuse adapter**

Build one complete input, invoke primary/challenger independently with identical
bytes, take model identity from adapter configuration only, and validate the
strict inner result rather than legacy align wrappers. Cache only a completed,
validated successful exchange; its persisted presence is the successful
process/validation state, so do not add single-value success enums or an
always-empty reasons field. Run the judge without the checkout writer lock.
The cache-aware entry validates the fixed prompt and complete semantic input.
Persist SI-111's profile/authority fields, recompute the complete key from the
record on every hit, and require the strict raw result to decode byte-identically
to the carried parsed result. Refuse a symlink at any managed cache-path
component rather than following a symlinked parent.
For cache directory creation and publication only, acquire the existing
nonblocking D3 `data/writer.lock`, call `CreateImmutable`, and release before
returning. A lock-holder refusal is operational. If another process published
the key before acquisition, strict-decode, canonical-reencode, verify the key,
and require byte identity. Do not cache blocked state, add a cache daemon, or
introduce another lock.

- [ ] **Step 4: Run focused and package race GREEN**

~~~bash
go test -race ./internal/policyconflict ./internal/atomicfile ./internal/store -run 'Test(BuildSemanticInput|ValidateJudgeResult|JudgeAdapter|PolicyConflictCache|CreateImmutable)' -count=1
go test -race ./internal/policyconflict ./internal/atomicfile ./internal/store -count=1
~~~

Expected: both exit 0, including concurrent-writer tests.

- [ ] **Step 5: Commit**

~~~bash
git add internal/policyconflict internal/atomicfile internal/store
git commit -m "Record semantic conflict judgments"
~~~

### Task 8: Resolve bounds, principals, and semantic dispositions

**Files:**
- Create: `internal/policyconflict/{authority.go,authority_test.go}`

**Interfaces:**
- Consumes: Task 1/2 dispositions, Task 3 semantic resolution rows, Task 4
  profile/actors/effective authority, Task 7 validated exchanges.
- Produces:

~~~go
type DateSource interface { TodayUTC(context.Context) (string, error) }
type AuthorityInput struct {
    EvaluatedOn string
    Profile governanceprincipal.Profile
    Actors []governanceprincipal.PrincipalResolution
    Exemptions []policyartifact.Exemption
    Dispositions []policyartifact.Disposition
}
func ResolveExemptionAuthority(AuthorityInput, MechanicalEvaluation) ([]ExemptionResolution, error)
func ResolveDispositionAuthority(AuthorityInput, SemanticInput, Primary, Challenger *ValidatedExchange) ([]DispositionResolution, error)
~~~

- [ ] **Step 1: Write complete authority-state RED tables**

Cover expiry before/on/after evaluation date, malformed or missing injected
date, review-condition-only unproven, exact/mismatched witness, target,
exemption, and judgment provenance, judge-result versus fallback eligibility,
fallback controls/bounds, absent/inconclusive/disagreeing judge results, conflict
and no-conflict conclusions, and cache absence after a recorded provenance
digest. Run kernel authorization tables for solo permitted collapse with
disclosure, team/high-assurance distinctness, proven/violated/unproven actors,
unknown roles/principals, and experimental profile refusal. A committed
principal string without a matching sealed resolution must remain unproven.

- [ ] **Step 2: Run the focused RED**

~~~bash
go test ./internal/policyconflict -run 'TestResolve(Exemption|Disposition)Authority' -count=1
~~~

Expected: undefined authority resolution APIs.

- [ ] **Step 3: Implement exact freshness and kernel delegation**

Compare complete canonical witness identities, not prose or partial IDs. Use
the injected date only. Delegate approval/separation meaning to
`governanceprincipal.Authorize`; never compare usernames or principal strings
inside `policyconflict`. Preserve stale/unauthorized/unproven records in the
resolution row while preventing them from changing the underlying conflict.

- [ ] **Step 4: Run focused and package race GREEN**

~~~bash
go test -race ./internal/policyconflict -run 'TestResolve(Exemption|Disposition)Authority' -count=1
go test -race ./internal/policyconflict -count=1
~~~

Expected: both exit 0.

- [ ] **Step 5: Commit**

~~~bash
git add internal/policyconflict/authority.go internal/policyconflict/authority_test.go
git commit -m "Authorize policy conflict departures"
~~~

### Task 9: Orchestrate the one verdict and canonical report

**Files:**
- Create: `internal/policyconflict/{errors.go,report.go,report_test.go,service.go,service_test.go}`

**Interfaces:**
- Consumes: every Task 3–8 interface.
- Produces:

~~~go
type ServiceDeps struct {
    Compiler contextcompile.Compiler
    Refs RefRelationResolver
    Primary Judge
    Challenger Judge
    Dates DateSource
    // managed callers may inject sealed actor facts; local construction does not
    Actors []governanceprincipal.PrincipalResolution
}
type Service struct { Root string; Deps ServiceDeps }
func NewService(root string, deps ServiceDeps) *Service
func (s *Service) Evaluate(context.Context, Request) (Result, error)
func ProbeAdoption(root string) (bool, error)
func IsOperational(error) bool
func IsNotAdopted(error) bool
~~~

- [ ] **Step 1: Write end-to-end service/report RED scenarios**

Use sealed fixture operands to cover: no relevant relation; satisfiable and
disjoint proof; uncovered mechanical conflict; exact effective exemption;
semantic no-conflict recommendation without disposition; effective
no-conflict disposition; effective conflict disposition; inconclusive/missing
judge; high-assurance missing/disagreeing challenger; mixed violated and
unproven rows; stale/unauthorized authority; experimental profile; and
constitution absence. Assert violated outranks unproven while both rows remain.

Ratchet byte-identical reports over repeated runs and permutations of claims,
policies, exemptions, dispositions, and actor facts. Assert one input identity,
no duplicate authority ledgers, no absolute/credential/secret/process-env data,
closed reason/disclosure codes, and report digest equality after decode.

- [ ] **Step 2: Run the focused RED**

~~~bash
go test ./internal/policyconflict -run 'Test(ServiceEvaluate|ReportDeterminism|ReportRedaction)' -count=1
~~~

Expected: undefined service/report derivation APIs.

- [ ] **Step 3: Implement the ordered ten-stage service**

Follow authority §3 order exactly: validate request; resolve one snapshot;
obtain/reverify sealed operands; run mechanical proof; build semantic input;
reuse/run judges; resolve exemptions/dispositions/principals; derive rows;
derive verdict; canonicalize/self-digest report. Return a completed `Result`
only for the three verdicts. Decode/authority/cache/I/O/process/cancellation
failures return typed operational errors and no report. Return a typed
not-adopted refusal before semantic work. `ProbeAdoption` delegates to
`policyauthority.Load`: only `ErrNotAdopted` returns `(false,nil)`; incomplete,
malformed, or symlinked policy stores remain operational errors.

- [ ] **Step 4: Run package and dependency race GREEN**

~~~bash
go test -race ./internal/policyconflict -run 'Test(ServiceEvaluate|ReportDeterminism|ReportRedaction)' -count=1
go test -race ./internal/policyconflict ./internal/contextcompile ./internal/policyauthority ./internal/policyartifact -count=1
~~~

Expected: both exit 0.

- [ ] **Step 5: Commit**

~~~bash
git add internal/policyconflict
git commit -m "Derive canonical conflict verdicts"
~~~

### Task 10: Add `verdi context conflict`

**Files:**
- Modify: `cmd/verdi/context.go`
- Modify: `cmd/verdi/context_test.go`
- Modify: `cmd/verdi/context_e2e_test.go`
- Modify: `cmd/verdi/{dispatch.go,dispatch_test.go}`

**Interfaces:**
- Consumes: Task 3 request codec, Task 9 `VerdictProvider`, existing context
  output-fence/redaction helpers, manifest `align.judge_cmd` and timeout.
- Produces exact grammar and exit mapping:

~~~text
verdi context conflict --request <path|-> [--out <path>]
pass -> 0; completed block/not-adopted -> 1; operational -> 2
~~~

- [ ] **Step 1: Write parser/unit and built-binary RED tests**

Cover file/stdin request, stdout versus explicit output, no stdout when writing,
missing/duplicate/unknown flags, positional arguments, same request/output,
`..`, symlink and every `.verdi`/managed-projection output fence, malformed and
noncanonical requests, absent constitution, pass, both block states, configured
fake judge, timeout/nonzero/malformed judge, cache hit, and checkout-root
redaction. Assert no actor flag, challenger flag, MCP registration, report file
inside store, or mutation beyond the named output/cache.

- [ ] **Step 2: Run the focused RED**

~~~bash
go test ./cmd/verdi -run 'Test.*ContextConflict' -count=1
~~~

Expected: `context` rejects the unknown `conflict` subcommand.

- [ ] **Step 3: Implement by reusing the existing context CLI boundary**

Add a subcommand dispatch and a dedicated flag extractor. Reuse canonical
output path/fence and diagnostic redaction; do not duplicate their logic.
Construct local service with primary judge transport only, no actor facts, and
no challenger. Write report bytes only after evaluation completes.

- [ ] **Step 4: Run focused race and built-binary GREEN**

~~~bash
go test -race ./cmd/verdi -run 'Test.*ContextConflict' -count=1
go test -race ./cmd/verdi -run 'TestContextConflictBuiltBinary' -count=1
~~~

Expected: both exit 0.

- [ ] **Step 5: Commit**

~~~bash
git add cmd/verdi/context.go cmd/verdi/context_test.go cmd/verdi/context_e2e_test.go cmd/verdi/dispatch.go cmd/verdi/dispatch_test.go
git commit -m "Expose context conflict reports"
~~~

### Task 11: Gate lifecycle effects through one explicit request adapter

**Files:**
- Create: `cmd/verdi/conflictgate.go`
- Create: `cmd/verdi/conflictgate_test.go`
- Modify: `cmd/verdi/{gate.go,gate_test.go,gate_decisionconflict.go,gate_decisionconflict_test.go}`
- Modify: `cmd/verdi/{buildstart.go,buildstart_test.go}`
- Modify: `cmd/verdi/{close.go,close_test.go,closepreflight_test.go,closeprepare_test.go}`

**Interfaces:**
- Consumes: Task 9 `VerdictProvider`, SI-102, existing target resolution and Git
  facts. Produces one shared lifecycle adapter:

~~~go
type conflictGateInput struct {
    RequestPath string
    Phase contextcompile.Phase
    Spec string
    Candidate bool
    Branch, Head string
}
type conflictGateResult struct {
    Adopted bool
    Result policyconflict.Result
}
func runConflictGate(context.Context, string, conflictGateInput, policyconflict.VerdictProvider) (conflictGateResult, error)
func conflictCondition(policyconflict.Result) gateCondition
func renderConflictSummary(io.Writer, policyconflict.Result)
~~~

The implementation may adjust signatures to existing dependency structs, but
there remains exactly one shared loader/adapter and one provider port. It must
not add separate `gate`/`build`/`close` request decoders.

- [ ] **Step 1: Write shared adapter grammar/validation RED tests**

Cover one `--context-request <path>` in any allowed flag position, duplicate,
missing value, `-`, symlink, unreadable, malformed, noncanonical, phase/spec
mismatch, optional expected mismatch, and exact computed expected replacement.
Cover pre-adoption without flag as `Adopted=false` without reading operands;
pre-adoption with flag as exit-2 misuse; adopted without flag as exit 2. Prove
design builds candidate phase design, build start/build gate accepted phase
build, and all close modes accepted phase review.

- [ ] **Step 2: Run shared-adapter RED**

~~~bash
go test ./cmd/verdi -run 'Test.*ConflictGate(Request|Adoption|Target)' -count=1
~~~

Expected: undefined adapter and existing verbs reject the new flag.

- [ ] **Step 3: Implement shared parsing and request construction**

Parse the optional path without reading it, call
`policyconflict.ProbeAdoption`, then apply SI-102. Decode only through
`contextcompile.DecodeRequest`; require exact
phase/spec; compare optional expected through computed repository facts; bind
computed branch/HEAD; construct candidate/accepted arm; call one provider.
Render only state, report digest, closed reasons, and witness IDs.

- [ ] **Step 4: Add design/build gate pre-effect tests**

For design `gate`, assert one numbered constitutional-conflict condition in the
spec-MR condition set. For build `gate`, assert the same result beside (not
inside) evidence fold. Inject pass, both block states, and operational error.
Snapshot Git index, branch, HEAD, worktree bytes, decision-conflict report, and
alignment/evidence files before each refusal; assert exact equality afterward.

- [ ] **Step 5: Run design/build gate GREEN**

~~~bash
go test -race ./cmd/verdi -run 'Test.*Gate.*(Conflict|PreEffect|Legacy)' -count=1
~~~

Expected: exit 0 and exact legacy stdout/stderr fixtures unchanged when no
constitution exists.

- [ ] **Step 6: Add build-start pre-effect tests and implement its call**

Place evaluation after accepted-state, cascade, and obligation-quality checks,
but before resolving the new branch HEAD, branch creation, or baseline
regeneration. Inject pass/block/operational results and assert branch list,
HEAD, index, and projection bytes unchanged on refusal.

~~~bash
go test -race ./cmd/verdi -run 'TestBuildStart.*(Conflict|PreEffect|Legacy)' -count=1
~~~

Expected: exit 0.

- [ ] **Step 7: Add close/preflight/prepare pre-effect tests and implement calls**

All three modes use the same review request/result. `--prepare` must evaluate
before refreshing a living alignment report. For every block/operational case
snapshot branch, HEAD, index, active/archive trees, alignment report, frozen
report, staged paths, commits, and fake publish calls; assert no change/call.

~~~bash
go test -race ./cmd/verdi -run 'TestClose.*(Conflict|PreEffect|Legacy)|TestClose(Preflight|Prepare).*Conflict' -count=1
~~~

Expected: exit 0.

- [ ] **Step 8: Run the combined built-binary lifecycle GREEN**

~~~bash
go test -race ./cmd/verdi -run 'Test.*ContextRequest.*BuiltBinary|Test.*Conflict.*BuiltBinary' -count=1
go test -race ./cmd/verdi -count=1
~~~

Expected: both exit 0; all legacy no-constitution snapshots remain unchanged.

- [ ] **Step 9: Commit**

~~~bash
git add cmd/verdi/conflictgate.go cmd/verdi/conflictgate_test.go cmd/verdi/gate.go cmd/verdi/gate_test.go cmd/verdi/gate_decisionconflict.go cmd/verdi/gate_decisionconflict_test.go cmd/verdi/buildstart.go cmd/verdi/buildstart_test.go cmd/verdi/close.go cmd/verdi/close_test.go cmd/verdi/closepreflight_test.go cmd/verdi/closeprepare_test.go
git commit -m "Gate lifecycle policy conflicts"
~~~

### Task 12: Audit the complete range and run merge gates

**Files:**
- Modify only files implicated by a reproduced defect; no speculative cleanup.

**Interfaces:**
- Consumes: Tasks 1–11 exact commit range.
- Produces: clean exact head, source-coverage evidence, and full gate evidence.

- [ ] **Step 1: Review the range against all fifteen authority verification items**

Run targeted searches and inspect every changed production path. Confirm strict
codecs; seal/cross-snapshot checks; pair/N-way scope truth tables; all four
mechanical domains; pre/post exemption proof; prose/prompt coverage; independent
judge/challenger; immutable cache; disposition freshness/auth; report
determinism; candidate versus accepted Git behavior; constitution compatibility;
pre-effect lifecycle ordering; built-binary 0/1/2 behavior; and no
UI/MCP/network/new-root additions. For each defect, add a failing test before a
minimal correction and commit separately.

- [ ] **Step 2: Scan the plan implementation for placeholders and forbidden expansion**

~~~bash
rg -n 'FIXME|XXX|panic\("stub|map\[string\]any|http\.|net/http|mcp|Playwright' internal/policyconflict internal/contextcompile internal/policyartifact internal/policyauthority cmd/verdi
git diff --check 16394012e4ab4110371e231faf5e6d495e70b4c1..HEAD
~~~

Expected: no placeholder or unapproved surface in the new range; diff check has
no output. Existing unrelated matches must be named and excluded by exact path.

- [ ] **Step 3: Run focused race gates**

~~~bash
go test -race ./internal/policyartifact ./internal/policyauthority ./internal/humanartifact ./internal/contextcompile ./internal/policyconflict ./internal/atomicfile ./internal/store -count=1
go test -race ./cmd/verdi -run 'Test.*(ContextConflict|ConflictGate|BuildStart.*Conflict|Close.*Conflict)' -count=1
~~~

Expected: both exit 0.

- [ ] **Step 4: Run repository gates foreground and unpiped**

~~~bash
make fixture
make spec-align
go test -race ./...
make verify
~~~

Expected: every command exits 0; `make verify` ends in `verify OK`. If sandbox
socket denial occurs, rerun the identical command unsandboxed and distinguish it
from a product failure. Do not pipe a gate through `tail` or mask its exit code.

- [ ] **Step 5: Capture exact-head hygiene and commit any final test-only ratchet**

~~~bash
git diff --check
git status --short
git log --oneline 16394012e4ab4110371e231faf5e6d495e70b4c1..HEAD
~~~

Expected: diff check/status have no output; log contains small imperative task
commits. If no final correction exists, do not create an empty ceremony commit.

- [ ] **Step 6: Request one independent exact-head review**

Prepare one immutable review package containing base/head, status, changed-file
inventory, consolidated diff, binding authority, and gate evidence. The
reviewer is read-only. The main Fable agent adjudicates every finding; accepted
runtime defects go to one Opus correction pass, followed by one closure check.
No automatic third round.

## Plan Self-Review Record

- **Spec coverage:** Tasks 1–2 implement disposition/storage/kernel and the
  principal operand correction; Tasks 3–4 implement strict wire and sealed
  target operands; Tasks 5–6 implement scope and mechanical proof/exemptions;
  Tasks 7–8 implement semantic judgment/cache and authenticated bounds;
  Task 9 implements report/verdict; Tasks 10–11 implement explicit and all
  lifecycle consumers; Task 12 covers all authority §13 verification items.
  No binding section is uncovered.
- **Placeholder scan:** Clear. Every step names exact files, symbols, test
  cases, commands, expected failures, and expected passing evidence; every
  referenced type is defined by an earlier task.
- **Type consistency:** `Request`, `Result`, `VerdictProvider`,
  `ConflictOperands`, `ScopeProof`, `MechanicalEvaluation`, `SemanticInput`,
  `ValidatedExchange`, and authority-resolution names flow in task order and
  are not renamed downstream.
- **YAGNI/overengineering check:** No generic SAT engine, policy DSL, cache
  service, second lock, request-template artifact, manifest default block,
  authoring CLI, UI/MCP surface, local identity reader, or generalized
  condition resolver is planned. The only shared primitives added are required
  by at least two actual consumers or by the no-clobber cache guarantee.
