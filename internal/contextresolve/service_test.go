// The frozen behavioral producer for the read-only context-item resolver
// (VATC F12 controller-owner bridge correction §2.2 as amended, PLAN IL-099,
// sibling ledger SI-182, implementation plan Task 3 Steps 1 and 3).
//
// Every wire expectation below is a literal: the canonical request and result
// bytes are written out in full rather than produced by the codec under test,
// so a mistake in this package's own encoder cannot confirm itself. The nested
// documents are literals too, and each one is separately cross-checked against
// the encoder that owns it (internal/contextcompile for the compile request,
// base manifest and data item; internal/contextevent for the acknowledgment).
//
// The corrected model this file freezes has three parts the earlier candidate
// got wrong. The manifest in the request is the BASE manifest — the compiler's
// own output before any expansion — and is compared byte-for-byte against a
// fresh recompile. The post-expansion state is a separate explicit terminal
// revision/digest/root tuple that the replay must arrive at, never the same
// value the replay started from. And the flight identity is supplied
// explicitly by the request, so a lineage can no longer authenticate itself by
// naming whatever identity its own rows happen to carry.
//
// Every transition identity comes from internal/sealedexec's one owning
// helper, ProveInstalledExpansion. The fixtures call it for exactly the reason
// the resolver must: a second transcription of those preimages is the drift
// that would let a rewritten lineage replay cleanly.
package contextresolve

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/instructionprojection"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/sealedexec"
)

// --- fixture identities --------------------------------------------------

const (
	fixtureCommit  = "1111111111111111111111111111111111111111"
	fixtureBlob    = "2222222222222222222222222222222222222222"
	fixtureDigestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fixtureDigestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	fixtureDigestC = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	fixtureDigestD = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	fixtureDigestE = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	fixtureEventD  = "sha256:0101010101010101010101010101010101010101010101010101010101010101"
	fixtureEventD2 = "sha256:0202020202020202020202020202020202020202020202020202020202020202"
)

// fixtureBaseManifestDigest is the self digest fixtureManifest carries. It is
// spelled out because the terminal tuple of a zero-expansion request is that
// digest, and a test that read it back out of the manifest it is asserting
// would be comparing one value to itself.
const fixtureBaseManifestDigest = "sha256:b8389dcf6d645fb0e6be9aefeebf380cdc227b23edf459c142752b948db9f10b"

const (
	fixtureAlphaRef = "spec/feature-alpha"
	fixtureBetaRef  = "spec/feature-beta"
	fixtureGammaRef = "spec/feature-gamma"
	fixtureDeltaRef = "spec/feature-delta"
)

// fixtureIdentity is the flight identity the durable dispatch binds. The
// request carries it explicitly; the replay uses it as the helper operand and
// requires every terminal acknowledgment to match it.
func fixtureIdentity() Identity {
	return Identity{Flight: "flight-1", Lane: "lane-1", Epoch: "epoch-1", Session: "session-1"}
}

// --- the fixture compile result ------------------------------------------

// fixtureManifest is the smallest BASE manifest the resolver can be asked
// about: every ledger is explicitly empty except the one excluded row the
// inapplicable arm needs, and every repository fact is known, so the document
// carries no disclosure it did not earn.
func fixtureManifest() contextcompile.Manifest {
	gammaRef := fixtureGammaRef
	return contextcompile.Manifest{
		Schema:  contextcompile.ManifestSchema,
		Phase:   contextcompile.PhaseDesign,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Revisions: contextcompile.Revisions{
			Authority: fixtureDigestA,
			Context:   1,
		},
		AcceptedSpec: contextcompile.AcceptedSpec{
			Ref:           fixtureAlphaRef,
			Path:          ".verdi/specs/active/feature-alpha/spec.md",
			Blob:          fixtureBlob,
			Commit:        fixtureCommit,
			ContentDigest: fixtureDigestB,
		},
		ParentFeatures: []contextcompile.ParentFeature{},
		Decisions:      []contextcompile.DecisionRef{},
		Obligations:    []contextcompile.Obligation{},
		Repository: contextcompile.RepositoryFacts{
			RemoteOrigin: contextcompile.StringFact{Known: true, Value: "git@example.invalid:fixture.git"},
			Branch:       contextcompile.StringFact{Known: true, Value: "main"},
			Head:         contextcompile.StringFact{Known: true, Value: fixtureCommit},
			DefaultBranch: contextcompile.DefaultBranchFact{
				Known: true, Name: "main", Ref: "refs/heads/main", Head: fixtureCommit,
			},
			Relationship: contextcompile.RelationshipEqual,
			Dirty:        contextcompile.BoolFact{Known: true},
			Staged:       contextcompile.BoolFact{Known: true},
			Worktree:     contextcompile.WorktreeFact{},
			Source:       contextcompile.RepoSourceHead,
			Disclosures:  []contextcompile.DisclosureCode{},
		},
		Policy: contextcompile.PolicySection{
			EffectiveDigest:    fixtureDigestC,
			ConstitutionDigest: fixtureDigestD,
			ProfileID:          "solo-default",
			ProfileDigest:      fixtureDigestE,
			Entries:            []contextcompile.PolicyEntry{},
		},
		Owners: []string{},
		Scope: policyartifact.Scope{
			Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{},
		},
		GovernanceProfile: contextcompile.GovernanceProfileRef{
			ID: "solo-default", Class: governanceprincipal.ClassSolo, Digest: fixtureDigestE,
		},
		Actors: contextcompile.ActorsSection{
			Posture:     contextcompile.ResolutionUnproven,
			Resolutions: []governanceprincipal.PrincipalResolution{},
			Disclosures: []contextcompile.DisclosureCode{contextcompile.DisclosureActorResolutionUnproven},
		},
		Included: []contextcompile.IncludedEntry{},
		Excluded: []contextcompile.ExcludedEntry{{
			ID:            "ref:" + fixtureGammaRef,
			Source:        contextcompile.SourceStoreAuthority,
			Reason:        contextcompile.ExclusionPhaseInapplicable,
			Applicability: contextcompile.ApplicabilityInapplicable,
			Ref:           &gammaRef,
			Disclosures:   []contextcompile.DisclosureCode{},
		}},
		Opaque:          []contextcompile.OpaqueEntry{},
		Capabilities:    execworkspace.GrantSet{},
		ProjectionFiles: []contextcompile.ProjectionFileRef{},
		RequiredInputs:  []contextcompile.RequiredInput{},
		Evidence: contextcompile.EvidenceSection{
			Authority:       contextcompile.EvidenceAuthorityAdvisory,
			Freshness:       contextcompile.EvidenceFreshnessUnknown,
			ConsumedReports: []string{},
			Disclosures:     []contextcompile.DisclosureCode{contextcompile.DisclosureFreshnessUnknown},
		},
		Disclosures: []contextcompile.DisclosureCode{
			contextcompile.DisclosureActorResolutionUnproven,
			contextcompile.DisclosureFreshnessUnknown,
		},
	}
}

