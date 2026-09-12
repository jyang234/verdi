package publicrelease

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func sourceTestUnit(t *testing.T, source string) *sourceUnit {
	t.Helper()
	u, err := parseSourceUnit("test.go", []byte(source))
	if err != nil {
		t.Fatal(err)
	}
	return u
}
func TestSourceOrderRejectsMissingReversedAndUnguardedValidation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", `v,err:=Validate(x);if err!=nil{return err};Write(v);return nil`, true},
		{"missing", `Write(x);return nil`, false},
		{"reversed", `Write(x);v,err:=Validate(x);if err!=nil{return err};_ = v;return nil`, false},
		{"wrong error guard", `v,err:=Validate(x);if other!=nil{return err};Write(v);return nil`, false},
		{"early write", `Write(x);v,err:=Validate(x);if err!=nil{return err};Write(v);return nil`, false},
		{"unguarded", `v,_:=Validate(x);Write(v);return nil`, false},
		{"dead validation", `if false {v,err:=Validate(x);if err!=nil{return err};_ = v};Write(x);return nil`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := sourceTestUnit(t, "package p;func route()error{"+tc.body+"}")
			err := sourceOrdered(u, "route", []string{"Validate", "Write"}, []string{"Validate"})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
func TestSourceConsumersAreClosed(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{
		{"valid", "package p;func run(){openSealedController()}", true},
		{"missing", "package p;func run(){}", false},
		{"extra", "package p;func run(){openSealedController()};func hidden(){openSealedController()}", false},
		{"alias", "package p;func run(){openSealedController()};var hidden=openSealedController", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := sourceTree{"test.go": sourceTestUnit(t, tc.source)}
			err := sourceConsumers(tree, map[string]string{"test.go": "run"})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
func TestSourceRejectsProductionTranslation(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"local", "NewOwnerCall(op,payload)", true},
		{"reachable bridge", "bridge.DecodeOwner(ctx,op,payload)", false},
		{"constructor alias", "factory:=verdiproto.OpenOwnerBridge;_=factory", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := sourceTree{"internal/executor/owner.go": sourceTestUnit(t, "package executor;func route(){"+tc.body+"}")}
			err := sourceNoTranslations(tree)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
func TestSourceDurableDeclarationsCompareStructure(t *testing.T) {
	baseline := sourceTestUnit(t, "package p;const Schema=\"record/v1\";type Record struct{Digest string `json:\"digest\"`}")
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{
		{"format only", "package p\nconst Schema = \"record/v1\"\ntype Record struct { Digest string `json:\"digest\"` }", true},
		{"schema mutation", "package p;const Schema=\"record/v2\";type Record struct{Digest string `json:\"digest\"`}", false},
		{"durable field", "package p;const Schema=\"record/v1\";type Record struct{Digest string `json:\"renamed\"`}", false},
		{"added field", "package p;const Schema=\"record/v1\";type Record struct{Digest string `json:\"digest\"`; Other int}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := sourceSameDeclarations(baseline, sourceTestUnit(t, tc.source))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
func TestSourceBoundedReadChecksBeforeAppend(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", `for{part,err:=r.ReadSlice('\n');if len(part)>=Ceiling-len(frame){return nil,err};frame=append(frame,part...);if err!=nil{return frame,err}}`, true},
		{"unbounded before incremental", `ReadAll(r);for{part,err:=r.ReadSlice('\n');if len(part)>=Ceiling-len(frame){return nil,err};frame=append(frame,part...)}`, false},
		{"unbounded", `part,err:=r.ReadBytes('\n');return part,err`, false},
		{"late bound", `for{part,err:=r.ReadSlice('\n');frame=append(frame,part...);if len(part)>=Ceiling-len(frame){return nil,err}}`, false},
		{"inclusive", `for{part,err:=r.ReadSlice('\n');if len(part)>Ceiling-len(frame){return nil,err};frame=append(frame,part...)}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := sourceTestUnit(t, "package p;func read()([]byte,error){"+tc.body+"}")
			err := sourceBoundedRead(u, "read", "Ceiling")
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestSourceLiteralRegistryRejectsMissingDuplicateAndChangedOperation(t *testing.T) {
	for _, tc := range []struct {
		name, entries string
		valid         bool
	}{{"valid", "One,Two", true}, {"missing", "One", false}, {"duplicate", "One,One", false}, {"changed", "One,Three", false}} {
		t.Run(tc.name, func(t *testing.T) {
			u := sourceTestUnit(t, `package p;const One="one";const Two="two";const Three="three";var operations=[]string{`+tc.entries+`}`)
			err := sourceLiteralRegistry(sourceValues(u), "operations", []string{"one", "two"})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
func TestSourceFixedStorageRejectsCodingAndMigrationChanges(t *testing.T) {
	baseline := sourceTestUnit(t, `package persistence;const migration="CREATE TABLE receipt (digest TEXT)";func encode(v Record)[]byte{return Canonical(v)}`)
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{{"valid", string(baseline.raw), true}, {"migration", `package persistence;const migration="CREATE TABLE receipt (digest TEXT, renamed TEXT)";func encode(v Record)[]byte{return Canonical(v)}`, false}, {"encoding", `package persistence;const migration="CREATE TABLE receipt (digest TEXT)";func encode(v Record)[]byte{return Lossy(v)}`, false}} {
		t.Run(tc.name, func(t *testing.T) {
			err := sourceSameUnit(baseline, sourceTestUnit(t, tc.source))
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
func TestSourceReceiptWritesCannotBeReorderedOrMerged(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{{"valid", `v,err:=receiptPosition();if err!=nil{return err};Marshal(v);err=commitControl();if err!=nil{return err};advanceRecorder();return nil`, true}, {"merged", `v,err:=receiptPosition();if err!=nil{return err};Marshal(v);advanceRecorder();return nil`, false}, {"reordered", `err:=commitControl();if err!=nil{return err};v,err:=receiptPosition();if err!=nil{return err};Marshal(v);advanceRecorder();return nil`, false}} {
		t.Run(tc.name, func(t *testing.T) {
			u := sourceTestUnit(t, "package p;func write()error{"+tc.body+"}")
			err := sourceOrdered(u, "write", []string{"receiptPosition", "Marshal", "commitControl", "advanceRecorder"}, []string{"receiptPosition", "commitControl"})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
func TestSourceRecoveryCutPlacement(t *testing.T) {
	// These small source operands model the hook grammar, never private source.
	owner := sourceTestUnit(t, "package p\nfunc(rt ownerRuntime) receiptPosition(){\n\tevent()\n}\nfunc(rt ownerRuntime) appendReceipt(){\n\tack()\n\tadvance()\n}\n")
	terminal := sourceTestUnit(t, "package p\nfunc(s Supervisor) terminal(){\n\tappendCandidate()\n\tcheckCandidate()\n}\n")
	tree := sourceTree{"internal/executor/ownerimpl.go": owner, "internal/executor/executor.go": terminal}
	cuts := []string{"before-receipt-event", "after-receipt-event", "after-receipt-control", "before-candidate-ledger", "after-candidate-ledger"}
	anchors := []string{"\tevent()", "\tack()", "\tadvance()", "\tappendCandidate()", "\tcheckCandidate()"}
	for _, mode := range []string{"valid", "missing", "moved", "wrong anchor"} {
		t.Run(mode, func(t *testing.T) {
			var b strings.Builder
			b.WriteString("package p\nvar recoveryCuts=[]string{")
			for _, cut := range cuts {
				fmt.Fprintf(&b, "%q,", cut)
			}
			b.WriteString("}\nvar patches=map[string][][2]string{\"internal/executor/ownerimpl.go\":{")
			for i, cut := range cuts {
				if i == 3 {
					b.WriteString("},\"internal/executor/executor.go\":{")
				}
				if mode == "missing" && i == 0 {
					continue
				}
				old := anchors[i]
				replacement := "\ttask3Boundary(" + strconv.Quote(cut) + ")\n" + old
				if mode == "moved" && i == 0 {
					replacement = old + "\n\ttask3Boundary(" + strconv.Quote(cut) + ")\n"
				}
				if mode == "wrong anchor" && i == 0 {
					old = "\tmissing()"
					replacement = "\ttask3Boundary(" + strconv.Quote(cut) + ")\n" + old
				}
				fmt.Fprintf(&b, "{%q,%q},", old, replacement)
			}
			b.WriteString("}}")
			err := sourceRecoveryCuts([]byte(b.String()), tree)
			if (err == nil) != (mode == "valid") {
				t.Fatalf("mode=%s err=%v", mode, err)
			}
		})
	}
}

func TestSourceRejectsPrivateWireAndReverseDependency(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		valid            bool
	}{
		{"domain only", "internal/sealedexec/controller_domain.go", "package p;type Domain struct{Value string}", true},
		{"named private wire", "internal/sealedexec/controller_domain.go", "package p;type PrivateWire struct{Value string `json:\"value\"`}", false},
		{"anonymous private wire", "internal/sealedexec/controller_domain.go", "package p;func encode(){_ = struct{Value string `json:\"value\"`}{}}", false},
		{"framing", "internal/sealedexec/controller_schema.go", "package p;type controllerCallWire struct{Value string `json:\"value\"`}", true},
		{"reverse dependency", "internal/contextowner/codec.go", `package p;import "github.com/jyang234/verdi/internal/sealedexec"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := sourceNoPrivateOwnerWire(sourceTree{tc.path: sourceTestUnit(t, tc.body)})
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestSourceNegotiationCannotUseCachedBranch(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{{"always", `if _,err:=Negotiate();err!=nil{return err};return nil`, true}, {"cached", `if !cached {if _,err:=Negotiate();err!=nil{return err}};return nil`, false}, {"missing", `return nil`, false}} {
		t.Run(tc.name, func(t *testing.T) {
			u := sourceTestUnit(t, "package p;func run()error{"+tc.body+"}")
			err := sourceUnconditionalCall(u, "run", "Negotiate")
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
