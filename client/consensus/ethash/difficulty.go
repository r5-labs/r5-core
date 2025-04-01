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
	"github.com/r5-labs/r5-core/core/types"
)

const (
	// Custom difficulty parameters (used for blocks before hardfork)
	minimumDifficulty         = 1000000
	difficultyBoundDivisor    = 512
	// R5 difficulty parameters (used for blocks >= hardfork)
	r5MinimumDifficulty       = 1000000
	r5DifficultyBoundDivisor  = 11
	// Hardfork block number
	hardForkBlock             = 45000
	// Block time target (in seconds) for both calculations;
	// note: for R5 (post-hardfork) this replaces original target.
	targetBlockTime           = 7
)

func roundRat(r *big.Rat) *big.Int {
	num := new(big.Int).Set(r.Num())
	denom := new(big.Int).Set(r.Denom())
	quotient, remainder := new(big.Int).QuoRem(num, denom, new(big.Int))
	if new(big.Int).Mul(remainder, big.NewInt(2)).Cmp(denom) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}

// CalcDifficultyFrontierU256 calculates the block difficulty using the Frontier rules.
// For blocks with parent.Number >= hardForkBlock the calculation uses the R5 new formula
func CalcDifficultyFrontierU256(time uint64, parent *types.Header) *big.Int {
	// If past the hardfork block, use R5 difficulty calculation.
	if parent.Number.Uint64() >= hardForkBlock {
		pDiff, _ := uint256.FromBig(parent.Difficulty)
		adjust := pDiff.Clone()
		adjust.Rsh(adjust, r5DifficultyBoundDivisor)
		// With a 7-second target, add difficulty if block time is less than 7 sec.
		if time-parent.Time < targetBlockTime {
			pDiff.Add(pDiff, adjust)
		} else {
			pDiff.Sub(pDiff, adjust)
		}
		if pDiff.LtUint64(r5MinimumDifficulty) {
			pDiff.SetUint64(r5MinimumDifficulty)
		}
		return pDiff.ToBig()
	}

	// Custom (pre-hardfork) calculation.
	pDiff, _ := uint256.FromBig(parent.Difficulty)
	adjust := pDiff.Clone()
	adjust.Rsh(adjust, difficultyBoundDivisor)
	diffSec := int64(time - parent.Time)
	x := new(big.Rat).SetFrac64(diffSec, targetBlockTime)

	cVal := int64(1)
	if parent.UncleHash != types.EmptyUncleHash {
		cVal = 2
	}
	cRat := new(big.Rat).SetInt64(cVal)

	delta := new(big.Rat).Sub(new(big.Rat).Quo(x, cRat), big.NewRat(1, 1))

	adjustBig := adjust.ToBig()
	prod := new(big.Rat).Mul(new(big.Rat).SetInt(adjustBig), delta)
	adjAmount := roundRat(prod)

	newDiff := new(big.Int).Set(parent.Difficulty)
	if delta.Sign() >= 0 {
		newDiff.Sub(newDiff, adjAmount)
	} else {
		newDiff.Add(newDiff, adjAmount)
	}

	if newDiff.Cmp(big.NewInt(minimumDifficulty)) < 0 {
		newDiff.SetUint64(minimumDifficulty)
	}
	if time-parent.Time < 2 {
		newDiff.Mul(newDiff, big.NewInt(11))
		newDiff.Div(newDiff, big.NewInt(10))
	}
	if parent.Number.Uint64() < 300 {
		newDiff.Mul(newDiff, big.NewInt(3))
		newDiff.Div(newDiff, big.NewInt(2))
	}
	return newDiff
}

