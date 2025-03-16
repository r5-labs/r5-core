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
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/holiman/uint256"
	"github.com/r5-labs/r5-core/common"
	"github.com/r5-labs/r5-core/consensus"
	"github.com/r5-labs/r5-core/consensus/misc"
	"github.com/r5-labs/r5-core/core/state"
	"github.com/r5-labs/r5-core/core/types"
	"github.com/r5-labs/r5-core/params"
	"github.com/r5-labs/r5-core/rlp"
	"github.com/r5-labs/r5-core/trie"
	"golang.org/x/crypto/sha3"
)

// Ethash-R5 proof-of-work protocol constants.
var (
	FrontierBlockReward           	= big.NewInt(1e+18)		// Block reward in wei for successfully mining a block
	ByzantiumBlockReward          	= big.NewInt(1e+18)		// Block reward in wei for successfully mining a block upward from Byzantium
	ConstantinopleBlockReward     	= big.NewInt(1e+18)		// Block reward in wei for successfully mining a block upward from Constantinople
	maxUncles                     	= 2						// Maximum number of uncles allowed in a single block
	allowedFutureBlockTimeSeconds 	= int64(8)				// Max seconds from current time allowed for blocks, before they're considered future blocks
	
	// Wallet that will collect all the transaction fees up until supply cap is reached
	feePoolWallet 					= common.HexToAddress("0xc657de8D48cAB170e98782815670f8B019005473")
	
	// Supply cap definitions, SupplyCap needs to be validated by finalBlock, according
	// to the emission schedule
	SupplyCap 						= new(big.Int).Mul(big.NewInt(66337700), big.NewInt(1e18))
	preminedSupply 					= new(big.Int).Mul(big.NewInt(2000000), big.NewInt(1000000000000000000))
	finalBlock 						= uint64(1290406400)

	// calcDifficultyEip5133 is the difficulty adjustment algorithm as specified by EIP 5133.
	// It offsets the bomb a total of 11.4M blocks.
	// Specification EIP-5133: https://eips.ethereum.org/EIPS/eip-5133
	calcDifficultyEip5133 = makeDifficultyCalculator()

	// calcDifficultyEip4345 is the difficulty adjustment algorithm as specified by EIP 4345.
	// It offsets the bomb a total of 10.7M blocks.
	// Specification EIP-4345: https://eips.ethereum.org/EIPS/eip-4345
	calcDifficultyEip4345 = makeDifficultyCalculator()

	// calcDifficultyEip3554 is the difficulty adjustment algorithm as specified by EIP 3554.
	// It offsets the bomb a total of 9.7M blocks.
	// Specification EIP-3554: https://eips.ethereum.org/EIPS/eip-3554
	calcDifficultyEip3554 = makeDifficultyCalculator()

	// calcDifficultyEip2384 is the difficulty adjustment algorithm as specified by EIP 2384.
	// It offsets the bomb 4M blocks from Constantinople, so in total 9M blocks.
	// Specification EIP-2384: https://eips.ethereum.org/EIPS/eip-2384
	calcDifficultyEip2384 = makeDifficultyCalculator()

	// calcDifficultyConstantinople is the difficulty adjustment algorithm for Constantinople.
	// It returns the difficulty that a new block should have when created at time given the
	// parent block's time and difficulty. The calculation uses the Byzantium rules, but with
	// bomb offset 5M.
	// Specification EIP-1234: https://eips.ethereum.org/EIPS/eip-1234
	calcDifficultyConstantinople = makeDifficultyCalculator()

	// calcDifficultyByzantium is the difficulty adjustment algorithm. It returns
	// the difficulty that a new block should have when created at time given the
	// parent block's time and difficulty. The calculation uses the Byzantium rules.
	// Specification EIP-649: https://eips.ethereum.org/EIPS/eip-649
	calcDifficultyByzantium = makeDifficultyCalculator()
)

