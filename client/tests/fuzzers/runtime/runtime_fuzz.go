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

package runtime

import (
	"github.com/r5-labs/r5-core/core/vm/runtime"
)

// Fuzz is the basic entry point for the go-fuzz tool
//
// This returns 1 for valid parse:able/runnable code, 0
// for invalid opcode.
func Fuzz(input []byte) int {
	_, _, err := runtime.Execute(input, input, &runtime.Config{
		GasLimit: 12000000,
	})
	// invalid opcode
	if err != nil && len(err.Error()) > 6 && err.Error()[:7] == "invalid" {
		return 0
	}
	return 1
}
