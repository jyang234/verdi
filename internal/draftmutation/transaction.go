package draftmutation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/store"
)

const (
	journalSchema = "verdi.draftmutation-journal/v1"
	phasePrepared = "prepared"

	StepJournalWrite            = "journal-write"
	StepJournalSync             = "journal-fsync"
	StepJournalDirectorySync    = "journal-directory-fsync"
	StepSpecStageWrite          = "spec-stage-write"
	StepSpecStageSync           = "spec-stage-fsync"
	StepProvenanceStageWrite    = "provenance-stage-write"
	StepProvenanceStageSync     = "provenance-stage-fsync"
	StepStageDirectorySync      = "stage-directory-fsync"
	StepProvenanceRename        = "provenance-rename"
	StepProvenanceDirectorySync = "provenance-directory-fsync"
	StepSpecRename              = "spec-rename"
	StepSpecDirectorySync       = "spec-directory-fsync"
	StepCleanupJournal          = "cleanup-journal"
	StepCleanupStage            = "cleanup-stage"
	StepCleanupDirectorySync    = "cleanup-directory-fsync"
	StepCleanupTransactionRoot  = "cleanup-transaction-root"
	StepCleanupParentDirectory  = "cleanup-parent-directory-fsync"
	StepDirectoryParentSync     = "directory-parent-fsync:"
)

// Coordinator exposes deterministic boundary fault injection for tests.
// Nil hooks make the production zero value use real filesystem operations.
type Coordinator struct {
	After         func(step string) error
	DirectorySync func(directory *os.File) error
}

func (c Coordinator) after(step string) error {
	if c.After == nil {
		return nil
	}
	if err := c.After(step); err != nil {
		return fmt.Errorf("draftmutation: after %s: %w", step, err)
	}
	return nil
}

func (c Coordinator) syncDirectory(directory *os.File) error {
	if c.DirectorySync != nil {
		return c.DirectorySync(directory)
	}
	return directory.Sync()
}

// LockedWriter is only handed to callbacks after acquisition of D3's one
// checkout-wide writer lock.
type LockedWriter struct {
	ctx         context.Context
	root        string
	coordinator Coordinator
}

// WithWriterLock acquires the existing checkout-wide lock, performs work, and
// releases it. It never creates a per-spec lock.
func WithWriterLock(ctx context.Context, root string, coordinator Coordinator, work func(*LockedWriter) error) (resultErr error) {
	if ctx == nil {
		return fmt.Errorf("draftmutation: nil context")
	}
	if work == nil {
		return fmt.Errorf("draftmutation: nil writer callback")
	}
	writer := &LockedWriter{ctx: ctx, root: root, coordinator: coordinator}
	dataDir := filepath.Join(root, ".verdi", "data")
	if err := writer.ensureDirectory(dataDir); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	lockPath := store.WriterLockPath(root)
	if err := validateRegularDestination(lockPath, false); err != nil {
		return fmt.Errorf("draftmutation: validating global writer lock: %w", err)
	}
	lockFile, err := filelock.Acquire(lockPath)
	if err != nil {
		return fmt.Errorf("draftmutation: acquiring global writer lock: %w", err)
	}
	defer func() {
		if releaseErr := filelock.Release(lockFile, lockPath); releaseErr != nil && resultErr == nil {
			resultErr = releaseErr
		}
	}()
	return work(writer)
}

// Transaction carries exact old/new bytes for one spec and its complete
// provenance sidecar.
type Transaction struct {
	Spec                string
	OldSpec             []byte
	NewSpec             []byte
	OldProvenance       []byte
	OldProvenanceExists bool
	NewProvenance       []byte
}

type journal struct {
	Schema              string `json:"schema"`
	Spec                string `json:"spec"`
	SpecPath            string `json:"spec_path"`
	ProvenancePath      string `json:"provenance_path"`
	OldSpecDigest       string `json:"old_spec_digest"`
	NewSpecDigest       string `json:"new_spec_digest"`
	OldProvenanceDigest string `json:"old_provenance_digest"`
	NewProvenanceDigest string `json:"new_provenance_digest"`
	OldProvenanceExists bool   `json:"old_provenance_exists"`
	Phase               string `json:"phase"`
}