// fixtureDecodedManifest returns fixtureManifest as the decoder returns it,
// so the fixture carries the same self digest the wire does.
func fixtureDecodedManifest(t *testing.T) contextcompile.Manifest {
	t.Helper()
	encoded, err := contextcompile.EncodeManifest(fixtureManifest())
	if err != nil {
		t.Fatalf("EncodeManifest: %v", err)
	}
	decoded, err := contextcompile.DecodeManifest(encoded)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if decoded.Digest != fixtureBaseManifestDigest {
		t.Fatalf("base manifest digest = %q, want the frozen %q", decoded.Digest, fixtureBaseManifestDigest)
	}
	return decoded
}

// fixtureDataItem builds one canonical data item through the owning
// contextcompile constructor, so the fixture can never carry a shape that
// package would refuse.
func fixtureDataItem(t *testing.T, source contextcompile.Source, kind contextcompile.IncludedKind, ref, content string) contextcompile.DataItem {
	t.Helper()
	item, _, err := contextcompile.BuildDataItem(
		contextcompile.Candidate{Source: source, ID: "ref:" + ref, Ref: ref}, kind, []byte(content))
	if err != nil {
		t.Fatalf("BuildDataItem(%s, %s): %v", source, ref, err)
	}
	return item
}

// fixtureRefLessDataItem builds a valid data item that carries NO artifact
// ref. §2.2 keeps `ref` optional in the data-item grammar, so the row ref is
// the correlation operand and an item without one must still install, replay
// and resolve.
func fixtureRefLessDataItem(t *testing.T, path, content string) contextcompile.DataItem {
	t.Helper()
	item, _, err := contextcompile.BuildDataItem(
		contextcompile.Candidate{Source: contextcompile.SourceHeadTree, ID: "path:" + path, Path: path},
		contextcompile.IncludedRepositoryFile, []byte(content))
	if err != nil {
		t.Fatalf("BuildDataItem(%s): %v", path, err)
	}
	if item.Ref != nil {
		t.Fatalf("the ref-less fixture carries ref %q", *item.Ref)
	}
	return item
}

// fixtureCompileRequest is the original compile request every resolve request
// replays.
func fixtureCompileRequest() contextcompile.Request {
	return contextcompile.Request{
		Schema:  contextcompile.RequestSchema,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Phase:   contextcompile.PhaseDesign,
		Scope: policyartifact.Scope{
			Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{},
		},
		Spec: fixtureAlphaRef,
	}
}

// fixtureResult is what the fake compiler returns for the fixture request:
// one uniquely-reffed item, and two items sharing a second ref so the
// ambiguous arm is exercised against a state the resolver must refuse rather
// than pick from.
func fixtureResult(t *testing.T) contextcompile.Result {
	t.Helper()
	return contextcompile.Result{
		Manifest: fixtureDecodedManifest(t),
		DataItems: []contextcompile.DataItem{
			fixtureDataItem(t, contextcompile.SourceStoreAuthority, contextcompile.IncludedAcceptedSpec, fixtureAlphaRef, "alpha\n"),
			fixtureDataItem(t, contextcompile.SourceStoreAuthority, contextcompile.IncludedAcceptedSpec, fixtureBetaRef, "beta\n"),
			fixtureDataItem(t, contextcompile.SourceDeclaredContext, contextcompile.IncludedDeclaredContextRef, fixtureBetaRef, "beta-declared\n"),
		},
	}
}

// fakeCompiler answers with a fixed result (or a fixed failure), so every
// lineage row below prosecutes the replay rather than the compiler.
type fakeCompiler struct {
	result contextcompile.Result
	err    error
	roots  []string
}

func (c *fakeCompiler) Compile(_ context.Context, root string, _ contextcompile.Request) (contextcompile.Result, error) {
	c.roots = append(c.roots, root)
	if c.err != nil {
		return contextcompile.Result{}, c.err
	}
	return c.result, nil
}

// --- lineage rows, derived through the one owning helper ------------------

// fixtureExpansion builds one genuine installed row. Every transition
// identity comes from internal/sealedexec.ProveInstalledExpansion, the single
// owner SI-182 fixes for these preimages: a fixture that recomputed them here
// would agree with a resolver that had drifted the same way.
func fixtureExpansion(t *testing.T, identity Identity, parentRevision uint64, parentDigest, priorRoot, ref, purpose string, item contextcompile.DataItem, global uint64) Expansion {
	t.Helper()
	proof, err := sealedexec.ProveInstalledExpansion(sealedexec.InstalledExpansionInput{
		Key: sealedexec.ExecutionKey{
			Flight: identity.Flight, Lane: identity.Lane, Epoch: identity.Epoch,
		},
		ParentRevision:       parentRevision,
		ParentManifestDigest: parentDigest,
		Ref:                  ref,
		Purpose:              purpose,
		Item:                 item,
		PriorExpansionRoot:   priorRoot,
	})
	if err != nil {
		t.Fatalf("sealedexec.ProveInstalledExpansion: %v", err)
	}
	eventDigest := fixtureEventD
	if parentRevision != 1 {
		eventDigest = fixtureEventD2
	}
	return Expansion{
		RequestID:            proof.RequestID,
		Ref:                  ref,
		Purpose:              purpose,
		ParentRevision:       parentRevision,
		ParentManifestDigest: parentDigest,
		ChildRevision:        parentRevision + 1,
		ChildManifestDigest:  proof.ChildManifestDigest,
		ExpansionDigest:      proof.ExpansionDigest,
		ExpansionRoot:        proof.ExpansionRoot,
		TerminalAck: contextevent.EventAck{
			Schema: contextevent.AckSchemaID, Flight: identity.Flight, Lane: identity.Lane,
			Epoch: identity.Epoch, Session: identity.Session, ManifestRevision: parentRevision,
			Kind: contextevent.KindChildManifest, SourceSequence: 3,
			EventDigest: eventDigest, GlobalSequence: global,
		},
		Data: item,
	}
}

// fixtureLineageUnder builds the two-expansion lineage under a caller-chosen
// flight identity: alpha installed on the base manifest, then a ref-less
// repository payload installed on top of it. The second row deliberately
// carries an item with no artifact ref, so the ordinary happy path already
// proves the optional-ref grammar rather than leaving it to one edge row.
func fixtureLineageUnder(t *testing.T, identity Identity) []Expansion {
	t.Helper()
	base := fixtureDecodedManifest(t)
	first := fixtureExpansion(t, identity, 1, base.Digest, "", fixtureAlphaRef, "review the accepted spec",
		fixtureDataItem(t, contextcompile.SourceStoreAuthority, contextcompile.IncludedAcceptedSpec, fixtureAlphaRef, "alpha\n"), 7)
	second := fixtureExpansion(t, identity, first.ChildRevision, first.ChildManifestDigest, first.ExpansionRoot,
		fixtureDeltaRef, "read the delta payload",
		fixtureRefLessDataItem(t, "docs/delta.md", "delta\n"), 11)
	return []Expansion{first, second}
}

