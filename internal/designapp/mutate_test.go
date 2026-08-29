package designapp

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/draftmutation"
)

func mutateRequest(t *testing.T, root, branch, head string, base []byte, operations []map[string]any) draftmutation.Request {
	t.Helper()
	raw, err := canonjson.Marshal(map[string]any{
		"schema": draftmutation.RequestSchema, "spec": "spec/sample",
		"base_digest": draftmutation.DigestBytes(base), "base_spec_b64": base64.StdEncoding.EncodeToString(base),
		"expected":   map[string]any{"checkout": filepath.ToSlash(root), "branch": branch, "head": head},
		"operations": operations,
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := draftmutation.DecodeRequest(raw)
	if err != nil {
		t.Fatalf("DecodeRequest: %v", err)
	}
	return request
}

func mutateActor(t *testing.T) draftmutation.Actor {
	t.Helper()
	actor, err := draftmutation.NewDelegatedAgent("codex", "session-1")
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

func gitHead(t *testing.T, root string) string {
	t.Helper()
	// designapp's own tests never import internal/gitx's private helpers;
	// resolving HEAD once for a request's "expected" assertion is exactly
	// what draftmutation.ResolveCanonicalIdentity itself would do, so
	// reusing that public entrypoint (with the real GitIdentityReader) is
	// composition, not a second Git-resolution algorithm.
	identity, err := draftmutation.ResolveCanonicalIdentity(context.Background(), root, "spec/sample", draftmutation.GitIdentityReader{})
	if err != nil {
		t.Fatalf("resolving HEAD: %v", err)
	}
	return identity.Head
}

// TestMutateDraft proves MutateDraft is a byte-identical pass-through onto
// draftmutation.Service.Mutate: same Response/*Error union, no
// re-encoding (doc.go, mutate.go).
func TestMutateDraft(t *testing.T) {
	t.Run("happy path applies the mutation and returns the closed result", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		base := []byte(testSpec)
		req := mutateRequest(t, root, "design/sample", gitHead(t, root), base, []map[string]any{
			{"op": "set-problem", "text": "new problem", "anchor": "#problem"},
		})
		svc := NewService()
		response, typed := svc.MutateDraft(context.Background(), root, req, mutateActor(t))
		if typed != nil {
			t.Fatalf("MutateDraft: %v", typed)
		}
		if response.Result == nil || response.Stale != nil {
			t.Fatalf("MutateDraft response = %+v, want a clean Result", response)
		}
		if len(response.Result.Changes) != 1 || response.Result.Changes[0].Target != "problem" {
			t.Fatalf("MutateDraft changes = %+v", response.Result.Changes)
		}
	})

	t.Run("stale base digest is refused with zero effect", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		base := []byte(testSpec)
		req := mutateRequest(t, root, "design/sample", gitHead(t, root), base, []map[string]any{
			{"op": "set-problem", "text": "new problem", "anchor": "#problem"},
		})
		req.BaseDigest = draftmutation.DigestBytes([]byte("stale"))
		svc := NewService()
		response, typed := svc.MutateDraft(context.Background(), root, req, mutateActor(t))
		if typed == nil || typed.Code != draftmutation.CodeStaleBase || !typed.Verdict() {
			t.Fatalf("MutateDraft(stale) = %+v, %v, want verdict stale-base", response, typed)
		}
		if response.Stale == nil {
			t.Fatal("MutateDraft(stale) did not return the structured stale refusal")
		}
	})

	t.Run("mode off refuses the delegated agent", func(t *testing.T) {
		root := newTestStore(t, "off")
		base := []byte(testSpec)
		req := mutateRequest(t, root, "design/sample", gitHead(t, root), base, []map[string]any{
			{"op": "set-problem", "text": "new problem", "anchor": "#problem"},
		})
		svc := NewService()
		_, typed := svc.MutateDraft(context.Background(), root, req, mutateActor(t))
		if typed == nil || typed.Code != draftmutation.CodePolicyForbidden {
			t.Fatalf("MutateDraft(mode off) = %v, want policy-forbidden", typed)
		}
	})

	t.Run("nil mutation service is identity-invalid operational", func(t *testing.T) {
		root := newTestStore(t, "draft-write")
		base := []byte(testSpec)
		req := mutateRequest(t, root, "design/sample", gitHead(t, root), base, []map[string]any{
			{"op": "set-problem", "text": "x", "anchor": "#problem"},
		})
		svc := NewService()
		svc.Mutation = nil
		_, typed := svc.MutateDraft(context.Background(), root, req, mutateActor(t))
		if typed == nil || typed.Code != draftmutation.CodeIdentityInvalid || typed.Verdict() {
			t.Fatalf("MutateDraft(nil mutation) = %v, want operational identity-invalid", typed)
		}
	})
}
