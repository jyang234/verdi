// Package contextresolve defines the read-only context-item query's strict
// wire contracts (VATC F12 controller-owner bridge correction §2.2): the
// request an authority owner supplies and the closed proven/non-proven result
// it receives back. This file declares only the shapes; codec.go owns
// byte<->value conversion and every grammar rule, and service.go owns the
// replay that decides which arm of the result union is true.
//
// The package answers one question — "which canonical data item does this ref
// name, in the state the caller says it is in?" — and answers it only after
// recompiling that state itself. It is a fact source, never an approval: it
// resolves no authority, persists nothing, and reaches no port beyond the
// existing read-only compiler.
package contextresolve

import (
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
)

// Schema identifiers for the three wire documents this package owns
// (correction §2.2). The witness is its own schema because §2.2 sorts
// witnesses "by raw byte order of each standalone canonical witness
// encoding" — a rule about a document, not about a bare string.
const (
	RequestSchema = "verdi.context-resolve-request/v1"
	ResultSchema  = "verdi.context-resolve-result/v1"
	WitnessSchema = "verdi.context-resolve-witness/v1"
)

// State is the closed two-valued result discriminator correction §2.2 fixes.
//
// It is deliberately not internal/contextcompile's three-valued Resolution.
// That vocabulary distinguishes a violated proof from an unattempted one,
// which is the right shape for a compile disclosure and the wrong shape here:
// an absent ref is something this query PROVED about the state it replayed,
// not something it failed to look at. §2.2 names exactly two arms, so exactly
// two exist, and an unknown value fails closed.
type State string

const (
	// StateProven carries exactly one canonical data item and no witness.
	StateProven State = "proven"
	// StateNonProven carries no item and one or more sorted witnesses.
	StateNonProven State = "non-proven"
)

// WitnessCode is the closed public vocabulary a non-proven result may
// disclose. It is exactly correction §2.2's exit-1 list — "absent, ambiguous,
// inapplicable, stale, or lineage-inconsistent" — and nothing else: a witness
// crosses a repository boundary, so it names a class the caller can act on and
// never a value from the document that produced it.
type WitnessCode string

const (
	// WitnessLineageInconsistent means some installed row did not replay to
	// the identity it claims.
	WitnessLineageInconsistent WitnessCode = "lineage-inconsistent"
	// WitnessManifestStale means the supplied manifest is not the one this
	// store compiles to now.
	WitnessManifestStale WitnessCode = "manifest-stale"
	// WitnessRefAbsent means the replayed state names no candidate for the ref.
	WitnessRefAbsent WitnessCode = "ref-absent"
	// WitnessRefAmbiguous means more than one candidate answers to the ref.
	WitnessRefAmbiguous WitnessCode = "ref-ambiguous"
	// WitnessRefInapplicable means the one candidate for the ref carries no
	// data item in this phase or scope.
	WitnessRefInapplicable WitnessCode = "ref-inapplicable"
)

// witnessCodes is the closed registry both grammar validation and the
// non-proven constructor read, so a code can never be published from one and
// refused by the other.
var witnessCodes = map[WitnessCode]bool{
	WitnessLineageInconsistent: true,
	WitnessManifestStale:       true,
	WitnessRefAbsent:           true,
	WitnessRefAmbiguous:        true,
	WitnessRefInapplicable:     true,
}

// Witness is one public non-proven reason.
type Witness struct {
	Schema string      `json:"schema"`
	Code   WitnessCode `json:"code"`
}

// Identity is the flight, lane, epoch and provider session the durable
// dispatch bound (correction §2.2, ledger SI-177).
//
// It is carried EXPLICITLY rather than read out of the lineage it authorizes.
// A replay that took its identity from the rows it is checking would let any
// internally consistent lineage authenticate itself: every row could name
// whatever flight it liked and still agree with the identity derived from it.
// The request states the identity the dispatch bound, the replay uses that
// value as the proof operand, and every row is measured against it.
type Identity struct {
	Flight  string `json:"flight"`
	Lane    string `json:"lane"`
	Epoch   string `json:"epoch"`
	Session string `json:"session"`
}

// Terminal is the explicit current post-expansion state (correction §2.2):
// the revision, manifest digest and expansion root the installed lineage
// leaves behind.
//
// It is a separate document from the base manifest on purpose. One canonical
// manifest cannot be both the compiler's pre-expansion output and the state a
// lineage arrived at, so a request carrying only one of the two could not
// express a post-expansion state at all — the defect SI-177 corrects. With no
// installed row the tuple is the base state, stated rather than inferred.
type Terminal struct {
	Revision       uint64 `json:"revision"`
	ManifestDigest string `json:"manifest_digest"`
	ExpansionRoot  string `json:"expansion_root"`
}

// Expansion is one row of the ordered installed-expansion lineage
// (correction §2.2). Every member is an operand of an identity the service
// reproves through internal/sealedexec's single owning helper: the request id
// binds (flight, lane, epoch, parent revision, parent digest, ref, purpose);
// the child manifest digest binds those plus the exact installed data item;
// the expansion digest binds the whole transition; and the expansion root
// chains it to its predecessor.
//
// Ref and Purpose are carried because the two digests above are computed over
// them. A row without them could be echoed back but never recomputed, and a
// lineage that cannot be recomputed is exactly what §2.2 forbids trusting.
// Data keeps the public `verdi.context-data-item/v1` grammar, in which the
// item's own ref is optional; the row ref is therefore the correlation
// operand, and an item that does carry a ref must carry this row's.
type Expansion struct {
	RequestID            string
	Ref                  string
	Purpose              string
	ParentRevision       uint64
	ParentManifestDigest string
	ChildRevision        uint64
	ChildManifestDigest  string
	ExpansionDigest      string
	ExpansionRoot        string
	TerminalAck          contextevent.EventAck
	Data                 contextcompile.DataItem
}

// Request is the decoded, validated `verdi.context-resolve-request/v1`
// document: the original compile request, the exact canonical BASE manifest
// that request produced before any expansion, the flight identity the durable
// dispatch bound, the ordered installed lineage, the explicit current terminal
// state, and the requested ref. Request is never marshaled directly; codec.go
// goes through a private wire document so each nested canonical document is
// judged by the package that owns it.
//
// The three members beyond compile/lineage/ref are what make the document
// restartable. BaseManifest is compared byte-for-byte against a fresh
// recompile, Identity is the operand every row is proved against, and
// Terminal is the state the replay must arrive at — none of them recoverable
// from process memory a restart no longer has.
type Request struct {
	Schema       string
	Compile      contextcompile.Request
	BaseManifest contextcompile.Manifest
	Identity     Identity
	Expansions   []Expansion
	Terminal     Terminal
	Ref          string
}

// Result is the decoded, validated `verdi.context-resolve-result/v1`
// document: the closed union of exactly one item with no witnesses, or no
// item with one or more sorted witnesses. Ref echoes the question so a caller
// holding several answers can bind each one to what it asked, rather than to
// the order the answers arrived in.
type Result struct {
	Schema    string
	State     State
	Ref       string
	Item      *contextcompile.DataItem
	Witnesses []Witness
}
