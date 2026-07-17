package store

import (
	"path"
	"path/filepath"
)

// This file is the single assembler for the .verdi store layout
// (01-store-layout.md §Directory layout). Before it existed, ~140 sites
// across eleven packages hand-rolled these paths, in two drifting idioms:
// filepath.Join(root, ".verdi", "specs", "active", name, "spec.md") and the
// string-concatenated ".verdi/specs/active/"+name+"/spec.md". Both name the
// same on-disk file; the divergence was accidental, not semantic. The
// accessors here are the one place the layout literals live, so a future
// layout change is a single edit and no two call sites can silently disagree
// (ADJ-71; the pattern BoundaryContractRelPath already established for
// upstream's boundary contract, generalized to verdi's own artifacts).
//
// Two consumer families exist and are BOTH served here:
//
//   - Absolute filesystem paths (the *Dir / *Path functions): rooted at a
//     store root and built with filepath.Join, so they carry the host's
//     native separator and are what os.ReadFile/Stat/Rename consume.
//
//   - Store-relative, slash-canonical paths (the *RelPath / *RelDir
//     functions): rooted at .verdi, always "/"-separated regardless of host,
//     because they are compared and stored as stable identifiers — git
//     tree/blob paths, derivation-record Input paths, lint doc keys, and the
//     store-relative forms disclosures print. These must never pick up a
//     backslash on any host, so they are built with path.Join, not
//     filepath.Join.
//
// zone is the specs/ lifecycle subdirectory: exactly ZoneActive or
// ZoneArchive. It is a plain string (not a validated enum) because these are
// pure path assemblers over caller-controlled input — an unknown zone yields
// a path that simply will not exist on disk, exactly as the hand-rolled sites
// behaved before. Callers that already branch or iterate over both zones
// pass their loop variable straight through; fixed-zone callers use the
// ActiveSpec*/ArchiveSpec* conveniences or the ZoneActive constant.
const (
	// ZoneActive is the specs/active/ zone: a spec that has not (yet) been
	// closed. ZoneArchive is specs/archive/: a spec moved there by the
	// closure ritual's active→archive rename (02 §Identity and references —
	// the move changes the path but never the ref).
	ZoneActive  = "active"
	ZoneArchive = "archive"
)

// Layout segments — the literals every accessor below is built from, named
// once so the strings appear in exactly one place.
const (
	verdiDir        = ".verdi"
	specsDir        = "specs"
	attestationsDir = "attestations"
	dataDir         = "data"
	derivedDir      = "derived"

	specFile             = "spec.md"
	deviationReportFile  = "deviation-report.md"
	decisionConflictFile = "decision-conflict-report.md"
)

// --- absolute (root-joined, host-native separator) ---

// SpecDir is the directory holding a spec's quartet under root:
// <root>/.verdi/specs/<zone>/<name>/ (01 §Directory layout).
func SpecDir(root, zone, name string) string {
	return filepath.Join(root, verdiDir, specsDir, zone, name)
}

// SpecPath is the spec.md inside SpecDir — the one file every spec directory
// is guaranteed to carry (01 §Directory layout; 02 §Kind registry).
func SpecPath(root, zone, name string) string {
	return filepath.Join(SpecDir(root, zone, name), specFile)
}

// ActiveSpecDir is SpecDir in the active zone.
func ActiveSpecDir(root, name string) string { return SpecDir(root, ZoneActive, name) }

// ActiveSpecPath is SpecPath in the active zone — the dominant case, since
// the workbench, gates, and every author-side verb read the working-tree
// (active) spec.
func ActiveSpecPath(root, name string) string { return SpecPath(root, ZoneActive, name) }

// ArchiveSpecDir is SpecDir in the archive zone.
func ArchiveSpecDir(root, name string) string { return SpecDir(root, ZoneArchive, name) }

// ArchiveSpecPath is SpecPath in the archive zone.
func ArchiveSpecPath(root, name string) string { return SpecPath(root, ZoneArchive, name) }