// CalculateCirculatingSupply returns the current circulating supply (in wei) at the given block number.
// It sums the pre-mined supply and the cumulative block rewards as defined by the Super Epoch schedule.
// For blocks >= finalBlock, it returns the maximum supply (i.e. premined supply plus all block rewards).
func CalculateCirculatingSupply(blockNum uint64) *big.Int {
	// 1 R5 = 1e18 wei.
	weiPerR5 := big.NewInt(1000000000000000000)
	// Start with the pre-mined supply.
	supply := new(big.Int).Set(preminedSupply)

	// Define epoch endpoints (block numbers).
	const (
		epoch1End = 4000000
		epoch2End = 8000000
		epoch3End = 16000000
		epoch4End = 32000000
		epoch5End = 64000000
		epoch6End = 128000000
	)
	// If blockNum is at or beyond the final block number, return the full issuance.
	if blockNum >= finalBlock {
		// When blockNum is at or beyond finalBlock, the total block rewards issued
		// should equal 66,337,700 - 2,000,000 = 64,337,700 R5.
		totalBlockRewards := new(big.Int).Mul(big.NewInt(64337700), weiPerR5)
		return new(big.Int).Add(preminedSupply, totalBlockRewards)
	}	

	// Helper: minBlocks returns the number of blocks in the interval [start, min(blockNum, end)].
	minBlocks := func(start, end uint64) uint64 {
		if blockNum < start {
			return 0
		}
		if blockNum > end {
			return end - start + 1
		}
		return blockNum - start + 1
	}

	var epochReward *big.Int

	// Epoch 1: Blocks 1 to 4,000,000, reward 2 R5 per block.
	blocks := minBlocks(1, epoch1End)
	epochReward = new(big.Int).Mul(big.NewInt(2), weiPerR5)
	supply.Add(supply, new(big.Int).Mul(big.NewInt(int64(blocks)), epochReward))

	// Epoch 2: Blocks 4,000,001 to 8,000,000, reward 1 R5 per block.
	blocks = minBlocks(epoch1End+1, epoch2End)
	epochReward = new(big.Int).Set(weiPerR5)
	supply.Add(supply, new(big.Int).Mul(big.NewInt(int64(blocks)), epochReward))

	// Epoch 3: Blocks 8,000,001 to 16,000,000, reward 0.5 R5 per block.
	blocks = minBlocks(epoch2End+1, epoch3End)
	epochReward = big.NewInt(500000000000000000) // 0.5 R5 in wei
	supply.Add(supply, new(big.Int).Mul(big.NewInt(int64(blocks)), epochReward))

	// Epoch 4: Blocks 16,000,001 to 32,000,000, reward 0.25 R5 per block.
	blocks = minBlocks(epoch3End+1, epoch4End)
	epochReward = big.NewInt(250000000000000000) // 0.25 R5 in wei
	supply.Add(supply, new(big.Int).Mul(big.NewInt(int64(blocks)), epochReward))

	// Epoch 5: Blocks 32,000,001 to 64,000,000, reward 0.125 R5 per block.
	blocks = minBlocks(epoch4End+1, epoch5End)
	epochReward = big.NewInt(125000000000000000) // 0.125 R5 in wei
	supply.Add(supply, new(big.Int).Mul(big.NewInt(int64(blocks)), epochReward))

	// Epoch 6: Blocks 64,000,001 to 128,000,000, reward 0.0625 R5 per block.
	blocks = minBlocks(epoch5End+1, epoch6End)
	epochReward = big.NewInt(62500000000000000) // 0.0625 R5 in wei
	supply.Add(supply, new(big.Int).Mul(big.NewInt(int64(blocks)), epochReward))

	// Epoch 7: Blocks 128,000,001 up to blockNum, reward 0.03125 R5 per block.
	if blockNum > epoch6End {
		blocks = minBlocks(epoch6End+1, blockNum)
		epochReward = big.NewInt(31250000000000000) // 0.03125 R5 in wei
		supply.Add(supply, new(big.Int).Mul(big.NewInt(int64(blocks)), epochReward))
	}

	return supply
}

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	errOlderBlockTime    = errors.New("timestamp older than parent")
	errTooManyUncles     = errors.New("too many uncles")
	errDuplicateUncle    = errors.New("duplicate uncle")
	errUncleIsAncestor   = errors.New("uncle is ancestor")
	errDanglingUncle     = errors.New("uncle's parent is not ancestor")
	errInvalidDifficulty = errors.New("non-positive difficulty")
	errInvalidMixDigest  = errors.New("invalid mix digest")
	errInvalidPoW        = errors.New("invalid proof-of-work")
)

