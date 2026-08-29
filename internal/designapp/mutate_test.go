package designapp

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/store"
)

func mutateRequest(t *testing.T, root, branch, head string, base []byte, operations []map[string]any) draftmutation.Request {
	t.Helper()
	return mutateRequestWithExcerpts(t, root, branch, head, base, operations, nil)
}

// mutateRequestWithExcerpts is mutateRequest plus AC-4's optional bounded
// supporting excerpts, so a test can seed a provenance sidecar carrying a
// real ai-inferred/unresolved classification through the typed core rather
// than hand-writing a sidecar file the core never produced.
func mutateRequestWithExcerpts(t *testing.T, root, branch, head string, base []byte, operations, excerpts []map[string]any) draftmutation.Request {
	t.Helper()
	payload := map[string]any{
		"schema": draftmutation.RequestSchema, "spec": "spec/sample",
		"base_digest": draftmutation.DigestBytes(base), "base_spec_b64": base64.StdEncoding.EncodeToString(base),
		"expected":   map[string]any{"checkout": filepath.ToSlash(root), "branch": branch, "head": head},
		"operations": operations,
	}
	if len(excerpts) > 0 {
		payload["excerpts"] = excerpts
	}
	raw, err := canonjson.Marshal(payload)
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
		specPath := store.SpecPath(root, store.ZoneActive, "sample")
		before, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatal(err)
		}
		provenancePath := store.DesignProvenancePath(root, store.ZoneActive, "sample")
		if _, statErr := os.Stat(provenancePath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("provenance sidecar exists before any mutation: %v", statErr)
		}

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
		// "no write" (CO-1's first failure-behavior bullet) is a claim about
		// BYTES, not about the returned union: prove the on-disk spec is
		// byte-identical and no provenance sidecar was created.
		after, err := os.ReadFile(specPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("stale refusal changed the on-disk spec bytes:\nbefore: %q\nafter:  %q", before, after)
		}
		if _, statErr := os.Stat(provenancePath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("stale refusal created a provenance sidecar: %v", statErr)
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
