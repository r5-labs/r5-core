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

package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/r5-labs/r5-core/client/cmd/utils"
	"github.com/r5-labs/r5-core/client/consensus/ethash"
	"github.com/r5-labs/r5-core/client/internal/version"
	"github.com/r5-labs/r5-core/client/params"
	"github.com/urfave/cli/v2"
)

var (
	VersionCheckUrlFlag = &cli.StringFlag{
		Name:  "check.url",
		Usage: "URL to use when checking vulnerabilities",
		Value: "https://geth.ethereum.org/docs/vulnerabilities/vulnerabilities.json",
	}
	VersionCheckVersionFlag = &cli.StringFlag{
		Name:  "check.version",
		Usage: "Version to check",
		Value: version.ClientName(clientIdentifier),
	}
	makecacheCommand = &cli.Command{
		Action:    makecache,
		Name:      "makecache",
		Usage:     "Generate ethash verification cache (for testing)",
		ArgsUsage: "<blockNum> <outputDir>",
		Description: `
The makecache command generates an ethash cache in <outputDir>.

This command exists to support the system testing project.
Regular users do not need to execute it.
`,
	}
	makedagCommand = &cli.Command{
		Action:    makedag,
		Name:      "makedag",
		Usage:     "Generate ethash mining DAG (for testing)",
		ArgsUsage: "<blockNum> <outputDir>",
		Description: `
The makedag command generates an ethash DAG in <outputDir>.

This command exists to support the system testing project.
Regular users do not need to execute it.
`,
	}
	versionCommand = &cli.Command{
		Action:    printVersion,
		Name:      "version",
		Usage:     "Print version numbers",
		ArgsUsage: " ",
		Description: `
The output of this command is supposed to be machine-readable.
`,
	}
	versionCheckCommand = &cli.Command{
		Action: versionCheck,
		Flags: []cli.Flag{
			VersionCheckUrlFlag,
			VersionCheckVersionFlag,
		},
		Name:      "version-check",
		Usage:     "Checks (online) for known Geth security vulnerabilities",
		ArgsUsage: "<versionstring (optional)>",
		Description: `
The version-check command fetches vulnerability-information from https://geth.ethereum.org/docs/vulnerabilities/vulnerabilities.json, 
and displays information about any security vulnerabilities that affect the currently executing version.
`,
	}
	licenseCommand = &cli.Command{
		Action:    license,
		Name:      "license",
		Usage:     "Display license information",
		ArgsUsage: " ",
	}
)

// makecache generates an ethash verification cache into the provided folder.
func makecache(ctx *cli.Context) error {
	args := ctx.Args().Slice()
	if len(args) != 2 {
		utils.Fatalf(`Usage: geth makecache <block number> <outputdir>`)
	}
	block, err := strconv.ParseUint(args[0], 0, 64)
	if err != nil {
		utils.Fatalf("Invalid block number: %v", err)
	}
	ethash.MakeCache(block, args[1])

	return nil
}

// makedag generates an ethash mining DAG into the provided folder.
func makedag(ctx *cli.Context) error {
	args := ctx.Args().Slice()
	if len(args) != 2 {
		utils.Fatalf(`Usage: geth makedag <block number> <outputdir>`)
	}
	block, err := strconv.ParseUint(args[0], 0, 64)
	if err != nil {
		utils.Fatalf("Invalid block number: %v", err)
	}
	ethash.MakeDataset(block, args[1])

	return nil
}

func printVersion(ctx *cli.Context) error {
	git, _ := version.VCS()

	fmt.Println(strings.Title(clientIdentifier))
	fmt.Println("Version:", params.VersionWithMeta)
	if git.Commit != "" {
		fmt.Println("Git Commit:", git.Commit)
	}
	if git.Date != "" {
		fmt.Println("Git Commit Date:", git.Date)
	}
	fmt.Println("Architecture:", runtime.GOARCH)
	fmt.Println("Go Version:", runtime.Version())
	fmt.Println("Operating System:", runtime.GOOS)
	fmt.Printf("GOPATH=%s\n", os.Getenv("GOPATH"))
	fmt.Printf("GOROOT=%s\n", runtime.GOROOT())
	return nil
}

func license(_ *cli.Context) error {
	fmt.Println(`Geth is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Geth is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with geth. If not, see <http://www.gnu.org/licenses/>.`)
	return nil
}
