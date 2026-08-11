package contextcompile

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuildDataItem(t *testing.T) {
	t.Parallel()

	path := "README.md"
	candidate := Candidate{Source: SourceHeadTree, ID: "path:README.md", Path: path, Object: "1234567", Mode: "100644", Type: "blob"}
	item, encoded, err := BuildDataItem(candidate, IncludedRepositoryFile, []byte("hello\n"))
	if err != nil {
		t.Fatalf("BuildDataItem() error = %v", err)
	}
	if item.Schema != DataItemSchema || item.ID != candidate.ID || item.Source != SourceHeadTree || item.Kind != IncludedRepositoryFile {
		t.Fatalf("BuildDataItem() identity = %#v", item)
	}
	if item.Path == nil || *item.Path != path || item.Ref != nil {
		t.Fatalf("BuildDataItem() path/ref = %#v/%#v", item.Path, item.Ref)
	}
	if item.Classification != DataItemClassification || item.Content != "hello\n" {
		t.Fatalf("BuildDataItem() classification/content = %q/%q", item.Classification, item.Content)
	}
	const wantContentDigest = "sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
	if item.ContentDigest != wantContentDigest {
		t.Errorf("ContentDigest = %q, want %q", item.ContentDigest, wantContentDigest)
	}
	if item.Digest == "" || !bytes.Equal(encoded[len(encoded)-1:], []byte("\n")) {
		t.Fatalf("BuildDataItem() digest/encoding = %q/%q", item.Digest, encoded)
	}
	decoded, err := DecodeDataItem(encoded)
	if err != nil {
		t.Fatalf("DecodeDataItem(BuildDataItem bytes) = %v", err)
	}
	if decoded.Digest != item.Digest || decoded.ContentDigest != item.ContentDigest || decoded.Content != item.Content {
		t.Fatalf("decoded item = %#v, want digest/content binding from %#v", decoded, item)
	}
}

func TestBuildDataItemRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		candidate Candidate
		kind      IncludedKind
		content   []byte
		want      string
	}{
		{
			name:      "invalid UTF-8",
			candidate: Candidate{Source: SourceHeadTree, ID: "path:bad.bin", Path: "bad.bin", Object: "1234567", Mode: "100644", Type: "blob"},
			kind:      IncludedRepositoryFile,
			content:   []byte{0xff},
			want:      "not text",
		},
		{
			name:      "valid UTF-8 containing NUL",
			candidate: Candidate{Source: SourceHeadTree, ID: "path:nul.txt", Path: "nul.txt", Object: "1234567", Mode: "100644", Type: "blob"},
			kind:      IncludedRepositoryFile,
			content:   []byte("valid UTF-8\x00with NUL"),
			want:      "not text",
		},
		{
			name:      "projection never gets data wrapper",
			candidate: Candidate{Source: SourceProjection, ID: "path:AGENTS.md", Path: "AGENTS.md"},
			kind:      IncludedInstructionProjection,
			content:   []byte("generated\n"),
			want:      "instruction-projection",
		},
		{
			name:      "source and kind mismatch",
			candidate: Candidate{Source: SourceDeclaredContext, ID: "ref:spec/widget", Ref: "spec/widget"},
			kind:      IncludedRepositoryFile,
			content:   []byte("context\n"),
			want:      "source",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := BuildDataItem(tt.candidate, tt.kind, tt.content); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildDataItem() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
