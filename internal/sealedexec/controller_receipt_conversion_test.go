package sealedexec

import (
	"bytes"
	"context"
	"testing"
)

func TestControllerAppendReceiptPreservesTypedDigest(t *testing.T) {
	for _, missingDigest := range []bool{false, true} {
		name := "frozen control"
		if missingDigest {
			name = "missing receipt digest"
		}
		t.Run(name, func(t *testing.T) {
			want := publicFixture(t, "append-receipt.call.v2.json")
			call, err := DecodeControllerCall(bytes.NewReader(want))
			if err != nil {
				t.Fatal(err)
			}
			if missingDigest {
				call.AppendReceipt.Append.Receipt.Digest = ""
			}
			t.Run("encode", func(t *testing.T) {
				got, err := EncodeControllerCall(call)
				if missingDigest {
					if err == nil || len(got) != 0 {
						t.Fatalf("missing typed receipt digest: err=%v, encoded bytes=%d; want refusal without output", err, len(got))
					}
					return
				}
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("frozen append-receipt call changed: err=%v, encoded bytes=%d", err, len(got))
				}
			})
			t.Run("client", func(t *testing.T) {
				transport := &controllerMemoryTransport{read: bytes.NewReader(publicFixture(t, "append-receipt.result.v2.json"))}
				client, err := NewControllerClient(transport)
				if err != nil {
					t.Fatal(err)
				}
				_, err = client.AppendReceipt(context.Background(), call.AppendReceipt.Append)
				if missingDigest {
					if err == nil || transport.written.Len() != 0 {
						t.Fatalf("missing typed receipt digest: err=%v, transport bytes=%d; want refusal before transport", err, transport.written.Len())
					}
					return
				}
				if err != nil || !bytes.Equal(transport.written.Bytes(), want) {
					t.Fatalf("frozen append-receipt client call changed: err=%v, transport bytes=%d", err, transport.written.Len())
				}
			})
		})
	}
}
