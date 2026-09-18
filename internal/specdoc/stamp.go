// internal/specdoc/stamp.go
package specdoc

import "github.com/jyang234/verdi/internal/canonjson"

// Stamp identifies exactly which bytes a document was rendered from.
type Stamp struct {
	// Ref is the canonical spec ref, e.g. "spec/uat-round-1".
	Ref string
	// Commit is the full commit the spec bytes were read at.
	Commit string
	// Engine is EngineDigest() at render time.
	Engine string
	// Proposed is true when the bytes are not the accepted bytes on the
	// default branch (a design-branch render). The header says so.
	Proposed bool
}

const (
	engineID      = "verdi.specdoc"
	engineVersion = 1
)

type engineDescriptor struct {
	ID       string               `json:"id"`
	Version  int                  `json:"version"`
	Sections map[Kind][]SectionID `json:"sections"`
}

func engineDescriptorDigest(d engineDescriptor) string {
	digest, err := canonjson.Digest(d)
	if err != nil {
		// The descriptor is a fixed literal; a marshal failure is a
		// programming error, never an input error.
		panic("specdoc: engine descriptor is not canonical-JSON encodable: " + err.Error())
	}
	return digest
}

// EngineDigest is the canonical digest of the renderer's identity: its
// id, version, and every kind's section order. It changes whenever the
// document shape changes, so two renders with equal stamps have equal
// shape.
func EngineDigest() string {
	return engineDescriptorDigest(engineDescriptor{
		ID:      engineID,
		Version: engineVersion,
		Sections: map[Kind][]SectionID{
			KindSpec:  KindSpec.Sections(),
			KindPlan:  KindPlan.Sections(),
			KindTasks: KindTasks.Sections(),
		},
	})
}
