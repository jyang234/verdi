package decisionsweep

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
)

// DefaultExemptsConflictThreshold is audit.exempts_conflict_threshold's
// documented default (01 §Store manifest, R4-I-10: "both fields ... default
// to 3 — the smallest reversible starting point"). internal/store decodes
// the raw manifest value only; applying this default when the field is
// absent (zero) is this phase's job, mirroring
// internal/evidence.DefaultDeviationsStaleThreshold's own precedent
// exactly.
const DefaultExemptsConflictThreshold = 3

// Filing is one auto-filed conflict record PlanAutoFilings has computed but
// not yet written: the store-relative path (deterministic, keyed on the
// ADR ref — the idempotency key) and the fully-rendered file content.
type Filing struct {
	ADRRef  string
	RelPath string // e.g. ".verdi/conflicts/exempts-threshold-retry-policy.md"
	Content []byte
}

// conflictFilingName derives the auto-filed conflict's deterministic name
// from the ADR ref alone (never from the exemption count or sources) —
// CLAUDE.md's idempotency requirement ("re-running audit must not file
// duplicates — key on the ADR ref"): the SAME ADR crossing the threshold
// twice, or with a different exemption count, must always resolve to the
// SAME filename.
func conflictFilingName(adrRef string) (string, error) {
	ref, err := artifact.ParseRef(adrRef)
	if err != nil {
		return "", fmt.Errorf("decisionsweep: %q is not a valid ADR ref: %w", adrRef, err)
	}
	return "exempts-threshold-" + ref.Name, nil
}

// PlanAutoFilings computes, for every ADR in counts whose active-exemption
// Count reaches threshold (threshold <= 0 uses DefaultExemptsConflictThreshold
// — "at a configured threshold", read as >=, matching 03 §Exemption audit's
// exit-criterion example literally: three exempts edges plus a threshold of
// 3 files), the conflict record to auto-file — skipping any ADR whose
// deterministic path already exists on disk (idempotent: a prior run, or a
// human, already filed it; PlanAutoFilings never re-files or overwrites).
// Pure except for the existence check (os.Stat) — no file is written here;
// see WriteFilings.
func PlanAutoFilings(root string, counts map[string]*ExemptionCount, threshold int) ([]Filing, error) {
	if threshold <= 0 {
		threshold = DefaultExemptsConflictThreshold
	}

	var filings []Filing
	for _, adrRef := range SortedADRRefs(counts) {
		count := counts[adrRef]
		if count.Count() < threshold {
			continue
		}
		name, err := conflictFilingName(adrRef)
		if err != nil {
			return nil, err
		}
		relPath := filepath.Join(".verdi", "conflicts", name+".md")
		absPath := filepath.Join(root, relPath)
		if _, err := os.Stat(absPath); err == nil {
			continue // already filed — idempotent, never re-file
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("decisionsweep: checking %s: %w", absPath, err)
		}

		content, err := buildConflictFiling(adrRef, count)
		if err != nil {
			return nil, err
		}
		filings = append(filings, Filing{ADRRef: adrRef, RelPath: relPath, Content: content})
	}
	return filings, nil
}

