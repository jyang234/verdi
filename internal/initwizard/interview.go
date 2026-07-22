package initwizard

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/model"
)

// ErrAborted is returned by RunInterview when stdin ends before every
// prompt has been answered (spec/init-wizard ac-3: "a mid-interview
// abort ... leaves nothing whatsoever at the real root") — never
// silently treated as a default answer.
var ErrAborted = errors.New("initwizard: interview aborted — stdin ended before every prompt was answered")

// ErrDeclinedWrite is returned by RunInterview when the operator answers
// "no" at the final "write .verdi/?" confirmation — a clean, deliberate
// decline, distinguished from ErrAborted (an unexpectedly-ended script)
// so cmd/verdi/init.go's refusal message can name which one happened.
var ErrDeclinedWrite = errors.New("initwizard: declined to write at the final confirmation")

// InterviewResult is what a completed (non-aborted, confirmed) interview
// produced: the accumulated vocabulary renames and whether the operator
// chose to copy the canonical templates into .verdi/templates/ for local
// customization — the v1 frontier's two configurable axes (design doc
// §12; internal/model's checkFrontier admits nothing else on top of the
// canonical model).
type InterviewResult struct {
	Vocabulary    model.Vocabulary
	CopyTemplates bool
}

// vocabPhase is one rename phase (classes, states, or verbs): label is
// the generic, unrouted category word the prompt uses ("class"/"state"/
// "verb" — never a vocabulary WORD itself, so no vocab-prose hit is
// possible here by construction: the actual ids are runtime data from
// RenameableIDs(), never literal tokens in this source), ids is the
// phase's own renameable id list, and apply mutates vocab in place with
// a confirmed, non-empty answer.
type vocabPhase struct {
	label string
	ids   []string
	apply func(vocab *model.Vocabulary, id, val string)
}

// RunInterview drives the whole guided interview against in (the
// operator's scripted or real terminal input) and out (where every
// prompt, preview, and the final summary print), returning the
// confirmed InterviewResult or one of ErrAborted/ErrDeclinedWrite. It
// writes nothing to any filesystem itself — cmd/verdi/init.go owns
// staging the result's content and promoting it; this function is pure
// I/O over the given reader/writer, which is what lets it be driven
// hermetically by a stdin-script built-binary test (this story's own
// disclosed harness choice) without any real terminal or temp directory
// at all.
func RunInterview(in io.Reader, out io.Writer) (InterviewResult, error) {
	sc := bufio.NewScanner(in)

	fmt.Fprintln(out, "verdi init --wizard — configuring a store in this directory.")
	fmt.Fprintln(out, "Every answer is written to editable config; nothing here is final.")
	fmt.Fprintln(out)

	ids := RenameableIDs()
	phases := []vocabPhase{
		{label: "class", ids: ids.Classes, apply: func(v *model.Vocabulary, id, val string) {
			if v.Classes == nil {
				v.Classes = map[string]string{}
			}
			v.Classes[id] = val
		}},
		{label: "state", ids: ids.States, apply: func(v *model.Vocabulary, id, val string) {
			if v.States == nil {
				v.States = map[string]string{}
			}
			v.States[id] = val
		}},
		{label: "verb", ids: ids.Verbs, apply: func(v *model.Vocabulary, id, val string) {
			if v.Verbs == nil {
				v.Verbs = map[string]string{}
			}
			v.Verbs[id] = val
		}},
	}

	var vocab model.Vocabulary
	for _, phase := range phases {
		fmt.Fprintf(out, "Vocabulary — %s display words (Enter to keep the id unchanged):\n", phase.label)
		for _, id := range phase.ids {
			if err := runRenamePrompt(sc, out, phase, id, &vocab); err != nil {
				return InterviewResult{}, err
			}
		}
	}

	copyTemplates, err := promptYesNo(sc, out, "Copy the canonical templates into .verdi/templates/ for local customization?", false)
	if err != nil {
		return InterviewResult{}, err
	}

	requestedStructural, err := promptYesNo(sc, out, "Add, remove, or restructure the class hierarchy, lifecycle states, or per-transition obligations?", false)
	if err != nil {
		return InterviewResult{}, err
	}
	if requestedStructural {
		fmt.Fprintln(out, "verdi init: that is structural configuration, behind the v1 frontier today")
		fmt.Fprintln(out, "  (only vocabulary display words and a template-file choice are configurable")
		fmt.Fprintln(out, "  now — structural configuration unlocks per-verb later). Continuing —")
		fmt.Fprintln(out, "  nothing structural was changed.")
	}

	printSummary(out, vocab, copyTemplates)

	write, err := promptYesNo(sc, out, "Write .verdi/ ?", true)
	if err != nil {
		return InterviewResult{}, err
	}
	if !write {
		return InterviewResult{}, ErrDeclinedWrite
	}

	return InterviewResult{Vocabulary: vocab, CopyTemplates: copyTemplates}, nil
}

