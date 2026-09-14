package specimport

import (
	"bytes"
	"errors"
	"testing"
)

func TestDecodeRequest_AcceptsWellFormedRequest(t *testing.T) {
	req := minimalRequest()
	data := mustJSON(req)

	got, err := DecodeRequest(data)
	if err != nil {
		t.Fatalf("DecodeRequest: unexpected error: %v", err)
	}
	if got.Schema != RequestSchema {
		t.Fatalf("schema = %q, want %q", got.Schema, RequestSchema)
	}
	if got.Target.Slug != "sample-feature" || got.Primary != "source" {
		t.Fatalf("decoded request lost fields: %+v", got)
	}
	if !bytes.Equal(got.Sources[0].Data, []byte(validMarkdownSource)) {
		t.Fatalf("decoded source data does not round-trip: %q", got.Sources[0].Data)
	}
}

func TestDecodeRequest_RejectsTopLevelNull(t *testing.T) {
	_, err := DecodeRequest([]byte("null"))
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("DecodeRequest(null): got err %v, want ErrInvalidRequest", err)
	}
}

func TestDecodeRequest_RejectsNonObjectTopLevel(t *testing.T) {
	cases := [][]byte{
		[]byte(`[1,2,3]`),
		[]byte(`"hello"`),
		[]byte(`42`),
		[]byte(`true`),
		[]byte(``),
		[]byte(`   `),
	}
	for _, data := range cases {
		if _, err := DecodeRequest(data); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("DecodeRequest(%q): got err %v, want ErrInvalidRequest", data, err)
		}
	}
}

func TestDecodeRequest_RejectsUnknownField(t *testing.T) {
	raw := append(bytes.TrimSuffix(mustJSON(minimalRequest()), []byte("}")), []byte(`,"unexpected_field":true}`)...)
	if _, err := DecodeRequest(raw); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("DecodeRequest with unknown field: got err %v, want ErrInvalidRequest", err)
	}
}

func TestDecodeRequest_RejectsDuplicateKey(t *testing.T) {
	raw := []byte(`{"schema":"verdi.spec-import-request/v1","schema":"verdi.spec-import-request/v1","target":{"slug":"a","class":"feature","title":"A"},"format":"manual-v1","primary":"s","sources":[{"id":"s","label":"s.md","data":"aGVsbG8="}]}`)
	if _, err := DecodeRequest(raw); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("DecodeRequest with duplicate key: got err %v, want ErrInvalidRequest", err)
	}
}

func TestDecodeRequest_RejectsTrailingData(t *testing.T) {
	raw := append(mustJSON(minimalRequest()), []byte(`{}`)...)
	if _, err := DecodeRequest(raw); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("DecodeRequest with trailing data: got err %v, want ErrInvalidRequest", err)
	}
}

func TestDecodeRequest_RejectsOversizedEnvelope(t *testing.T) {
	raw := bytes.Repeat([]byte("a"), MaxEnvelopeBytes+1)
	if _, err := DecodeRequest(raw); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("DecodeRequest over envelope limit: got err %v, want ErrInvalidRequest", err)
	}
}

func TestDecodeRequest_RejectsUnsupportedFormat(t *testing.T) {
	req := minimalRequest()
	req.Format = "bogus-format"
	if _, err := DecodeRequest(mustJSON(req)); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("DecodeRequest with bogus format: got err %v, want ErrUnsupportedFormat", err)
	}
}

func TestDecodeRequest_RejectsBadSchema(t *testing.T) {
	req := minimalRequest()
	req.Schema = "verdi.spec-import-request/v2"
	if _, err := DecodeRequest(mustJSON(req)); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("DecodeRequest with bad schema: got err %v, want ErrInvalidRequest", err)
	}
}

func TestDecodeRequest_RejectsInvalidUTF8Envelope(t *testing.T) {
	raw := []byte("{\"schema\":\"verdi.spec-import-request/v1\xff\"}")
	if _, err := DecodeRequest(raw); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("DecodeRequest with invalid UTF-8: got err %v, want ErrInvalidRequest", err)
	}
}