// Author implements consensus.Engine, returning the header's coinbase as the
// proof-of-work verified author of the block.
func (ethash *Ethash) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of the
// stock Ethereum ethash engine.
func (ethash *Ethash) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	// If we're running a full engine faking, accept any input as valid
	if ethash.config.PowMode == ModeFullFake {
		return nil
	}
	// Short circuit if the header is known, or its parent not
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	// Sanity checks passed, do a proper verification
	return ethash.verifyHeader(chain, header, parent, false, seal, time.Now().Unix())
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (ethash *Ethash) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	// If we're running a full engine faking, accept any input as valid
	if ethash.config.PowMode == ModeFullFake || len(headers) == 0 {
		abort, results := make(chan struct{}), make(chan error, len(headers))
		for i := 0; i < len(headers); i++ {
			results <- nil
		}
		return abort, results
	}

	// Spawn as many workers as allowed threads
	workers := runtime.GOMAXPROCS(0)
	if len(headers) < workers {
		workers = len(headers)
	}

	// Create a task channel and spawn the verifiers
	var (
		inputs  = make(chan int)
		done    = make(chan int, workers)
		errors  = make([]error, len(headers))
		abort   = make(chan struct{})
		unixNow = time.Now().Unix()
	)
	for i := 0; i < workers; i++ {
		go func() {
			for index := range inputs {
				errors[index] = ethash.verifyHeaderWorker(chain, headers, seals, index, unixNow)
				done <- index
			}
		}()
	}

	errorsOut := make(chan error, len(headers))
	go func() {
		defer close(inputs)
		var (
			in, out = 0, 0
			checked = make([]bool, len(headers))
			inputs  = inputs
		)
		for {
			select {
			case inputs <- in:
				if in++; in == len(headers) {
					// Reached end of headers. Stop sending to workers.
					inputs = nil
				}
			case index := <-done:
				for checked[index] = true; checked[out]; out++ {
					errorsOut <- errors[out]
					if out == len(headers)-1 {
						return
					}
				}
			case <-abort:
				return
			}
		}
	}()
	return abort, errorsOut
}

func (ethash *Ethash) verifyHeaderWorker(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool, index int, unixNow int64) error {
	var parent *types.Header
	if index == 0 {
		parent = chain.GetHeader(headers[0].ParentHash, headers[0].Number.Uint64()-1)
	} else if headers[index-1].Hash() == headers[index].ParentHash {
		parent = headers[index-1]
	}
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	return ethash.verifyHeader(chain, headers[index], parent, false, seals[index], unixNow)
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of the stock Ethereum ethash engine.
func (ethash *Ethash) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	// If we're running a full engine faking, accept any input as valid
	if ethash.config.PowMode == ModeFullFake {
		return nil
	}
	// Verify that there are at most 2 uncles included in this block
	if len(block.Uncles()) > maxUncles {
		return errTooManyUncles
	}
	if len(block.Uncles()) == 0 {
		return nil
	}
	// Gather the set of past uncles and ancestors
	uncles, ancestors := mapset.NewSet[common.Hash](), make(map[common.Hash]*types.Header)

	number, parent := block.NumberU64()-1, block.ParentHash()
	for i := 0; i < 7; i++ {
		ancestorHeader := chain.GetHeader(parent, number)
		if ancestorHeader == nil {
			break
		}
		ancestors[parent] = ancestorHeader
		// If the ancestor doesn't have any uncles, we don't have to iterate them
		if ancestorHeader.UncleHash != types.EmptyUncleHash {
			// Need to add those uncles to the banned list too
			ancestor := chain.GetBlock(parent, number)
			if ancestor == nil {
				break
			}
			for _, uncle := range ancestor.Uncles() {
				uncles.Add(uncle.Hash())
			}
		}
		parent, number = ancestorHeader.ParentHash, number-1
	}
	ancestors[block.Hash()] = block.Header()
	uncles.Add(block.Hash())

	// Verify each of the uncles that it's recent, but not an ancestor
	for _, uncle := range block.Uncles() {
		// Make sure every uncle is rewarded only once
		hash := uncle.Hash()
		if uncles.Contains(hash) {
			return errDuplicateUncle
		}
		uncles.Add(hash)

		// Make sure the uncle has a valid ancestry
		if ancestors[hash] != nil {
			return errUncleIsAncestor
		}
		if ancestors[uncle.ParentHash] == nil || uncle.ParentHash == block.ParentHash() {
			return errDanglingUncle
		}
		if err := ethash.verifyHeader(chain, uncle, ancestors[uncle.ParentHash], true, true, time.Now().Unix()); err != nil {
			return err
		}
	}
	return nil
}