func (j journal) validate(name string) error {
	wantSpec := "spec/" + name
	if j.Schema != journalSchema || j.Spec != wantSpec || j.Phase != phasePrepared {
		return fmt.Errorf("draftmutation: journal schema/spec/phase is invalid")
	}
	if j.SpecPath != store.SpecRelPath(store.ZoneActive, name) || j.ProvenancePath != store.DesignProvenanceRelPath(store.ZoneActive, name) {
		return fmt.Errorf("draftmutation: journal paths are not the canonical transaction destinations")
	}
	for _, digest := range []string{j.OldSpecDigest, j.NewSpecDigest, j.OldProvenanceDigest, j.NewProvenanceDigest} {
		if !artifact.ValidDigest(digest) {
			return fmt.Errorf("draftmutation: journal digest %q is invalid", digest)
		}
	}
	return nil
}

// Commit prepares both new files and installs provenance before spec. Recover
// is invoked first so an older interrupted transaction is never overwritten.
func (w *LockedWriter) Commit(transaction Transaction) error {
	ref, err := artifact.ParseRef(transaction.Spec)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() || ref.Fragment() {
		return fmt.Errorf("draftmutation: transaction spec %q is invalid", transaction.Spec)
	}
	name := ref.Name
	if err := w.Recover(name); err != nil {
		return err
	}
	if len(transaction.NewProvenance) == 0 {
		return fmt.Errorf("draftmutation: new provenance must be nonempty")
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
	currentSpec, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("draftmutation: reading current spec: %w", err)
	}
	if !bytes.Equal(currentSpec, transaction.OldSpec) {
		return fmt.Errorf("draftmutation: current spec does not match transaction old bytes")
	}
	currentProvenance, provenanceExists, err := readOptionalRegular(provenancePath)
	if err != nil {
		return err
	}
	wantProvenanceExists := transaction.OldProvenanceExists || len(transaction.OldProvenance) > 0
	if provenanceExists != wantProvenanceExists || !bytes.Equal(currentProvenance, transaction.OldProvenance) {
		return fmt.Errorf("draftmutation: current provenance does not match transaction old bytes/presence")
	}
	txDir := store.DraftMutationDir(w.root, name)
	if err := w.ensureDirectory(txDir); err != nil {
		return err
	}
	entries, err := os.ReadDir(txDir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("draftmutation: transaction root is not empty after recovery")
	}
	record := journal{
		Schema: journalSchema, Spec: transaction.Spec,
		SpecPath: store.SpecRelPath(store.ZoneActive, name), ProvenancePath: store.DesignProvenanceRelPath(store.ZoneActive, name),
		OldSpecDigest: DigestBytes(transaction.OldSpec), NewSpecDigest: DigestBytes(transaction.NewSpec),
		OldProvenanceDigest: DigestBytes(transaction.OldProvenance), NewProvenanceDigest: DigestBytes(transaction.NewProvenance),
		OldProvenanceExists: wantProvenanceExists, Phase: phasePrepared,
	}
	journalBytes, err := canonjson.Marshal(record)
	if err != nil {
		return err
	}
	if err := w.writeExclusive(store.DraftMutationJournalPath(w.root, name), journalBytes, StepJournalWrite, StepJournalSync); err != nil {
		return err
	}
	if err := w.syncDirectory(txDir, StepJournalDirectorySync); err != nil {
		return err
	}
	if err := w.writeExclusive(store.DraftMutationSpecStagePath(w.root, name), transaction.NewSpec, StepSpecStageWrite, StepSpecStageSync); err != nil {
		return err
	}
	if err := w.writeExclusive(store.DraftMutationProvenanceStagePath(w.root, name), transaction.NewProvenance, StepProvenanceStageWrite, StepProvenanceStageSync); err != nil {
		return err
	}
	if err := w.syncDirectory(txDir, StepStageDirectorySync); err != nil {
		return err
	}
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
}