func fixtureLineage(t *testing.T) []Expansion {
	t.Helper()
	return fixtureLineageUnder(t, fixtureIdentity())
}

// fixtureTerminal derives the explicit current terminal tuple a lineage
// leaves behind. With no installed row that tuple is the base state itself.
func fixtureTerminal(t *testing.T, expansions []Expansion) Terminal {
	t.Helper()
	if len(expansions) == 0 {
		return Terminal{Revision: 1, ManifestDigest: fixtureBaseManifestDigest, ExpansionRoot: ""}
	}
	last := expansions[len(expansions)-1]
	return Terminal{
		Revision: last.ChildRevision, ManifestDigest: last.ChildManifestDigest,
		ExpansionRoot: last.ExpansionRoot,
	}
}

func fixtureRequest(t *testing.T, ref string, expansions []Expansion) Request {
	t.Helper()
	rows := make([]Expansion, 0, len(expansions))
	rows = append(rows, expansions...)
	return Request{
		Schema:       RequestSchema,
		Compile:      fixtureCompileRequest(),
		BaseManifest: fixtureDecodedManifest(t),
		Identity:     fixtureIdentity(),
		Expansions:   rows,
		Terminal:     fixtureTerminal(t, rows),
		Ref:          ref,
	}
}

// --- frozen literal wire bytes -------------------------------------------

// fixtureManifestJSON is the literal canonical encoding of fixtureManifest.
// The frozen-fixture test below proves internal/contextcompile still produces
// exactly these bytes, so the larger literals that embed it stay honest.
const fixtureManifestJSON = `{"accepted_spec":{"blob":"2222222222222222222222222222222222222222","commit":"1111111111111111111111111111111111111111","content_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","path":".verdi/specs/active/feature-alpha/spec.md","ref":"spec/feature-alpha"},"actors":{"disclosures":["actor-resolution-unproven"],"posture":"unproven","resolutions":[]},"adapter":{"id":"codex","version":"1"},"capabilities":{"grants":[],"schema":"verdi.execution-grants/v1"},"decisions":[],"digest":"sha256:b8389dcf6d645fb0e6be9aefeebf380cdc227b23edf459c142752b948db9f10b","disclosures":["actor-resolution-unproven","freshness-unknown"],"dispositions":[],"evidence":{"authority":"advisory","consumed_reports":[],"disclosures":["freshness-unknown"],"freshness":"unknown"},"excluded":[{"applicability":"inapplicable","disclosures":[],"id":"ref:spec/feature-gamma","reason":"phase-inapplicable","ref":"spec/feature-gamma","source":"store-authority"}],"expansions":[],"governance_profile":{"class":"solo","digest":"sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","id":"solo-default"},"included":[],"obligations":[],"opaque":[],"owners":[],"parent_features":[],"phase":"design","policy":{"constitution_digest":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd","effective_digest":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","entries":[],"profile_digest":"sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee","profile_id":"solo-default"},"projection_files":[],"repository":{"branch":{"known":true,"value":"main"},"default_branch":{"head":"1111111111111111111111111111111111111111","known":true,"name":"main","ref":"refs/heads/main"},"dirty":{"known":true,"value":false},"disclosures":[],"head":{"known":true,"value":"1111111111111111111111111111111111111111"},"relationship":"equal","remote_origin":{"known":true,"value":"git@example.invalid:fixture.git"},"source":"head","staged":{"known":true,"value":false},"worktree":{"managed":false,"name":""}},"required_inputs":[],"revisions":{"authority":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","context":1},"schema":"verdi.context-manifest/v1","scope":{"environments":[],"paths":[],"phases":[],"refs":[]}}`

// fixtureCompileRequestJSON is the literal canonical compile request the
// resolve request carries, cross-checked below against its owning encoder.
const fixtureCompileRequestJSON = `{"adapter":{"id":"codex","version":"1"},"grants":{"grants":[],"schema":"verdi.execution-grants/v1"},"phase":"design","schema":"verdi.context-compile-request/v1","scope":{"environments":[],"paths":[],"phases":[],"refs":[]},"spec":"spec/feature-alpha"}`

// fixtureIdentityJSON is the literal canonical flight identity §2.2 requires
// the request to carry explicitly.
const fixtureIdentityJSON = `{"epoch":"epoch-1","flight":"flight-1","lane":"lane-1","session":"session-1"}`

// fixtureBaseTerminalJSON is the terminal tuple of a request with no
// installed expansion: the base state, stated explicitly rather than inferred
// from the base manifest it sits beside.
const fixtureBaseTerminalJSON = `{"expansion_root":"","manifest_digest":"sha256:b8389dcf6d645fb0e6be9aefeebf380cdc227b23edf459c142752b948db9f10b","revision":1}`

// TestContextResolveFrozenFixtureManifest proves the literal above is exactly
// what internal/contextcompile encodes today.
func TestContextResolveFrozenFixtureManifest(t *testing.T) {
	got, err := contextcompile.EncodeManifest(fixtureManifest())
	if err != nil {
		t.Fatalf("EncodeManifest: %v", err)
	}
	if want := fixtureManifestJSON + "\n"; string(got) != want {
		t.Fatalf("fixture manifest bytes drifted\n got: %s\nwant: %s", got, want)
	}
}

func TestContextResolveFrozenFixtureCompileRequest(t *testing.T) {
	got, err := contextcompile.EncodeRequest(fixtureCompileRequest())
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	if want := fixtureCompileRequestJSON + "\n"; string(got) != want {
		t.Fatalf("fixture compile request bytes drifted\n got: %s\nwant: %s", got, want)
	}
}

// TestContextResolveFrozenBaseRequest proves the exact canonical encoding of
// a resolve request that carries the base manifest, no expansion at all, and
// the explicit base terminal tuple — in both directions.
//
// The member set is the whole contract: `base_manifest` names the compiler's
// own output, `terminal` names where the lineage ended up, and `identity`
// names the flight that owns the rows. A document missing any of the three
// cannot be replayed after a restart, which is exactly what IL-099 corrects.
func TestContextResolveFrozenBaseRequest(t *testing.T) {
	want := `{"base_manifest":` + fixtureManifestJSON +
		`,"expansions":[],"identity":` + fixtureIdentityJSON +
		`,"ref":"spec/feature-alpha","request":` + fixtureCompileRequestJSON +
		`,"schema":"verdi.context-resolve-request/v1","terminal":` + fixtureBaseTerminalJSON + "}\n"

	got, err := EncodeRequest(fixtureRequest(t, fixtureAlphaRef, nil))
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	if string(got) != want {
		t.Fatalf("base request bytes\n got: %s\nwant: %s", got, want)
	}
	decoded, err := DecodeRequest([]byte(want))
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	round, err := EncodeRequest(decoded)
	if err != nil {
		t.Fatalf("re-EncodeRequest: %v", err)
	}
	if string(round) != want {
		t.Fatalf("request did not round-trip\n got: %s\nwant: %s", round, want)
	}
	if decoded.Identity != fixtureIdentity() {
		t.Fatalf("identity = %#v, want %#v", decoded.Identity, fixtureIdentity())
	}
	if decoded.Terminal != (Terminal{Revision: 1, ManifestDigest: fixtureBaseManifestDigest}) {
		t.Fatalf("terminal = %#v, want the explicit base tuple", decoded.Terminal)
	}
}