// verifyHeader checks whether a header conforms to the consensus rules of the
// stock Ethereum ethash engine.
// See YP section 4.3.4. "Block Header Validity"
func (ethash *Ethash) verifyHeader(chain consensus.ChainHeaderReader, header, parent *types.Header, uncle bool, seal bool, unixNow int64) error {
	// Ensure that the header's extra-data section is of a reasonable size
	if uint64(len(header.Extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), params.MaximumExtraDataSize)
	}
	// Verify the header's timestamp
	if !uncle {
		if header.Time > uint64(unixNow+allowedFutureBlockTimeSeconds) {
			return consensus.ErrFutureBlock
		}
	}
	if header.Time <= parent.Time {
		return errOlderBlockTime
	}
	// Verify the block's difficulty based on its timestamp and parent's difficulty
	expected := ethash.CalcDifficulty(chain, header.Time, parent)

	if expected.Cmp(header.Difficulty) != 0 {
		return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, expected)
	}
	// Verify that the gas limit is <= 2^63-1
	if header.GasLimit > params.MaxGasLimit {
		return fmt.Errorf("invalid gasLimit: have %v, max %v", header.GasLimit, params.MaxGasLimit)
	}
	// Verify that the gasUsed is <= gasLimit
	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}
	// Verify the block's gas usage and (if applicable) verify the base fee.
	if !chain.Config().IsLondon(header.Number) {
		// Verify BaseFee not present before EIP-1559 fork.
		if header.BaseFee != nil {
			return fmt.Errorf("invalid baseFee before fork: have %d, expected 'nil'", header.BaseFee)
		}
		if err := misc.VerifyGaslimit(parent.GasLimit, header.GasLimit); err != nil {
			return err
		}
	} else if err := misc.VerifyEip1559Header(chain.Config(), parent, header); err != nil {
		// Verify the header's EIP-1559 attributes.
		return err
	}
	// Verify that the block number is parent's +1
	if diff := new(big.Int).Sub(header.Number, parent.Number); diff.Cmp(big.NewInt(1)) != 0 {
		return consensus.ErrInvalidNumber
	}
	if chain.Config().IsShanghai(header.Time) {
		return fmt.Errorf("ethash does not support shanghai fork")
	}
	if chain.Config().IsCancun(header.Time) {
		return fmt.Errorf("ethash does not support cancun fork")
	}
	// Verify the engine specific seal securing the block
	if seal {
		if err := ethash.verifySeal(chain, header, false); err != nil {
			return err
		}
	}
	// If all checks passed, validate any special fields for hard forks
	if err := misc.VerifyDAOHeaderExtraData(chain.Config(), header); err != nil {
		return err
	}
	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time
