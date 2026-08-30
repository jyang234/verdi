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

func testSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
