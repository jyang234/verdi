// Command oq4digest is spike/self-governance oq-4's helper: it reads a
// real verdi.policy/v1 artifact from disk, decodes it through the exact
// same policyartifact.DecodePolicy seam the built binary uses, and prints
// the real canonical ClaimDigest of one named claim — the exact-witness
// value an exemption's witnesses[].claim_digest must carry (DC-8) so the
// scratch exemption fixture is not a synthetic digest.
//
// Usage: go run ./docs/spikes/self-governance/_scratch/oq4digest <policy-file> <claim-id>
package main

import (
	"fmt"
	"os"

	"github.com/jyang234/verdi/internal/policyartifact"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: oq4digest <policy-file> <claim-id>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(2)
	}
	p, err := policyartifact.DecodePolicy(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "decode:", err)
		os.Exit(2)
	}
	claim, ok := p.Claim(os.Args[2])
	if !ok {
		fmt.Fprintf(os.Stderr, "policy %s has no claim %q\n", p.ID, os.Args[2])
		os.Exit(2)
	}
	digest, err := policyartifact.ClaimDigest(claim)
	if err != nil {
		fmt.Fprintln(os.Stderr, "digest:", err)
		os.Exit(2)
	}
	policyDigest, err := p.Digest()
	if err != nil {
		fmt.Fprintln(os.Stderr, "policy digest:", err)
		os.Exit(2)
	}
	fmt.Printf("policy=%s policy_digest=%s claim=%s claim_digest=%s\n", p.ID, policyDigest, claim.ID, digest)
}
