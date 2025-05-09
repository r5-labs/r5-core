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

// checkpoint-admin is a utility that can be used to query checkpoint information
// and register stable checkpoints into an oracle contract.
package main

import (
	"fmt"
	"os"

	"github.com/r5-labs/r5-core/client/common/fdlimit"
	"github.com/r5-labs/r5-core/client/internal/flags"
	"github.com/r5-labs/r5-core/client/log"
	"github.com/urfave/cli/v2"
)

var app = flags.NewApp("ethereum checkpoint helper tool")

func init() {
	app.Commands = []*cli.Command{
		commandStatus,
		commandDeploy,
		commandSign,
		commandPublish,
	}
	app.Flags = []cli.Flag{
		oracleFlag,
		nodeURLFlag,
	}
}

// Commonly used command line flags.
var (
	indexFlag = &cli.Int64Flag{
		Name:  "index",
		Usage: "Checkpoint index (query latest from remote node if not specified)",
	}
	hashFlag = &cli.StringFlag{
		Name:  "hash",
		Usage: "Checkpoint hash (query latest from remote node if not specified)",
	}
	oracleFlag = &cli.StringFlag{
		Name:  "oracle",
		Usage: "Checkpoint oracle address (query from remote node if not specified)",
	}
	thresholdFlag = &cli.Int64Flag{
		Name:  "threshold",
		Usage: "Minimal number of signatures required to approve a checkpoint",
	}
	nodeURLFlag = &cli.StringFlag{
		Name:  "rpc",
		Value: "http://localhost:8545",
		Usage: "The rpc endpoint of a local or remote geth node",
	}
	clefURLFlag = &cli.StringFlag{
		Name:  "clef",
		Value: "http://localhost:8550",
		Usage: "The rpc endpoint of clef",
	}
	signerFlag = &cli.StringFlag{
		Name:  "signer",
		Usage: "Signer address for clef signing",
	}
	signersFlag = &cli.StringFlag{
		Name:  "signers",
		Usage: "Comma separated accounts of trusted checkpoint signers",
	}
	signaturesFlag = &cli.StringFlag{
		Name:  "signatures",
		Usage: "Comma separated checkpoint signatures to submit",
	}
)

func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
	fdlimit.Raise(2048)

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
