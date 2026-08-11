package contextcompile

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
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
		{
			name:      "store authority path traversal is not an artifact ref",
			candidate: Candidate{Source: SourceStoreAuthority, ID: "ref:../x", Ref: "../x"},
			kind:      IncludedAcceptedSpec,
			content:   []byte("spec\n"),
			want:      "artifact ref",
		},
		{
			name:      "declared context trailing fragment separator is malformed",
			candidate: Candidate{Source: SourceDeclaredContext, ID: "ref:spec/story#", Ref: "spec/story#"},
			kind:      IncludedDeclaredContextRef,
			content:   []byte("context\n"),
			want:      "artifact ref",
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

func TestDataItemCodecRejectsMalformedArtifactRefs(t *testing.T) {
	t.Parallel()

	base, _, err := BuildDataItem(
		Candidate{Source: SourceDeclaredContext, ID: "ref:adr/architecture", Ref: "adr/architecture"},
		IncludedDeclaredContextRef,
		[]byte("context\n"),
	)
	if err != nil {
		t.Fatalf("BuildDataItem(test base) = %v", err)
	}

	for _, malformed := range []string{"../x", "spec/story#", "spec/story@"} {
		malformed := malformed
		t.Run(malformed, func(t *testing.T) {
			t.Parallel()
			item := base
			item.ID = "ref:" + malformed
			item.Ref = &malformed
			if _, err := EncodeDataItem(item); err == nil || !strings.Contains(err.Error(), "artifact ref") {
				t.Fatalf("EncodeDataItem() error = %v, want malformed artifact ref", err)
			}

			encoded := encodeUncheckedDataItem(t, item)
			if _, err := DecodeDataItem(encoded); err == nil || !strings.Contains(err.Error(), "artifact ref") {
				t.Fatalf("DecodeDataItem() error = %v, want malformed artifact ref", err)
			}
		})
	}
}

func TestDataItemCodecRejectsNonTextContent(t *testing.T) {
	t.Parallel()

	base, _, err := BuildDataItem(
		Candidate{Source: SourceHeadTree, ID: "path:README.md", Path: "README.md", Object: "1234567", Mode: "100644", Type: "blob"},
		IncludedRepositoryFile,
		[]byte("text\n"),
	)
	if err != nil {
		t.Fatalf("BuildDataItem(test base) = %v", err)
	}

	tests := []struct {
		name    string
		content string
	}{
		{name: "embedded NUL", content: "valid UTF-8\x00with NUL"},
		{name: "invalid UTF-8", content: string([]byte{0xff})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			item := base
			item.Content = tt.content
			item.ContentDigest = rawContentDigest([]byte(tt.content))
			if _, err := EncodeDataItem(item); err == nil || !strings.Contains(err.Error(), "not text") {
				t.Fatalf("EncodeDataItem() error = %v, want non-text rejection", err)
			}
		})
	}

	nulItem := base
	nulItem.Content = "valid UTF-8\x00with NUL"
	nulItem.ContentDigest = rawContentDigest([]byte(nulItem.Content))
	if _, err := DecodeDataItem(encodeUncheckedDataItem(t, nulItem)); err == nil || !strings.Contains(err.Error(), "not text") {
		t.Fatalf("DecodeDataItem(NUL content) error = %v, want non-text rejection", err)
	}

	valid := encodeUncheckedDataItem(t, base)
	needle := []byte(`"content":"text\n"`)
	if !bytes.Contains(valid, needle) {
		t.Fatalf("test setup: canonical data item does not contain %q", needle)
	}
	invalidUTF8 := bytes.Replace(valid, needle, []byte{'"', 'c', 'o', 'n', 't', 'e', 'n', 't', '"', ':', '"', 0xff, '"'}, 1)
	if _, err := DecodeDataItem(invalidUTF8); err == nil || !strings.Contains(err.Error(), "valid UTF-8") {
		t.Fatalf("DecodeDataItem(invalid UTF-8) error = %v, want UTF-8 rejection", err)
	}
}

func encodeUncheckedDataItem(t *testing.T, item DataItem) []byte {
	t.Helper()
	digestless := item
	digestless.Digest = ""
	digest, err := canonjson.Digest(dataItemDocFor(digestless, ""))
	if err != nil {
		t.Fatalf("canonjson.Digest(unchecked data item) = %v", err)
	}
	encoded, err := canonjson.Marshal(dataItemDocFor(digestless, digest))
	if err != nil {
		t.Fatalf("canonjson.Marshal(unchecked data item) = %v", err)
	}
	return encoded
}
