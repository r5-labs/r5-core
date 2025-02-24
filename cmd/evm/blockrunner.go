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
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/r5-codebase/r5-core/log"
	"github.com/r5-codebase/r5-core/tests"
	"github.com/urfave/cli/v2"
)

var blockTestCommand = &cli.Command{
	Action:    blockTestCmd,
	Name:      "blocktest",
	Usage:     "executes the given blockchain tests",
	ArgsUsage: "<file>",
}

func blockTestCmd(ctx *cli.Context) error {
	if len(ctx.Args().First()) == 0 {
		return errors.New("path-to-test argument required")
	}
	// Configure the go-ethereum logger
	glogger := log.NewGlogHandler(log.StreamHandler(os.Stderr, log.TerminalFormat(false)))
	glogger.Verbosity(log.Lvl(ctx.Int(VerbosityFlag.Name)))
	log.Root().SetHandler(glogger)

	// Load the test content from the input file
	src, err := os.ReadFile(ctx.Args().First())
	if err != nil {
		return err
	}
	var tests map[string]tests.BlockTest
	if err = json.Unmarshal(src, &tests); err != nil {
		return err
	}
	for i, test := range tests {
		if err := test.Run(false); err != nil {
			return fmt.Errorf("test %v: %w", i, err)
		}
	}
	return nil
}
