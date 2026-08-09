package humanartifact

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/jyang234/verdi/internal/designscaffold"
)

// Scaffold is one resolved template's identity, digest, and bytes — the
// return of the ONE resolver every human-artifact creation surface
// shares (AC-1: "CLI, workbench, and agent-assisted creation use one
// resolver and renderer"). Identity and Digest are exactly what a
// created artifact's own `template:` record must carry (AC-1: "A
// created artifact records the resolved template identity and digest";
// internal/policyartifact.TemplateRecord's own Identity/Digest fields).
type Scaffold struct {
	Identity string
	Digest   string
	Template []byte
}

// ResolveScaffold resolves filename's template exactly like
// designscaffold.LoadTemplate does — a store override at
// .verdi/templates/<filename> under root winning over the embedded
// canonical default of the same name (designscaffold.LoadOverride /
// designscaffold.Canonical) — but additionally records the resolved
// source's own identity ("store:.verdi/templates/<filename>" or
// "embedded:<filename>") and content digest ("sha256:" plus the hex
// SHA-256 of the resolved template bytes), so a caller can fill a
// created artifact's own template record without a second resolution
// pass. An unsafe filename (LoadOverride's own bare-filename containment
// guard) or a filename naming neither a store override nor an embedded
// canonical default fails closed, propagated verbatim.
func ResolveScaffold(root, filename string) (Scaffold, error) {
	data, ok, err := designscaffold.LoadOverride(root, filename)
	if err != nil {
		return Scaffold{}, err
	}
	identity := "embedded:" + filename
	if ok {
		identity = "store:.verdi/templates/" + filename
	} else {
		data, err = designscaffold.Canonical(filename)
		if err != nil {
			return Scaffold{}, err
		}
	}
	sum := sha256.Sum256(data)
	return Scaffold{
		Identity: identity,
		Digest:   "sha256:" + hex.EncodeToString(sum[:]),
		Template: data,
	}, nil
}
