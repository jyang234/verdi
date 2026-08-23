package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

type request struct {
	Candidate string `json:"candidate"`
}

func main() {
	capabilitiesPath := flag.String("capabilities", "", "canonical capabilities response")
	mode := flag.String("mode", "completed", "fixture response mode")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "expected one operation")
		os.Exit(2)
	}

	switch flag.Arg(0) {
	case "describe":
		describe(*capabilitiesPath)
	case "run":
		run(*mode)
	default:
		fmt.Fprintln(os.Stderr, "unknown operation")
		os.Exit(2)
	}
}

func describe(path string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "capabilities path is empty")
		os.Exit(3)
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil || len(stdin) != 0 {
		fmt.Fprintln(os.Stderr, "describe stdin must be empty")
		os.Exit(3)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}

func run(mode string) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	var input request
	if err := json.Unmarshal(data, &input); err != nil || input.Candidate == "" {
		fmt.Fprintln(os.Stderr, "invalid request")
		os.Exit(4)
	}

	switch mode {
	case "completed":
		value := 100
		if input.Candidate == "beta" {
			value = 70
		}
		fmt.Printf("{\"disclosures\":[],\"guards\":[{\"id\":\"correctness\",\"verdict\":\"pass\",\"witness\":null}],\"measurements\":[{\"id\":\"latency\",\"source\":\"evaluator-measured\",\"unit\":\"ms\",\"value\":%d}],\"outcome\":{\"kind\":\"completed\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n", value)
	case "candidate-crash", "candidate-timeout":
		fmt.Printf("{\"disclosures\":[],\"guards\":[],\"measurements\":[],\"outcome\":{\"kind\":%q,\"witness\":%q},\"schema\":\"verdi.experiment-evaluator/v1\"}\n", mode, "fixture "+mode)
	case "malformed":
		fmt.Print("{\"schema\":\n")
	case "evaluator-crash":
		fmt.Fprintln(os.Stderr, "fixture evaluator crash")
		os.Exit(17)
	case "evaluator-timeout":
		time.Sleep(5 * time.Second)
	default:
		fmt.Fprintln(os.Stderr, "unknown mode")
		os.Exit(5)
	}
}