func (w *LockedWriter) writeExclusive(path string, data []byte, writeStep, syncStep string) (resultErr error) {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("draftmutation: creating %s: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && resultErr == nil {
			resultErr = closeErr
		}
	}()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("draftmutation: writing %s: %w", path, err)
	}
	if err := w.coordinator.after(writeStep); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("draftmutation: syncing %s: %w", path, err)
	}
	return w.coordinator.after(syncStep)
}

func (w *LockedWriter) rename(oldPath, newPath, step string) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("draftmutation: renaming %s to %s: %w", oldPath, newPath, err)
	}
	return w.coordinator.after(step)
}

func (w *LockedWriter) syncDirectory(path, step string) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("draftmutation: opening directory %s for fsync: %w", path, err)
	}
	err = w.coordinator.syncDirectory(directory)
	closeErr := directory.Close()
	if err != nil {
		return fmt.Errorf("draftmutation: syncing directory %s: %w", path, err)
	}
	if closeErr != nil {
		return closeErr
	}
	return w.coordinator.after(step)
}

func (w *LockedWriter) ensureDirectory(target string) error {
	absoluteRoot, err := filepath.Abs(w.root)
	if err != nil {
		return err
	}
	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteTarget)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("draftmutation: directory %q escapes checkout root", target)
	}
	rootInfo, err := os.Lstat(absoluteRoot)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("draftmutation: checkout root is missing, a symlink, or not a directory")
	}
	current := absoluteRoot
	if relative == "." {
		return nil
	}
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			if err := os.Mkdir(current, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("draftmutation: creating directory %s: %w", current, err)
			}
			// Re-read entries created here or by a racing process and apply
			// the same symlink/type checks; Mkdir success or EEXIST alone is
			// never sufficient authority.
			info, statErr = os.Lstat(current)
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("draftmutation: directory %s is a symlink or not a directory", current)
		}
		ensured, relErr := filepath.Rel(absoluteRoot, current)
		if relErr != nil {
			return fmt.Errorf("draftmutation: resolving ensured directory %s: %w", current, relErr)
		}
		if err := w.syncDirectory(filepath.Dir(current), StepDirectoryParentSync+filepath.ToSlash(ensured)); err != nil {
			return err
		}
	}
	return nil
}

func validateDirectoryChain(root, target string) error {
	_, err := validateDirectoryChainPresence(root, target, false)
	return err
}

func validateOptionalDirectoryChain(root, target string) (bool, error) {
	return validateDirectoryChainPresence(root, target, true)
}

func validateDirectoryChainPresence(root, target string, allowMissing bool) (bool, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	relative, err := filepath.Rel(absoluteRoot, absoluteTarget)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false, fmt.Errorf("draftmutation: directory %q escapes checkout root", target)
	}
	current := absoluteRoot
	components := []string{}
	if relative != "." {
		components = strings.Split(relative, string(filepath.Separator))
	}
	for _, component := range append([]string{""}, components...) {
		if component != "" {
			current = filepath.Join(current, component)
		}
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) && allowMissing && component != "" {
			return false, nil
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return false, fmt.Errorf("draftmutation: directory %s is missing, a symlink, or not a directory", current)
		}
	}
	return true, nil
}

func validateRegularDestination(path string, required bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && !required {
		return nil
	}
	if err != nil {
		return fmt.Errorf("draftmutation: inspecting destination %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("draftmutation: destination %s is a symlink", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("draftmutation: destination %s is not a regular file", path)
	}
	return nil
}

func readOptionalRegular(path string) ([]byte, bool, error) {
	if err := validateRegularDestination(path, false); err != nil {
		return nil, false, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []byte{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, true, fmt.Errorf("draftmutation: reading %s: %w", path, readErr)
	}
	if closeErr != nil {
		return nil, true, fmt.Errorf("draftmutation: closing %s: %w", path, closeErr)
	}
	return data, true, nil
}
