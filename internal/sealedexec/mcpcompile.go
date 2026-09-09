package sealedexec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"

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

	proof, err := ProveInstalledExpansion(InstalledExpansionInput{
		Key:                  request.Snapshot.Key,
		ParentRevision:       request.Snapshot.Revision,
		ParentManifestDigest: request.Snapshot.ManifestDigest,
		Ref:                  request.Ref,
		Purpose:              request.Purpose,
		Item:                 request.Data,
		PriorExpansionRoot:   request.Snapshot.ExpansionRoot,
	})
	if err != nil {
		return ChildManifest{}, operational("prove context child transition", err)
	}

	return ChildManifest{
		Verification:         Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}},
		RequestID:            request.RequestID,
		ParentManifestDigest: request.Snapshot.ManifestDigest,
		ChildManifestDigest:  proof.ChildManifestDigest,
		ParentRevision:       request.Snapshot.Revision,
		ChildRevision:        request.Snapshot.Revision + 1,
		ExpansionDigest:      proof.ExpansionDigest,
		ExpansionRoot:        proof.ExpansionRoot,
	}, nil
}

// InstalledExpansionInput is the complete explicit operand set of one
// installed-expansion transition: the flight identity, the parent manifest
// state the transition leaves, the requested ref and request purpose, the
// canonical installed item, and the expansion root the ledger already holds.
//
// Nothing here is read from ambient state. That is the point: a durable
// install row plus the parent state it names is exactly what a restarted
// process holds, so a replay from these operands either reproduces the
// recorded identities or proves the row was rewritten.
type InstalledExpansionInput struct {
	Key                  ExecutionKey
	ParentRevision       uint64
	ParentManifestDigest string
	Ref                  string
	Purpose              string
	Item                 contextcompile.DataItem
	PriorExpansionRoot   string
}

// InstalledExpansionProof is the transition identity the operands determine.
// The child revision is deliberately absent: it is parent+1 by construction,
// and publishing it here would invite a consumer to carry a second copy.
type InstalledExpansionProof struct {
	RequestID           string
	ChildManifestDigest string
	ExpansionDigest     string
	ExpansionRoot       string
}

// ProveInstalledExpansion is the one owner of the installed-expansion
// preimages fixed by PLAN I-84 and SI-182 (VATC F12 correction §2.2).
//
// The live child compiler above and the read-only context resolver both call
// it rather than restating these four schema literals and their canonical
// preimages. A second copy is precisely the drift that would let a replay
// silently accept a rewritten lineage, so this function computes and no
// consumer re-derives.
func ProveInstalledExpansion(input InstalledExpansionInput) (InstalledExpansionProof, error) {
	if err := validateExecutionKey(input.Key); err != nil {
		return InstalledExpansionProof{}, err
	}
	for field, value := range map[string]string{"installed expansion ref": input.Ref, "installed expansion purpose": input.Purpose} {
		if err := requireText(field, value); err != nil {
			return InstalledExpansionProof{}, err
		}
	}
	if err := validateDigest("parent manifest digest", input.ParentManifestDigest); err != nil {
		return InstalledExpansionProof{}, err
	}
	if input.PriorExpansionRoot != "" {
		if err := validateDigest("prior expansion root", input.PriorExpansionRoot); err != nil {
			return InstalledExpansionProof{}, err
		}
	}
	if input.ParentRevision == math.MaxUint64 {
		return InstalledExpansionProof{}, errors.New("sealedexec: parent manifest revision cannot advance")
	}
	// The item's own ref is optional in the accepted grammar; when it is
	// present it names the same context the row requested.
	if input.Item.Ref != nil && *input.Item.Ref != input.Ref {
		return InstalledExpansionProof{}, errors.New("sealedexec: installed data item ref does not match the requested ref")
	}
	dataBytes, err := contextcompile.EncodeDataItem(input.Item)
	if err != nil {
		return InstalledExpansionProof{}, fmt.Errorf("sealedexec: encode installed expansion item: %w", err)
	}

	requestID, err := installedContextRequestID(input.Key, input.ParentRevision, input.ParentManifestDigest, input.Ref, input.Purpose)
	if err != nil {
		return InstalledExpansionProof{}, fmt.Errorf("sealedexec: digest installed expansion request: %w", err)
	}
	dataDigest := digestBytes(dataBytes)
	childRevision := input.ParentRevision + 1
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
		Schema: contextChildManifestSchema, RequestID: requestID,
		Ref: input.Ref, Purpose: input.Purpose,
		ParentRevision: input.ParentRevision, ParentManifestDigest: input.ParentManifestDigest,
		ChildRevision: childRevision, DataDigest: dataDigest,
	})
	if err != nil {
		return InstalledExpansionProof{}, fmt.Errorf("sealedexec: digest context child manifest: %w", err)
	}
	expansionDigest, err := contextExpansionDigest(contextreceipt.Expansion{
		RequestID: requestID, ParentRevision: input.ParentRevision, ParentManifestDigest: input.ParentManifestDigest,
		ChildRevision: childRevision, ChildManifestDigest: childDigest,
	}, dataDigest)
	if err != nil {
		return InstalledExpansionProof{}, fmt.Errorf("sealedexec: digest context expansion: %w", err)
	}
	expansionRoot, err := canonjson.Digest(struct {
		Schema             string `json:"schema"`
		PriorExpansionRoot string `json:"prior_expansion_root"`
		ExpansionDigest    string `json:"expansion_digest"`
	}{Schema: contextExpansionRootSchema, PriorExpansionRoot: input.PriorExpansionRoot, ExpansionDigest: expansionDigest})
	if err != nil {
		return InstalledExpansionProof{}, fmt.Errorf("sealedexec: digest context expansion root: %w", err)
	}
	return InstalledExpansionProof{
		RequestID: requestID, ChildManifestDigest: childDigest,
		ExpansionDigest: expansionDigest, ExpansionRoot: expansionRoot,
	}, nil
}

// installedContextRequestID is the accepted context-request preimage, taking
// the flight identity and parent manifest state explicitly. contextRequestID
// is the flight-state-shaped call site of this same function.
func installedContextRequestID(key ExecutionKey, revision uint64, manifestDigest, ref, purpose string) (string, error) {
	digest, err := canonjson.Digest(struct {
		Flight         string `json:"flight"`
		Lane           string `json:"lane"`
		Epoch          string `json:"epoch"`
		Revision       uint64 `json:"revision"`
		ManifestDigest string `json:"manifest_digest"`
		Ref            string `json:"ref"`
		Purpose        string `json:"purpose"`
	}{
		Flight: key.Flight, Lane: key.Lane, Epoch: key.Epoch,
		Revision: revision, ManifestDigest: manifestDigest, Ref: ref, Purpose: purpose,
	})
	if err != nil {
		return "", err
	}
	return "context-request:" + strings.TrimPrefix(digest, "sha256:"), nil
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
