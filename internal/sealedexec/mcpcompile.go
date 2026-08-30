package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextreceipt"
)

const (
	contextChildManifestSchema = "verdi.context-child-manifest/v1"
	contextExpansionSchema     = "verdi.context-expansion/v1"
	contextExpansionRootSchema = "verdi.context-expansion-root/v1"
)

type canonicalChildCompiler struct{}

// NewCanonicalChildCompiler returns the sealed child compiler whose digest
// preimages are fixed by PLAN I-84.
func NewCanonicalChildCompiler() ChildCompiler { return canonicalChildCompiler{} }

func (canonicalChildCompiler) CompileChild(ctx context.Context, request ChildCompileRequest) (ChildManifest, error) {
	if ctx == nil {
		return ChildManifest{}, operational("compile context child", errors.New("nil context"))
	}
	if err := validateChildCompileRequest(request); err != nil {
		return ChildManifest{}, operational("compile context child", err)
	}

	dataBytes, err := contextcompile.EncodeDataItem(request.Data)
	if err != nil {
		return ChildManifest{}, operational("encode context child data", err)
	}
	decoded, err := contextcompile.DecodeDataItem(dataBytes)
	if err != nil {
		return ChildManifest{}, operational("round-trip context child data", err)
	}
	roundTrip, err := contextcompile.EncodeDataItem(decoded)
	if err != nil {
		return ChildManifest{}, operational("re-encode context child data", err)
	}
	if !reflect.DeepEqual(decoded, request.Data) || !bytes.Equal(roundTrip, dataBytes) {
		return ChildManifest{}, operational("round-trip context child data", errors.New("data item does not round-trip exactly"))
	}

	dataDigest := digestBytes(dataBytes)
	childRevision := request.Snapshot.Revision + 1
	childDigest, err := canonjson.Digest(struct {
		Schema               string `json:"schema"`
		RequestID            string `json:"request_id"`
		Ref                  string `json:"ref"`
		Purpose              string `json:"purpose"`
		ParentRevision       uint64 `json:"parent_revision"`
		ParentManifestDigest string `json:"parent_manifest_digest"`
		ChildRevision        uint64 `json:"child_revision"`
		DataDigest           string `json:"data_digest"`
	}{
		Schema: contextChildManifestSchema, RequestID: request.RequestID,
		Ref: request.Ref, Purpose: request.Purpose,
		ParentRevision: request.Snapshot.Revision, ParentManifestDigest: request.Snapshot.ManifestDigest,
		ChildRevision: childRevision, DataDigest: dataDigest,
	})
	if err != nil {
		return ChildManifest{}, operational("digest context child manifest", err)
	}
	expansionDigest, err := contextExpansionDigest(contextreceipt.Expansion{
		RequestID: request.RequestID, ParentRevision: request.Snapshot.Revision, ParentManifestDigest: request.Snapshot.ManifestDigest,
		ChildRevision: childRevision, ChildManifestDigest: childDigest,
	}, dataDigest)
	if err != nil {
		return ChildManifest{}, operational("digest context expansion", err)
	}
	expansionRoot, err := canonjson.Digest(struct {
		Schema             string `json:"schema"`
		PriorExpansionRoot string `json:"prior_expansion_root"`
		ExpansionDigest    string `json:"expansion_digest"`
	}{Schema: contextExpansionRootSchema, PriorExpansionRoot: request.Snapshot.ExpansionRoot, ExpansionDigest: expansionDigest})
	if err != nil {
		return ChildManifest{}, operational("digest context expansion root", err)
	}

	return ChildManifest{
		Verification:         Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
		RequestID:            request.RequestID,
		ParentManifestDigest: request.Snapshot.ManifestDigest,
		ChildManifestDigest:  childDigest,
		ParentRevision:       request.Snapshot.Revision,
		ChildRevision:        childRevision,
		ExpansionDigest:      expansionDigest,
		ExpansionRoot:        expansionRoot,
	}, nil
}

func contextExpansionDigest(expansion contextreceipt.Expansion, dataDigest string) (string, error) {
	return canonjson.Digest(struct {
		Schema               string `json:"schema"`
		RequestID            string `json:"request_id"`
		ParentRevision       uint64 `json:"parent_revision"`
		ParentManifestDigest string `json:"parent_manifest_digest"`
		ChildRevision        uint64 `json:"child_revision"`
		ChildManifestDigest  string `json:"child_manifest_digest"`
		DataDigest           string `json:"data_digest"`
	}{
		Schema: contextExpansionSchema, RequestID: expansion.RequestID,
		ParentRevision: expansion.ParentRevision, ParentManifestDigest: expansion.ParentManifestDigest,
		ChildRevision: expansion.ChildRevision, ChildManifestDigest: expansion.ChildManifestDigest, DataDigest: dataDigest,
	})
}

