package publicrelease

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseWholeDocumentReaders(t *testing.T) {
	for _, reader := range []string{"declarations", "outcome", "copy", "cuts", "baselines"} {
		t.Run(reader, func(t *testing.T) {
			for _, mutation := range []string{"valid", "closing-object", "closing-array", "second-value", "garbage", "unknown-field"} {
				t.Run(mutation, func(t *testing.T) {
					path, read := wholeDocumentReader(t, reader)
					b, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					switch mutation {
					case "closing-object":
						b = append(b, '}')
					case "closing-array":
						b = append(b, ']')
					case "second-value":
						b = append(b, []byte("\n{}")...)
					case "garbage":
						b = append(b, []byte("garbage")...)
					case "unknown-field":
						b = bytes.Replace(b, []byte("{"), []byte(`{"unknown_release_field":true,`), 1)
					}
					if err = os.WriteFile(path, b, 0600); err != nil {
						t.Fatal(err)
					}
					err = read()
					if (err != nil) != (mutation != "valid") {
						t.Fatalf("%s %s: %v", reader, mutation, err)
					}
				})
			}
		})
	}
}

func wholeDocumentReader(t *testing.T, reader string) (string, func() error) {
	t.Helper()
	root := t.TempDir()
	write := func(path string, b []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	copyFixture := func(dir string) {
		t.Helper()
		if err := os.CopyFS(dir, os.DirFS("testdata/copy-control/candidate-controls")); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "input.json")
	switch reader {
	case "declarations":
		write(path, checkBytes)
		return path, func() error {
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			_, e = readDeclarations(b)
			return e
		}
	case "outcome", "copy":
		dir := filepath.Join(root, "candidate-controls")
		copyFixture(dir)
		path = filepath.Join(dir, "uninstrumented.json")
		return path, func() error {
			if reader == "copy" {
				_, e := readCopy(dir)
				return e
			}
			_, e := outcome(path)
			return e
		}
	case "cuts":
		name := "TestBoundaryRecoveryFiveProcessCuts-1/matrix.json"
		path = filepath.Join(root, name)
		b, e := os.ReadFile("testdata/cuts.json")
		if e != nil {
			t.Fatal(e)
		}
		write(path, b)
		files := map[string]string{name: "hash"}
		for _, pair := range []string{"baseline", "candidate"} {
			dir := "TestBoundaryRecoveryControlsAndCopiedCompletedStores-1/" + pair + "-controls"
			copyFixture(filepath.Join(root, dir))
			files[dir+"/uninstrumented.json"] = "hash"
		}
		return path, func() error {
			_, e := readRuntime(root, files, []execution{{Name: "A-cmd-vatc", StdoutPath: "testdata/arm-events.jsonl"}})
			return e
		}
	case "baselines":
		rows := []baselineRecord{
			{Name: "verdi", Commit: baselineVerdi, Tree: "2c1e7d6c9f64af7d45488d782046f9eb3025b171", ArchiveSHA: "7598ce0e993518657cec201c1799fe5a45b68bc7b842cc5be4d06dbafd629862"},
			{Name: "atc", Commit: baselineATC, Tree: "a11970fa32c297766099d0733779cfd85b92dbf8", ArchiveSHA: "33861f01f423aa615372ac08e40c93edba2c0c23ccf9bd84a29f0e4a2079e609"},
		}
		bound := inputs{Binaries: map[string]string{}}
		for i := range rows {
			r := &rows[i]
			r.BinarySHA = strings.Repeat("a", 64)
			bound.Binaries["baseline_"+r.Name] = r.BinarySHA
			r.GoVersion = runtime.Version()
			r.GOOS = runtime.GOOS
			r.GOARCH = runtime.GOARCH
			pkg := "./cmd/verdi"
			if r.Name == "atc" {
				pkg = "./cmd/vatc"
			}
			r.BuildArgs = []string{"go", "build", "-trimpath", "-o", "<binary>", pkg}
			r.BuildEnv = baselineEnvironment()
		}
		b, e := json.Marshal(rows)
		if e != nil {
			t.Fatal(e)
		}
		path = filepath.Join(root, "baseline-authentication.json")
		write(path, b)
		return path, func() error {
			_, e := baselineRecords(root, map[string]string{"baseline-authentication.json": "hash"}, bound)
			return e
		}
	default:
		t.Fatalf("unknown reader %s", reader)
		return "", nil
	}
}