// given the parent block's time and difficulty.
func (ethash *Ethash) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return CalcDifficulty(chain.Config(), time, parent)
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time
// given the parent block's time and difficulty.
func CalcDifficulty(config *params.ChainConfig, time uint64, parent *types.Header) *big.Int {
	next := new(big.Int).Add(parent.Number, big1)
	switch {
	case config.IsGrayGlacier(next):
		return calcDifficultyEip5133(time, parent)
	case config.IsArrowGlacier(next):
		return calcDifficultyEip4345(time, parent)
	case config.IsLondon(next):
		return calcDifficultyEip3554(time, parent)
	case config.IsMuirGlacier(next):
		return calcDifficultyEip2384(time, parent)
	case config.IsConstantinople(next):
		return calcDifficultyConstantinople(time, parent)
	case config.IsByzantium(next):
		return calcDifficultyByzantium(time, parent)
	case config.IsHomestead(next):
		return calcDifficultyHomestead(time, parent)
	default:
		return calcDifficultyFrontier(time, parent)
	}
}

// Some weird constants to avoid constant memory allocs for them.
var (
	big1			= big.NewInt(1)
//	big2			= big.NewInt(2) // no longer used with new diff calculation
	big7			= big.NewInt(7)
//	big9 			= big.NewInt(9) // no longer used with new diff calculation
//	big10			= big.NewInt(10) // no longer used with new diff calculation
	bigMinus99		= big.NewInt(-99)
)

// makeDifficultyCalculator creates a difficulty calculator using Byzantium rules,
// with an adjustment factor computed for a 7-second target.
func makeDifficultyCalculator() func(time uint64, parent *types.Header) *big.Int {
	// Note: calculations below use parent's block time (which is one less than the block number).
	return func(time uint64, parent *types.Header) *big.Int {
		/*
			Byzantium adjustment:
			child_diff = parent_diff + (parent_diff / 2048) * adjustment_factor
			where adjustment_factor = |( (timestamp - parent_timestamp) / 7 - C)|
			and C = 1 if no uncles, 2 if uncles exist, capped at 99.
		*/
		x := (time - parent.Time) / 7  // changed divisor from 9 to 7
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
		return y.ToBig()
	}
}

// calcDifficultyHomestead computes the block difficulty using Homestead rules
// without applying any exponential bomb factor.
// New formula: diff = parent_diff + (parent_diff / 2048 * max(1 - ((time - parent.Time) // 7), -99))
func calcDifficultyHomestead(time uint64, parent *types.Header) *big.Int {
	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).SetUint64(parent.Time)

	// x will hold the adjustment factor: 1 - ((time - parent.Time) // 7)
	x := new(big.Int)
	// y will temporarily hold parent_diff / 2048.
	y := new(big.Int)

	// Compute (time - parent.Time) // 7
	x.Sub(bigTime, bigParentTime)
	x.Div(x, big7) // changed divisor from big10 to big7
	// Now compute: 1 - ((time - parent.Time) // 7)
	x.Sub(big1, x)

	// Ensure the adjustment is at least -99.
	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}

	// Compute parent_diff / 2048 (using DifficultyBoundDivisor == 11)
	y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
	// Multiply the adjustment factor by (parent_diff / 2048)
	x.Mul(y, x)
	// Add the adjustment to the parent difficulty.
	x.Add(parent.Difficulty, x)

	// Ensure difficulty does not fall below the minimum threshold.
	if x.Cmp(params.MinimumDifficulty) < 0 {
		x.Set(params.MinimumDifficulty)
	}
	return x
}

// calcDifficultyFrontier computes the block difficulty using Frontier rules
// without any exponential bomb component.
func calcDifficultyFrontier(time uint64, parent *types.Header) *big.Int {
	diff := new(big.Int)
	// Calculate adjustment = parent_diff / 2048
	adjust := new(big.Int).Div(parent.Difficulty, params.DifficultyBoundDivisor)
	
	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).SetUint64(parent.Time)

	// If the time difference is less than the duration limit, increase difficulty;
	// otherwise, decrease it.
	if bigTime.Sub(bigTime, bigParentTime).Cmp(params.DurationLimit) < 0 {
		diff.Add(parent.Difficulty, adjust)
	} else {
		diff.Sub(parent.Difficulty, adjust)
	}
	// Ensure the difficulty does not drop below the minimum
	if diff.Cmp(params.MinimumDifficulty) < 0 {
		diff.Set(params.MinimumDifficulty)
	}
	return diff
}