// DeviationReportPath is a spec directory's deviation-report.md — align's
// per-feature-spec computed/preserved report (04 §Alignment report).
func DeviationReportPath(root, zone, name string) string {
	return filepath.Join(SpecDir(root, zone, name), deviationReportFile)
}

// DecisionConflictReportPath is a spec directory's decision-conflict-report.md
// — align's per-design-spec twin of the deviation report (03 §Decision-
// conflict gate).
func DecisionConflictReportPath(root, zone, name string) string {
	return filepath.Join(SpecDir(root, zone, name), decisionConflictFile)
}

// AttestationDir is a story's attestation directory under root:
// <root>/.verdi/attestations/<storySlug>/ (I-6). storySlug is the story
// ref's slug (store.RefSlug of the spec's story: ref), not the spec name.
func AttestationDir(root, storySlug string) string {
	return filepath.Join(root, verdiDir, attestationsDir, storySlug)
}

// AttestationPath is the attestation file for (storySlug, acID):
// <AttestationDir>/<acID>.md (I-6/I-31) — the ONE construction the evidence
// fold's existence check and every writer resolve through, so a disclosure
// that must name this path never hand-derives a second, possibly-drifting
// copy. Passing an empty root yields the store-relative display form
// (".verdi/attestations/<storySlug>/<acID>.md") a disclosure prints instead
// of a temp-dir- or checkout-rooted absolute path, because filepath.Join
// drops an empty leading element.
func AttestationPath(root, storySlug, acID string) string {
	return filepath.Join(AttestationDir(root, storySlug), acID+".md")
}

// DerivedRoot is the derived-artifact tree root under root:
// <root>/.verdi/data/derived/ (01 §Directory layout). It is gitignored data,
// keyed one level down by ref-slug and then by commit.
func DerivedRoot(root string) string {
	return filepath.Join(root, verdiDir, dataDir, derivedDir)
}

// DerivedSpecDir is a single spec's derived subtree: <DerivedRoot>/<refSlug>/
// (01 §Directory layout). refSlug is already slugged by the caller
// (store.RefSlug of the spec id, or a fixture's literal slug); commit
// subdirectories are joined on top by callers that need a specific run.
func DerivedSpecDir(root, refSlug string) string {
	return filepath.Join(DerivedRoot(root), refSlug)
}

// --- store-relative (slash-canonical, .verdi-rooted) ---

// SpecRelPath is SpecPath's store-relative, slash-canonical form:
// ".verdi/specs/<zone>/<name>/spec.md". Used where the path is an identifier
// (git tree paths, derivation-record inputs, lint keys, disclosures), never a
// host filesystem path.
func SpecRelPath(zone, name string) string {
	return path.Join(verdiDir, specsDir, zone, name, specFile)
}

// ActiveSpecRelPath is SpecRelPath in the active zone.
func ActiveSpecRelPath(name string) string { return SpecRelPath(ZoneActive, name) }

// DeviationReportRelPath is DeviationReportPath's store-relative,
// slash-canonical form.
func DeviationReportRelPath(zone, name string) string {
	return path.Join(verdiDir, specsDir, zone, name, deviationReportFile)
}

// DecisionConflictReportRelPath is DecisionConflictReportPath's
// store-relative, slash-canonical form.
func DecisionConflictReportRelPath(zone, name string) string {
	return path.Join(verdiDir, specsDir, zone, name, decisionConflictFile)
}

// AttestationDirRelPath is AttestationDir's store-relative, slash-canonical
// form: ".verdi/attestations/<storySlug>" — the prefix acceptlint scopes a
// story spec's attestation reads to.
func AttestationDirRelPath(storySlug string) string {
	return path.Join(verdiDir, attestationsDir, storySlug)
}

// DerivedSpecRelDir is DerivedSpecDir's store-relative, slash-canonical form:
// ".verdi/data/derived/<refSlug>" — the derived-tree root a missing-evidence
// disclosure names.
func DerivedSpecRelDir(refSlug string) string {
	return path.Join(verdiDir, dataDir, derivedDir, refSlug)
}
