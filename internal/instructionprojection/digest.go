package instructionprojection

import (
	"crypto/sha256"
	"encoding/hex"
)

// contentDigest returns data's content address in the store's shared
// "sha256:"+hex form (internal/canonjson.Digest's own tail, restated
// here rather than imported because canonjson.Digest hashes a value's
// CANONICAL JSON ENCODING, not arbitrary bytes: a projected file's
// digest is defined as the hash of its exact on-disk bytes, so this
// package owns that one distinct tail itself).
func contentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}
