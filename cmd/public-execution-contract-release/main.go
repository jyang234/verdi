// Command public-execution-contract-release runs the auxiliary paired release
// checker. It does not extend the shipped Verdi command grammar.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/jyang234/verdi/internal/publicrelease"
)

func main() { os.Exit(run()) }
func run() int {
	var c publicrelease.Config
	var bootstrap string
	flag.StringVar(&c.VerdiDir, "verdi-source", "", "absolute clean Verdi source")
	flag.StringVar(&c.ATCDir, "atc-source", "", "absolute clean ATC source")
	flag.StringVar(&c.VerdiCommit, "verdi-commit", "", "approved exact Verdi commit")
	flag.StringVar(&c.ATCCommit, "atc-commit", "", "approved exact ATC commit")
	flag.StringVar(&c.VerdiBinary, "verdi-binary", "", "actual Verdi executable")
	flag.StringVar(&c.ATCBinary, "atc-binary", "", "actual ATC executable")
	flag.StringVar(&c.BaselineSource, "baseline-atc-source", "", "fixed ATC archive source")
	flag.StringVar(&c.BaselineVerdi, "baseline-verdi-binary", "", "fixed source-built baseline Verdi")
	flag.StringVar(&c.BaselineATC, "baseline-atc-binary", "", "fixed source-built baseline ATC")
	flag.StringVar(&c.OutputDir, "output", "", "fresh absolute output directory")
	flag.BoolVar(&c.WorkflowRun, "workflow-run", false, "explicit workflow context; authenticity requires genuine job artifact")
	flag.StringVar(&bootstrap, "bootstrap", "", "fresh workflow bootstrap directory (source inputs from protected environment)")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	var err error
	if bootstrap != "" {
		c, err = publicrelease.Bootstrap(ctx, c.VerdiDir, bootstrap, os.Getenv)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	if err = publicrelease.Run(ctx, c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if errors.Is(err, publicrelease.ErrVerdict) {
			return 1
		}
		return 2
	}
	fmt.Fprintln(os.Stdout, "Ten producer results and release report:", c.OutputDir+"/publish")
	return 0
}