// buildConflictFiling renders the auto-filed conflict record: kind
// conflict (artifact.ConflictFrontmatter, the existing kind — no new
// schema needed), status open, a mandatory `challenges` link naming the
// ADR (03 §Challenging closed decisions: "File a conflict ... with a
// challenges: link to the disputed artifact"), and owners routed to the
// ADR's own owners (the natural oracle). The triggering exemption sources
// are disclosed in the rendered body alongside a digest over them (03
// §Exemption audit: "Deterministic, a fold over committed records — no
// judgment, no LLM") rather than in a Base.Provenance record: Provenance's
// own Validate requires every input to be a pinned ref or path@commit
// (artifact/common.go's validateProvenanceInput), and this audit's inputs
// are working-tree decision/spec identities with no single commit this
// package resolves — inventing one would be dishonest provenance, not a
// real one, so the digest is disclosed in the body instead. Flagged in the
// phase report as a candidate follow-up if 02's Provenance contract is
// ever widened to accept unpinned identity inputs.
func buildConflictFiling(adrRef string, count *ExemptionCount) ([]byte, error) {
	name, err := conflictFilingName(adrRef)
	if err != nil {
		return nil, err
	}
	// count.Owners is copied straight from a successfully-decoded ADR's own
	// Base.Owners (backlinks.go), which artifact.Base.validateBase already
	// requires to be non-empty — this fallback exists only so a future
	// caller that builds an ExemptionCount by hand (e.g. a test) fails
	// loudly rather than producing an unowned, un-Validate-able conflict
	// record.
	owners := count.Owners
	if len(owners) == 0 {
		return nil, fmt.Errorf("decisionsweep: internal error: %s has no owners to route the auto-filed conflict to", adrRef)
	}

	digest, err := exemptionDigest(adrRef, count)
	if err != nil {
		return nil, err
	}

	fm := &artifact.ConflictFrontmatter{
		Base: artifact.Base{
			ID:     "conflict/" + name,
			Kind:   artifact.KindConflict,
			Title:  fmt.Sprintf("Exemption threshold crossed: %s", adrRef),
			Owners: owners,
			Links:  []artifact.Link{{Type: artifact.LinkChallenges, Ref: adrRef}},
		},
		Status: "open",
	}
	if err := fm.Validate(); err != nil {
		return nil, fmt.Errorf("decisionsweep: internal error: auto-filed conflict failed self-validation: %w", err)
	}

	body := renderExemptionBody(adrRef, count, digest)
	return renderConflictMarkdown(fm, body), nil
}

type exemptionDigestSource struct {
	SpecRef    string `json:"spec_ref"`
	DecisionID string `json:"decision_id"`
	Reason     string `json:"reason"`
}

// exemptionDigest hashes the triggering exemption sources deterministically
// (canonjson, sha256) — the same convention internal/align's ComputeDigest
// uses.
func exemptionDigest(adrRef string, count *ExemptionCount) (string, error) {
	sources := make([]exemptionDigestSource, 0, len(count.Sources))
	for _, s := range count.Sources {
		sources = append(sources, exemptionDigestSource(s))
	}
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].SpecRef != sources[j].SpecRef {
			return sources[i].SpecRef < sources[j].SpecRef
		}
		return sources[i].DecisionID < sources[j].DecisionID
	})
	payload := struct {
		ADRRef  string                  `json:"adr_ref"`
		Sources []exemptionDigestSource `json:"sources"`
	}{ADRRef: adrRef, Sources: sources}
	data, err := canonjson.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("decisionsweep: marshaling exemption digest input: %w", err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func renderExemptionBody(adrRef string, count *ExemptionCount, digest string) string {
	var b []byte
	b = append(b, []byte(fmt.Sprintf("# Exemption threshold crossed: %s\n\n", adrRef))...)
	b = append(b, []byte(fmt.Sprintf("Auto-filed by `verdi audit` (03 §Exemption audit): %d active `exempts` edges against %s crossed the configured threshold.\n\n", count.Count(), adrRef))...)
	b = append(b, []byte(fmt.Sprintf("Sources digest: %s (deterministic — a fold over committed records, no judgment, no LLM)\n\n", digest))...)
	b = append(b, []byte("## Sources\n\n")...)
	for _, s := range count.Sources {
		reason := s.Reason
		if reason == "" {
			reason = "(no reason recorded)"
		}
		b = append(b, []byte(fmt.Sprintf("- %s#%s: %s\n", s.SpecRef, s.DecisionID, reason))...)
	}
	return string(b)
}

// WriteFilings writes every filing under root, returning the absolute
// paths written. Deliberately separate from PlanAutoFilings (which is pure
// aside from its existence check) so a caller can inspect/log the plan
// before committing to disk I/O, and so tests can exercise planning
// without a writable root.
func WriteFilings(root string, filings []Filing) ([]string, error) {
	dir := filepath.Join(root, ".verdi", "conflicts")
	if len(filings) > 0 {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("decisionsweep: creating %s: %w", dir, err)
		}
	}
	var written []string
	for _, f := range filings {
		path := filepath.Join(root, f.RelPath)
		if err := os.WriteFile(path, f.Content, 0o644); err != nil {
			return nil, fmt.Errorf("decisionsweep: writing %s: %w", path, err)
		}
		written = append(written, path)
	}
	return written, nil
}
