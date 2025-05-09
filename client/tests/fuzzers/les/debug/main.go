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

	"github.com/r5-labs/r5-core/client/tests/fuzzers/les"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: debug <file>\n")
		fmt.Fprintf(os.Stderr, "Example\n")
		fmt.Fprintf(os.Stderr, "	$ debug ../crashers/4bbef6857c733a87ecf6fd8b9e7238f65eb9862a\n")
		os.Exit(1)
	}
	crasher := os.Args[1]
	data, err := os.ReadFile(crasher)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading crasher %v: %v", crasher, err)
		os.Exit(1)
	}
	les.Fuzz(data)
}