// CalcDifficultyHomesteadU256 calculates the block difficulty using the Homestead rules.
// For blocks with parent.Number >= hardForkBlock the calculation uses the R5 formula.
func CalcDifficultyHomesteadU256(time uint64, parent *types.Header) *big.Int {
	// R5 calculation for post-hardfork blocks.
	if parent.Number.Uint64() >= hardForkBlock {
		pDiff, _ := uint256.FromBig(parent.Difficulty)
		adjust := pDiff.Clone()
		adjust.Rsh(adjust, r5DifficultyBoundDivisor)
		// Replace the original divisor of 10 with our targetBlockTime (7 seconds)
		x := (time - parent.Time) / targetBlockTime
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
		adjust.Mul(adjust, z)
		if neg {
			pDiff.Sub(pDiff, adjust)
		} else {
			pDiff.Add(pDiff, adjust)
		}
		if pDiff.LtUint64(r5MinimumDifficulty) {
			pDiff.SetUint64(r5MinimumDifficulty)
		}
		return pDiff.ToBig()
	}

	// Custom (pre-hardfork) calculation.
	pDiff, _ := uint256.FromBig(parent.Difficulty)
	adjust := pDiff.Clone()
	adjust.Rsh(adjust, difficultyBoundDivisor)
	diffSec := int64(time - parent.Time)
	x := new(big.Rat).SetFrac64(diffSec, targetBlockTime)

	oneRat := new(big.Rat).SetInt64(1)
	factor := new(big.Rat).Sub(x, oneRat)
	neg := false
	if factor.Sign() < 0 {
		neg = true
		factor.Abs(factor)
	}

	adjustBig := adjust.ToBig()
	prod := new(big.Rat).Mul(new(big.Rat).SetInt(adjustBig), factor)
	adjAmount := roundRat(prod)

	newDiff := new(big.Int).Set(parent.Difficulty)
	if neg {
		newDiff.Add(newDiff, adjAmount)
	} else {
		newDiff.Sub(newDiff, adjAmount)
	}

	if newDiff.Cmp(big.NewInt(minimumDifficulty)) < 0 {
		newDiff.SetUint64(minimumDifficulty)
	}
	if time-parent.Time < 2 {
		newDiff.Mul(newDiff, big.NewInt(11))
		newDiff.Div(newDiff, big.NewInt(10))
	}
	if parent.Number.Uint64() < 300 {
		newDiff.Mul(newDiff, big.NewInt(3))
		newDiff.Div(newDiff, big.NewInt(2))
	}	
	return newDiff
}

// MakeDifficultyCalculatorU256 returns a function to calculate difficulty.
// For blocks with parent.Number >= hardForkBlock the returned function uses the R5 formula.
func MakeDifficultyCalculatorU256() func(time uint64, parent *types.Header) *big.Int {
	return func(time uint64, parent *types.Header) *big.Int {
		// R5 calculation for post-hardfork blocks.
		if parent.Number.Uint64() >= hardForkBlock {
			x := (time - parent.Time) / targetBlockTime
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
			y.Rsh(y, r5DifficultyBoundDivisor)
			z.Mul(y, z)
			if xNeg {
				y.Sub(pDiff, z)
			} else {
				y.Add(pDiff, z)
			}
			if y.LtUint64(r5MinimumDifficulty) {
				y.SetUint64(r5MinimumDifficulty)
			}
			return y.ToBig()
		}

		// Custom (pre-hardfork) calculation.
		diffSec := int64(time - parent.Time)
		xRat := new(big.Rat).SetFrac64(diffSec, targetBlockTime)

		cVal := int64(1)
		if parent.UncleHash != types.EmptyUncleHash {
			cVal = 2
		}
		cRat := new(big.Rat).SetInt64(cVal)

		delta := new(big.Rat).Sub(new(big.Rat).Quo(xRat, cRat), big.NewRat(1, 1))

		y := new(uint256.Int)
		y.SetFromBig(parent.Difficulty)
		y.Rsh(y, difficultyBoundDivisor)

		prod := new(big.Rat).Mul(new(big.Rat).SetInt(y.ToBig()), delta)
		adjAmount := roundRat(prod)

		if xRat.Cmp(cRat) >= 0 {
			newDiff := new(big.Int).Sub(parent.Difficulty, adjAmount)
			if newDiff.Cmp(big.NewInt(minimumDifficulty)) < 0 {
				newDiff.SetUint64(minimumDifficulty)
			}
			return newDiff
		} else {
			newDiff := new(big.Int).Add(parent.Difficulty, adjAmount)
			return newDiff
		}
	}
}
