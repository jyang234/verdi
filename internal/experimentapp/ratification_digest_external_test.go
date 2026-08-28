package experimentapp_test

import (
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
	"github.com/jyang234/verdi/internal/experimentapp"
)

func TestRatificationInputDigestExportBindsCompleteTypedInput(t *testing.T) {
	tests := []struct {
		name        string
		result      string
		disposition experiment.Disposition
		candidate   string
		reason      string
		want        string
	}{
		{
			name:        "select-other",
			result:      "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			disposition: experiment.DispositionSelectOther,
			candidate:   "cache",
			reason:      "latency evidence",
			want:        "sha256:2f143743908ac45ed163435e3daf843a0513744e0c68b72783f48c3aa4663306",
		},
		{
			name:        "empty-fields-remain-bound",
			result:      "sha256:2222222222222222222222222222222222222222222222222222222222222222",
			disposition: experiment.DispositionRejectAll,
			want:        "sha256:fd20f5004ef538319c7fafff63cd4d6017debe85efca157ad6dd167d6c8fed82",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := experimentapp.RatificationInputDigest(test.result, test.disposition, test.candidate, test.reason)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("digest = %q, want %q", got, test.want)
			}
		})
	}
}
