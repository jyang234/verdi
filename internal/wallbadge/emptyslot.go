// The empty-evidence-slot compute (spec/evidence-slot): for each of a
// STORY spec's acceptance criteria, the fold-derived record state of
// every DECLARED evidence kind, and a fold:empty-slot derivation record
// for each AC holding at least one empty slot. "Empty" is the REAL
// fold's definition and only there (dc-1/co-3): records load through the
// fold's own loader (evidence.LoadRecordsWithSources) from the derived
// tree, filter through the fold's own per-AC candidate filter
// (evidence.RecordsForAC), reduce through evidence.Current, and a kind
// is empty exactly when that current set holds no record of the kind —
// attestation-kind emptiness is evidence.LoadAttestationState's answer,
// with only the Authored state counting as held (spec/attest-helper dc-3:
// an unauthored `verdi attest` scaffold is not yet evidence, so it renders
// exactly as if no file existed at all). Never a wall-side
// reimplementation: if the fold's definition of "current" changes, this
// compute changes with it.
//
// A story wall with NO derived tree at all is the ordinary authoring
// state (derived records land at build time): every declared kind is a
// calm empty slot, never an error — and the derivation record still
// names the location probed, so the receipt is honest about what was
// looked at and found absent (dc-1).
package wallbadge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// SlotState is one declared evidence kind's fold-derived record presence
// for one acceptance criterion (spec/evidence-slot ac-1). Records counts
// the CURRENT records of the kind (after the fold's own per-AC filter
// and Current reduction; attestation: 1 exactly when the attestation
// file exists on disk AND has been authored — spec/attest-helper dc-3, an
// unauthored scaffold counts as 0, same as no file at all). Empty is true
// exactly when Records is 0 — the
// fold's per-kind no-record state, carried explicitly so consumers never
// re-derive it. This is presence disclosure only, never the fold's
// evidenced/violated/pending verdicts (dc-4): no verdict field exists
// here by design.
type SlotState struct {
	Kind    string
	Empty   bool
	Records int
}

