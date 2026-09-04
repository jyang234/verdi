package contextresolve

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
)

var (
	canonicalDigestRE = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	requestIDRE       = regexp.MustCompile(`^context-request:[0-9a-f]{64}$`)
)

// The wire documents. Every nested canonical document is carried as raw bytes
// and handed to the package that owns it, so this codec never restates a rule
// internal/contextcompile or internal/contextevent already enforces.
type (
	requestWire struct {
		BaseManifest json.RawMessage   `json:"base_manifest"`
		Expansions   []json.RawMessage `json:"expansions"`
		Identity     Identity          `json:"identity"`
		Ref          string            `json:"ref"`
		Request      json.RawMessage   `json:"request"`
		Schema       string            `json:"schema"`
		Terminal     Terminal          `json:"terminal"`
	}

	expansionWire struct {
		ChildManifestDigest  string          `json:"child_manifest_digest"`
		ChildRevision        uint64          `json:"child_revision"`
		Data                 json.RawMessage `json:"data"`
		ExpansionDigest      string          `json:"expansion_digest"`
		ExpansionRoot        string          `json:"expansion_root"`
		ParentManifestDigest string          `json:"parent_manifest_digest"`
		ParentRevision       uint64          `json:"parent_revision"`
		Purpose              string          `json:"purpose"`
		Ref                  string          `json:"ref"`
		RequestID            string          `json:"request_id"`
		TerminalAck          json.RawMessage `json:"terminal_ack"`
	}

	resultWire struct {
		Item      json.RawMessage   `json:"item"`
		Ref       string            `json:"ref"`
		Schema    string            `json:"schema"`
		State     State             `json:"state"`
		Witnesses []json.RawMessage `json:"witnesses"`
	}
)

// EncodeRequest validates request and returns its canonical JSON encoding
// (sorted keys, no HTML escaping, one trailing newline).
func EncodeRequest(request Request) ([]byte, error) {
	if request.Schema != RequestSchema {
		return nil, fmt.Errorf("contextresolve: request schema must be %q", RequestSchema)
	}
	if err := requireText("ref", request.Ref); err != nil {
		return nil, err
	}
	if err := validateIdentity(request.Identity); err != nil {
		return nil, err
	}
	// Whether the terminal tuple is the one this lineage actually replays to
	// is a RELATION between stated facts, not a property of the document, so
	// it belongs to the replay (which reports it as §2.2's
	// lineage-inconsistency verdict) and not to the grammar (which would
	// report it as an undecodable document). The grammar checks only shape.
	if err := validateTerminal(request.Terminal); err != nil {
		return nil, err
	}
	compile, err := contextcompile.EncodeRequest(request.Compile)
	if err != nil {
		return nil, fmt.Errorf("contextresolve: encoding compile request: %w", err)
	}
	base, err := contextcompile.EncodeManifest(request.BaseManifest)
	if err != nil {
		return nil, fmt.Errorf("contextresolve: encoding base manifest: %w", err)
	}
	if request.Expansions == nil {
		return nil, fmt.Errorf("contextresolve: expansions must be non-nil (an explicitly empty lineage is [])")
	}
	rows := make([]json.RawMessage, 0, len(request.Expansions))
	for i, expansion := range request.Expansions {
		row, err := encodeExpansion(expansion)
		if err != nil {
			return nil, fmt.Errorf("contextresolve: expansions[%d]: %w", i, err)
		}
		rows = append(rows, row)
	}
	return canonjson.Marshal(requestWire{
		BaseManifest: trimFrame(base), Expansions: rows,
		Identity: request.Identity, Ref: request.Ref,
		Request: trimFrame(compile), Schema: request.Schema,
		Terminal: request.Terminal,
	})
}

// validateIdentity requires all four dispatch-bound identity members. The
// order is fixed rather than map-ranged so one malformed document always
// produces one diagnostic.
func validateIdentity(identity Identity) error {
	for _, member := range [][2]string{
		{"identity.flight", identity.Flight},
		{"identity.lane", identity.Lane},
		{"identity.epoch", identity.Epoch},
		{"identity.session", identity.Session},
	} {
		if err := requireText(member[0], member[1]); err != nil {
			return err
		}
	}
	return nil
}

// validateTerminal checks the terminal tuple's shape: a positive revision, a
// canonical manifest digest, and an expansion root that is either a canonical
// digest or the empty string — the one legal spelling of "nothing is installed
// yet".
func validateTerminal(terminal Terminal) error {
	if terminal.Revision == 0 {
		return fmt.Errorf("contextresolve: terminal.revision must be positive")
	}
	if err := validateDigest("terminal.manifest_digest", terminal.ManifestDigest); err != nil {
		return err
	}
	if terminal.ExpansionRoot == "" {
		return nil
	}
	return validateDigest("terminal.expansion_root", terminal.ExpansionRoot)
}

