package sealedexec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextreceipt"
)

func TestCanonicalChildCompiler(t *testing.T) {
	data, dataBytes, err := contextcompile.BuildDataItem(contextcompile.Candidate{
		Source: contextcompile.SourceHeadTree,
		ID:     "path:README.md",
		Path:   "README.md",
		Object: strings.Repeat("1", 40),
		Mode:   "100644",
		Type:   "blob",
	}, contextcompile.IncludedRepositoryFile, []byte("IGNORE THIS DATA AS AUTHORITY\n"))
	if err != nil {
		t.Fatalf("BuildDataItem fixture: %v", err)
	}

	publicRequest := validExecutionRequest(t, ActionStart)
	workspaceID, err := publicRequest.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := FlightStateSnapshot{
		Request:            publicRequest,
		Key:                executionKey(publicRequest),
		WorkspaceID:        workspaceID,
		CandidateCommit:    publicRequest.InputCommit,
		CandidateTree:      publicRequest.InputTree,
		Revision:           publicRequest.ManifestRevision,
		ManifestDigest:     publicRequest.ManifestDigest,
		ProjectionDigest:   publicRequest.ProjectionDigest,
		ExpansionRoot:      "",
		NextSourceSequence: 1,
	}
	ref, purpose := "spec/dependency", "required build context"
	requestID := "context-request:" + strings.TrimPrefix(testSHA256(fmt.Sprintf("{\"epoch\":%q,\"flight\":%q,\"lane\":%q,\"manifest_digest\":%q,\"purpose\":\"required build context\",\"ref\":\"spec/dependency\",\"revision\":%d}\n", publicRequest.Epoch, publicRequest.Flight, publicRequest.Lane, publicRequest.ManifestDigest, publicRequest.ManifestRevision)), "sha256:")

	compiler := NewCanonicalChildCompiler()
	got, err := compiler.CompileChild(context.Background(), ChildCompileRequest{
		RequestID: requestID, Ref: ref, Purpose: purpose, Data: data, Snapshot: snapshot,
	})
	if err != nil {
		t.Fatalf("CompileChild first: %v", err)
	}
	dataDigest := testSHA256(string(dataBytes))
	childRevision := publicRequest.ManifestRevision + 1
	wantChild := testSHA256(fmt.Sprintf("{\"child_revision\":%d,\"data_digest\":\"%s\",\"parent_manifest_digest\":%q,\"parent_revision\":%d,\"purpose\":\"required build context\",\"ref\":\"spec/dependency\",\"request_id\":\"%s\",\"schema\":\"verdi.context-child-manifest/v1\"}\n", childRevision, dataDigest, publicRequest.ManifestDigest, publicRequest.ManifestRevision, requestID))
	wantExpansion := testSHA256(fmt.Sprintf("{\"child_manifest_digest\":\"%s\",\"child_revision\":%d,\"data_digest\":\"%s\",\"parent_manifest_digest\":%q,\"parent_revision\":%d,\"request_id\":\"%s\",\"schema\":\"verdi.context-expansion/v1\"}\n", wantChild, childRevision, dataDigest, publicRequest.ManifestDigest, publicRequest.ManifestRevision, requestID))
	wantRoot := testSHA256(fmt.Sprintf("{\"expansion_digest\":\"%s\",\"prior_expansion_root\":\"\",\"schema\":\"verdi.context-expansion-root/v1\"}\n", wantExpansion))
	if got.State != contextcompile.ResolutionProven || got.Failure != FailureNone || len(got.Witnesses) != 0 ||
		got.RequestID != requestID || got.ParentRevision != publicRequest.ManifestRevision || got.ChildRevision != childRevision ||
		got.ParentManifestDigest != publicRequest.ManifestDigest || got.ChildManifestDigest != wantChild ||
		got.ExpansionDigest != wantExpansion || got.ExpansionRoot != wantRoot {
		t.Fatalf("first child = %#v, want exact I-84 preimages", got)
	}

	later := snapshot
	later.Revision = childRevision
	later.ManifestDigest = wantChild
	later.ExpansionRoot = wantRoot
	later.NextSourceSequence = 1
	laterRequestID := "context-request:" + strings.TrimPrefix(testSHA256(fmt.Sprintf("{\"epoch\":%q,\"flight\":%q,\"lane\":%q,\"manifest_digest\":\"%s\",\"purpose\":\"required build context\",\"ref\":\"spec/dependency\",\"revision\":%d}\n", publicRequest.Epoch, publicRequest.Flight, publicRequest.Lane, wantChild, childRevision)), "sha256:")
	laterChild, err := compiler.CompileChild(context.Background(), ChildCompileRequest{
		RequestID: laterRequestID, Ref: ref, Purpose: purpose, Data: data, Snapshot: later,
	})
	if err != nil {
		t.Fatalf("CompileChild later: %v", err)
	}
	wantLaterRoot := testSHA256(fmt.Sprintf("{\"expansion_digest\":\"%s\",\"prior_expansion_root\":\"%s\",\"schema\":\"verdi.context-expansion-root/v1\"}\n", laterChild.ExpansionDigest, wantRoot))
	if laterChild.ParentRevision != childRevision || laterChild.ChildRevision != childRevision+1 || laterChild.ExpansionRoot != wantLaterRoot {
		t.Fatalf("later child = %#v, want accumulator rooted at %s", laterChild, wantRoot)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*ChildCompileRequest)
	}{
		{"nil context", nil},
		{"wrong request id", func(in *ChildCompileRequest) { in.RequestID += "x" }},
		{"invalidated snapshot", func(in *ChildCompileRequest) { in.Snapshot.Invalidated = true }},
		{"invalid public request", func(in *ChildCompileRequest) { in.Snapshot.Request.Schema = "" }},
		{"key mismatch", func(in *ChildCompileRequest) { in.Snapshot.Key.Epoch = "other" }},
		{"revision below request", func(in *ChildCompileRequest) { in.Snapshot.Revision = in.Snapshot.Request.ManifestRevision - 1 }},
		{"nonempty first root", func(in *ChildCompileRequest) { in.Snapshot.ExpansionRoot = wantRoot }},
		{"empty later root", func(in *ChildCompileRequest) {
			in.Snapshot.Revision = childRevision
			in.Snapshot.ManifestDigest = wantChild
		}},
		{"malformed later root", func(in *ChildCompileRequest) {
			in.Snapshot.Revision = childRevision
			in.Snapshot.ManifestDigest = wantChild
			in.Snapshot.ExpansionRoot = "relative"
		}},
		{"request manifest mismatch", func(in *ChildCompileRequest) { in.Snapshot.ManifestDigest = wantChild }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := ChildCompileRequest{RequestID: requestID, Ref: ref, Purpose: purpose, Data: data, Snapshot: snapshot}
			ctx := context.Background()
			if tc.mutate == nil {
				ctx = nil
			} else {
				tc.mutate(&in)
			}
			if _, err := compiler.CompileChild(ctx, in); err == nil {
				t.Fatal("CompileChild accepted invalid transition")
			}
		})
	}

	// I-115: at the public request revision a start has proven a pristine
	// recorder and an empty ledger, while a resume continues the authenticated
	// ledger its continuity names. The prior root is therefore read from the
	// request arm, not from whatever the current state happens to carry.
	t.Run("expansion at the request revision is action aware", func(t *testing.T) {
		resumeRequest := serviceRequest(t, ActionResume)
		resumeWorkspaceID, err := resumeRequest.ExecutionWorkspaceRequest.WorkspaceID()
		if err != nil {
			t.Fatal(err)
		}
		ledgerRoot := resumeRequest.Resume.Continuity.ExpansionLedgerRoot
		if ledgerRoot == "" {
			t.Fatal("resume fixture has no installed expansion ledger root")
		}
		base := FlightStateSnapshot{
			Request: resumeRequest, Key: executionKey(resumeRequest), WorkspaceID: resumeWorkspaceID,
			CandidateCommit: resumeRequest.InputCommit, CandidateTree: resumeRequest.InputTree,
			Revision: resumeRequest.ManifestRevision, ManifestDigest: resumeRequest.ManifestDigest,
			ProjectionDigest: resumeRequest.ProjectionDigest, NextSourceSequence: 4,
		}
		for _, tc := range []struct {
			name    string
			action  Action
			root    string
			wantErr bool
		}{
			{name: "start-empty", action: ActionStart, root: ""},
			{name: "start-nonempty", action: ActionStart, root: ledgerRoot, wantErr: true},
			{name: "resume-exact", action: ActionResume, root: ledgerRoot},
			{name: "resume-empty", action: ActionResume, root: "", wantErr: true},
			{name: "resume-mismatch", action: ActionResume, root: testDigest("other-ledger-root"), wantErr: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				in := base
				if tc.action == ActionStart {
					in.Request = publicRequest
					in.Key = executionKey(publicRequest)
					in.WorkspaceID = workspaceID
					in.CandidateCommit, in.CandidateTree = publicRequest.InputCommit, publicRequest.InputTree
					in.Revision, in.ManifestDigest = publicRequest.ManifestRevision, publicRequest.ManifestDigest
					in.ProjectionDigest = publicRequest.ProjectionDigest
				}
				in.ExpansionRoot = tc.root
				id, err := contextRequestID(in, ref, purpose)
				if err != nil {
					t.Fatal(err)
				}
				_, err = compiler.CompileChild(context.Background(), ChildCompileRequest{
					RequestID: id, Ref: ref, Purpose: purpose, Data: data, Snapshot: in,
				})
				if tc.wantErr && err == nil {
					t.Fatal("CompileChild accepted a prior expansion root the request does not authorize")
				}
				if !tc.wantErr && err != nil {
					t.Fatalf("CompileChild: %v", err)
				}
			})
		}
	})
}

