package publicrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

type historicalSkip struct {
	Package string `json:"package"`
	Test    string `json:"test"`
	Reason  string `json:"reason"`
}

var historicalATCSkips = map[string]bool{
	"TestIntegrationStateMatrixJourneyParity": true, "TestIntegrationUnprovableFeatureTargetInventsNothing": true, "TestIntegrationVerdictExitDecodesItsWitness": true, "TestIntegrationOperationalExitStaysOperational": true, "TestIntegrationRefusesATamperedBinary": true, "TestIntegrationCompileContextEnvelopeReachesTheAuthority": true, "TestIntegrationSealedExecutionIsRefused": true, "TestIntegrationAcceptedBuildRefusesAStorelessRequest": true, "TestIntegrationEffectOnlyBuildStartReadsNothingBack": true,
}

func inspectRace(out execution, atc bool) ([]historicalSkip, error) {
	if out.Exit != 0 {
		return nil, fmt.Errorf("%w: race process exited %d", ErrVerdict, out.Exit)
	}
	f, err := os.Open(out.StdoutPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return raceEvents(f, atc)
}
func raceEvents(r io.Reader, atc bool) ([]historicalSkip, error) {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	open := map[string]bool{}
	done := map[string]bool{}
	runs := map[string]bool{}
	outputs := map[string]string{}
	skips := []historicalSkip{}
	for {
		var e testEvent
		if err := d.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		key := e.Package + ":" + e.Test
		switch e.Action {
		case "start":
			if open[e.Package] || done[e.Package] {
				return nil, fmt.Errorf("duplicate race package")
			}
			open[e.Package] = true
		case "run":
			if !open[e.Package] || runs[key] {
				return nil, fmt.Errorf("invalid race test run")
			}
			runs[key] = true
		case "output":
			outputs[key] += e.Output
		case "pass", "skip":
			if !open[e.Package] {
				return nil, fmt.Errorf("race completion without start")
			}
			if e.Test == "" {
				for k := range runs {
					if strings.HasPrefix(k, e.Package+":") {
						return nil, fmt.Errorf("incomplete race test %s", k)
					}
				}
				if e.Action == "skip" && !strings.Contains(outputs[key], "[no test files]") {
					return nil, fmt.Errorf("unexplained package skip")
				}
				delete(open, e.Package)
				done[e.Package] = true
				continue
			}
			if !runs[key] {
				return nil, fmt.Errorf("race terminal without run")
			}
			delete(runs, key)
			if e.Action == "skip" {
				if !allowedHistoricalSkip(e.Package, e.Test, outputs[key], atc) {
					return nil, fmt.Errorf("unexpected race skip %s", key)
				}
				skips = append(skips, historicalSkip{e.Package, e.Test, outputs[key]})
			}
		case "pause", "cont":
		default:
			return nil, fmt.Errorf("race action %s", e.Action)
		}
	}
	if len(open) != 0 || len(done) == 0 || len(runs) != 0 {
		return nil, fmt.Errorf("incomplete race stream")
	}
	sort.Slice(skips, func(i, j int) bool { return skips[i].Test < skips[j].Test })
	return skips, nil
}

func fullGates(ctx context.Context, x executor, c Config, env []string) ([]execution, []historicalSkip, error) {
	type gate struct {
		name, dir string
		args      []string
		race, atc bool
	}
	lanes := [][]gate{
		{{"verdi-verify", c.VerdiDir, []string{"make", "verify"}, false, false}, {"verdi-race", c.VerdiDir, []string{"go", "test", "-race", "-json", "./..."}, true, false}},
		{{"atc-verify", c.ATCDir, []string{"make", "verify"}, false, true}, {"atc-race", c.ATCDir, []string{"go", "test", "-race", "-json", "-count=1", "-timeout=30m", "./..."}, true, true}, {"boundary-check", c.ATCDir, []string{"make", "boundary-check"}, false, true}},
	}
	type result struct {
		executions []execution
		skips      []historicalSkip
		err        error
	}
	channels := []chan result{make(chan result, 1), make(chan result, 1)}
	for i, lane := range lanes {
		go func() {
			r := result{}
			defer func() { channels[i] <- r }()
			for _, g := range lane {
				out, err := x.Run(ctx, command{Dir: g.dir, Name: g.name, Args: g.args, Env: env})
				r.executions = append(r.executions, out)
				if err != nil {
					r.err = err
					return
				}
				if out.Exit != 0 {
					r.err = fmt.Errorf("%w: %s exited %d", ErrVerdict, g.name, out.Exit)
					return
				}
				if g.race {
					skips, err := inspectRace(out, g.atc)
					if err != nil {
						r.err = err
						return
					}
					r.skips = append(r.skips, skips...)
				}
			}
		}()
	}
	executions := []execution{}
	skips := []historicalSkip{}
	var err error
	for _, ch := range channels {
		r := <-ch
		executions = append(executions, r.executions...)
		skips = append(skips, r.skips...)
		if err == nil && r.err != nil {
			err = r.err
		}
	}
	return executions, skips, err
}

func allowedHistoricalSkip(pkg, name, reason string, atc bool) bool {
	if atc {
		return pkg == "github.com/jyang234/verdi-atc/internal/verdiproto" && historicalATCSkips[name] && reason != ""
	}

	// These unchanged, non-required suite cases are disclosures, never passes.
	// Match the whole captured skip message; only Go's location and elapsed time
	// may vary. A generator remains disabled and optional integrations stay optional.
	var message string
	switch pkg + ":" + name {
	case "github.com/jyang234/verdi/internal/bundle:TestGenerateBundleGolden":
		message = "set VERDI_GENGOLDEN=1 to (re)generate testdata/svcfix-canned/bundle-golden/"
	case "github.com/jyang234/verdi/internal/execworkspace:TestEncodeDecodeSidecar_RoundTripsToAnEqualIdentity/invalid_utf-8":
		message = `NewExactIdentity("run-\xff\xfe-x") rejected the run id: execworkspace: identity: run id "run-\xff\xfe-x" is not valid UTF-8`
	case "github.com/jyang234/verdi/internal/execworkspace:TestExecworkspaceHelperSleep":
		message = "not the re-exec helper: -execworkspace.helper.sleep unset"
	case "github.com/jyang234/verdi/internal/upstream:TestIntegration_LocalBinaries":
		message = "VERDI_S1_BIN not set: skipping the optional real-toolchain integration test (disclosed skip, not a silent pass — see localbin_test.go)"
	}
	if message != "" {
		pattern := `^=== RUN   ` + regexp.QuoteMeta(name) + `\n    [^\n]+\.go:[0-9]+: ` + regexp.QuoteMeta(message) + `\n--- SKIP: ` + regexp.QuoteMeta(name) + ` \([0-9]+\.[0-9]{2}s\)\n$`
		return regexp.MustCompile(pattern).MatchString(reason)
	}

	if pkg != "github.com/jyang234/verdi/internal/specalign" || !strings.Contains(reason, "DISCLOSURE:") || !strings.Contains(reason, "SKIP, not a pass") {
		return false
	}
	switch name {
	case "TestSelfHostedSpecFidelity":
		return strings.Contains(reason, "workspace docs dir") && strings.Contains(reason, "CANNOT be verified in this layout")
	case "TestGuideClaimsTranscriptionFidelity_AppendixB", "TestGuideClaimsCite_ResolutionWorkspaceSideOnly":
		return strings.Contains(reason, "no workspace marker docs/design/plans found")
	default:
		return false
	}
}
