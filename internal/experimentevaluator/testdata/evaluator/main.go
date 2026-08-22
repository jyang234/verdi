package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

const capabilities = "{\"evaluator_version\":\"fixture-evaluator/1\",\"metrics\":[],\"protocol_versions\":[\"verdi.experiment-evaluator/v1\",\"verdi.experiment-observation/v2\"],\"requires_elevated\":false,\"requires_network\":false,\"schema\":\"verdi.experiment-evaluator-capabilities/v2\"}\n"

const request = "{\"candidate\":\"candidate-a\",\"contract\":{\"digest\":\"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc\",\"id\":\"contract-a\",\"path\":\"contracts/a.json\"},\"cycle\":{\"kind\":\"measured\",\"number\":1},\"experiment_digest\":\"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\",\"fixtures\":[{\"digest\":\"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd\",\"id\":\"fixture-a\",\"path\":\"fixtures/a.json\"}],\"run\":\"run-1\",\"schema\":\"verdi.experiment-evaluator/v1\",\"workload\":{\"digest\":\"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\",\"id\":\"workload-a\",\"path\":\"workloads/a.json\"}}\n"

const response = "{\"disclosures\":[],\"guards\":[],\"measurements\":[],\"outcome\":{\"kind\":\"completed\"},\"schema\":\"verdi.experiment-evaluator/v1\"}\n"

func main() {
	if len(os.Args) == 3 && os.Args[1] == "--mode=retain-output-child" {
		waitForRelease(os.Args[2])
		return
	}
	if len(os.Args) == 4 && os.Args[1] == "--mode=retain-output" && os.Args[3] == "run" {
		retainOutput(os.Args[2])
		return
	}
	if len(os.Args) != 3 || os.Args[1] != "--mode=ok" {
		fmt.Fprintln(os.Stderr, "unexpected argv")
		os.Exit(9)
	}
	stdin, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(8)
	}
	switch os.Args[2] {
	case "describe":
		if len(stdin) != 0 {
			fmt.Fprintln(os.Stderr, "describe stdin was not empty")
			os.Exit(7)
		}
		fmt.Print(capabilities)
	case "run":
		if string(stdin) != request {
			fmt.Fprintln(os.Stderr, "run stdin was not the exact canonical request")
			os.Exit(6)
		}
		fmt.Print(response)
	default:
		fmt.Fprintln(os.Stderr, "unknown operation")
		os.Exit(5)
	}
}

func retainOutput(release string) {
	child := exec.Command(os.Args[0], "--mode=retain-output-child", release)
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	if err := os.WriteFile(release+".ready", nil, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	time.Sleep(time.Hour)
}

func waitForRelease(release string) {
	for {
		_, err := os.Stat(release)
		if err == nil {
			if err := os.WriteFile(release+".done", nil, 0o600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(2)
			}
			return
		}
		if !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
