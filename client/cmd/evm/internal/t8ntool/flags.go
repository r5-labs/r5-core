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

package t8ntool

import (
	"fmt"
	"strings"

	"github.com/r5-labs/r5-core/client/core/vm"
	"github.com/r5-labs/r5-core/client/tests"
	"github.com/urfave/cli/v2"
)

var (
	TraceFlag = &cli.BoolFlag{
		Name:  "trace",
		Usage: "Output full trace logs to files <txhash>.jsonl",
	}
	TraceDisableMemoryFlag = &cli.BoolFlag{
		Name:  "trace.nomemory",
		Value: true,
		Usage: "Disable full memory dump in traces (deprecated)",
	}
	TraceEnableMemoryFlag = &cli.BoolFlag{
		Name:  "trace.memory",
		Usage: "Enable full memory dump in traces",
	}
	TraceDisableStackFlag = &cli.BoolFlag{
		Name:  "trace.nostack",
		Usage: "Disable stack output in traces",
	}
	TraceDisableReturnDataFlag = &cli.BoolFlag{
		Name:  "trace.noreturndata",
		Value: true,
		Usage: "Disable return data output in traces (deprecated)",
	}
	TraceEnableReturnDataFlag = &cli.BoolFlag{
		Name:  "trace.returndata",
		Usage: "Enable return data output in traces",
	}
	OutputBasedir = &cli.StringFlag{
		Name:  "output.basedir",
		Usage: "Specifies where output files are placed. Will be created if it does not exist.",
		Value: "",
	}
	OutputBodyFlag = &cli.StringFlag{
		Name:  "output.body",
		Usage: "If set, the RLP of the transactions (block body) will be written to this file.",
		Value: "",
	}
	OutputAllocFlag = &cli.StringFlag{
		Name: "output.alloc",
		Usage: "Determines where to put the `alloc` of the post-state.\n" +
			"\t`stdout` - into the stdout output\n" +
			"\t`stderr` - into the stderr output\n" +
			"\t<file> - into the file <file> ",
		Value: "alloc.json",
	}
	OutputResultFlag = &cli.StringFlag{
		Name: "output.result",
		Usage: "Determines where to put the `result` (stateroot, txroot etc) of the post-state.\n" +
			"\t`stdout` - into the stdout output\n" +
			"\t`stderr` - into the stderr output\n" +
			"\t<file> - into the file <file> ",
		Value: "result.json",
	}
	OutputBlockFlag = &cli.StringFlag{
		Name: "output.block",
		Usage: "Determines where to put the `block` after building.\n" +
			"\t`stdout` - into the stdout output\n" +
			"\t`stderr` - into the stderr output\n" +
			"\t<file> - into the file <file> ",
		Value: "block.json",
	}
	InputAllocFlag = &cli.StringFlag{
		Name:  "input.alloc",
		Usage: "`stdin` or file name of where to find the prestate alloc to use.",
		Value: "alloc.json",
	}
	InputEnvFlag = &cli.StringFlag{
		Name:  "input.env",
		Usage: "`stdin` or file name of where to find the prestate env to use.",
		Value: "env.json",
	}
	InputTxsFlag = &cli.StringFlag{
		Name: "input.txs",
		Usage: "`stdin` or file name of where to find the transactions to apply. " +
			"If the file extension is '.rlp', then the data is interpreted as an RLP list of signed transactions." +
			"The '.rlp' format is identical to the output.body format.",
		Value: "txs.json",
	}
	InputHeaderFlag = &cli.StringFlag{
		Name:  "input.header",
		Usage: "`stdin` or file name of where to find the block header to use.",
		Value: "header.json",
	}
	InputOmmersFlag = &cli.StringFlag{
		Name:  "input.ommers",
		Usage: "`stdin` or file name of where to find the list of ommer header RLPs to use.",
	}
	InputWithdrawalsFlag = &cli.StringFlag{
		Name:  "input.withdrawals",
		Usage: "`stdin` or file name of where to find the list of withdrawals to use.",
	}
	InputTxsRlpFlag = &cli.StringFlag{
		Name:  "input.txs",
		Usage: "`stdin` or file name of where to find the transactions list in RLP form.",
		Value: "txs.rlp",
	}
	SealCliqueFlag = &cli.StringFlag{
		Name:  "seal.clique",
		Usage: "Seal block with Clique. `stdin` or file name of where to find the Clique sealing data.",
	}
	SealEthashFlag = &cli.BoolFlag{
		Name:  "seal.ethash",
		Usage: "Seal block with ethash.",
	}
	SealEthashDirFlag = &cli.StringFlag{
		Name:  "seal.ethash.dir",
		Usage: "Path to ethash DAG. If none exists, a new DAG will be generated.",
	}
	SealEthashModeFlag = &cli.StringFlag{
		Name:  "seal.ethash.mode",
		Usage: "Defines the type and amount of PoW verification an ethash engine makes.",
		Value: "normal",
	}
	RewardFlag = &cli.Int64Flag{
		Name:  "state.reward",
		Usage: "Mining reward. Set to -1 to disable",
		Value: 0,
	}
	ChainIDFlag = &cli.Int64Flag{
		Name:  "state.chainid",
		Usage: "ChainID to use",
		Value: 1,
	}
	ForknameFlag = &cli.StringFlag{
		Name: "state.fork",
		Usage: fmt.Sprintf("Name of ruleset to use."+
			"\n\tAvailable forknames:"+
			"\n\t    %v"+
			"\n\tAvailable extra eips:"+
			"\n\t    %v"+
			"\n\tSyntax <forkname>(+ExtraEip)",
			strings.Join(tests.AvailableForks(), "\n\t    "),
			strings.Join(vm.ActivateableEips(), ", ")),
		Value: "GrayGlacier",
	}
	VerbosityFlag = &cli.IntFlag{
		Name:  "verbosity",
		Usage: "sets the verbosity level",
		Value: 3,
	}
)