// Exported for fuzzing
var FrontierDifficultyCalculator = calcDifficultyFrontier
var HomesteadDifficultyCalculator = calcDifficultyHomestead
var DynamicDifficultyCalculator = makeDifficultyCalculator

// verifySeal checks whether a block satisfies the PoW difficulty requirements,
// either using the usual ethash cache for it, or alternatively using a full DAG
// to make remote mining fast.
func (ethash *Ethash) verifySeal(chain consensus.ChainHeaderReader, header *types.Header, fulldag bool) error {
	// If we're running a fake PoW, accept any seal as valid
	if ethash.config.PowMode == ModeFake || ethash.config.PowMode == ModeFullFake {
		time.Sleep(ethash.fakeDelay)
		if ethash.fakeFail == header.Number.Uint64() {
			return errInvalidPoW
		}
		return nil
	}
	// If we're running a shared PoW, delegate verification to it
	if ethash.shared != nil {
		return ethash.shared.verifySeal(chain, header, fulldag)
	}
	// Ensure that we have a valid difficulty for the block
	if header.Difficulty.Sign() <= 0 {
		return errInvalidDifficulty
	}
	// Recompute the digest and PoW values
	number := header.Number.Uint64()

	var (
		digest []byte
		result []byte
	)
	// If fast-but-heavy PoW verification was requested, use an ethash dataset
	if fulldag {
		dataset := ethash.dataset(number, true)
		if dataset.generated() {
			digest, result = hashimotoFull(dataset.dataset, ethash.SealHash(header).Bytes(), header.Nonce.Uint64())

			// Datasets are unmapped in a finalizer. Ensure that the dataset stays alive
			// until after the call to hashimotoFull so it's not unmapped while being used.
			runtime.KeepAlive(dataset)
		} else {
			// Dataset not yet generated, don't hang, use a cache instead
			fulldag = false
		}
	}
	// If slow-but-light PoW verification was requested (or DAG not yet ready), use an ethash cache
	if !fulldag {
		cache := ethash.cache(number)

		size := datasetSize(number)
		if ethash.config.PowMode == ModeTest {
			size = 32 * 1024
		}
		digest, result = hashimotoLight(size, cache.cache, ethash.SealHash(header).Bytes(), header.Nonce.Uint64())

		// Caches are unmapped in a finalizer. Ensure that the cache stays alive
		// until after the call to hashimotoLight so it's not unmapped while being used.
		runtime.KeepAlive(cache)
	}
	// Verify the calculated values against the ones provided in the header
	if !bytes.Equal(header.MixDigest[:], digest) {
		return errInvalidMixDigest
	}
	target := new(big.Int).Div(two256, header.Difficulty)
	if new(big.Int).SetBytes(result).Cmp(target) > 0 {
		return errInvalidPoW
	}
	return nil
}

// Prepare implements consensus.Engine, initializing the difficulty field of a
// header to conform to the ethash protocol. The changes are done inline.
func (ethash *Ethash) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Difficulty = ethash.CalcDifficulty(chain, header.Time, parent)
	return nil
}