// EmptySlotBadges computes, for one STORY spec, every AC's per-declared-
// kind slot state (keyed by AC id, in the AC's own declared kind order)
// and one fold:empty-slot derivation record per AC holding at least one
// empty slot (ac-2/dc-3): Target is the AC's own id (the card the badge
// anchors to), Inputs name the spec (specRelPath at specRevision, the
// caller's already-computed content digest), the derived-tree location
// probed, and every record file the fold's loader actually read (each
// with the sha256 of the exact bytes read), and Records disclose
// per-kind what was found or that nothing was. Revisions are digests or
// commit shas, never wall-clock time (co-1).
//
// The derived-tree input's revision is the HEAD commit the ancestry
// filter ran against (an honest pin: the current set is a function of
// the tree's files AND that commit), or the literal "absent" when the
// tree does not exist on disk — the calm never-synced authoring state,
// disclosed rather than dressed up as a digest of nothing.
//
// A spec declaring no evidence kinds on any AC returns (nil, nil, nil):
// no slots exist, so nothing is empty. Errors are operational (an
// unreadable store, a derived tree present in a non-git checkout), never
// a verdict.
func EmptySlotBadges(ctx context.Context, root, specRelPath, specRevision string, fm *artifact.SpecFrontmatter) (map[string][]SlotState, []DerivationRecord, error) {
	anyDeclared := false
	for _, ac := range fm.AcceptanceCriteria {
		if len(ac.Evidence) > 0 {
			anyDeclared = true
			break
		}
	}
	if !anyDeclared {
		return nil, nil, nil
	}

	derivedRel := ".verdi/data/derived/" + store.RefSlug(fm.ID)
	derivedRoot := filepath.Join(root, filepath.FromSlash(derivedRel))
	storySlug := store.RefSlug(fm.Story)

	// Probe the derived tree first: a missing tree is the ordinary
	// authoring state (dc-1) and needs no git at all, while a present
	// tree pins its current set to HEAD (the fold's own ancestry filter,
	// evidence.LoadRecords's contract).
	var records []artifact.Evidence
	var files []evidence.RecordFile
	treeRevision := "absent"
	if _, err := os.Stat(derivedRoot); err == nil {
		commit, err := gitx.RevParse(ctx, root, "HEAD")
		if err != nil {
			return nil, nil, fmt.Errorf("wallbadge: empty-slot: resolving HEAD for the derived-tree probe: %w", err)
		}
		records, files, err = evidence.LoadRecordsWithSources(ctx, root, derivedRoot, commit)
		if err != nil {
			return nil, nil, fmt.Errorf("wallbadge: empty-slot: %w", err)
		}
		treeRevision = commit
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("wallbadge: empty-slot: probing %s: %w", derivedRoot, err)
	}

	// The badge's pinned inputs are shared by every AC's record: the one
	// probe read one tree for the whole spec. Fully sorted by Name
	// ("derived-tree" < "record:…" < "spec"), matching this package's
	// deterministic-construction contract (record.go).
	inputs := []InputRecord{{Name: "derived-tree", Path: derivedRel, Revision: treeRevision}}
	for _, f := range files {
		inputs = append(inputs, InputRecord{Name: "record:" + f.Path, Path: derivedRel + "/" + f.Path, Revision: f.Digest})
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].Name < inputs[j].Name })
	inputs = append(inputs, InputRecord{Name: "spec", Path: specRelPath, Revision: specRevision})

	slots := make(map[string][]SlotState, len(fm.AcceptanceCriteria))
	var badges []DerivationRecord
	for _, ac := range fm.AcceptanceCriteria {
		if len(ac.Evidence) == 0 {
			continue
		}
		current := evidence.Current(evidence.RecordsForAC(records, ac.ID))

		states := make([]SlotState, 0, len(ac.Evidence))
		empties := 0
		for _, kind := range ac.Evidence {
			st := SlotState{Kind: string(kind)}
			if kind == artifact.EvidenceAttestation {
				// spec/attest-helper dc-3: only the AUTHORED state fills
				// the slot — an unauthored `verdi attest` scaffold is not
				// yet evidence (parent spec/closure-ergonomics dc-2), so it
				// renders exactly as if no file existed at all.
				state, err := evidence.LoadAttestationState(root, storySlug, ac.ID)
				if err != nil {
					return nil, nil, fmt.Errorf("wallbadge: empty-slot: %w", err)
				}
				if state == evidence.AttestationAuthored {
					st.Records = 1
				}
			} else {
				for _, r := range current {
					if string(r.Kind) == string(kind) {
						st.Records++
					}
				}
			}
			st.Empty = st.Records == 0
			if st.Empty {
				empties++
			}
			states = append(states, st)
		}
		slots[ac.ID] = states

		if empties == 0 {
			continue // every declared kind holds a record: nothing to badge
		}
		badges = append(badges, DerivationRecord{
			Source:  "fold:empty-slot",
			Label:   emptySlotLabel(empties),
			Target:  ac.ID,
			Inputs:  inputs,
			Records: slotRecordLines(states),
		})
	}
	return slots, badges, nil
}

// emptySlotLabel is the chip's short text — a count, never the kind
// names (each kind already reads on its own obligation row, ac-3: no
// card element repeats a kind).
func emptySlotLabel(empties int) string {
	if empties == 1 {
		return "empty slot"
	}
	return fmt.Sprintf("%d empty slots", empties)
}

// slotRecordLines renders one AC's per-kind findings as the derivation
// record's Records: what was found, or the explicit statement that
// nothing was (ac-2 — never silence about an empty kind). Sorted by kind
// name, matching this package's fully-sorted-Records contract
// (record.go); the card's rows keep the AC's declared order, this is the
// receipt's canonical order.
func slotRecordLines(states []SlotState) []string {
	lines := make([]string, 0, len(states))
	for _, st := range states {
		switch {
		case st.Kind == string(artifact.EvidenceAttestation) && st.Empty:
			lines = append(lines, st.Kind+": no attestation file on disk")
		case st.Kind == string(artifact.EvidenceAttestation):
			lines = append(lines, st.Kind+": attestation file present")
		case st.Empty:
			lines = append(lines, st.Kind+": no current record")
		case st.Records == 1:
			lines = append(lines, st.Kind+": 1 current record")
		default:
			lines = append(lines, fmt.Sprintf("%s: %d current records", st.Kind, st.Records))
		}
	}
	sort.Strings(lines)
	return lines
}
