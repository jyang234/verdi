package initwizard

import (
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
)

// VerdiYAMLContent is the entire verdi.yaml content BOTH verdi init
// paths write (spec/init-wizard ac-1: "R4-I-56's conservative scope: no
// invented forge or tracker defaults") — schema only. Forge
// auto-detects from the origin remote when omitted and providers/
// services/align/audit/etc. are all optional (internal/store/
// manifest.go's Manifest — Schema is the only non-omitempty field), so
// this is a complete, valid manifest, not an abbreviation of one. The
// frozen feature ac-1 and this story's own three ACs name only
// vocabulary/template-set/display-convention configuration for
// --wizard; forge and tracker configuration are out of this story's
// scope (disclosed in the build report, never silently assumed).
const VerdiYAMLContent = "schema: verdi.layout/v1\n"

// VocabularyEmpty reports whether vocab carries no renames at all — the
// "model.yaml only on divergence from canonical" predicate (spec/
// init-wizard outcome, ledger L-N5) cmd/verdi/init.go's staging step
// uses to decide whether to write model.yaml at all.
func VocabularyEmpty(vocab model.Vocabulary) bool {
	return len(vocab.Classes) == 0 && len(vocab.States) == 0 && len(vocab.Verbs) == 0
}

// CandidateModel returns model.Canonical() with vocab applied as its
// Vocabulary field — the in-memory value the wizard's live validation
// preview and the final W-4 decode-compare gate both prove staged
// content against. Canonical() decodes fresh on every call (its own doc
// comment: "never a shared, cached pointer"), so mutating the returned
// value's Vocabulary field here can never corrupt another caller's copy.
func CandidateModel(vocab model.Vocabulary) *model.Model {
	cand := model.Canonical()
	cand.Vocabulary = vocab
	return cand
}

// RenderModelYAML hand-renders a model.yaml candidate: model.
// CanonicalYAML()'s own bytes (the classes:/lifecycle: block — the
// canonical shape the v1 frontier requires verbatim, template filenames
// and vocabulary excepted) with a vocabulary: block appended describing
// vocab. Hand-built via string concatenation, never decode→struct→
// yaml.Marshal→reassemble — the module-wide posture internal/workbench/
// obligationauthor.go's renderObligation already documents, and design
// doc §12 W-4's own disclosed choice for this story ("string-build from
// canonical.yaml"). Every display VALUE is passed through
// artifact.YAMLDoubleQuote (the same safe-quoting renderObligation
// already uses for title/owners): a value carrying a newline, a double
// quote, or a ": " sequence could otherwise smuggle a second top-level
// key or corrupt the surrounding plain scalar (designscaffold's own K4
// class of defect, closed there by safeScalar) — double-quoting
// unconditionally is simplest and, unlike designscaffold's conditional
// safeScalar, carries no byte-identity obligation to protect here (this
// is a fresh render, not a stability pin against pre-existing bytes).
//
// An empty vocab (VocabularyEmpty) renders WITHOUT any vocabulary: block
// at all — decoding back to a Model whose own Vocabulary is itself
// empty, mirroring model.Canonical()'s deliberately-empty display layer.
// Only sections with at least one entry are emitted (classes/states/
// verbs independently), and every map is walked in sorted key order
// (CLAUDE.md: deterministic outputs) — map iteration order never leaks
// into the rendered bytes.
func RenderModelYAML(vocab model.Vocabulary) []byte {
	var b strings.Builder
	b.Write(model.CanonicalYAML())

	if VocabularyEmpty(vocab) {
		return []byte(b.String())
	}

	b.WriteString("\nvocabulary:\n")
	writeVocabSection(&b, "classes", vocab.Classes)
	writeVocabSection(&b, "states", vocab.States)
	writeVocabSection(&b, "verbs", vocab.Verbs)
	return []byte(b.String())
}

// writeVocabSection appends one vocabulary.<section> block (classes,
// states, or verbs) to b in sorted-key order, or nothing at all when
// entries is empty — the per-section independence RenderModelYAML's own
// doc comment promises (a rename touching only classes never emits
// empty states:/verbs: keys).
func writeVocabSection(b *strings.Builder, section string, entries map[string]string) {
	if len(entries) == 0 {
		return
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteString("  ")
	b.WriteString(section)
	b.WriteString(":\n")
	for _, k := range keys {
		b.WriteString("    ")
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(artifact.YAMLDoubleQuote(entries[k]))
		b.WriteString("\n")
	}
}
