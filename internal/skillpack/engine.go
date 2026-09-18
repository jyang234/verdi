package skillpack

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/specdoc"
)

//go:embed templates/*.md
var templates embed.FS

var skillNames = []string{"specify", "clarify", "plan", "tasks"}

// Skills lists the four skill names in render order.
func Skills() []string { return append([]string(nil), skillNames...) }

const (
	engineID      = "verdi.skillpack"
	engineVersion = 1
)

// engineDescriptor is what EngineDigest hashes: the renderer's identity,
// the skill and host inventories, and the document engine the skills read
// through get_document, so a document-shape change re-stamps every skill.
type engineDescriptor struct {
	ID       string   `json:"id"`
	Version  int      `json:"version"`
	Skills   []string `json:"skills"`
	Hosts    []string `json:"hosts"`
	Document string   `json:"document_engine"`
}

func engineDescriptorDigest(d engineDescriptor) string {
	digest, err := canonjson.Digest(d)
	if err != nil {
		panic("skillpack: engine descriptor is not canonical-JSON encodable: " + err.Error())
	}
	return digest
}

// EngineDigest is the canonical digest of the skillpack renderer's identity.
func EngineDigest() string {
	hosts := make([]string, 0, len(Hosts()))
	for _, h := range Hosts() {
		hosts = append(hosts, string(h))
	}
	return engineDescriptorDigest(engineDescriptor{
		ID: engineID, Version: engineVersion, Skills: Skills(), Hosts: hosts, Document: specdoc.EngineDigest(),
	})
}

// Template returns the embedded template bytes for skill.
func Template(skill string) ([]byte, error) {
	for _, s := range skillNames {
		if s == skill {
			return templates.ReadFile("templates/" + skill + ".md")
		}
	}
	return nil, fmt.Errorf("skillpack: unknown skill %q", skill)
}

// TemplateDigest is "sha256:"+hex over the exact embedded template bytes.
func TemplateDigest(skill string) (string, error) {
	b, err := Template(skill)
	if err != nil {
		return "", err
	}
	return contentDigest(b), nil
}

func contentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