func TestVerifyExpansionDataProof(t *testing.T) {
	data, dataBytes, err := contextcompile.BuildDataItem(contextcompile.Candidate{
		Source: contextcompile.SourceHeadTree, ID: "path:README.md", Path: "README.md",
		Object: strings.Repeat("1", 40), Mode: "100644", Type: "blob",
	}, contextcompile.IncludedRepositoryFile, []byte("expansion proof\n"))
	if err != nil {
		t.Fatal(err)
	}
	dataDigest := testSHA256(string(dataBytes))
	expansion := contextreceipt.Expansion{
		RequestID: "context-request:fixture", ParentRevision: 2, ParentManifestDigest: testSHA256("parent"),
		ChildRevision: 3, ChildManifestDigest: testSHA256("child"),
	}
	expansion.ExpansionDigest = testSHA256(fmt.Sprintf("{\"child_manifest_digest\":\"%s\",\"child_revision\":3,\"data_digest\":\"%s\",\"parent_manifest_digest\":\"%s\",\"parent_revision\":2,\"request_id\":\"context-request:fixture\",\"schema\":\"verdi.context-expansion/v1\"}\n", expansion.ChildManifestDigest, dataDigest, expansion.ParentManifestDigest))

	projection, err := VerifyExpansionDataProof(dataBytes, expansion)
	if err != nil {
		t.Fatalf("VerifyExpansionDataProof: %v", err)
	}
	if projection.DataDigest != dataDigest || projection.ExpansionDigest != expansion.ExpansionDigest || projection.DataItemDigest != data.Digest {
		t.Fatalf("projection = %#v", projection)
	}
	if _, err := VerifyExpansionDataProof(append(dataBytes, []byte("{}\n")...), expansion); err == nil {
		t.Fatal("VerifyExpansionDataProof accepted trailing data")
	}
}

