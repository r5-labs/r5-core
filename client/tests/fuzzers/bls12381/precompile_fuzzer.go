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

package bls

import (
	"bytes"
	"fmt"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/vm"
)

const (
	blsG1Add      = byte(10)
	blsG1Mul      = byte(11)
	blsG1MultiExp = byte(12)
	blsG2Add      = byte(13)
	blsG2Mul      = byte(14)
	blsG2MultiExp = byte(15)
	blsPairing    = byte(16)
	blsMapG1      = byte(17)
	blsMapG2      = byte(18)
)

func FuzzG1Add(data []byte) int      { return fuzz(blsG1Add, data) }
func FuzzG1Mul(data []byte) int      { return fuzz(blsG1Mul, data) }
func FuzzG1MultiExp(data []byte) int { return fuzz(blsG1MultiExp, data) }
func FuzzG2Add(data []byte) int      { return fuzz(blsG2Add, data) }
func FuzzG2Mul(data []byte) int      { return fuzz(blsG2Mul, data) }
func FuzzG2MultiExp(data []byte) int { return fuzz(blsG2MultiExp, data) }
func FuzzPairing(data []byte) int    { return fuzz(blsPairing, data) }
func FuzzMapG1(data []byte) int      { return fuzz(blsMapG1, data) }
func FuzzMapG2(data []byte) int      { return fuzz(blsMapG2, data) }

func checkInput(id byte, inputLen int) bool {
	switch id {
	case blsG1Add:
		return inputLen == 256
	case blsG1Mul:
		return inputLen == 160
	case blsG1MultiExp:
		return inputLen%160 == 0
	case blsG2Add:
		return inputLen == 512
	case blsG2Mul:
		return inputLen == 288
	case blsG2MultiExp:
		return inputLen%288 == 0
	case blsPairing:
		return inputLen%384 == 0
	case blsMapG1:
		return inputLen == 64
	case blsMapG2:
		return inputLen == 128
	}
	panic("programmer error")
}

// The function must return
//
//   - 1 if the fuzzer should increase priority of the
//     given input during subsequent fuzzing (for example, the input is lexically
//     correct and was parsed successfully);
//   - -1 if the input must not be added to corpus even if gives new coverage; and
//   - 0 otherwise
//
// other values are reserved for future use.
func fuzz(id byte, data []byte) int {
	// Even on bad input, it should not crash, so we still test the gas calc
	precompile := vm.PrecompiledContractsBLS[common.BytesToAddress([]byte{id})]
	gas := precompile.RequiredGas(data)
	if !checkInput(id, len(data)) {
		return 0
	}
	// If the gas cost is too large (25M), bail out
	if gas > 25*1000*1000 {
		return 0
	}
	cpy := make([]byte, len(data))
	copy(cpy, data)
	_, err := precompile.Run(cpy)
	if !bytes.Equal(cpy, data) {
		panic(fmt.Sprintf("input data modified, precompile %d: %x %x", id, data, cpy))
	}
	if err != nil {
		return 0
	}
	return 1
}
