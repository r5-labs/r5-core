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

package ethash

import (
	"math/big"

	"github.com/holiman/uint256"
	"github.com/r5-labs/r5-core/client/core/types"
)

const (
	// targetDurationLimit is the block time target.
	targetDurationLimit = 7
	// minimumDifficulty The minimum that the difficulty may ever be.
	minimumDifficulty = 131072
	// expDiffPeriodUint = 100000
	// difficultyBoundDivisor is the bound divisor (2048),
	// and the right-shifts to use for the division.
	difficultyBoundDivisor = 11
)

// CalcDifficultyFrontierU256 is the difficulty adjustment algorithm using the Frontier rules.
func CalcDifficultyFrontierU256(time uint64, parent *types.Header) *big.Int {
	/*
		Modified algorithm:
		block_diff = pdiff + pdiff / 2048 * (1 if time - ptime < 7 else -1)
	*/

	pDiff, _ := uint256.FromBig(parent.Difficulty) // pDiff: parent's difficulty
	adjust := pDiff.Clone()
	adjust.Rsh(adjust, difficultyBoundDivisor) // adjust = parent's difficulty / 2048

	if time-parent.Time < targetDurationLimit {
		pDiff.Add(pDiff, adjust)
	} else {
		pDiff.Sub(pDiff, adjust)
	}
	if pDiff.LtUint64(minimumDifficulty) {
		pDiff.SetUint64(minimumDifficulty)
	}
	return pDiff.ToBig()
}

// CalcDifficultyHomesteadU256 is the difficulty adjustment algorithm using the Homestead rules.
func CalcDifficultyHomesteadU256(time uint64, parent *types.Header) *big.Int {
	/*
		Modified algorithm:
		block_diff = pdiff + pdiff / 2048 * (adjustment factor)
		where adjustment factor is derived from (time - ptime)/7, adjusted so that a gap below 7 sec increases difficulty.
	*/

	pDiff, _ := uint256.FromBig(parent.Difficulty) // pDiff: parent's difficulty
	adjust := pDiff.Clone()
	adjust.Rsh(adjust, difficultyBoundDivisor) // adjust = parent's difficulty / 2048

	x := (time - parent.Time) / 7 // use 7-second target instead of 10
	var neg = true
	if x == 0 {
		x = 1
		neg = false
	} else if x >= 100 {
		x = 99
	} else {
		x = x - 1
	}
	z := new(uint256.Int).SetUint64(x)
	adjust.Mul(adjust, z) // adjust now holds: (parent difficulty / 2048) * adjustment factor
	if neg {
		pDiff.Sub(pDiff, adjust)
	} else {
		pDiff.Add(pDiff, adjust)
	}
	if pDiff.LtUint64(minimumDifficulty) {
		pDiff.SetUint64(minimumDifficulty)
	}
	return pDiff.ToBig()
}

// MakeDifficultyCalculatorU256 creates a difficulty calculator function using Byzantium rules.
func MakeDifficultyCalculatorU256(bombDelay *big.Int) func(time uint64, parent *types.Header) *big.Int {
	// bombDelay parameter is still accepted for compatibility but is now unused.
	_ = bombDelay
	return func(time uint64, parent *types.Header) *big.Int {
		/*
			Modified algorithm:
			pDiff = parent.difficulty + parent.difficulty/2048 *
			           ( (adjustment constant) - ((time - ptime) / 7) )
			The adjustment constant is 1 or 2 depending on the presence of uncles.
			The difficulty bomb is removed.
		*/
		x := (time - parent.Time) / 7 // adjust using 7-second target
		c := uint64(1)
		if parent.UncleHash != types.EmptyUncleHash {
			c = 2
		}
		xNeg := x >= c
		if xNeg {
			x = x - c
		} else {
			x = c - x
		}
		if x > 99 {
			x = 99
		}
		y := new(uint256.Int)
		y.SetFromBig(parent.Difficulty)
		pDiff := y.Clone()
		z := new(uint256.Int).SetUint64(x)
		y.Rsh(y, difficultyBoundDivisor)
		z.Mul(y, z)
		if xNeg {
			y.Sub(pDiff, z)
		} else {
			y.Add(pDiff, z)
		}
		if y.LtUint64(minimumDifficulty) {
			y.SetUint64(minimumDifficulty)
		}
		return y.ToBig()
	}
}