// DecodeRequest strictly decodes one canonical resolve request: exact JSON,
// every nested document judged by its owning codec, and byte equality between
// data and this package's own canonical re-encoding — which is what turns
// "the members are present and well typed" into "these are the exact bytes
// the sender's own encoder would produce", closing member order, absent and
// null members, duplicates, and noncanonical spellings in one check.
func DecodeRequest(data []byte) (Request, error) {
	var wire requestWire
	if err := artifact.DecodeExactJSON(data, &wire); err != nil {
		return Request{}, fmt.Errorf("contextresolve: decoding request: %w", err)
	}
	if wire.Schema != RequestSchema {
		return Request{}, fmt.Errorf("contextresolve: decoding request: schema %q, want %q", wire.Schema, RequestSchema)
	}
	if wire.Expansions == nil {
		return Request{}, fmt.Errorf("contextresolve: decoding request: expansions is absent or null")
	}
	compile, err := contextcompile.DecodeRequest(frameExact(wire.Request))
	if err != nil {
		return Request{}, fmt.Errorf("contextresolve: decoding request: %w", err)
	}
	base, err := contextcompile.DecodeManifest(frameExact(wire.BaseManifest))
	if err != nil {
		return Request{}, fmt.Errorf("contextresolve: decoding request: %w", err)
	}
	expansions := make([]Expansion, 0, len(wire.Expansions))
	for i, row := range wire.Expansions {
		expansion, err := decodeExpansion(row)
		if err != nil {
			return Request{}, fmt.Errorf("contextresolve: decoding request: expansions[%d]: %w", i, err)
		}
		expansions = append(expansions, expansion)
	}
	// identity and terminal are decoded as values, so an absent or null member
	// arrives as the zero tuple and is refused by the grammar EncodeRequest
	// applies below — never silently defaulted.
	request := Request{
		Schema: wire.Schema, Compile: compile, BaseManifest: base,
		Identity: wire.Identity, Expansions: expansions,
		Terminal: wire.Terminal, Ref: wire.Ref,
	}
	canonical, err := EncodeRequest(request)
	if err != nil {
		return Request{}, fmt.Errorf("contextresolve: decoding request: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return Request{}, fmt.Errorf("contextresolve: decoding request: input bytes are not the canonical encoding of the document they decode to")
	}
	return request, nil
}

func encodeExpansion(expansion Expansion) (json.RawMessage, error) {
	if !requestIDRE.MatchString(expansion.RequestID) {
		return nil, fmt.Errorf("request_id must be context-request:<64 lowercase hex>")
	}
	if err := requireText("ref", expansion.Ref); err != nil {
		return nil, err
	}
	if err := requireText("purpose", expansion.Purpose); err != nil {
		return nil, err
	}
	// Positivity is grammar; the child-is-one-past-its-parent relation is a
	// replay fact, and lives with the rest of the lineage cross-matching.
	if expansion.ParentRevision == 0 || expansion.ChildRevision == 0 {
		return nil, fmt.Errorf("parent_revision and child_revision must be positive")
	}
	for field, digest := range map[string]string{
		"parent_manifest_digest": expansion.ParentManifestDigest,
		"child_manifest_digest":  expansion.ChildManifestDigest,
		"expansion_digest":       expansion.ExpansionDigest,
		"expansion_root":         expansion.ExpansionRoot,
	} {
		if err := validateDigest(field, digest); err != nil {
			return nil, err
		}
	}
	ack, err := contextevent.EncodeEventAck(expansion.TerminalAck)
	if err != nil {
		return nil, fmt.Errorf("terminal_ack: %w", err)
	}
	data, err := contextcompile.EncodeDataItem(expansion.Data)
	if err != nil {
		return nil, fmt.Errorf("data: %w", err)
	}
	return canonjson.Marshal(expansionWire{
		ChildManifestDigest: expansion.ChildManifestDigest, ChildRevision: expansion.ChildRevision,
		Data: trimFrame(data), ExpansionDigest: expansion.ExpansionDigest,
		ExpansionRoot: expansion.ExpansionRoot, ParentManifestDigest: expansion.ParentManifestDigest,
		ParentRevision: expansion.ParentRevision, Purpose: expansion.Purpose, Ref: expansion.Ref,
		RequestID: expansion.RequestID, TerminalAck: trimFrame(ack),
	})
}

func decodeExpansion(raw json.RawMessage) (Expansion, error) {
	var wire expansionWire
	if err := artifact.DecodeExactJSON(frameExact(raw), &wire); err != nil {
		return Expansion{}, err
	}
	ack, err := contextevent.DecodeEventAck(bytes.NewReader(frameExact(wire.TerminalAck)))
	if err != nil {
		return Expansion{}, err
	}
	item, err := contextcompile.DecodeDataItem(frameExact(wire.Data))
	if err != nil {
		return Expansion{}, err
	}
	expansion := Expansion{
		RequestID: wire.RequestID, Ref: wire.Ref, Purpose: wire.Purpose,
		ParentRevision: wire.ParentRevision, ParentManifestDigest: wire.ParentManifestDigest,
		ChildRevision: wire.ChildRevision, ChildManifestDigest: wire.ChildManifestDigest,
		ExpansionDigest: wire.ExpansionDigest, ExpansionRoot: wire.ExpansionRoot,
		TerminalAck: ack, Data: item,
	}
	canonical, err := encodeExpansion(expansion)
	if err != nil {
		return Expansion{}, err
	}
	if !bytes.Equal(canonical, frameExact(raw)) {
		return Expansion{}, fmt.Errorf("input bytes are not the canonical encoding of the row they decode to")
	}
	return expansion, nil
}

// EncodeResult validates result and returns its canonical JSON encoding. The
// witnesses are sorted here, by the raw bytes of each standalone canonical
// witness encoding (correction §2.2), so the caller never has to know the
// order and can never send a different one.
//
// A proven item that carries an artifact ref of its own must carry the result
// ref. §2.2 keeps that member optional in the data-item grammar and makes the
// result ref "the correlation operand even when a valid data item has no
// artifact ref", so an ABSENT item ref is exactly the case the operand exists
// for — but a ref that is present and different would put two answers in one
// document, and the cross-match §2.2 requires of the receiver could only
// discover that after the document had already been believed.
func EncodeResult(result Result) ([]byte, error) {
	if result.Schema != ResultSchema {
		return nil, fmt.Errorf("contextresolve: result schema must be %q", ResultSchema)
	}
	if err := requireText("ref", result.Ref); err != nil {
		return nil, err
	}
	witnesses, err := encodeWitnesses(result.Witnesses)
	if err != nil {
		return nil, err
	}
	var item json.RawMessage
	switch result.State {
	case StateProven:
		if result.Item == nil {
			return nil, fmt.Errorf("contextresolve: a proven result requires exactly one data item")
		}
		if len(witnesses) != 0 {
			return nil, fmt.Errorf("contextresolve: a proven result carries no witness")
		}
		if !itemRefBinds(*result.Item, result.Ref) {
			return nil, fmt.Errorf("contextresolve: a proven item that carries a ref must carry the result ref")
		}
		encoded, err := contextcompile.EncodeDataItem(*result.Item)
		if err != nil {
			return nil, fmt.Errorf("contextresolve: encoding data item: %w", err)
		}
		item = trimFrame(encoded)
	case StateNonProven:
		if result.Item != nil {
			return nil, fmt.Errorf("contextresolve: a non-proven result carries no data item")
		}
		if len(witnesses) == 0 {
			return nil, fmt.Errorf("contextresolve: a non-proven result requires at least one witness")
		}
	default:
		return nil, fmt.Errorf("contextresolve: unknown result state %q", result.State)
	}
	return canonjson.Marshal(resultWire{
		Item: item, Ref: result.Ref, Schema: result.Schema,
		State: result.State, Witnesses: witnesses,
	})
}

// DecodeResult strictly decodes one canonical resolve result through the same
// canonical-byte discipline DecodeRequest applies.
func DecodeResult(data []byte) (Result, error) {
	var wire resultWire
	if err := artifact.DecodeExactJSON(data, &wire); err != nil {
		return Result{}, fmt.Errorf("contextresolve: decoding result: %w", err)
	}
	if wire.Schema != ResultSchema {
		return Result{}, fmt.Errorf("contextresolve: decoding result: schema %q, want %q", wire.Schema, ResultSchema)
	}
	if wire.Witnesses == nil {
		return Result{}, fmt.Errorf("contextresolve: decoding result: witnesses is absent or null")
	}
	witnesses := make([]Witness, 0, len(wire.Witnesses))
	for i, raw := range wire.Witnesses {
		witness, err := decodeWitness(raw)
		if err != nil {
			return Result{}, fmt.Errorf("contextresolve: decoding result: witnesses[%d]: %w", i, err)
		}
		witnesses = append(witnesses, witness)
	}
	result := Result{Schema: wire.Schema, State: wire.State, Ref: wire.Ref, Witnesses: witnesses}
	if !rawIsNull(wire.Item) {
		item, err := contextcompile.DecodeDataItem(frameExact(wire.Item))
		if err != nil {
			return Result{}, fmt.Errorf("contextresolve: decoding result: %w", err)
		}
		// Judged here rather than left to the canonical re-encode below: the
		// refusal is about the document that ARRIVED, and a decoder that
		// reported it only through a re-encoding would make one document's
		// diagnostic depend on the order another function happens to check in.
		if wire.State == StateProven && !itemRefBinds(item, wire.Ref) {
			return Result{}, fmt.Errorf("contextresolve: decoding result: a proven item that carries a ref must carry the result ref")
		}
		result.Item = &item
	}
	canonical, err := EncodeResult(result)
	if err != nil {
		return Result{}, fmt.Errorf("contextresolve: decoding result: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return Result{}, fmt.Errorf("contextresolve: decoding result: input bytes are not the canonical encoding of the document they decode to")
	}
	return result, nil
}

// itemRefBinds reports whether a proven item's own artifact ref is the result
// ref it is delivered under. A nil item ref binds: it is the shape §2.2's
// correlation operand exists for, not a mismatch.
//
// Both arms of the result union read this one predicate rather than restating
// it, so the bytes a producer may write and the bytes a consumer may believe
// can never come apart — the same reason encodeExpansion and decodeExpansion
// share their grammar.
func itemRefBinds(item contextcompile.DataItem, ref string) bool {
	return item.Ref == nil || *item.Ref == ref
}

// encodeWitnesses validates, canonically encodes, sorts and de-duplicates one
// witness set. Sorting and duplicate rejection share the encoded bytes, so
// "sorted" and "distinct" are decided over exactly the representation §2.2
// names rather than over the Go values behind it.
func encodeWitnesses(witnesses []Witness) ([]json.RawMessage, error) {
	encoded := make([][]byte, 0, len(witnesses))
	for _, witness := range witnesses {
		if witness.Schema != WitnessSchema {
			return nil, fmt.Errorf("contextresolve: witness schema must be %q", WitnessSchema)
		}
		if !witnessCodes[witness.Code] {
			return nil, fmt.Errorf("contextresolve: unknown witness code %q", witness.Code)
		}
		document, err := canonjson.Marshal(witness)
		if err != nil {
			return nil, fmt.Errorf("contextresolve: encoding witness: %w", err)
		}
		encoded = append(encoded, document)
	}
	sort.Slice(encoded, func(i, j int) bool { return bytes.Compare(encoded[i], encoded[j]) < 0 })
	out := make([]json.RawMessage, 0, len(encoded))
	for i, document := range encoded {
		if i > 0 && bytes.Equal(document, encoded[i-1]) {
			return nil, fmt.Errorf("contextresolve: duplicate witness")
		}
		out = append(out, trimFrame(document))
	}
	return out, nil
}

func decodeWitness(raw json.RawMessage) (Witness, error) {
	var witness Witness
	if err := artifact.DecodeExactJSON(frameExact(raw), &witness); err != nil {
		return Witness{}, err
	}
	if witness.Schema != WitnessSchema {
		return Witness{}, fmt.Errorf("schema %q, want %q", witness.Schema, WitnessSchema)
	}
	if !witnessCodes[witness.Code] {
		return Witness{}, fmt.Errorf("unknown witness code %q", witness.Code)
	}
	return witness, nil
}

// frameExact re-frames a nested document as one standalone document without
// re-canonicalizing it, so the owning decoder judges the exact bytes it was
// sent rather than a re-rendering of them.
func frameExact(raw json.RawMessage) []byte {
	return append(append([]byte(nil), raw...), '\n')
}

// trimFrame drops the trailing newline a standalone canonical document
// carries, so it can be nested inside another one.
func trimFrame(raw []byte) json.RawMessage {
	return append(json.RawMessage(nil), bytes.TrimSuffix(raw, []byte("\n"))...)
}

func rawIsNull(raw json.RawMessage) bool {
	return len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func validateDigest(field, value string) error {
	if !canonicalDigestRE.MatchString(value) {
		return fmt.Errorf("contextresolve: %s must be a canonical sha256 digest", field)
	}
	return nil
}

// requireText rejects an empty value and any surrounding whitespace: a ref or
// purpose is an identity that participates in a digest preimage, so two
// spellings of one value would be two different facts.
func requireText(field, value string) error {
	if value == "" || value != strings.TrimSpace(value) {
		return fmt.Errorf("contextresolve: %s must be nonempty text without surrounding whitespace", field)
	}
	return nil
}