// TestProveInstalledExpansion freezes Task 2A's single owning pure proof of an
// installed expansion (SI-177, correction §2.2). The helper takes explicit
// flight identity, parent state, the requested ref, the request purpose, the
// canonical installed item, and the prior expansion root, and returns the
// request id, child-manifest digest, expansion digest, and next root.
//
// Every expectation below is written as the literal I-84 preimage rather than
// by calling the helper twice, so a helper that agreed with itself but not with
// the accepted transition algebra would fail here. The point of one owner is
// that the live child compiler and the Task 3 resolver replay the SAME bytes:
// a second copy of these preimages is exactly the drift this test forbids.
func TestProveInstalledExpansion(t *testing.T) {
	request := validExecutionRequest(t, ActionStart)
	key := executionKey(request)
	item, itemBytes, err := contextcompile.BuildDataItem(contextcompile.Candidate{
		Source: contextcompile.SourceHeadTree, ID: "path:README.md", Path: "README.md",
		Object: strings.Repeat("1", 40), Mode: "100644", Type: "blob",
	}, contextcompile.IncludedRepositoryFile, []byte("installed expansion data\n"))
	if err != nil {
		t.Fatalf("BuildDataItem fixture: %v", err)
	}
	ref, purpose := "spec/dependency", "required build context"
	input := InstalledExpansionInput{
		Key:                  key,
		ParentRevision:       request.ManifestRevision,
		ParentManifestDigest: request.ManifestDigest,
		Ref:                  ref,
		Purpose:              purpose,
		Item:                 item,
		PriorExpansionRoot:   "",
	}

	dataDigest := testSHA256(string(itemBytes))
	childRevision := request.ManifestRevision + 1
	wantRequestID := "context-request:" + strings.TrimPrefix(testSHA256(fmt.Sprintf(
		"{\"epoch\":%q,\"flight\":%q,\"lane\":%q,\"manifest_digest\":%q,\"purpose\":%q,\"ref\":%q,\"revision\":%d}\n",
		key.Epoch, key.Flight, key.Lane, request.ManifestDigest, purpose, ref, request.ManifestRevision)), "sha256:")
	wantChild := testSHA256(fmt.Sprintf(
		"{\"child_revision\":%d,\"data_digest\":%q,\"parent_manifest_digest\":%q,\"parent_revision\":%d,\"purpose\":%q,\"ref\":%q,\"request_id\":%q,\"schema\":\"verdi.context-child-manifest/v1\"}\n",
		childRevision, dataDigest, request.ManifestDigest, request.ManifestRevision, purpose, ref, wantRequestID))
	wantExpansion := testSHA256(fmt.Sprintf(
		"{\"child_manifest_digest\":%q,\"child_revision\":%d,\"data_digest\":%q,\"parent_manifest_digest\":%q,\"parent_revision\":%d,\"request_id\":%q,\"schema\":\"verdi.context-expansion/v1\"}\n",
		wantChild, childRevision, dataDigest, request.ManifestDigest, request.ManifestRevision, wantRequestID))
	wantRoot := testSHA256(fmt.Sprintf(
		"{\"expansion_digest\":%q,\"prior_expansion_root\":\"\",\"schema\":\"verdi.context-expansion-root/v1\"}\n",
		wantExpansion))

	proof, err := ProveInstalledExpansion(input)
	if err != nil {
		t.Fatalf("ProveInstalledExpansion: %v", err)
	}
	if proof.RequestID != wantRequestID || proof.ChildManifestDigest != wantChild ||
		proof.ExpansionDigest != wantExpansion || proof.ExpansionRoot != wantRoot {
		t.Fatalf("proof = %#v, want the exact accepted transition preimages", proof)
	}

	// Restart replay: the proof is a pure function of its declared operands, so
	// a later process that holds only the durable install row reconstructs the
	// identical transition rather than an ambiently recompiled one.
	again, err := ProveInstalledExpansion(input)
	if err != nil || !sameInstalledExpansionProof(again, proof) {
		t.Fatalf("replayed proof = %#v/%v, want the identical pure result", again, err)
	}

	// A later expansion accumulates onto the exact prior root.
	later := input
	later.ParentRevision = childRevision
	later.ParentManifestDigest = wantChild
	later.PriorExpansionRoot = wantRoot
	laterProof, err := ProveInstalledExpansion(later)
	if err != nil {
		t.Fatalf("ProveInstalledExpansion(later): %v", err)
	}
	wantLaterRoot := testSHA256(fmt.Sprintf(
		"{\"expansion_digest\":%q,\"prior_expansion_root\":%q,\"schema\":\"verdi.context-expansion-root/v1\"}\n",
		laterProof.ExpansionDigest, wantRoot))
	if laterProof.ExpansionRoot != wantLaterRoot || laterProof.RequestID == proof.RequestID {
		t.Fatalf("later proof = %#v, want a distinct request rooted at %s", laterProof, wantRoot)
	}

	// The four members are not independently supplied facts: changing any
	// operand the preimages bind must move the proof. A helper that ignored one
	// of them would let a restart replay accept a rewritten row.
	changedItem, _, err := contextcompile.BuildDataItem(contextcompile.Candidate{
		Source: contextcompile.SourceHeadTree, ID: "path:README.md", Path: "README.md",
		Object: strings.Repeat("1", 40), Mode: "100644", Type: "blob",
	}, contextcompile.IncludedRepositoryFile, []byte("other installed expansion data\n"))
	if err != nil {
		t.Fatalf("BuildDataItem changed fixture: %v", err)
	}
	for _, tc := range []struct {
		name          string
		mutate        func(*InstalledExpansionInput)
		sameRequestID bool
	}{
		{name: "changed purpose", mutate: func(in *InstalledExpansionInput) { in.Purpose = "another purpose" }},
		{name: "changed ref", mutate: func(in *InstalledExpansionInput) { in.Ref = "spec/other" }},
		{name: "changed parent manifest", mutate: func(in *InstalledExpansionInput) {
			in.ParentManifestDigest = testDigest("other-parent")
		}},
		{name: "changed flight", mutate: func(in *InstalledExpansionInput) { in.Key.Flight = "flight-2" }},
		{name: "changed lane", mutate: func(in *InstalledExpansionInput) { in.Key.Lane = "lane-2" }},
		{name: "changed epoch", mutate: func(in *InstalledExpansionInput) { in.Key.Epoch = "epoch-2" }},
		{name: "changed item", mutate: func(in *InstalledExpansionInput) { in.Item = changedItem }, sameRequestID: true},
		{name: "changed prior root", mutate: func(in *InstalledExpansionInput) {
			in.PriorExpansionRoot = testDigest("other-prior-root")
		}, sameRequestID: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := input
			tc.mutate(&in)
			got, err := ProveInstalledExpansion(in)
			if err != nil {
				t.Fatalf("ProveInstalledExpansion: %v", err)
			}
			if sameInstalledExpansionProof(got, proof) {
				t.Fatal("a changed operand produced the identical installed-expansion proof")
			}
			if got.ExpansionRoot == proof.ExpansionRoot {
				t.Fatal("a changed operand left the next expansion root unchanged")
			}
			if tc.sameRequestID != (got.RequestID == proof.RequestID) {
				t.Fatalf("request id = %q, want sameRequestID=%v", got.RequestID, tc.sameRequestID)
			}
		})
	}

	// The optional data-item ref is part of the accepted grammar: an item that
	// carries one must carry the requested ref, and an item that carries none
	// remains provable.
	t.Run("optional item ref", func(t *testing.T) {
		refItem, _, err := contextcompile.BuildDataItem(contextcompile.Candidate{
			Source: contextcompile.SourceDeclaredContext, ID: "ref:" + ref, Ref: ref,
		}, contextcompile.IncludedDeclaredContextRef, []byte("declared context bytes\n"))
		if err != nil {
			t.Fatalf("BuildDataItem declared-context fixture: %v", err)
		}
		if refItem.Ref == nil || *refItem.Ref != ref {
			t.Fatalf("declared-context fixture ref = %v, want %q", refItem.Ref, ref)
		}
		matching := input
		matching.Item = refItem
		if _, err := ProveInstalledExpansion(matching); err != nil {
			t.Fatalf("ProveInstalledExpansion(item carrying the requested ref): %v", err)
		}
		mismatched := matching
		mismatched.Ref = "spec/other"
		if _, err := ProveInstalledExpansion(mismatched); err == nil {
			t.Fatal("proved an installed expansion whose item ref contradicts the requested ref")
		}
		if _, err := ProveInstalledExpansion(input); err != nil {
			t.Fatalf("ProveInstalledExpansion(item without a ref): %v", err)
		}
	})

	for _, tc := range []struct {
		name   string
		mutate func(*InstalledExpansionInput)
	}{
		{"missing ref", func(in *InstalledExpansionInput) { in.Ref = "" }},
		{"padded ref", func(in *InstalledExpansionInput) { in.Ref = " " + ref }},
		{"missing purpose", func(in *InstalledExpansionInput) { in.Purpose = "" }},
		{"padded purpose", func(in *InstalledExpansionInput) { in.Purpose = purpose + " " }},
		{"missing flight", func(in *InstalledExpansionInput) { in.Key.Flight = "" }},
		{"missing lane", func(in *InstalledExpansionInput) { in.Key.Lane = "" }},
		{"missing epoch", func(in *InstalledExpansionInput) { in.Key.Epoch = "" }},
		{"malformed parent manifest", func(in *InstalledExpansionInput) { in.ParentManifestDigest = "relative" }},
		{"malformed prior root", func(in *InstalledExpansionInput) { in.PriorExpansionRoot = "relative" }},
		{"zero item", func(in *InstalledExpansionInput) { in.Item = contextcompile.DataItem{} }},
		{"item declaring a foreign schema", func(in *InstalledExpansionInput) {
			foreign := item
			foreign.Schema = "verdi.other-item/v1"
			in.Item = foreign
		}},
		{"item with no content digest", func(in *InstalledExpansionInput) {
			incomplete := item
			incomplete.ContentDigest = ""
			in.Item = incomplete
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := input
			tc.mutate(&in)
			if _, err := ProveInstalledExpansion(in); err == nil {
				t.Fatal("ProveInstalledExpansion accepted an invalid installed-expansion operand")
			}
		})
	}

	// One owner: the live child compiler answers with exactly the helper's
	// digests, so no consumer restates these schema literals or preimages.
	t.Run("live child compiler uses the owning helper", func(t *testing.T) {
		workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
		if err != nil {
			t.Fatal(err)
		}
		snapshot := FlightStateSnapshot{
			Request: request, Key: key, WorkspaceID: workspaceID,
			CandidateCommit: request.InputCommit, CandidateTree: request.InputTree,
			Revision: request.ManifestRevision, ManifestDigest: request.ManifestDigest,
			ProjectionDigest: request.ProjectionDigest, ExpansionRoot: "", NextSourceSequence: 1,
		}
		child, err := NewCanonicalChildCompiler().CompileChild(context.Background(), ChildCompileRequest{
			RequestID: proof.RequestID, Ref: ref, Purpose: purpose, Data: item, Snapshot: snapshot,
		})
		if err != nil {
			t.Fatalf("CompileChild: %v", err)
		}
		if child.RequestID != proof.RequestID || child.ChildManifestDigest != proof.ChildManifestDigest ||
			child.ExpansionDigest != proof.ExpansionDigest || child.ExpansionRoot != proof.ExpansionRoot {
			t.Fatalf("compiled child = %#v, want the owning helper's proof %#v", child, proof)
		}
	})
}

// sameInstalledExpansionProof compares the four members the helper owns
// without requiring the proof value itself to stay comparable.
func sameInstalledExpansionProof(a, b InstalledExpansionProof) bool {
	return a.RequestID == b.RequestID && a.ChildManifestDigest == b.ChildManifestDigest &&
		a.ExpansionDigest == b.ExpansionDigest && a.ExpansionRoot == b.ExpansionRoot
}

func testSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
