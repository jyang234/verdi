package publicrelease

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

type launchCounts struct{ Total, Sealed, Decode, Encode int }
type cutRow struct {
	Pair       string                                                           `json:"pair"`
	Cut        string                                                           `json:"cut"`
	KillExit   int                                                              `json:"kill_exit"`
	CutRows    []int                                                            `json:"cut_receipt_control_candidate"`
	Exit       int                                                              `json:"reentry_exit"`
	Classifier string                                                           `json:"classifier"`
	Rows       []int                                                            `json:"reentry_receipt_control_candidate"`
	Launches   launchCounts                                                     `json:"reentry_launches"`
	Shape      struct{ LedgerKinds, RecorderKinds, ControlOperations []string } `json:"reentry_shape"`
	Evidence   string                                                           `json:"evidence"`
}
type rawOutcome struct {
	Exit           int
	Stdout, Stderr string
	Trace          []struct {
		Kind string   `json:"kind"`
		Name string   `json:"name,omitempty"`
		PID  int      `json:"pid"`
		Args []string `json:"args,omitempty"`
	}
	Records json.RawMessage
}
type copyOutcome struct {
	Pair                 string       `json:"pair"`
	ShippedExit          int          `json:"shipped_exit"`
	ReplayExit           int          `json:"opposite_shipped_replay_exit"`
	DirtyGitExit         int          `json:"opposite_shipped_dirty_git_exit"`
	HandoffSHA           string       `json:"handoff_sha256"`
	RecordsSHA           string       `json:"records_sha256"`
	ReplayBytesEqual     bool         `json:"serialized_outcome_fields_equal"`
	DirtyGitRecordsEqual bool         `json:"dirty_git_serialized_records_equal"`
	TraceLaunches        launchCounts `json:"nonblocking_trace_launches"`
}
type armOutcome struct {
	Test         string `json:"test"`
	CallSHA      string `json:"call_sha256"`
	ResultSHA    string `json:"result_sha256"`
	LegacyDigest string `json:"legacy_request_digest,omitempty"`
}
type runtimeOutcomes struct {
	Cuts   [][]cutRow    `json:"cut_matrices"`
	Copies []copyOutcome `json:"copied_store_runs"`
	Arms   []armOutcome  `json:"arm_exchanges"`
}

func readRuntime(root string, files map[string]string, executions []execution) (runtimeOutcomes, error) {
	out := runtimeOutcomes{Cuts: [][]cutRow{}, Copies: []copyOutcome{}, Arms: []armOutcome{}}
	for _, name := range sortedKeys(files) {
		if strings.HasPrefix(name, "TestBoundaryRecoveryFiveProcessCuts-") && filepath.Base(name) == "matrix.json" {
			b, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				return out, err
			}
			var rows []cutRow
			if err = artifact.DecodeStrictJSON(b, &rows); err != nil {
				return out, err
			}
			if err = validateCuts(rows); err != nil {
				return out, err
			}
			for i := range rows {
				rows[i].Evidence = "private:" + filepath.Base(filepath.Dir(name)) + "/" + rows[i].Pair + "-" + rows[i].Cut
			}
			out.Cuts = append(out.Cuts, rows)
		}
		if strings.HasPrefix(name, "TestBoundaryRecoveryControlsAndCopiedCompletedStores-") && filepath.Base(name) == "uninstrumented.json" {
			row, err := readCopy(filepath.Join(root, filepath.Dir(name)))
			if err != nil {
				return out, err
			}
			out.Copies = append(out.Copies, row)
		}
	}
	if len(out.Cuts) == 0 || len(out.Copies) < 2 {
		return out, fmt.Errorf("missing actual cuts or copied directions")
	}
	for _, ex := range executions {
		if ex.Name == "A-cmd-vatc" {
			arms, err := readArms(ex.StdoutPath)
			if err != nil {
				return out, err
			}
			out.Arms = arms
		}
	}
	if len(out.Arms) != 25 {
		return out, fmt.Errorf("missing actual 23-arm plus two nonproven exchanges")
	}
	return out, nil
}
func validateCuts(rows []cutRow) error {
	cuts := []string{"before-receipt-event", "after-receipt-event", "after-receipt-control", "before-candidate-ledger", "after-candidate-ledger"}
	if len(rows) != 10 {
		return fmt.Errorf("incomplete five-cut pair matrix")
	}
	seen := map[string]bool{}
	for _, r := range rows {
		index := -1
		for i, c := range cuts {
			if c == r.Cut {
				index = i
			}
		}
		if index < 0 || (r.Pair != "baseline" && r.Pair != "candidate") || seen[r.Pair+":"+r.Cut] {
			return fmt.Errorf("invalid cut identity")
		}
		seen[r.Pair+":"+r.Cut] = true
		wantExit, wantClass, wantSealed := 2, "resume", 2
		if index >= 3 {
			wantSealed = 0
		}
		if index == 4 {
			wantExit, wantClass = 0, "replay"
		}
		if r.KillExit != -1 || r.Exit != wantExit || r.Classifier != wantClass || r.Launches.Sealed != wantSealed || len(r.CutRows) != 3 || len(r.Rows) != 3 {
			return fmt.Errorf("changed cut outcome %s/%s", r.Pair, r.Cut)
		}
		if r.Pair == "candidate" && (r.Launches.Decode != 0 || r.Launches.Encode != 0) {
			return fmt.Errorf("candidate translation launch")
		}
	}
	return nil
}
func outcome(path string) (rawOutcome, error) {
	var out rawOutcome
	b, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	err = artifact.DecodeStrictJSON(b, &out)
	return out, err
}
func readCopy(dir string) (copyOutcome, error) {
	out := copyOutcome{Pair: strings.TrimSuffix(filepath.Base(dir), "-controls")}
	base, err := outcome(filepath.Join(dir, "uninstrumented.json"))
	if err != nil {
		return out, err
	}
	replay, err := outcome(filepath.Join(dir, "opposite-shipped-replay.json"))
	if err != nil {
		return out, err
	}
	dirty, err := outcome(filepath.Join(dir, "opposite-shipped-dirty-git.json"))
	if err != nil {
		return out, err
	}
	observed, err := outcome(filepath.Join(dir, "nonblocking/outcome.json"))
	if err != nil {
		return out, err
	}
	out.ShippedExit = base.Exit
	out.ReplayExit = replay.Exit
	out.DirtyGitExit = dirty.Exit
	out.HandoffSHA = digest([]byte(base.Stdout))
	out.RecordsSHA = digest(base.Records)
	out.ReplayBytesEqual = base.Stdout == replay.Stdout && bytes.Equal(base.Records, replay.Records)
	out.DirtyGitRecordsEqual = bytes.Equal(base.Records, dirty.Records)
	for _, t := range observed.Trace {
		if t.Kind != "launch" {
			continue
		}
		out.TraceLaunches.Total++
		switch strings.Join(t.Args, " ") {
		case "context execution --request":
			out.TraceLaunches.Sealed++
		case "context owner decode":
			out.TraceLaunches.Decode++
		case "context owner encode":
			out.TraceLaunches.Encode++
		}
	}
	if base.Exit != 0 || replay.Exit != 0 || dirty.Exit != 2 || dirty.Stdout != "" || !strings.Contains(dirty.Stderr, "runway is no longer clean") || !out.ReplayBytesEqual || !out.DirtyGitRecordsEqual || observed.Exit != 0 || out.TraceLaunches.Sealed != 1 {
		return out, fmt.Errorf("copied-store control or refusal differs")
	}
	if out.Pair == "candidate" && (out.TraceLaunches.Decode != 0 || out.TraceLaunches.Encode != 0) {
		return out, fmt.Errorf("candidate control translated")
	}
	return out, nil
}
func readArms(path string) ([]armOutcome, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	rows := map[string]armOutcome{}
	hashes := regexp.MustCompile(`call_sha256=([0-9a-f]{64}) result_sha256=([0-9a-f]{64})(?: legacy_request_digest=(sha256:[0-9a-f]{64}))?`)
	for {
		var e testEvent
		if err = d.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if e.Action != "output" || !strings.HasPrefix(e.Test, "TestBoundaryPublicContractConformance/") {
			continue
		}
		m := hashes.FindStringSubmatch(e.Output)
		if len(m) == 0 {
			continue
		}
		if _, ok := rows[e.Test]; ok {
			return nil, fmt.Errorf("duplicate arm capture")
		}
		rows[e.Test] = armOutcome{e.Test, m[1], m[2], m[3]}
	}
	keys := make([]string, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]armOutcome, 0, len(keys))
	for _, key := range keys {
		out = append(out, rows[key])
	}
	return out, nil
}

