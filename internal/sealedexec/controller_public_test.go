package sealedexec

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextowner"
)

func publicFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "contextowner", "testdata", "public-contract", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestControllerPublicFrozenFrames(t *testing.T) {
	for _, op := range ControllerOperations() {
		t.Run(string(op), func(t *testing.T) {
			callBytes := publicFixture(t, string(op)+".call.v2.json")
			call, err := DecodeControllerCall(bytes.NewReader(callBytes))
			if err != nil {
				t.Fatalf("v2 call rejected: %v", err)
			}
			got, err := EncodeControllerCall(call)
			if err != nil || !bytes.Equal(got, callBytes) {
				t.Fatalf("v2 call changed: %v", err)
			}
			resultBytes := publicFixture(t, string(op)+".result.v2.json")
			result, err := DecodeControllerResult(bytes.NewReader(resultBytes))
			if err != nil {
				t.Fatalf("v2 result rejected: %v", err)
			}
			got, err = EncodeControllerResult(result)
			if err != nil || !bytes.Equal(got, resultBytes) {
				t.Fatalf("v2 result changed: %v", err)
			}
			if _, err := DecodeControllerCall(bytes.NewReader(publicFixture(t, string(op)+".call.v1.json"))); err == nil {
				t.Fatal("accepted v1 call")
			}
			if _, err := DecodeControllerResult(bytes.NewReader(publicFixture(t, string(op)+".result.v1.json"))); err == nil {
				t.Fatal("accepted v1 result")
			}
			transport := &controllerMemoryTransport{read: bytes.NewReader(resultBytes)}
			client, err := NewControllerClient(transport)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.invoke(context.Background(), call); err != nil {
				t.Fatalf("runtime v2: %v", err)
			}
			if !bytes.Equal(transport.written.Bytes(), callBytes) {
				t.Fatal("runtime request differs from frozen v2")
			}
			if op == ControllerOperationResolveClaimMCP {
				return
			}
			local, err := DecodeOwnerCall(op, publicFixture(t, string(op)+".legacy-request.json"))
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := contextowner.EncodeCall(local)
			if err != nil || !bytes.Equal(encoded, publicFixture(t, string(op)+".owner-call.json")) {
				t.Fatalf("compatibility request changed: %v", err)
			}
			reply, err := contextowner.DecodeReply(bytes.NewReader(publicFixture(t, string(op)+".owner-reply.json")))
			if err != nil {
				t.Fatal(err)
			}
			encoded, err = EncodeOwnerReply(op, reply)
			if err != nil || !bytes.Equal(encoded, publicFixture(t, string(op)+".legacy-result.json")) {
				t.Fatalf("compatibility result changed: %v", err)
			}
		})
	}
}

// An endless unterminated peer must be refused at the limit, without reading
// its next byte or waiting for EOF. The reader synthesizes bytes, not a line.
type limitControllerReader struct{ n int }

func (r *limitControllerReader) Read(p []byte) (int, error) {
	if r.n >= OwnerFrameCeiling {
		return 0, io.ErrUnexpectedEOF
	}
	n := len(p)
	if n > OwnerFrameCeiling-r.n {
		n = OwnerFrameCeiling - r.n
	}
	for i := 0; i < n; i++ {
		p[i] = ' '
	}
	r.n += n
	return n, nil
}
func TestControllerPublicReadBound(t *testing.T) {
	source := &limitControllerReader{}
	transport := &controllerMemoryTransport{read: source}
	client, err := NewControllerClient(transport)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.NextStamp(context.Background())
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("frame ceiling")) {
		t.Fatalf("expected incremental ceiling refusal, got %v", err)
	}
	if client.Usable() {
		t.Fatal("oversized peer left capability usable")
	}
}