// runRenamePrompt asks a single rename question for one id in phase,
// applies a non-empty answer to vocab, and — the live validation
// preview, spec/init-wizard ac-2 — proves the resulting candidate model
// decodes cleanly through the SAME model.DecodeModel path `verdi model
// check` itself runs, by hand-rendering the vocabulary-so-far
// (RenderModelYAML) and decoding it back, before moving to the next
// prompt. A rename that somehow produces an undecodable candidate (no
// known input reaches this: every id offered is already a legal
// Vocabulary key per RenameableIDs, and a non-empty answer is the only
// value ever stored) is refused and the SAME id is re-asked, rather than
// silently accepted — the honest defense-in-depth this package's own
// doc comment promises, mirroring model.Canonical()'s own "packaging
// defect, never a user-facing condition" posture for its one
// unreachable-in-practice failure path.
func runRenamePrompt(sc *bufio.Scanner, out io.Writer, phase vocabPhase, id string, vocab *model.Vocabulary) error {
	for {
		fmt.Fprintf(out, "  %s %q [Enter to keep %q]: ", phase.label, id, id)
		val, ok := readLine(sc)
		if !ok {
			return ErrAborted
		}
		if val == "" {
			return nil
		}

		candidate := *vocab
		phase.apply(&candidate, id, val)

		rendered := RenderModelYAML(candidate)
		if _, err := model.DecodeModel(rendered); err != nil {
			fmt.Fprintf(out, "    -> rejected, not applied: %v\n", err)
			continue
		}

		*vocab = candidate
		digest, derr := CandidateModel(candidate).Digest()
		if derr != nil {
			fmt.Fprintf(out, "    -> valid\n")
		} else {
			fmt.Fprintf(out, "    -> valid (candidate digest %s)\n", digest)
		}
		return nil
	}
}

// printSummary renders the interview's own end-of-run summary —
// mirroring the guide's own "Summary — your operating model:" worked
// example shape — before the final write confirmation.
func printSummary(out io.Writer, vocab model.Vocabulary, copyTemplates bool) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Summary:")
	if VocabularyEmpty(vocab) {
		fmt.Fprintln(out, "  vocabulary: unchanged (every rename left at its default)")
	} else {
		printVocabSummaryLine(out, "classes", vocab.Classes)
		printVocabSummaryLine(out, "states", vocab.States)
		printVocabSummaryLine(out, "verbs", vocab.Verbs)
	}
	if copyTemplates {
		fmt.Fprintln(out, "  templates: canonical templates will be copied to .verdi/templates/ for local editing")
	} else {
		fmt.Fprintln(out, "  templates: unchanged (no local override copies)")
	}
	fmt.Fprintln(out)
}

func printVocabSummaryLine(out io.Writer, section string, entries map[string]string) {
	if len(entries) == 0 {
		return
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s -> %s", k, entries[k]))
	}
	fmt.Fprintf(out, "  %s: %s\n", section, strings.Join(pairs, ", "))
}

// promptYesNo asks a yes/no question, returning def when the answer is
// blank; "y"/"yes"/"n"/"no" (case-insensitive) are accepted, anything
// else re-prompts rather than guessing.
func promptYesNo(sc *bufio.Scanner, out io.Writer, question string, def bool) (bool, error) {
	hint := "y/N"
	if def {
		hint = "Y/n"
	}
	for {
		fmt.Fprintf(out, "%s [%s]: ", question, hint)
		val, ok := readLine(sc)
		if !ok {
			return false, ErrAborted
		}
		switch strings.ToLower(val) {
		case "":
			return def, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintf(out, "  please answer y or n\n")
		}
	}
}

// readLine reads one line from sc, trimmed, reporting ok=false when
// input ended before a line could be read (EOF or a scan error) — the
// one signal RunInterview's callers translate into ErrAborted, never
// into a silently-assumed default.
func readLine(sc *bufio.Scanner) (line string, ok bool) {
	if !sc.Scan() {
		return "", false
	}
	return strings.TrimSpace(sc.Text()), true
}