// VerifyExpansionDataProof strict-decodes one canonical DataItem and
// recomputes the existing I-84 expansion preimage without consulting state.
func VerifyExpansionDataProof(data []byte, expansion contextreceipt.Expansion) (contextreceipt.ExpansionProofProjection, error) {
	item, err := contextcompile.DecodeDataItem(data)
	if err != nil {
		return contextreceipt.ExpansionProofProjection{}, fmt.Errorf("sealedexec: decode expansion data proof: %w", err)
	}
	dataDigest := digestBytes(data)
	expansionDigest, err := contextExpansionDigest(expansion, dataDigest)
	if err != nil {
		return contextreceipt.ExpansionProofProjection{}, fmt.Errorf("sealedexec: digest expansion data proof: %w", err)
	}
	return contextreceipt.ExpansionProofProjection{DataItemDigest: item.Digest, DataDigest: dataDigest, ExpansionDigest: expansionDigest}, nil
}

func validateChildCompileRequest(request ChildCompileRequest) error {
	if err := requireText("request_id", request.RequestID); err != nil {
		return err
	}
	if err := requireText("ref", request.Ref); err != nil {
		return err
	}
	if err := requireText("purpose", request.Purpose); err != nil {
		return err
	}
	snapshot := request.Snapshot
	if snapshot.Invalidated {
		return errors.New("current flight state is invalidated")
	}
	if _, err := flightSnapshotToWire(snapshot); err != nil {
		return fmt.Errorf("invalid current flight-state snapshot: %w", err)
	}
	if err := validateExecutionKey(snapshot.Key); err != nil {
		return err
	}
	if snapshot.Key != executionKey(snapshot.Request) {
		return errors.New("current flight-state key does not match the public execution request")
	}
	if err := requireText("workspace_id", snapshot.WorkspaceID); err != nil {
		return err
	}
	if err := validateGitOID("candidate_commit", snapshot.CandidateCommit, true); err != nil {
		return err
	}
	if err := validateGitOID("candidate_tree", snapshot.CandidateTree, false); err != nil {
		return err
	}
	if snapshot.NextSourceSequence == 0 {
		return errors.New("next source sequence must be positive")
	}
	if snapshot.Revision == math.MaxUint64 {
		return errors.New("current manifest revision cannot advance")
	}
	if err := validateDigest("public request manifest digest", snapshot.Request.ManifestDigest); err != nil {
		return err
	}
	if err := validateDigest("public request projection digest", snapshot.Request.ProjectionDigest); err != nil {
		return err
	}
	if err := validateDigest("current manifest digest", snapshot.ManifestDigest); err != nil {
		return err
	}
	if snapshot.ProjectionDigest != snapshot.Request.ProjectionDigest {
		return errors.New("current projection digest does not match the public execution request")
	}
	if snapshot.Revision < snapshot.Request.ManifestRevision {
		return errors.New("current manifest revision is below the public execution request")
	}
	if snapshot.Revision == snapshot.Request.ManifestRevision {
		if snapshot.ManifestDigest != snapshot.Request.ManifestDigest {
			return errors.New("first expansion manifest digest does not match the public execution request")
		}
		// I-115: at the public request revision a start has proven a pristine
		// recorder and therefore an empty expansion ledger, while a resume
		// continues an authenticated flight whose ledger root the continuity
		// already names. The two arms are distinguished by the request itself
		// rather than by trusting whichever root the state happens to hold.
		if snapshot.Request.Action == ActionResume {
			if snapshot.Request.Resume == nil {
				return errors.New("resume expansion requires the canonical resume arm")
			}
			if err := validateDigest("resumed prior expansion root", snapshot.ExpansionRoot); err != nil {
				return err
			}
			if snapshot.ExpansionRoot != snapshot.Request.Resume.Continuity.ExpansionLedgerRoot {
				return errors.New("resumed prior expansion root does not match the request continuity")
			}
		} else if snapshot.ExpansionRoot != "" {
			return errors.New("first expansion requires an empty prior expansion root")
		}
	} else {
		if snapshot.ExpansionRoot == "" {
			return errors.New("later expansion requires a prior expansion root")
		}
		if err := validateDigest("prior expansion root", snapshot.ExpansionRoot); err != nil {
			return err
		}
	}
	wantRequestID, err := contextRequestID(snapshot, request.Ref, request.Purpose)
	if err != nil {
		return fmt.Errorf("derive request id: %w", err)
	}
	if request.RequestID != wantRequestID {
		return errors.New("request id does not match the current flight state")
	}
	return nil
}
