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

package difficulty

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"

	"github.com/r5-labs/r5-core/client/consensus/ethash"
	"github.com/r5-labs/r5-core/client/core/types"
)

type fuzzer struct {
	input     io.Reader
	exhausted bool
}

func (f *fuzzer) read(size int) []byte {
	out := make([]byte, size)
	if _, err := f.input.Read(out); err != nil {
		f.exhausted = true
	}
	return out
}

func (f *fuzzer) readSlice(min, max int) []byte {
	var a uint16
	binary.Read(f.input, binary.LittleEndian, &a)
	size := min + int(a)%(max-min)
	out := make([]byte, size)
	if _, err := f.input.Read(out); err != nil {
		f.exhausted = true
	}
	return out
}

func (f *fuzzer) readUint64(min, max uint64) uint64 {
	if min == max {
		return min
	}
	var a uint64
	if err := binary.Read(f.input, binary.LittleEndian, &a); err != nil {
		f.exhausted = true
	}
	a = min + a%(max-min)
	return a
}
func (f *fuzzer) readBool() bool {
	return f.read(1)[0]&0x1 == 0
}

// Fuzz function must return
//
//   - 1 if the fuzzer should increase priority of the
//     given input during subsequent fuzzing (for example, the input is lexically
//     correct and was parsed successfully);
//   - -1 if the input must not be added to corpus even if gives new coverage; and
//   - 0 otherwise
//
// other values are reserved for future use.
func Fuzz(data []byte) int {
	f := fuzzer{
		input:     bytes.NewReader(data),
		exhausted: false,
	}
	return f.fuzz()
}

var minDifficulty = big.NewInt(0x2000)

type calculator func(time uint64, parent *types.Header) *big.Int

func (f *fuzzer) fuzz() int {
	// A parent header
	header := &types.Header{}
	if f.readBool() {
		header.UncleHash = types.EmptyUncleHash
	}
	// Difficulty can range between 0x2000 (2 bytes) and up to 32 bytes
	{
		diff := new(big.Int).SetBytes(f.readSlice(2, 32))
		if diff.Cmp(minDifficulty) < 0 {
			diff.Set(minDifficulty)
		}
		header.Difficulty = diff
	}
	// Number can range between 0 and up to 32 bytes (but not so that the child exceeds it)
	{
		// However, if we use astronomic numbers, then the bomb exp karatsuba calculation
		// in the legacy methods)
		// times out, so we limit it to fit within reasonable bounds
		number := new(big.Int).SetBytes(f.readSlice(0, 4)) // 4 bytes: 32 bits: block num max 4 billion
		header.Number = number
	}
	// Both parent and child time must fit within uint64
	var time uint64
	{
		childTime := f.readUint64(1, 0xFFFFFFFFFFFFFFFF)
		//fmt.Printf("childTime: %x\n",childTime)
		delta := f.readUint64(1, childTime)
		//fmt.Printf("delta: %v\n", delta)
		pTime := childTime - delta
		header.Time = pTime
		time = childTime
	}
	// Bomb delay will never exceed uint64
	bombDelay := new(big.Int).SetUint64(f.readUint64(1, 0xFFFFFFFFFFFFFFFe))

	if f.exhausted {
		return 0
	}

	for i, pair := range []struct {
		bigFn  calculator
		u256Fn calculator
	}{
		{ethash.FrontierDifficultyCalculator, ethash.CalcDifficultyFrontierU256},
		{ethash.HomesteadDifficultyCalculator, ethash.CalcDifficultyHomesteadU256},
	} {
		want := pair.bigFn(time, header)
		have := pair.u256Fn(time, header)
		if want.Cmp(have) != 0 {
			panic(fmt.Sprintf("pair %d: want %x have %x\nparent.Number: %x\np.Time: %x\nc.Time: %x\nBombdelay: %v\n", i, want, have,
				header.Number, header.Time, time, bombDelay))
		}
	}
	return 1
}
