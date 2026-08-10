package draftmutation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

// Recover validates and deterministically completes or rolls back one journal.
// Malformed/tampered state is retained and returned as an operational error.
func (w *LockedWriter) Recover(name string) error {
	if _, err := artifact.ParseRef("spec/" + name); err != nil {
		return fmt.Errorf("draftmutation: recovery spec name %q is invalid", name)
	}
	txDir := store.DraftMutationDir(w.root, name)
	info, err := os.Lstat(txDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("draftmutation: inspecting recovery root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("draftmutation: recovery root %s is a symlink or not a directory", txDir)
	}
	entries, err := os.ReadDir(txDir)
	if err != nil {
		return err
	}
	allowed := map[string]bool{"journal.json": true, "spec.new": true, "provenance.new": true}
	for _, entry := range entries {
		if !allowed[entry.Name()] || entry.Type()&os.ModeSymlink != 0 || entry.IsDir() {
			return fmt.Errorf("draftmutation: recovery root contains unexpected or symlink entry %q; retained", entry.Name())
		}
	}
	journalPath := store.DraftMutationJournalPath(w.root, name)
	if err := validateRegularDestination(journalPath, false); err != nil {
		return fmt.Errorf("draftmutation: recovery journal; retained: %w", err)
	}
	journalBytes, err := os.ReadFile(journalPath)
	if errors.Is(err, os.ErrNotExist) {
		if len(entries) != 0 {
			return fmt.Errorf("draftmutation: recovery staging exists without journal; retained")
		}
		return w.removeEmptyTransactionRoot(name)
	}
	if err != nil {
		return fmt.Errorf("draftmutation: reading recovery journal: %w", err)
	}
	var record journal
	if err := artifact.DecodeExactJSON(journalBytes, &record); err != nil {
		return fmt.Errorf("draftmutation: decoding recovery journal; retained: %w", err)
	}
	if err := record.validate(name); err != nil {
		return fmt.Errorf("draftmutation: invalid recovery journal; retained: %w", err)
	}
	specPath := store.SpecPath(w.root, store.ZoneActive, name)
	provenancePath := store.DesignProvenancePath(w.root, store.ZoneActive, name)
	if err := validateDirectoryChain(w.root, filepath.Dir(specPath)); err != nil {
		return err
	}
	if err := validateRegularDestination(specPath, true); err != nil {
		return err
	}
	if err := validateRegularDestination(provenancePath, false); err != nil {
		return err
	}
	specDigest, specExists, err := fileDigest(specPath)
	if err != nil || !specExists {
		return fmt.Errorf("draftmutation: reading recovery spec destination: %w", err)
	}
	provenanceDigest, provenanceExists, err := fileDigest(provenancePath)
	if err != nil {
		return err
	}
	specOld, specNew := specDigest == record.OldSpecDigest, specDigest == record.NewSpecDigest
	provenanceOld := provenanceExists == record.OldProvenanceExists && (!provenanceExists || provenanceDigest == record.OldProvenanceDigest)
	provenanceNew := provenanceExists && provenanceDigest == record.NewProvenanceDigest
	if !specOld && !specNew {
		return fmt.Errorf("draftmutation: recovery spec destination digest is neither old nor new; retained")
	}
	if !provenanceOld && !provenanceNew {
		return fmt.Errorf("draftmutation: recovery provenance destination digest is neither old nor new; retained")
	}
	if specNew && !provenanceNew {
		return fmt.Errorf("draftmutation: invalid spec-ahead recovery state; retained")
	}
	specStage, err := w.validStage(store.DraftMutationSpecStagePath(w.root, name), record.NewSpecDigest)
	if err != nil {
		return err
	}
	provenanceStage, err := w.validStage(store.DraftMutationProvenanceStagePath(w.root, name), record.NewProvenanceDigest)
	if err != nil {
		return err
	}

	switch {
	case provenanceNew && specNew:
		return w.cleanup(name)
	case provenanceNew && specOld:
		if !specStage {
			return fmt.Errorf("draftmutation: provenance is installed but spec stage is missing; retained")
		}
		if err := w.rename(store.DraftMutationSpecStagePath(w.root, name), specPath, StepSpecRename); err != nil {
			return err
		}
		if err := w.syncDirectory(filepath.Dir(specPath), StepSpecDirectorySync); err != nil {
			return err
		}
		return w.cleanup(name)
	case provenanceOld && specOld && specStage && provenanceStage:
		if err := w.rename(store.DraftMutationProvenanceStagePath(w.root, name), provenancePath, StepProvenanceRename); err != nil {
			return err
		}
		if err := w.syncDirectory(filepath.Dir(provenancePath), StepProvenanceDirectorySync); err != nil {
			return err
		}
		if err := w.rename(store.DraftMutationSpecStagePath(w.root, name), specPath, StepSpecRename); err != nil {
			return err
		}
		if err := w.syncDirectory(filepath.Dir(specPath), StepSpecDirectorySync); err != nil {
			return err
		}
		return w.cleanup(name)
	case provenanceOld && specOld:
		return w.cleanup(name)
	default:
		return fmt.Errorf("draftmutation: unrecognized recovery state; retained")
	}
}

func fileDigest(path string) (string, bool, error) {
	data, exists, err := readOptionalRegular(path)
	if err != nil || !exists {
		return "", exists, err
	}
	return DigestBytes(data), true, nil
}

func (w *LockedWriter) validStage(path, wantDigest string) (bool, error) {
	digest, exists, err := fileDigest(path)
	if err != nil {
		return false, err
	}
	if exists && digest != wantDigest {
		return false, fmt.Errorf("draftmutation: staged file %s digest is invalid; retained", path)
	}
	return exists, nil
}

func (w *LockedWriter) cleanup(name string) error {
	txDir := store.DraftMutationDir(w.root, name)
	for _, path := range []string{store.DraftMutationSpecStagePath(w.root, name), store.DraftMutationProvenanceStagePath(w.root, name)} {
		if err := os.Remove(path); err == nil {
			if err := w.coordinator.after(StepCleanupStage); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("draftmutation: removing stage %s: %w", path, err)
		}
	}
	journalPath := store.DraftMutationJournalPath(w.root, name)
	if err := os.Remove(journalPath); err == nil {
		if err := w.coordinator.after(StepCleanupJournal); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("draftmutation: removing journal: %w", err)
	}
	if err := w.syncDirectory(txDir, StepCleanupDirectorySync); err != nil {
		return err
	}
	return w.removeEmptyTransactionRoot(name)
}

func (w *LockedWriter) removeEmptyTransactionRoot(name string) error {
	txDir := store.DraftMutationDir(w.root, name)
	if err := os.Remove(txDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("draftmutation: removing empty transaction root: %w", err)
	}
	if err := w.coordinator.after(StepCleanupTransactionRoot); err != nil {
		return err
	}
	parent := filepath.Dir(txDir)
	if err := w.syncDirectory(parent, StepCleanupParentDirectory); err != nil {
		return err
	}
	return nil
}
