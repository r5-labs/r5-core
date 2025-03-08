// Copyright 2025 R5
// This file is part of the R5 Core library.
//
// This software is provided "as is", without warranty of any kind,
// express or implied, including but not limited to the warranties
// of merchantability, fitness for a particular purpose and
// noninfringement. In no event shall the authors or copyright
// holders be liable for any claim, damages, or other liability,
// whether in an action of contract, tort or otherwise, arising
// from, out of or in connection with the software or the use or
// other dealings in the software.

package main

import (
	"os"

	"github.com/r5-codebase/r5-core/cmd/devp2p/internal/v4test"
	"github.com/r5-codebase/r5-core/internal/utesting"
	"github.com/r5-codebase/r5-core/log"
	"github.com/urfave/cli/v2"
)

var (
	testPatternFlag = &cli.StringFlag{
		Name:  "run",
		Usage: "Pattern of test suite(s) to run",
	}
	testTAPFlag = &cli.BoolFlag{
		Name:  "tap",
		Usage: "Output TAP",
	}
	// These two are specific to the discovery tests.
	testListen1Flag = &cli.StringFlag{
		Name:  "listen1",
		Usage: "IP address of the first tester",
		Value: v4test.Listen1,
	}
	testListen2Flag = &cli.StringFlag{
		Name:  "listen2",
		Usage: "IP address of the second tester",
		Value: v4test.Listen2,
	}
)

func runTests(ctx *cli.Context, tests []utesting.Test) error {
	// Filter test cases.
	if ctx.IsSet(testPatternFlag.Name) {
		tests = utesting.MatchTests(tests, ctx.String(testPatternFlag.Name))
	}
	// Disable logging unless explicitly enabled.
	if !ctx.IsSet("verbosity") && !ctx.IsSet("vmodule") {
		log.Root().SetHandler(log.DiscardHandler())
	}
	// Run the tests.
	var run = utesting.RunTests
	if ctx.Bool(testTAPFlag.Name) {
		run = utesting.RunTAP
	}
	results := run(tests, os.Stdout)
	if utesting.CountFailures(results) > 0 {
		os.Exit(1)
	}
	return nil
}