// TestContextResolveFrozenResults proves the exact canonical encoding of both
// arms of the closed result union, and that the document carries exactly the
// five members §2.2 names.
func TestContextResolveFrozenResults(t *testing.T) {
	item := fixtureDataItem(t, contextcompile.SourceStoreAuthority, contextcompile.IncludedAcceptedSpec, fixtureAlphaRef, "alpha\n")
	itemBytes, err := contextcompile.EncodeDataItem(item)
	if err != nil {
		t.Fatalf("EncodeDataItem: %v", err)
	}
	itemJSON := strings.TrimSuffix(string(itemBytes), "\n")

	proven := `{"item":` + itemJSON + `,"ref":"spec/feature-alpha","schema":"verdi.context-resolve-result/v1",` +
		`"state":"proven","witnesses":[]}` + "\n"
	gotProven, err := EncodeResult(Result{
		Schema: ResultSchema, State: StateProven, Ref: fixtureAlphaRef,
		Item: &item, Witnesses: []Witness{},
	})
	if err != nil {
		t.Fatalf("EncodeResult(proven): %v", err)
	}
	if string(gotProven) != proven {
		t.Fatalf("proven result bytes\n got: %s\nwant: %s", gotProven, proven)
	}

	nonProven := `{"item":null,"ref":"spec/feature-alpha","schema":"verdi.context-resolve-result/v1",` +
		`"state":"non-proven","witnesses":[{"code":"lineage-inconsistent","schema":"verdi.context-resolve-witness/v1"},` +
		`{"code":"manifest-stale","schema":"verdi.context-resolve-witness/v1"}]}` + "\n"
	gotNonProven, err := EncodeResult(Result{
		Schema: ResultSchema, State: StateNonProven, Ref: fixtureAlphaRef,
		Witnesses: []Witness{
			{Schema: WitnessSchema, Code: WitnessManifestStale},
			{Schema: WitnessSchema, Code: WitnessLineageInconsistent},
		},
	})
	if err != nil {
		t.Fatalf("EncodeResult(non-proven): %v", err)
	}
	if string(gotNonProven) != nonProven {
		t.Fatalf("non-proven result bytes (witnesses must be sorted by their standalone canonical encoding)\n got: %s\nwant: %s", gotNonProven, nonProven)
	}
	for _, document := range []string{proven, nonProven} {
		decoded, err := DecodeResult([]byte(document))
		if err != nil {
			t.Fatalf("DecodeResult(%s): %v", document, err)
		}
		round, err := EncodeResult(decoded)
		if err != nil {
			t.Fatalf("re-EncodeResult: %v", err)
		}
		if string(round) != document {
			t.Fatalf("result did not round-trip\n got: %s\nwant: %s", round, document)
		}
	}
}

// provenResultDocument renders the exact canonical proven-result document for
// one item under one result ref WITHOUT going through EncodeResult, so a
// document the encoder refuses to produce can still be handed to DecodeResult
// as the well-formed canonical bytes a peer could put on the wire. The member
// order and framing are the ones TestContextResolveFrozenResults freezes; the
// accepted rows below re-prove them byte for byte against the encoder itself.
func provenResultDocument(t *testing.T, item contextcompile.DataItem, ref string) []byte {
	t.Helper()
	encoded, err := contextcompile.EncodeDataItem(item)
	if err != nil {
		t.Fatalf("EncodeDataItem: %v", err)
	}
	return []byte(`{"item":` + strings.TrimSuffix(string(encoded), "\n") +
		`,"ref":"` + ref + `","schema":"` + ResultSchema + `","state":"proven","witnesses":[]}` + "\n")
}

// TestContextResolveProvenItemRefBindsResultRef proves §2.2's correlation rule
// in the codec that owns the wire: the result `ref` is what a caller binds an
// answer to, so a proven item that carries an artifact ref of its own must
// carry exactly that one.
//
// An item with NO ref stays legal. §2.2 keeps `ref` optional in the data-item
// grammar precisely so the result ref can be the correlation operand "even
// when a valid data item has no artifact ref", so the rule is a cross-match
// and not a requirement that an item be reffed — only a DIFFERENT ref is
// refused. ATC strict-decodes the returned item and cross-matches it before
// use; a wire that could carry the mismatch at all would make that cross-match
// a property of the receiver's diligence rather than of the document.
//
// Both directions are proved, because the two are separately reachable: the
// resolver's own producer never builds such a result, so the encoder alone
// would leave a caller free to construct one, and the decoder alone would
// leave one arriving from elsewhere unjudged. The refused rows are refused for
// the RELATION rather than for being noncanonical: every row here is rendered
// by the same constructor whose output the accepted rows prove byte-identical
// to EncodeResult's own.
func TestContextResolveProvenItemRefBindsResultRef(t *testing.T) {
	cases := []struct {
		name     string
		item     contextcompile.DataItem
		ref      string
		accepted bool
	}{
		{"item ref equals the result ref",
			fixtureDataItem(t, contextcompile.SourceStoreAuthority, contextcompile.IncludedAcceptedSpec, fixtureAlphaRef, "alpha\n"),
			fixtureAlphaRef, true},
		{"item carries no artifact ref at all",
			fixtureRefLessDataItem(t, "docs/delta.md", "delta\n"),
			fixtureAlphaRef, true},
		{"item names some other ref",
			fixtureDataItem(t, contextcompile.SourceStoreAuthority, contextcompile.IncludedAcceptedSpec, fixtureBetaRef, "beta\n"),
			fixtureAlphaRef, false},
		{"result names some other ref",
			fixtureDataItem(t, contextcompile.SourceStoreAuthority, contextcompile.IncludedAcceptedSpec, fixtureAlphaRef, "alpha\n"),
			fixtureBetaRef, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := tc.item
			document := provenResultDocument(t, item, tc.ref)
			encoded, encodeErr := EncodeResult(Result{
				Schema: ResultSchema, State: StateProven, Ref: tc.ref,
				Item: &item, Witnesses: []Witness{},
			})
			decoded, decodeErr := DecodeResult(document)

			if !tc.accepted {
				if encodeErr == nil {
					t.Fatalf("EncodeResult produced a proven result whose item ref is not its own ref: %s", encoded)
				}
				if decodeErr == nil {
					t.Fatalf("DecodeResult accepted a proven result whose item ref is not its own ref: %s", document)
				}
				return
			}
			if encodeErr != nil {
				t.Fatalf("EncodeResult: %v", encodeErr)
			}
			if string(encoded) != string(document) {
				t.Fatalf("proven result bytes\n got: %s\nwant: %s", encoded, document)
			}
			if decodeErr != nil {
				t.Fatalf("DecodeResult(%s): %v", document, decodeErr)
			}
			if decoded.Item == nil || decoded.Ref != tc.ref {
				t.Fatalf("decoded ref %q with item %v, want the document's ref and its one item", decoded.Ref, decoded.Item)
			}
		})
	}
}

