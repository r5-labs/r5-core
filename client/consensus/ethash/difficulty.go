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
	// Minimum difficulty level
	minimumDifficulty = 1000000
	// More aggressive difficulty adjustment
	difficultyBoundDivisor = 512
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

func CalcDifficultyFrontierU256(time uint64, parent *types.Header) *big.Int {
	pDiff, _ := uint256.FromBig(parent.Difficulty)
	adjust := pDiff.Clone()
	adjust.Rsh(adjust, difficultyBoundDivisor)
	diffSec := int64(time - parent.Time)
	x := new(big.Rat).SetFrac64(diffSec, 7)

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

func CalcDifficultyHomesteadU256(time uint64, parent *types.Header) *big.Int {
	pDiff, _ := uint256.FromBig(parent.Difficulty)
	adjust := pDiff.Clone()
	adjust.Rsh(adjust, difficultyBoundDivisor)
	diffSec := int64(time - parent.Time)
	x := new(big.Rat).SetFrac64(diffSec, 7)

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

func MakeDifficultyCalculatorU256() func(time uint64, parent *types.Header) *big.Int {
	return func(time uint64, parent *types.Header) *big.Int {
		diffSec := int64(time - parent.Time)
		x := new(big.Rat).SetFrac64(diffSec, 7)

		cVal := int64(1)
		if parent.UncleHash != types.EmptyUncleHash {
			cVal = 2
		}
		cRat := new(big.Rat).SetInt64(cVal)

		delta := new(big.Rat).Sub(new(big.Rat).Quo(x, cRat), big.NewRat(1, 1))

		y := new(uint256.Int)
		y.SetFromBig(parent.Difficulty)
		y.Rsh(y, difficultyBoundDivisor)

		prod := new(big.Rat).Mul(new(big.Rat).SetInt(y.ToBig()), delta)
		adjAmount := roundRat(prod)

		if x.Cmp(cRat) >= 0 {
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
