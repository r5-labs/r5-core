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

package ethash

import (
	"math/big"

	"github.com/holiman/uint256"
	"github.com/r5-labs/r5-core/core/types"
)

const (
	// frontierDurationLimit is for Frontier:
	// The decision boundary on the blocktime duration used to determine
	// whether difficulty should go up or down.
	frontierDurationLimit = 13
	// minimumDifficulty is the minimum that the difficulty may ever be.
	minimumDifficulty = 131072
	// difficultyBoundDivisor is the bit shift divisor for the difficulty adjustment (i.e. division by 2048).
	difficultyBoundDivisor = 11
)

// CalcDifficultyFrontierU256 computes the difficulty for a new block using Frontier rules,
// without applying any exponential difficulty bomb.
func CalcDifficultyFrontierU256(time uint64, parent *types.Header) *big.Int {
	pDiff, _ := uint256.FromBig(parent.Difficulty)
	adjust := pDiff.Clone()
	adjust.Rsh(adjust, difficultyBoundDivisor) // adjust = parent.difficulty / 2048

	if time-parent.Time < frontierDurationLimit {
		pDiff.Add(pDiff, adjust)
	} else {
		pDiff.Sub(pDiff, adjust)
	}
	if pDiff.LtUint64(minimumDifficulty) {
		pDiff.SetUint64(minimumDifficulty)
	}
	// Bomb logic removed.
	return pDiff.ToBig()
}

// CalcDifficultyHomesteadU256 computes the difficulty for a new block using Homestead rules,
// without applying any exponential difficulty bomb.
func CalcDifficultyHomesteadU256(time uint64, parent *types.Header) *big.Int {
	pDiff, _ := uint256.FromBig(parent.Difficulty)
	adjust := pDiff.Clone()
	adjust.Rsh(adjust, difficultyBoundDivisor) // adjust = parent.difficulty / 2048

	// Compute time adjustment factor.
	x := (time - parent.Time) / 10
	neg := true
	if x == 0 {
		x = 1
		neg = false
	} else if x >= 100 {
		x = 99
	} else {
		x = x - 1
	}
	z := new(uint256.Int).SetUint64(x)
	adjust.Mul(adjust, z) // adjust factor = (parent.difficulty / 2048) * adjustment factor
	if neg {
		pDiff.Sub(pDiff, adjust)
	} else {
		pDiff.Add(pDiff, adjust)
	}
	if pDiff.LtUint64(minimumDifficulty) {
		pDiff.SetUint64(minimumDifficulty)
	}
	// Bomb logic removed.
	return pDiff.ToBig()
}

// MakeDifficultyCalculatorU256 returns a difficulty calculator function using Byzantium rules
// without any difficulty bomb adjustments.
func MakeDifficultyCalculatorU256() func(time uint64, parent *types.Header) *big.Int {
	return func(time uint64, parent *types.Header) *big.Int {
		/*
			Byzantium adjustment:
			child_diff = parent_diff + (parent_diff / 2048) * adjustment_factor
			where adjustment_factor = |( (timestamp - parent_timestamp) / 9 - C)|
			and C = 1 if no uncles, 2 if uncles exist, capped at 99.
		*/
		x := (time - parent.Time) / 9
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
		y.Rsh(y, difficultyBoundDivisor) // y becomes parent.difficulty / 2048
		z.Mul(y, z)
		if xNeg {
			y.Sub(pDiff, z)
		} else {
			y.Add(pDiff, z)
		}
		if y.LtUint64(minimumDifficulty) {
			y.SetUint64(minimumDifficulty)
		}
		// Bomb logic removed.
		return y.ToBig()
	}
}