// TestContextResolveFrozenWitnessCodes proves the closed witness vocabulary
// is exactly §2.2's five codes: each encodes to its exact standalone
// canonical document, and nothing else decodes at all.
func TestContextResolveFrozenWitnessCodes(t *testing.T) {
	want := map[WitnessCode]string{
		WitnessRefAbsent:           `{"code":"ref-absent","schema":"verdi.context-resolve-witness/v1"}`,
		WitnessRefAmbiguous:        `{"code":"ref-ambiguous","schema":"verdi.context-resolve-witness/v1"}`,
		WitnessRefInapplicable:     `{"code":"ref-inapplicable","schema":"verdi.context-resolve-witness/v1"}`,
		WitnessManifestStale:       `{"code":"manifest-stale","schema":"verdi.context-resolve-witness/v1"}`,
		WitnessLineageInconsistent: `{"code":"lineage-inconsistent","schema":"verdi.context-resolve-witness/v1"}`,
	}
	for code, witness := range want {
		document := `{"item":null,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"non-proven","witnesses":[` + witness + `]}` + "\n"
		decoded, err := DecodeResult([]byte(document))
		if err != nil {
			t.Fatalf("DecodeResult(%s): %v", code, err)
		}
		if len(decoded.Witnesses) != 1 || decoded.Witnesses[0].Code != code ||
			decoded.Witnesses[0].Schema != WitnessSchema {
			t.Fatalf("witness = %#v, want the single %s", decoded.Witnesses, code)
		}
	}
}

// --- strict decode -------------------------------------------------------

// TestContextResolveStrictRequestDecode covers every malformed shape §2.2
// closes: an unknown or duplicate member, trailing data, a null document, the
// wrong schema at any level, a noncanonical member order, and each of the
// three members IL-099 adds.
func TestContextResolveStrictRequestDecode(t *testing.T) {
	valid := func(t *testing.T) []byte {
		t.Helper()
		data, err := EncodeRequest(fixtureRequest(t, fixtureAlphaRef, nil))
		if err != nil {
			t.Fatalf("EncodeRequest: %v", err)
		}
		return data
	}

	cases := []struct {
		name  string
		mutta func(t *testing.T) []byte
	}{
		{"empty", func(*testing.T) []byte { return nil }},
		{"null", func(*testing.T) []byte { return []byte("null\n") }},
		{"not-an-object", func(*testing.T) []byte { return []byte("[]\n") }},
		{"trailing-data", func(t *testing.T) []byte {
			return append(valid(t), []byte("{}\n")...)
		}},
		{"unknown-member", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"ref":`, `"extra":1,"ref":`, 1))
		}},
		{"duplicate-member", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"ref":"spec/feature-alpha"`,
				`"ref":"spec/feature-alpha","ref":"spec/feature-alpha"`, 1))
		}},
		{"wrong-top-level-schema", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), RequestSchema, "verdi.context-resolve-request/v2", 1))
		}},
		{"wrong-nested-manifest-schema", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), contextcompile.ManifestSchema, "verdi.context-manifest/v2", 1))
		}},
		{"noncanonical-member-order", func(t *testing.T) []byte {
			return []byte(`{"schema":"` + RequestSchema + `","ref":"spec/feature-alpha","identity":` + fixtureIdentityJSON +
				`,"terminal":` + fixtureBaseTerminalJSON + `,"expansions":[],"base_manifest":` + fixtureManifestJSON +
				`,"request":` + fixtureCompileRequestJSON + "}\n")
		}},
		{"absent-base-manifest", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"base_manifest":`+fixtureManifestJSON, `"base_manifest":null`, 1))
		}},
		{"absent-identity", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"identity":`+fixtureIdentityJSON, `"identity":null`, 1))
		}},
		{"absent-terminal", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"terminal":`+fixtureBaseTerminalJSON, `"terminal":null`, 1))
		}},
		{"empty-identity-session", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"session":"session-1"`, `"session":""`, 1))
		}},
		{"empty-identity-flight", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"flight":"flight-1"`, `"flight":""`, 1))
		}},
		{"empty-ref", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"ref":"spec/feature-alpha"`, `"ref":""`, 1))
		}},
		{"zero-terminal-revision", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"revision":1}`, `"revision":0}`, 1))
		}},
		{"noncanonical-terminal-digest", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"manifest_digest":"`+fixtureBaseManifestDigest+`"`,
				`"manifest_digest":"not-a-digest"`, 1))
		}},
		{"noncanonical-terminal-root", func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"expansion_root":""`, `"expansion_root":"not-a-digest"`, 1))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeRequest(tc.mutta(t)); err == nil {
				t.Fatalf("DecodeRequest accepted the %s document", tc.name)
			}
		})
	}
}

// TestContextResolveStrictLineageDecode proves the installed-row grammar:
// every member IL-099 makes durable is required, and an item that carries a
// ref must carry the row's own.
func TestContextResolveStrictLineageDecode(t *testing.T) {
	valid := func(t *testing.T) []byte {
		t.Helper()
		data, err := EncodeRequest(fixtureRequest(t, fixtureAlphaRef, fixtureLineage(t)))
		if err != nil {
			t.Fatalf("EncodeRequest: %v", err)
		}
		return data
	}
	cases := map[string]func(t *testing.T) []byte{
		"unknown-row-member": func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"purpose":"review the accepted spec"`,
				`"extra":1,"purpose":"review the accepted spec"`, 1))
		},
		"empty-row-purpose": func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"purpose":"review the accepted spec"`, `"purpose":""`, 1))
		},
		"empty-row-ref": func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"ref":"spec/feature-alpha","request_id"`,
				`"ref":"","request_id"`, 1))
		},
		"malformed-request-id": func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), `"request_id":"context-request:`,
				`"request_id":"context-demand:`, 1))
		},
		"wrong-ack-schema": func(t *testing.T) []byte {
			return []byte(strings.Replace(string(valid(t)), contextevent.AckSchemaID, "verdi.context-event-ack/v2", 1))
		},
	}
	for name, mutta := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeRequest(mutta(t)); err == nil {
				t.Fatalf("DecodeRequest accepted the %s document", name)
			}
		})
	}
}

