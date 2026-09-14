package specimport

// Span identifies one source-backed contribution to a Field's text: the
// exact half-open byte range [Start,End) of SourceID's SELECTED bytes
// (post StartLine/EndLine resolution, spec-import-contract.md: "All
// mapping offsets refer to selected UTF-8 byte slices") and the declared
// Transform that produced the field's displayed text from those raw
// bytes.
type Span struct {
	SourceID  string `json:"source_id"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	Transform string `json:"transform,omitempty"`
}

// Field is one resolved destination value: a statement (problem/outcome)
// or an object (ac-<n>/co-<n>/dc-<n>/oq-<n>), automatically extracted,
// explicitly mapped, or explicitly added by the user.
type Field struct {
	Target   string   `json:"target"`
	Text     string   `json:"text"`
	Origin   string   `json:"origin"`
	Spans    []Span   `json:"spans,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

// Interval is one disjoint, ordered slice of a source's selected bytes in
// the byte-coverage partition (spec-import-contract.md, "RetainUnmapped is
// the user's explicit disposition..."): every selected byte belongs to
// exactly one Interval, and Targets lists every Field this interval
// contributes to when Disposition is "mapped" (more than one when
// overlapping field references share the interval).
type Interval struct {
	Start       int      `json:"start"`
	End         int      `json:"end"`
	Disposition string   `json:"disposition"`
	Targets     []string `json:"targets,omitempty"`
}

// Coverage is one source's byte-accounting record: TotalBytes always
// equals MappedBytes+RetainedBytes+UnresolvedBytes, and Intervals
// partitions [0,TotalBytes) in order with no gap or overlap.
type Coverage struct {
	SourceID        string     `json:"source_id"`
	TotalBytes      int        `json:"total_bytes"`
	MappedBytes     int        `json:"mapped_bytes"`
	RetainedBytes   int        `json:"retained_bytes"`
	UnresolvedBytes int        `json:"unresolved_bytes"`
	Intervals       []Interval `json:"intervals"`
}

// Finding is one structural or coverage diagnostic from the contract's
// closed vocabulary (spec-import-contract.md, "Errors and browser
// behavior"). Blocking findings prevent a ready candidate; every Finding
// this package produces explains the rule and the target it concerns.
type Finding struct {
	Code     string `json:"code"`
	Target   string `json:"target,omitempty"`
	Message  string `json:"message"`
	Blocking bool   `json:"blocking"`
}

// Snapshot is one source's retained record: the full original bytes plus
// its original-file digest, its selected-slice digest, and the original
// line coordinates that selection came from. This is the value later
// tasks persist verbatim as the durable, non-authoritative source sidecar
// (.verdi/imports/<slug>/<preview-digest>/sources/<source-id>.md).
type Snapshot struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Data           []byte `json:"data"`
	OriginalDigest string `json:"original_digest"`
	Digest         string `json:"digest"`
	StartLine      int    `json:"start_line,omitempty"`
	EndLine        int    `json:"end_line,omitempty"`
}

// Plan is Normalize's pure, deterministic output: every source's retained
// snapshot, every resolved field, every source's byte-coverage partition,
// every structural/coverage finding, and — for native format only — the
// validated native document bytes. Plan is internal-only working state,
// never a public request/response format of its own
// (spec-import-contract.md: "Internal Plan is not a public request
// format").
type Plan struct {
	Sources  []Snapshot `json:"sources"`
	Fields   []Field    `json:"fields"`
	Coverage []Coverage `json:"coverage"`
	Findings []Finding  `json:"findings"`
	Native   []byte     `json:"native,omitempty"`
}
