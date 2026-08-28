package sealedexec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
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
}

func testSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