// TestContextResolveStrictResultDecode proves the closed union refuses every
// shape that is neither arm.
func TestContextResolveStrictResultDecode(t *testing.T) {
	item := fixtureDataItem(t, contextcompile.SourceStoreAuthority, contextcompile.IncludedAcceptedSpec, fixtureAlphaRef, "alpha\n")
	itemBytes, err := contextcompile.EncodeDataItem(item)
	if err != nil {
		t.Fatalf("EncodeDataItem: %v", err)
	}
	itemJSON := strings.TrimSuffix(string(itemBytes), "\n")
	witness := `{"code":"ref-absent","schema":"verdi.context-resolve-witness/v1"}`

	cases := map[string]string{
		"proven-with-witness": `{"item":` + itemJSON + `,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"proven","witnesses":[` + witness + `]}` + "\n",
		"proven-without-item": `{"item":null,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"proven","witnesses":[]}` + "\n",
		"non-proven-with-item": `{"item":` + itemJSON + `,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"non-proven","witnesses":[` + witness + `]}` + "\n",
		"non-proven-without-witness": `{"item":null,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"non-proven","witnesses":[]}` + "\n",
		"unknown-state": `{"item":null,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"maybe","witnesses":[` + witness + `]}` + "\n",
		"empty-ref": `{"item":null,"ref":"","schema":"` + ResultSchema +
			`","state":"non-proven","witnesses":[` + witness + `]}` + "\n",
		"unknown-witness-code": `{"item":null,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"non-proven","witnesses":[{"code":"nonsense","schema":"verdi.context-resolve-witness/v1"}]}` + "\n",
		"wrong-witness-schema": `{"item":null,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"non-proven","witnesses":[{"code":"ref-absent","schema":"verdi.context-resolve-witness/v2"}]}` + "\n",
		"unsorted-witnesses": `{"item":null,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"non-proven","witnesses":[{"code":"ref-absent","schema":"verdi.context-resolve-witness/v1"},` +
			`{"code":"manifest-stale","schema":"verdi.context-resolve-witness/v1"}]}` + "\n",
		"duplicate-witnesses": `{"item":null,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"non-proven","witnesses":[` + witness + `,` + witness + `]}` + "\n",
		"unknown-member": `{"correlation":"x","item":null,"ref":"spec/feature-alpha","schema":"` + ResultSchema +
			`","state":"non-proven","witnesses":[` + witness + `]}` + "\n",
	}
	for name, document := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeResult([]byte(document)); err == nil {
				t.Fatalf("DecodeResult accepted the %s document", name)
			}
		})
	}
}

// --- resolution ----------------------------------------------------------

func resolveFixture(t *testing.T, request Request) (Result, error) {
	t.Helper()
	return newServiceWithCompiler(&fakeCompiler{result: fixtureResult(t)}).
		Resolve(context.Background(), "/fixture/root", request)
}

func requireWitnesses(t *testing.T, result Result, ref string, codes ...WitnessCode) {
	t.Helper()
	if result.State != StateNonProven {
		t.Fatalf("state = %q, want non-proven", result.State)
	}
	if result.Item != nil {
		t.Fatal("a non-proven result carried an item")
	}
	if result.Ref != ref {
		t.Fatalf("result ref = %q, want the request ref %q", result.Ref, ref)
	}
	got := make([]WitnessCode, 0, len(result.Witnesses))
	for _, w := range result.Witnesses {
		if w.Schema != WitnessSchema {
			t.Fatalf("witness schema = %q, want %q", w.Schema, WitnessSchema)
		}
		got = append(got, w.Code)
	}
	if fmt.Sprint(got) != fmt.Sprint(codes) {
		t.Fatalf("witnesses = %v, want exactly %v", got, codes)
	}
}

// TestContextResolveBaseManifest proves the zero-expansion arm: a fresh
// recompile equal to the base, a terminal tuple equal to the base state, and
// one resolved item.
func TestContextResolveBaseManifest(t *testing.T) {
	compiler := &fakeCompiler{result: fixtureResult(t)}
	service := newServiceWithCompiler(compiler)
	result, err := service.Resolve(context.Background(), "/fixture/root", fixtureRequest(t, fixtureAlphaRef, nil))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.State != StateProven {
		t.Fatalf("state = %q with witnesses %v, want proven", result.State, result.Witnesses)
	}
	if result.Item == nil {
		t.Fatal("a proven result carried no item")
	}
	if result.Ref != fixtureAlphaRef {
		t.Fatalf("result ref = %q, want the request ref %q", result.Ref, fixtureAlphaRef)
	}
	if result.Item.Ref == nil || *result.Item.Ref != fixtureAlphaRef {
		t.Fatalf("item ref = %v, want %q", result.Item.Ref, fixtureAlphaRef)
	}
	if len(result.Witnesses) != 0 {
		t.Fatalf("witnesses = %v, want none", result.Witnesses)
	}
	if len(compiler.roots) != 1 || compiler.roots[0] != "/fixture/root" {
		t.Fatalf("compiler saw roots %v, want exactly one /fixture/root", compiler.roots)
	}
}

// TestContextResolveMultiExpansionLineage proves a genuine base-to-terminal
// replay: two installed rows, the second carrying an item with no artifact
// ref, replay forward from the base and arrive at exactly the explicit
// terminal tuple.
func TestContextResolveMultiExpansionLineage(t *testing.T) {
	rows := fixtureLineage(t)
	if rows[1].Data.Ref != nil {
		t.Fatal("the terminal fixture row must carry a ref-less installed item")
	}
	if rows[0].ChildRevision != 2 || rows[1].ChildRevision != 3 {
		t.Fatalf("lineage revisions = %d/%d, want 2/3", rows[0].ChildRevision, rows[1].ChildRevision)
	}
	request := fixtureRequest(t, fixtureAlphaRef, rows)
	if request.Terminal.Revision != 3 || request.Terminal.ManifestDigest == fixtureBaseManifestDigest {
		t.Fatalf("terminal = %#v, want a post-expansion tuple distinct from the base", request.Terminal)
	}
	result, err := resolveFixture(t, request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.State != StateProven || result.Item == nil {
		t.Fatalf("state = %q with witnesses %v, want proven with an item", result.State, result.Witnesses)
	}
}

// TestContextResolveRefArms proves the three ref outcomes the correction
// closes, each against the same replayed terminal state.
func TestContextResolveRefArms(t *testing.T) {
	cases := []struct {
		name string
		ref  string
		code WitnessCode
	}{
		{"absent", "spec/feature-nowhere", WitnessRefAbsent},
		{"ambiguous", fixtureBetaRef, WitnessRefAmbiguous},
		{"inapplicable", fixtureGammaRef, WitnessRefInapplicable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := resolveFixture(t, fixtureRequest(t, tc.ref, fixtureLineage(t)))
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			requireWitnesses(t, result, tc.ref, tc.code)
		})
	}
}