type baselineRecord struct {
	Name       string   `json:"name"`
	Commit     string   `json:"commit"`
	Tree       string   `json:"tree"`
	ArchiveSHA string   `json:"archive_sha256"`
	BinarySHA  string   `json:"binary_sha256"`
	GoVersion  string   `json:"go_version"`
	GOOS       string   `json:"goos"`
	GOARCH     string   `json:"goarch"`
	BuildArgs  []string `json:"build_args"`
	BuildEnv   []string `json:"build_env"`
}

func baselineRecords(root string, files map[string]string, bound inputs) ([]baselineRecord, error) {
	var result []baselineRecord
	for _, name := range sortedKeys(files) {
		if filepath.Base(name) != "baseline-authentication.json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		var rows []baselineRecord
		if err = artifact.DecodeStrictJSON(b, &rows); err != nil {
			return nil, err
		}
		if len(rows) != 2 {
			return nil, fmt.Errorf("missing baseline pair source authentication")
		}
		seen := map[string]bool{}
		for _, r := range rows {
			wantCommit, wantTree, wantArchive, pkg := baselineVerdi, "2c1e7d6c9f64af7d45488d782046f9eb3025b171", "7598ce0e993518657cec201c1799fe5a45b68bc7b842cc5be4d06dbafd629862", "./cmd/verdi"
			if r.Name == "atc" {
				wantCommit, wantTree, wantArchive, pkg = baselineATC, "a11970fa32c297766099d0733779cfd85b92dbf8", "33861f01f423aa615372ac08e40c93edba2c0c23ccf9bd84a29f0e4a2079e609", "./cmd/vatc"
			} else if r.Name != "verdi" {
				return nil, fmt.Errorf("unknown authenticated baseline")
			}
			if seen[r.Name] || r.Commit != wantCommit || r.Tree != wantTree || r.ArchiveSHA != wantArchive || r.BinarySHA != bound.Binaries["baseline_"+r.Name] || r.GoVersion == "" || r.GOOS != runtime.GOOS || r.GOARCH != runtime.GOARCH || !reflect.DeepEqual(r.BuildArgs, []string{"go", "build", "-trimpath", "-o", "<binary>", pkg}) || !reflect.DeepEqual(r.BuildEnv, baselineEnvironment()) {
				return nil, fmt.Errorf("misbound baseline source build")
			}
			seen[r.Name] = true
		}
		result = append(result, rows...)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("baseline sources were not independently authenticated")
	}
	return result, nil
}
