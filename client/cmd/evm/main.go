// Copyright 2025 R5 Labs
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

// evm executes EVM code snippets.
package main

import (
	"fmt"
	"math/big"
	"os"

	"github.com/r5-labs/r5-core/client/cmd/evm/internal/t8ntool"
	"github.com/r5-labs/r5-core/client/internal/flags"
	"github.com/urfave/cli/v2"
)

var (
	DebugFlag = &cli.BoolFlag{
		Name:  "debug",
		Usage: "output full trace logs",
	}
	MemProfileFlag = &cli.StringFlag{
		Name:  "memprofile",
		Usage: "creates a memory profile at the given path",
	}
	CPUProfileFlag = &cli.StringFlag{
		Name:  "cpuprofile",
		Usage: "creates a CPU profile at the given path",
	}
	StatDumpFlag = &cli.BoolFlag{
		Name:  "statdump",
		Usage: "displays stack and heap memory information",
	}
	CodeFlag = &cli.StringFlag{
		Name:  "code",
		Usage: "EVM code",
	}
	CodeFileFlag = &cli.StringFlag{
		Name:  "codefile",
		Usage: "File containing EVM code. If '-' is specified, code is read from stdin ",
	}
	GasFlag = &cli.Uint64Flag{
		Name:  "gas",
		Usage: "gas limit for the evm",
		Value: 10000000000,
	}
	PriceFlag = &flags.BigFlag{
		Name:  "price",
		Usage: "price set for the evm",
		Value: new(big.Int),
	}
	ValueFlag = &flags.BigFlag{
		Name:  "value",
		Usage: "value set for the evm",
		Value: new(big.Int),
	}
	DumpFlag = &cli.BoolFlag{
		Name:  "dump",
		Usage: "dumps the state after the run",
	}
	InputFlag = &cli.StringFlag{
		Name:  "input",
		Usage: "input for the EVM",
	}
	InputFileFlag = &cli.StringFlag{
		Name:  "inputfile",
		Usage: "file containing input for the EVM",
	}
	VerbosityFlag = &cli.IntFlag{
		Name:  "verbosity",
		Usage: "sets the verbosity level",
	}
	BenchFlag = &cli.BoolFlag{
		Name:  "bench",
		Usage: "benchmark the execution",
	}
	CreateFlag = &cli.BoolFlag{
		Name:  "create",
		Usage: "indicates the action should be create rather than call",
	}
	GenesisFlag = &cli.StringFlag{
		Name:  "prestate",
		Usage: "JSON file with prestate (genesis) config",
	}
	MachineFlag = &cli.BoolFlag{
		Name:  "json",
		Usage: "output trace logs in machine readable format (json)",
	}
	SenderFlag = &cli.StringFlag{
		Name:  "sender",
		Usage: "The transaction origin",
	}
	ReceiverFlag = &cli.StringFlag{
		Name:  "receiver",
		Usage: "The transaction receiver (execution context)",
	}
	DisableMemoryFlag = &cli.BoolFlag{
		Name:  "nomemory",
		Value: true,
		Usage: "disable memory output",
	}
	DisableStackFlag = &cli.BoolFlag{
		Name:  "nostack",
		Usage: "disable stack output",
	}
	DisableStorageFlag = &cli.BoolFlag{
		Name:  "nostorage",
		Usage: "disable storage output",
	}
	DisableReturnDataFlag = &cli.BoolFlag{
		Name:  "noreturndata",
		Value: true,
		Usage: "enable return data output",
	}
)

var stateTransitionCommand = &cli.Command{
	Name:    "transition",
	Aliases: []string{"t8n"},
	Usage:   "executes a full state transition",
	Action:  t8ntool.Transition,
	Flags: []cli.Flag{
		t8ntool.TraceFlag,
		t8ntool.TraceDisableMemoryFlag,
		t8ntool.TraceEnableMemoryFlag,
		t8ntool.TraceDisableStackFlag,
		t8ntool.TraceDisableReturnDataFlag,
		t8ntool.TraceEnableReturnDataFlag,
		t8ntool.OutputBasedir,
		t8ntool.OutputAllocFlag,
		t8ntool.OutputResultFlag,
		t8ntool.OutputBodyFlag,
		t8ntool.InputAllocFlag,
		t8ntool.InputEnvFlag,
		t8ntool.InputTxsFlag,
		t8ntool.ForknameFlag,
		t8ntool.ChainIDFlag,
		t8ntool.RewardFlag,
		t8ntool.VerbosityFlag,
	},
}

var transactionCommand = &cli.Command{
	Name:    "transaction",
	Aliases: []string{"t9n"},
	Usage:   "performs transaction validation",
	Action:  t8ntool.Transaction,
	Flags: []cli.Flag{
		t8ntool.InputTxsFlag,
		t8ntool.ChainIDFlag,
		t8ntool.ForknameFlag,
		t8ntool.VerbosityFlag,
	},
}

var blockBuilderCommand = &cli.Command{
	Name:    "block-builder",
	Aliases: []string{"b11r"},
	Usage:   "builds a block",
	Action:  t8ntool.BuildBlock,
	Flags: []cli.Flag{
		t8ntool.OutputBasedir,
		t8ntool.OutputBlockFlag,
		t8ntool.InputHeaderFlag,
		t8ntool.InputOmmersFlag,
		t8ntool.InputWithdrawalsFlag,
		t8ntool.InputTxsRlpFlag,
		t8ntool.SealCliqueFlag,
		t8ntool.SealEthashFlag,
		t8ntool.SealEthashDirFlag,
		t8ntool.SealEthashModeFlag,
		t8ntool.VerbosityFlag,
	},
}

var app = flags.NewApp("the evm command line interface")

func init() {
	app.Flags = []cli.Flag{
		BenchFlag,
		CreateFlag,
		DebugFlag,
		VerbosityFlag,
		CodeFlag,
		CodeFileFlag,
		GasFlag,
		PriceFlag,
		ValueFlag,
		DumpFlag,
		InputFlag,
		InputFileFlag,
		MemProfileFlag,
		CPUProfileFlag,
		StatDumpFlag,
		GenesisFlag,
		MachineFlag,
		SenderFlag,
		ReceiverFlag,
		DisableMemoryFlag,
		DisableStackFlag,
		DisableStorageFlag,
		DisableReturnDataFlag,
	}
	app.Commands = []*cli.Command{
		compileCommand,
		disasmCommand,
		runCommand,
		blockTestCommand,
		stateTestCommand,
		stateTransitionCommand,
		transactionCommand,
		blockBuilderCommand,
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		code := 1
		if ec, ok := err.(*t8ntool.NumberedError); ok {
			code = ec.ExitCode()
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(code)
	}
}