// TestContextResolveStaleBaseEmitsOnlyManifestStale proves §2.2's exclusive
// mapping: a base-manifest byte mismatch is manifest-stale and NOTHING else,
// even when the supplied lineage is itself unreplayable.
//
// The two codes are not independent findings to be reported together. A
// caller whose base is stale is holding a lineage this store cannot judge at
// all, so adding lineage-inconsistent would assert a second fact the resolver
// is in no position to have proved.
func TestContextResolveStaleBaseEmitsOnlyManifestStale(t *testing.T) {
	fresh := fixtureResult(t)
	fresh.Manifest.Phase = contextcompile.PhaseBuild
	freshBytes, err := contextcompile.EncodeManifest(fresh.Manifest)
	if err != nil {
		t.Fatalf("EncodeManifest: %v", err)
	}
	if fresh.Manifest, err = contextcompile.DecodeManifest(freshBytes); err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}

	cases := map[string]func(t *testing.T, r *Request){
		"lineage-intact":   func(*testing.T, *Request) {},
		"lineage-also-bad": func(_ *testing.T, r *Request) { r.Expansions[0].ExpansionRoot = fixtureDigestA },
		"terminal-also-bad": func(_ *testing.T, r *Request) {
			r.Terminal.ExpansionRoot = fixtureDigestA
		},
	}
	for name, mutta := range cases {
		t.Run(name, func(t *testing.T) {
			request := fixtureRequest(t, fixtureAlphaRef, fixtureLineage(t))
			mutta(t, &request)
			result, err := newServiceWithCompiler(&fakeCompiler{result: fresh}).
				Resolve(context.Background(), "/fixture/root", request)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			requireWitnesses(t, result, fixtureAlphaRef, WitnessManifestStale)
		})
	}
}

// TestContextResolveLineageInconsistencies is the adverse matrix Task 3 Step 1
// names: every single-field mutation of an otherwise genuine lineage must be
// refused with exactly lineage-inconsistent, and a supplied data item must
// never be believed merely because it strict-decodes.
func TestContextResolveLineageInconsistencies(t *testing.T) {
	cases := []struct {
		name  string
		mutta func(t *testing.T, r *Request)
	}{
		{"reordered-rows", func(_ *testing.T, r *Request) {
			r.Expansions[0], r.Expansions[1] = r.Expansions[1], r.Expansions[0]
		}},
		{"duplicate-row", func(_ *testing.T, r *Request) { r.Expansions[1] = r.Expansions[0] }},
		{"duplicate-request-id", func(_ *testing.T, r *Request) {
			r.Expansions[1].RequestID = r.Expansions[0].RequestID
		}},
		{"altered-parent-revision", func(_ *testing.T, r *Request) { r.Expansions[1].ParentRevision = 1 }},
		{"altered-child-revision", func(_ *testing.T, r *Request) { r.Expansions[1].ChildRevision = 9 }},
		{"altered-parent-digest", func(_ *testing.T, r *Request) { r.Expansions[1].ParentManifestDigest = fixtureDigestA }},
		{"altered-child-digest", func(_ *testing.T, r *Request) { r.Expansions[0].ChildManifestDigest = fixtureDigestA }},
		{"altered-expansion-digest", func(_ *testing.T, r *Request) { r.Expansions[0].ExpansionDigest = fixtureDigestA }},
		{"altered-expansion-root", func(_ *testing.T, r *Request) { r.Expansions[0].ExpansionRoot = fixtureDigestA }},
		{"altered-terminal-ack-revision", func(_ *testing.T, r *Request) { r.Expansions[1].TerminalAck.ManifestRevision = 5 }},
		{"altered-terminal-ack-kind", func(_ *testing.T, r *Request) {
			r.Expansions[1].TerminalAck.Kind = contextevent.KindContextDecision
		}},
		{"altered-terminal-ack-flight", func(_ *testing.T, r *Request) { r.Expansions[1].TerminalAck.Flight = "flight-9" }},
		{"altered-terminal-ack-lane", func(_ *testing.T, r *Request) { r.Expansions[1].TerminalAck.Lane = "lane-9" }},
		{"altered-terminal-ack-epoch", func(_ *testing.T, r *Request) { r.Expansions[1].TerminalAck.Epoch = "epoch-9" }},
		{"altered-terminal-ack-session", func(_ *testing.T, r *Request) { r.Expansions[1].TerminalAck.Session = "session-9" }},
		{"regressing-global-sequence", func(_ *testing.T, r *Request) { r.Expansions[1].TerminalAck.GlobalSequence = 2 }},
		{"altered-data-item", func(t *testing.T, r *Request) {
			r.Expansions[0].Data = fixtureDataItem(t, contextcompile.SourceStoreAuthority,
				contextcompile.IncludedAcceptedSpec, fixtureAlphaRef, "tampered\n")
		}},
		{"altered-row-purpose", func(_ *testing.T, r *Request) { r.Expansions[0].Purpose = "something else" }},
		{"altered-row-ref", func(_ *testing.T, r *Request) { r.Expansions[0].Ref = fixtureBetaRef }},
		{"dropped-terminal-row", func(_ *testing.T, r *Request) { r.Expansions = r.Expansions[:1] }},
		{"terminal-revision-mismatch", func(_ *testing.T, r *Request) { r.Terminal.Revision = 9 }},
		{"terminal-digest-mismatch", func(_ *testing.T, r *Request) { r.Terminal.ManifestDigest = fixtureDigestA }},
		{"terminal-root-mismatch", func(_ *testing.T, r *Request) { r.Terminal.ExpansionRoot = fixtureDigestA }},
		{"terminal-claims-base-state", func(t *testing.T, r *Request) {
			r.Terminal = fixtureTerminal(t, nil)
		}},
		{"root-without-any-expansion", func(t *testing.T, r *Request) {
			r.Expansions = []Expansion{}
			r.Terminal = Terminal{Revision: 1, ManifestDigest: fixtureBaseManifestDigest, ExpansionRoot: fixtureDigestA}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := fixtureRequest(t, fixtureAlphaRef, fixtureLineage(t))
			tc.mutta(t, &request)
			result, err := resolveFixture(t, request)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			requireWitnesses(t, result, fixtureAlphaRef, WitnessLineageInconsistent)
		})
	}
}

