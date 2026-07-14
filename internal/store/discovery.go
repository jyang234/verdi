package store

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
)

// flowmapFile is the upstream service-root marker (01 §Store manifest:
// "any directory containing .flowmap.yaml is a service root").
const flowmapFile = ".flowmap.yaml"

// BoundaryContractRelPath is upstream's own fixed path for a service's
// boundary contract, corrected by spike S1 (PLAN.md §3 "Boundary
// contracts" row): "flowmap boundary has no stdout mode or output flag —
// it always writes there".
const BoundaryContractRelPath = ".flowmap/boundary-contract.json"

// bindingsFile is the one verdi-owned file allowed in a service root
// (01 §notes; I-2).
const bindingsFile = "verdi.bindings.yaml"

// openAPICandidates are the OpenAPI file names checked, in priority order,
// at <service-dir>/api/ (05 §dex OpenAPI discovery; presence only in
// phase 3 — content is never decoded here).
var openAPICandidates = []string{"openapi.yaml", "openapi.yml", "openapi.json"}

// skipDirNames are directory names DiscoverServices never descends into,
// beyond the .verdi/data special case handled separately (01 §Store
// manifest service-discovery row: "skip .git, .verdi/data, node_modules-ish
// noise"). "testdata" joined this set in phase 4: this module's own
// testdata/svcfix and testdata/corpus fixtures are real, flowmap-shaped
// service roots (needed to exercise discovery itself), and without this
// exclusion self-hosting verdi's own store (PLAN.md Phase 4) discovers
// them as if they were live services of this repo — PLAN.md §2 already
// designates testdata/ as "the hermetic fixture" directory store-wide, so
// this is the same noise class .git/node_modules already are, not a new
// policy.
var skipDirNames = map[string]bool{
	".git":         true,
	"node_modules": true,
	"testdata":     true,
}

// Service is one discovered service root: a directory containing
// .flowmap.yaml, plus whatever of the fixed-path companion files
// (boundary contract, bindings sidecar, OpenAPI doc) are present.
type Service struct {
	// Dir is the service root's absolute path.
	Dir string
	// Name is the `service:` key from .flowmap.yaml, or Dir's base name if
	// that key is absent (upstream's own default-naming behavior).
	Name string
	// Obligations lists .flowmap.yaml's obligations[].name, in file order.
	Obligations []string
	// BoundaryContractPath is the absolute path to
	// .flowmap/boundary-contract.json if present, else "".
	BoundaryContractPath string
	// BindingsPath is the absolute path to verdi.bindings.yaml if present,
	// else "".
	BindingsPath string
	// Bindings is the strict-decoded sidecar, non-nil iff BindingsPath != "".
	Bindings *artifact.Bindings
	// OpenAPIPath is the absolute path to the discovered api/openapi.*
	// file if present, else "".
	OpenAPIPath string
}

// DiscoverServices walks root looking for service roots: any directory
// containing .flowmap.yaml (01 §Store manifest: "services.discovery:
// flowmap"). It skips .git, .verdi/data, and node_modules-ish noise, and
// returns services sorted by Dir for determinism.
func DiscoverServices(root string) ([]Service, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("store: DiscoverServices(%q): %w", root, err)
	}
	verdiData := filepath.Join(absRoot, ".verdi", "data")

	var services []Service
	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != absRoot {
				base := d.Name()
				if skipDirNames[base] {
					return filepath.SkipDir
				}
				if path == verdiData {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Name() != flowmapFile {
			return nil
		}

		dir := filepath.Dir(path)
		svc, err := loadService(dir)
		if err != nil {
			return fmt.Errorf("store: discovering service at %s: %w", dir, err)
		}
		services = append(services, *svc)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(services, func(i, j int) bool { return services[i].Dir < services[j].Dir })
	return services, nil
}

// loadService reads dir's .flowmap.yaml (loose decode — it is
// upstream-owned) and probes for the fixed-path companion files.
func loadService(dir string) (*Service, error) {
	data, err := os.ReadFile(filepath.Join(dir, flowmapFile))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", flowmapFile, err)
	}
	summary, err := artifact.DecodeFlowmapLoose(data)
	if err != nil {
		return nil, err
	}

	name := summary.Service
	if name == "" {
		name = filepath.Base(dir)
	}

	svc := &Service{
		Dir:         dir,
		Name:        name,
		Obligations: summary.Obligations,
	}

	if p := filepath.Join(dir, BoundaryContractRelPath); fileExists(p) {
		svc.BoundaryContractPath = p
	}

	if p := filepath.Join(dir, bindingsFile); fileExists(p) {
		bdata, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", bindingsFile, err)
		}
		bindings, err := artifact.DecodeBindings(bdata)
		if err != nil {
			return nil, fmt.Errorf("decoding %s: %w", bindingsFile, err)
		}
		svc.BindingsPath = p
		svc.Bindings = bindings
	}

	for _, name := range openAPICandidates {
		p := filepath.Join(dir, "api", name)
		if fileExists(p) {
			svc.OpenAPIPath = p
			break
		}
	}

	return svc, nil
}

// FilterImpacted returns the subset of services whose Name is listed in
// impacts, preserving services' original (Dir-sorted, deterministic) order.
// Shared by cmd/verdi's baseline regeneration (`design start`/`feature
// start`) and internal/align's computed-section regeneration — both scope a
// discovered service set down to one spec's declared impacts: (CLAUDE.md:
// "anything used by two or more packages lives in a shared internal/
// package").
func FilterImpacted(services []Service, impacts []string) []Service {
	want := make(map[string]bool, len(impacts))
	for _, i := range impacts {
		want[i] = true
	}
	var out []Service
	for _, svc := range services {
		if want[svc.Name] {
			out = append(out, svc)
		}
	}
	return out
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
