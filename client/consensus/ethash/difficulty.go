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
	// We'll use big.Rat for fixed-point arithmetic.
)

// Constants remain unchanged.
const (
	// frontierDurationLimit is for Frontier:
	// The decision boundary on the blocktime duration used to determine
	// whether difficulty should go up or down.
	frontierDurationLimit = 7
	// minimumDifficulty The minimum that the difficulty may ever be.
	minimumDifficulty = 131072
	// difficultyBoundDivisor is the bit shift used for dividing by 2048.
	difficultyBoundDivisor = 11
)

// roundRat rounds a big.Rat to the nearest integer.
func roundRat(r *big.Rat) *big.Int {
	// Get numerator and denominator.
	num := new(big.Int).Set(r.Num())
	denom := new(big.Int).Set(r.Denom())
	// Compute quotient and remainder.
	quotient, remainder := new(big.Int).QuoRem(num, denom, new(big.Int))
	// Multiply remainder by 2 and compare with denom.
	two := big.NewInt(2)
	if new(big.Int).Mul(remainder, two).Cmp(denom) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}

// CalcDifficultyFrontierU256 now uses fixed‐point division by 7.
func CalcDifficultyFrontierU256(time uint64, parent *types.Header) *big.Int {
	// pDiff = parent.Difficulty as a uint256.Int
	pDiff, _ := uint256.FromBig(parent.Difficulty)
	// adjust = pDiff >> difficultyBoundDivisor (i.e. parent.Difficulty / 2048)
	adjust := pDiff.Clone()
	adjust.Rsh(adjust, difficultyBoundDivisor)

	// Compute the exact time difference as a rational number:
	// x = (time - parent.Time) / 7
	diffSec := int64(time - parent.Time)
	x := new(big.Rat).SetFrac64(diffSec, 7)

	// Determine the expected target factor c:
	// c = 1 if no uncles, 2 if uncles exist.
	cVal := int64(1)
	if parent.UncleHash != types.EmptyUncleHash {
		cVal = 2
	}
	cRat := new(big.Rat).SetInt64(cVal)

	// Compute delta = x - c. (This can be negative.)
	delta := new(big.Rat).Sub(x, cRat)

	// Let absDelta = |delta|
	absDelta := new(big.Rat).Abs(delta)

	// Convert the "adjust" (parent.Difficulty/2048) to a big.Int.
	adjustBig := adjust.ToBig()

	// Multiply adjustBig * absDelta in rational space.
	prod := new(big.Rat).Mul(new(big.Rat).SetInt(adjustBig), absDelta)
	// Round the product to the nearest integer.
	adjAmount := roundRat(prod)

	// Now, if delta is nonnegative (i.e. x >= c) then we subtract adjAmount,
	// otherwise (x < c) we add it.
	newDiff := new(big.Int).Set(parent.Difficulty)
	if delta.Sign() >= 0 {
		newDiff.Sub(newDiff, adjAmount)
	} else {
		newDiff.Add(newDiff, adjAmount)
	}
	// Enforce minimum difficulty.
	if newDiff.Cmp(big.NewInt(minimumDifficulty)) < 0 {
		newDiff.SetUint64(minimumDifficulty)
	}
	return newDiff
}

// CalcDifficultyHomesteadU256 now uses fixed‐point division.
func CalcDifficultyHomesteadU256(time uint64, parent *types.Header) *big.Int {
	pDiff, _ := uint256.FromBig(parent.Difficulty)
	adjust := pDiff.Clone()
	adjust.Rsh(adjust, difficultyBoundDivisor) // adjust = parent.Difficulty / 2048

	// Compute x = (time - parent.Time) / 7 as a rational.
	diffSec := int64(time - parent.Time)
	x := new(big.Rat).SetFrac64(diffSec, 7)

	// For Homestead we subtract 1 from x (if possible) and then use that as the adjustment factor.
	// Let desired adjustment factor be: factor = (x - 1) if x >= 1, else (1 - x).
	oneRat := new(big.Rat).SetInt64(1)
	factor := new(big.Rat).Sub(x, oneRat)
	neg := false
	if factor.Sign() < 0 {
		neg = true
		factor.Abs(factor)
	}
	// Cap the factor at 99.
	ninetyNine := new(big.Rat).SetInt64(99)
	if factor.Cmp(ninetyNine) > 0 {
		factor.Set(ninetyNine)
	}
	adjustBig := adjust.ToBig()
	prod := new(big.Rat).Mul(new(big.Rat).SetInt(adjustBig), factor)
	adjAmount := roundRat(prod)
	newDiff := new(big.Int).Set(parent.Difficulty)
	if neg {
		// If x < 1, then we add.
		newDiff.Add(newDiff, adjAmount)
	} else {
		// If x >= 1, then subtract.
		newDiff.Sub(newDiff, adjAmount)
	}
	if newDiff.Cmp(big.NewInt(minimumDifficulty)) < 0 {
		newDiff.SetUint64(minimumDifficulty)
	}
	return newDiff
}

// MakeDifficultyCalculatorU256 returns a function that computes the Byzantium difficulty.
// It uses fixed-point arithmetic for the adjustment factor.
func MakeDifficultyCalculatorU256() func(time uint64, parent *types.Header) *big.Int {
	return func(time uint64, parent *types.Header) *big.Int {
		// Compute x = (time - parent.Time) / 7 as a rational.
		diffSec := int64(time - parent.Time)
		x := new(big.Rat).SetFrac64(diffSec, 7)

		// Determine c: 1 if no uncles, 2 if uncles exist.
		cVal := int64(1)
		if parent.UncleHash != types.EmptyUncleHash {
			cVal = 2
		}
		cRat := new(big.Rat).SetInt64(cVal)

		// Compute delta = |x - c|
		delta := new(big.Rat).Sub(x, cRat)
		delta.Abs(delta)
		// Cap delta at 99.
		ninetyNine := new(big.Rat).SetInt64(99)
		if delta.Cmp(ninetyNine) > 0 {
			delta.Set(ninetyNine)
		}

		// Convert parent's difficulty divided by 2048.
		y := new(uint256.Int)
		y.SetFromBig(parent.Difficulty)
		y.Rsh(y, difficultyBoundDivisor) // y = parent.Difficulty / 2048

		// Multiply y by delta.
		prod := new(big.Rat).Mul(new(big.Rat).SetInt(y.ToBig()), delta)
		adjAmount := roundRat(prod)

		// Determine sign: if x >= c, then subtract; else add.
		if x.Cmp(cRat) >= 0 {
			// Subtract adjustment.
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