// calculateBlockReward returns the block reward (in wei) for the given block number,
// according to the super epoch schedule, or zero if the supply cap is reached.
// (Assumes that totalSupply includes the premine.)
func calculateBlockReward(blockNumber uint64, totalSupply *big.Int) *big.Int {
	// If total supply is at or above the cap, no new block reward is issued.
	if totalSupply.Cmp(SupplyCap) >= 0 {
		return big.NewInt(0)
	}

	// Define 1 R5 in wei.
	oneR5 := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18))
	var reward *big.Int

	switch {
	case blockNumber <= 4000000:
		// Super Epoch 1: 2 R5 per block.
		reward = new(big.Int).Mul(big.NewInt(2), oneR5)
	case blockNumber <= 8000000:
		// Super Epoch 2: 1 R5 per block.
		reward = new(big.Int).Set(oneR5)
	case blockNumber <= 16000000:
		// Super Epoch 3: 0.5 R5 per block.
		reward = new(big.Int).Div(oneR5, big.NewInt(2))
	case blockNumber <= 32000000:
		// Super Epoch 4: 0.25 R5 per block.
		reward = new(big.Int).Div(oneR5, big.NewInt(4))
	case blockNumber <= 64000000:
		// Super Epoch 5: 0.125 R5 per block.
		reward = new(big.Int).Div(oneR5, big.NewInt(8))
	case blockNumber <= 128000000:
		// Super Epoch 6: 0.0625 R5 per block.
		reward = new(big.Int).Div(oneR5, big.NewInt(16))
	default:
		// Super Epoch 7: 0.03125 R5 per block.
		reward = new(big.Int).Div(oneR5, big.NewInt(32))
	}
	return reward
}

// Finalize implements consensus.Engine, accumulating the block and uncle rewards.
func (ethash *Ethash) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, withdrawals []*types.Withdrawal) {
	// Accumulate any block and uncle rewards
	accumulateRewards(chain.Config(), state, header, uncles)
}

// FinalizeAndAssemble finalizes the block by applying rewards and fee distribution.
// Transaction fees are diverted to the feePoolWallet (the global variable) if the
// total supply is below the cap; otherwise, fees go to the miner.
func (ethash *Ethash) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB,
	txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt, withdrawals []*types.Withdrawal) (*types.Block, error) {

	if len(withdrawals) > 0 {
		return nil, errors.New("ethash does not support withdrawals")
	}
	// Finalize block rewards and uncle handling.
	ethash.Finalize(chain, header, state, txs, uncles, nil)

	// Fee distribution:
	// Compute fees as GasUsed * BaseFee (if BaseFee is set, else zero).
	feeAmount := big.NewInt(0)
	if header.BaseFee != nil {
		feeAmount = new(big.Int).Mul(big.NewInt(int64(header.GasUsed)), header.BaseFee)
	}
	// Retrieve current total supply based on the block number.
	totalSupply := CalculateCirculatingSupply(header.Number.Uint64())
	// If total supply is below the cap, divert fees to the feePoolWallet.
	if totalSupply.Cmp(SupplyCap) < 0 {
		state.AddBalance(feePoolWallet, feeAmount)
	} else {
		// Otherwise, fees go to the miner.
		state.AddBalance(header.Coinbase, feeAmount)
	}

	// Assign the final state root to header.
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
	// Assemble the block.
	return types.NewBlock(header, txs, uncles, receipts, trie.NewStackTrie(nil)), nil
}

// SealHash returns the hash of a block prior to it being sealed.
func (ethash *Ethash) SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	if header.WithdrawalsHash != nil {
		panic("withdrawal hash set on ethash")
	}
	rlp.Encode(hasher, enc)
	hasher.Sum(hash[:0])
	return hash
}

// Some weird constants to avoid constant memory allocs for them.
var (
//	big8  = big.NewInt(8) // not used in new r5 consensus
//	big32 = big.NewInt(32) // not used in new r5 consensus
)

// In accumulateRewards, replace the call to state.GetTotalSupply() with CalculateCirculatingSupply.
// (Assuming your state does not provide a GetTotalSupply method.)
func accumulateRewards(config *params.ChainConfig, state *state.StateDB, header *types.Header, uncles []*types.Header) {
	// Calculate current total supply from the block number.
	totalSupply := CalculateCirculatingSupply(header.Number.Uint64())

	// Calculate the block reward for the current block.
	reward := calculateBlockReward(header.Number.Uint64(), totalSupply)

	// Credit the block reward to the miner's balance.
	state.AddBalance(header.Coinbase, reward)

	// Since uncle rewards are eliminated, we do nothing for uncles.
}
