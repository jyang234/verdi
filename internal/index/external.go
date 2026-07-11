package index

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/OWNER/verdi/internal/store"
)

// flowmapFileName mirrors internal/store's own unexported constant; kept
// here rather than exported from store because it is only needed to point
// an obligation entry's Path at the file it was discovered in.
const flowmapFileName = ".flowmap.yaml"

// externalEntries mints the index's external refs (02 §External refs) from
// store.DiscoverServices' results: "svc/<service>/boundary-contract" when a
// boundary contract was found, "svc/<service>/obligations/<name>" for each
// discovered obligation, and "svc/<service>/api" when an OpenAPI doc was
// found. Body carries the underlying file's raw content (or, for
// obligations — which have no file of their own — the obligation name) so
// these entries participate in full-text search.
func externalEntries(services []store.Service) ([]*Entry, error) {
	var entries []*Entry

	for _, svc := range services {
		if svc.BoundaryContractPath != "" {
			data, err := os.ReadFile(svc.BoundaryContractPath)
			if err != nil {
				return nil, fmt.Errorf("index: reading boundary contract for service %s: %w", svc.Name, err)
			}
			entries = append(entries, &Entry{
				Ref:   fmt.Sprintf("svc/%s/boundary-contract", svc.Name),
				Kind:  "external",
				Title: fmt.Sprintf("%s boundary contract", svc.Name),
				Path:  svc.BoundaryContractPath,
				Body:  string(data),
			})
		}

		for _, obligation := range svc.Obligations {
			entries = append(entries, &Entry{
				Ref:   fmt.Sprintf("svc/%s/obligations/%s", svc.Name, obligation),
				Kind:  "external",
				Title: fmt.Sprintf("%s obligation: %s", svc.Name, obligation),
				Path:  filepath.Join(svc.Dir, flowmapFileName),
				Body:  obligation,
			})
		}

		if svc.OpenAPIPath != "" {
			data, err := os.ReadFile(svc.OpenAPIPath)
			if err != nil {
				return nil, fmt.Errorf("index: reading OpenAPI doc for service %s: %w", svc.Name, err)
			}
			entries = append(entries, &Entry{
				Ref:   fmt.Sprintf("svc/%s/api", svc.Name),
				Kind:  "external",
				Title: fmt.Sprintf("%s API", svc.Name),
				Path:  svc.OpenAPIPath,
				Body:  string(data),
			})
		}
	}

	return entries, nil
}