func TestControllerPublicRejectsMutatedFrames(t *testing.T) {
	for _, op := range ControllerOperations() {
		for _, kind := range []string{"call", "result"} {
			t.Run(string(op)+"/"+kind, func(t *testing.T) {
				original := publicFixture(t, string(op)+"."+kind+".v2.json")
				for name, mutant := range map[string][]byte{
					"leading whitespace":    append([]byte(" "), original...),
					"duplicate envelope":    bytes.Replace(original, []byte(`"call_sequence":1`), []byte(`"call_sequence":1,"call_sequence":1`), 1),
					"unknown envelope":      bytes.Replace(original, []byte(`"operation":`), []byte(`"extra":true,"operation":`), 1),
					"null payload":          mutatePublicFrame(t, original, func(v map[string]any) { v["payload"] = nil }),
					"unknown arm":           mutatePublicFrame(t, original, func(v map[string]any) { publicFrameArm(v, kind)["extra"] = true }),
					"missing arm schema":    mutatePublicFrame(t, original, func(v map[string]any) { delete(publicFrameArm(v, kind), "schema") }),
					"null arm schema":       mutatePublicFrame(t, original, func(v map[string]any) { publicFrameArm(v, kind)["schema"] = nil }),
					"wrong arm schema":      mutatePublicFrame(t, original, func(v map[string]any) { publicFrameArm(v, kind)["schema"] = "verdi.other/v1" }),
					"noncanonical escaping": bytes.Replace(original, []byte(`"operation"`), []byte(`"\u006fperation"`), 1),
					"trailing document":     append(append([]byte(nil), original...), []byte("{}\n")...),
				} {
					t.Run(name, func(t *testing.T) {
						if bytes.Equal(mutant, original) {
							t.Fatal("mutation did not change frozen frame")
						}
						var err error
						if kind == "call" {
							_, err = DecodeControllerCall(bytes.NewReader(mutant))
						} else {
							_, err = DecodeControllerResult(bytes.NewReader(mutant))
						}
						if err == nil {
							t.Fatal("accepted mutated frame")
						}
					})
				}
			})
		}
	}
}

func publicFrameArm(v map[string]any, kind string) map[string]any {
	arm := v["payload"].(map[string]any)
	if kind == "result" {
		arm = arm["result"].(map[string]any)
	}
	return arm
}
func mutatePublicFrame(t *testing.T, b []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	mutate(v)
	got, err := canonjson.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestControllerPublicRelationRefusalReusesSequence(t *testing.T) {
	// Invoke directly so the domain method's historic relation check cannot
	// conceal omission of the mandatory shared runtime validator.
	op := ControllerOperationResolveRecorder
	call, err := DecodeControllerCall(bytes.NewReader(publicFixture(t, string(op)+".call.v2.json")))
	if err != nil {
		t.Fatal(err)
	}
	bad := mutatePublicFrame(t, publicFixture(t, string(op)+".result.v2.json"), func(v map[string]any) {
		publicFrameArm(v, "result")["facts"].(map[string]any)["ref"].(map[string]any)["id"] = "other-recorder"
	})
	if _, err := DecodeControllerResult(bytes.NewReader(bad)); err != nil {
		t.Fatalf("mutation must be structurally valid: %v", err)
	}
	second := mutatePublicFrame(t, publicFixture(t, "next-stamp.result.v2.json"), func(v map[string]any) { v["call_sequence"] = 2 })
	for name, first := range map[string][]byte{"relation mismatch": bad, "operational refusal": publicFixture(t, "error.result.v2.json")} {
		t.Run(name, func(t *testing.T) {
			first = mutatePublicFrame(t, first, func(v map[string]any) { v["operation"] = string(op) })
			transport := &controllerMemoryTransport{read: bytes.NewReader(append(first, second...))}
			client, err := NewControllerClient(transport)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.invoke(context.Background(), call)
			if !errors.Is(err, ErrOperational) {
				t.Fatalf("want operational refusal, got %v", err)
			}
			if name == "relation mismatch" && !errors.Is(err, contextowner.ErrRelationMismatch) {
				t.Fatalf("shared relation validator was bypassed: %v", err)
			}
			if !client.Usable() {
				t.Fatal("correctly framed refusal poisoned capability")
			}
			if _, err := client.NextStamp(context.Background()); err != nil {
				t.Fatalf("next sequence unusable: %v", err)
			}
			if !bytes.Contains(transport.written.Bytes(), []byte(`"call_sequence":2`)) {
				t.Fatal("sequence did not advance")
			}
		})
	}
}

func TestControllerPublicReaderEdges(t *testing.T) {
	for name, raw := range map[string][]byte{"below ceiling": append(bytes.Repeat([]byte(" "), OwnerFrameCeiling-2), '\n'), "at ceiling": append(bytes.Repeat([]byte(" "), OwnerFrameCeiling-1), '\n'), "partial": []byte("partial"), "EOF": nil} {
		t.Run(name, func(t *testing.T) {
			got, err := readControllerReply(bufio.NewReader(bytes.NewReader(raw)))
			if name == "below ceiling" {
				if err != nil || !bytes.Equal(got, raw) {
					t.Fatalf("below ceiling refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("invalid/closed frame accepted")
			}
			if name == "EOF" && !errors.Is(err, io.EOF) {
				t.Fatalf("normal EOF changed: %v", err)
			}
		})
	}
}