// TestContextResolveIdentityIsSuppliedNotDerived is the row the rest of the
// matrix cannot reach, and the one IL-099 exists for.
//
// The lineage below is internally perfect: every row was genuinely derived
// under flight-9, and every acknowledgment names flight-9 too. Only the
// request's explicit identity — the one the durable dispatch bound — says
// flight-1. A replay that took its identity from the rows it is checking
// would authenticate this lineage against itself and hand back an item that
// belongs to a different flight.
func TestContextResolveIdentityIsSuppliedNotDerived(t *testing.T) {
	foreign := Identity{Flight: "flight-9", Lane: "lane-9", Epoch: "epoch-9", Session: "session-9"}
	rows := fixtureLineageUnder(t, foreign)

	request := fixtureRequest(t, fixtureAlphaRef, rows)
	if request.Identity != fixtureIdentity() {
		t.Fatalf("identity = %#v, want the dispatch-bound %#v", request.Identity, fixtureIdentity())
	}
	result, err := resolveFixture(t, request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	requireWitnesses(t, result, fixtureAlphaRef, WitnessLineageInconsistent)

	// The same rows under their own identity are a genuine lineage, so the
	// refusal above is about the identity binding and not about the rows.
	matched := Request{
		Schema: RequestSchema, Compile: fixtureCompileRequest(),
		BaseManifest: fixtureDecodedManifest(t), Identity: foreign,
		Expansions: rows, Terminal: fixtureTerminal(t, rows), Ref: fixtureAlphaRef,
	}
	accepted, err := resolveFixture(t, matched)
	if err != nil {
		t.Fatalf("Resolve(matched identity): %v", err)
	}
	if accepted.State != StateProven {
		t.Fatalf("state = %q with witnesses %v, want proven under the rows' own identity", accepted.State, accepted.Witnesses)
	}
}

// TestContextResolveCompilerFailureIsOperational proves a compiler that
// cannot answer is an operational failure, never a non-proven verdict: the
// resolver reports only what it proved about a ref, and "the compiler broke"
// is not a fact about the ref.
func TestContextResolveCompilerFailureIsOperational(t *testing.T) {
	_, err := newServiceWithCompiler(&fakeCompiler{err: errors.New("compile unavailable")}).
		Resolve(context.Background(), "/fixture/root", fixtureRequest(t, fixtureAlphaRef, nil))
	if err == nil {
		t.Fatal("Resolve returned no error for a failing compiler")
	}
}

// TestContextResolveRejectsUnconstructedService proves the zero value fails
// closed rather than resolving through a nil compiler.
func TestContextResolveRejectsUnconstructedService(t *testing.T) {
	if _, err := (Service{}).Resolve(context.Background(), "/fixture/root", fixtureRequest(t, fixtureAlphaRef, nil)); err == nil {
		t.Fatal("the zero Service resolved")
	}
	if _, err := NewService().Resolve(nil, "/fixture/root", fixtureRequest(t, fixtureAlphaRef, nil)); err == nil { //nolint:staticcheck // deliberately nil
		t.Fatal("Resolve accepted a nil context")
	}
}

// --- read-only proof over a real store -----------------------------------

// resolveRepoCommitEnv pins authorship so the fixture repository's commits —
// and therefore every repository fact the real compiler reads — are identical
// on every machine.
var resolveRepoCommitEnv = []string{
	"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid",
	"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid",
	"GIT_AUTHOR_DATE=1704067200 +0000", "GIT_COMMITTER_DATE=1704067200 +0000",
}

func resolveGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), resolveRepoCommitEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// resolveRepo builds the same real, hermetic store fixture
// internal/contextcompile's own integration tests compile against: the real
// governed policy store, one accepted spec, and one committed managed
// instruction projection.
func resolveRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	files := map[string]string{}
	for _, rel := range []string{
		"constitution.md", "policies/go-toolchain.md", "overlays/frontend-go-version.md",
		"exemptions/legacy-service-go.md", "profiles/solo-default.md",
	} {
		data, err := os.ReadFile(filepath.Join("..", "policyartifact", "testdata", "store", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read policy fixture %s: %v", rel, err)
		}
		files[".verdi/policy/"+rel] = string(data)
	}
	spec, err := os.ReadFile(filepath.Join("..", "contextcompile", "testdata", "fragments", "feature-alpha.md"))
	if err != nil {
		t.Fatalf("read feature-alpha fixture: %v", err)
	}
	files[".verdi/specs/active/feature-alpha/spec.md"] = string(spec)
	files[".verdi/verdi.yaml"] = "schema: verdi.layout/v1\n"

	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "scaffold"}})
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	if _, err := instructionprojection.Generate(repo.Dir); err != nil {
		t.Fatalf("instructionprojection.Generate: %v", err)
	}
	resolveGit(t, repo.Dir, "add", "-A")
	resolveGit(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "generate instruction projection")
	repo.Head = strings.TrimSpace(resolveGit(t, repo.Dir, "rev-parse", "HEAD"))
	return repo
}

// resolveTreeSnapshot renders a directory as a deterministic string: every
// relative path with its mode, size and content digest. Comparing two
// snapshots is what turns "the query wrote nothing" into a checked statement.
func resolveTreeSnapshot(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			lines = append(lines, fmt.Sprintf("dir  %s %04o", rel, info.Mode().Perm()))
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		lines = append(lines, fmt.Sprintf("file %s %04o %d %s", rel, info.Mode().Perm(), info.Size(), hex.EncodeToString(sum[:])))
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return strings.Join(lines, "\n")
}

// TestContextResolveRealStoreIsReadOnly runs the production service over a
// real store with the real compiler and proves §2.2's read-only clause: the
// store tree, the Git worktree state, and HEAD are all byte-identical
// afterwards, and the answer is a genuine resolution rather than an echo of
// the supplied item.
func TestContextResolveRealStoreIsReadOnly(t *testing.T) {
	repo := resolveRepo(t)

	compileRequest := contextcompile.Request{
		Schema:  contextcompile.RequestSchema,
		Adapter: contextcompile.AdapterRef{ID: "codex", Version: "1"},
		Phase:   contextcompile.PhaseDesign,
		Scope: policyartifact.Scope{
			Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{},
		},
		Spec: fixtureAlphaRef,
	}
	compiled, err := contextcompile.NewCompiler().Compile(context.Background(), repo.Dir, compileRequest)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	treeBefore := resolveTreeSnapshot(t, repo.Dir)
	statusBefore := resolveGit(t, repo.Dir, "status", "--porcelain")
	headBefore := strings.TrimSpace(resolveGit(t, repo.Dir, "rev-parse", "HEAD"))

	result, err := NewService().Resolve(context.Background(), repo.Dir, Request{
		Schema: RequestSchema, Compile: compileRequest, BaseManifest: compiled.Manifest,
		Identity: fixtureIdentity(), Expansions: []Expansion{},
		Terminal: Terminal{Revision: uint64(compiled.Manifest.Revisions.Context), ManifestDigest: compiled.Manifest.Digest},
		Ref:      fixtureAlphaRef,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.State != StateProven || result.Item == nil {
		t.Fatalf("state = %q with witnesses %v, want proven with an item", result.State, result.Witnesses)
	}
	if result.Ref != fixtureAlphaRef {
		t.Fatalf("result ref = %q, want the request ref %q", result.Ref, fixtureAlphaRef)
	}
	if result.Item.Ref == nil || *result.Item.Ref != fixtureAlphaRef {
		t.Fatalf("item ref = %v, want %q", result.Item.Ref, fixtureAlphaRef)
	}
	if _, err := contextcompile.EncodeDataItem(*result.Item); err != nil {
		t.Fatalf("the resolved item is not a valid data item: %v", err)
	}

	if after := resolveTreeSnapshot(t, repo.Dir); after != treeBefore {
		t.Fatalf("the store changed\nbefore:\n%s\nafter:\n%s", treeBefore, after)
	}
	if after := resolveGit(t, repo.Dir, "status", "--porcelain"); after != statusBefore {
		t.Fatalf("git status changed: before=%q after=%q", statusBefore, after)
	}
	if after := strings.TrimSpace(resolveGit(t, repo.Dir, "rev-parse", "HEAD")); after != headBefore {
		t.Fatalf("HEAD changed: before=%s after=%s", headBefore, after)
	}
}
