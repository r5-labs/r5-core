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

package core

import (
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/consensus"
	"github.com/r5-labs/r5-core/client/consensus/beacon"
	"github.com/r5-labs/r5-core/client/consensus/clique"
	"github.com/r5-labs/r5-core/client/consensus/ethash"
	"github.com/r5-labs/r5-core/client/core/rawdb"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/core/vm"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/params"
)

// Tests that simple header verification works, for both good and bad blocks.
func TestHeaderVerification(t *testing.T) {
	// Create a simple chain to verify
	var (
		gspec        = &Genesis{Config: params.TestChainConfig}
		_, blocks, _ = GenerateChainWithGenesis(gspec, ethash.NewFaker(), 8, nil)
	)
	headers := make([]*types.Header, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}
	// Run the header checker for blocks one-by-one, checking for both valid and invalid nonces
	chain, _ := NewBlockChain(rawdb.NewMemoryDatabase(), nil, gspec, nil, ethash.NewFaker(), vm.Config{}, nil, nil)
	defer chain.Stop()

	for i := 0; i < len(blocks); i++ {
		for j, valid := range []bool{true, false} {
			var results <-chan error

			if valid {
				engine := ethash.NewFaker()
				_, results = engine.VerifyHeaders(chain, []*types.Header{headers[i]}, []bool{true})
			} else {
				engine := ethash.NewFakeFailer(headers[i].Number.Uint64())
				_, results = engine.VerifyHeaders(chain, []*types.Header{headers[i]}, []bool{true})
			}
			// Wait for the verification result
			select {
			case result := <-results:
				if (result == nil) != valid {
					t.Errorf("test %d.%d: validity mismatch: have %v, want %v", i, j, result, valid)
				}
			case <-time.After(time.Second):
				t.Fatalf("test %d.%d: verification timeout", i, j)
			}
			// Make sure no more data is returned
			select {
			case result := <-results:
				t.Fatalf("test %d.%d: unexpected result returned: %v", i, j, result)
			case <-time.After(25 * time.Millisecond):
			}
		}
		chain.InsertChain(blocks[i : i+1])
	}
}

func TestHeaderVerificationForMergingClique(t *testing.T) { testHeaderVerificationForMerging(t, true) }
func TestHeaderVerificationForMergingEthash(t *testing.T) { testHeaderVerificationForMerging(t, false) }

// Tests the verification for eth1/2 merging, including pre-merge and post-merge
func testHeaderVerificationForMerging(t *testing.T, isClique bool) {
	var (
		gspec      *Genesis
		preBlocks  []*types.Block
		postBlocks []*types.Block
		engine     consensus.Engine
		merger     = consensus.NewMerger(rawdb.NewMemoryDatabase())
	)
	if isClique {
		var (
			key, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
			addr   = crypto.PubkeyToAddress(key.PublicKey)
			config = *params.AllCliqueProtocolChanges
		)
		engine = beacon.New(clique.New(params.AllCliqueProtocolChanges.Clique, rawdb.NewMemoryDatabase()))
		gspec = &Genesis{
			Config:    &config,
			ExtraData: make([]byte, 32+common.AddressLength+crypto.SignatureLength),
			Alloc: map[common.Address]GenesisAccount{
				addr: {Balance: big.NewInt(1)},
			},
			BaseFee:    big.NewInt(params.InitialBaseFee),
			Difficulty: new(big.Int),
		}
		copy(gspec.ExtraData[32:], addr[:])

		td := 0
		genDb, blocks, _ := GenerateChainWithGenesis(gspec, engine, 8, nil)
		for i, block := range blocks {
			header := block.Header()
			if i > 0 {
				header.ParentHash = blocks[i-1].Hash()
			}
			header.Extra = make([]byte, 32+crypto.SignatureLength)
			header.Difficulty = big.NewInt(2)

			sig, _ := crypto.Sign(engine.SealHash(header).Bytes(), key)
			copy(header.Extra[len(header.Extra)-crypto.SignatureLength:], sig)
			blocks[i] = block.WithSeal(header)

			// calculate td
			td += int(block.Difficulty().Uint64())
		}
		preBlocks = blocks
		gspec.Config.TerminalTotalDifficulty = big.NewInt(int64(td))
		postBlocks, _ = GenerateChain(gspec.Config, preBlocks[len(preBlocks)-1], engine, genDb, 8, nil)
	} else {
		config := *params.TestChainConfig
		gspec = &Genesis{Config: &config}
		engine = beacon.New(ethash.NewFaker())
		td := int(params.GenesisDifficulty.Uint64())
		genDb, blocks, _ := GenerateChainWithGenesis(gspec, engine, 8, nil)
		for _, block := range blocks {
			// calculate td
			td += int(block.Difficulty().Uint64())
		}
		preBlocks = blocks
		gspec.Config.TerminalTotalDifficulty = big.NewInt(int64(td))
		t.Logf("Set ttd to %v\n", gspec.Config.TerminalTotalDifficulty)
		postBlocks, _ = GenerateChain(gspec.Config, preBlocks[len(preBlocks)-1], engine, genDb, 8, func(i int, gen *BlockGen) {
			gen.SetPoS()
		})
	}
	// Assemble header batch
	preHeaders := make([]*types.Header, len(preBlocks))
	for i, block := range preBlocks {
		preHeaders[i] = block.Header()
		t.Logf("Pre-merge header: %d", block.NumberU64())
	}
	postHeaders := make([]*types.Header, len(postBlocks))
	for i, block := range postBlocks {
		postHeaders[i] = block.Header()
		t.Logf("Post-merge header: %d", block.NumberU64())
	}
	// Run the header checker for blocks one-by-one, checking for both valid and invalid nonces
	chain, _ := NewBlockChain(rawdb.NewMemoryDatabase(), nil, gspec, nil, engine, vm.Config{}, nil, nil)
	defer chain.Stop()

	// Verify the blocks before the merging
	for i := 0; i < len(preBlocks); i++ {
		_, results := engine.VerifyHeaders(chain, []*types.Header{preHeaders[i]}, []bool{true})
		// Wait for the verification result
		select {
		case result := <-results:
			if result != nil {
				t.Errorf("pre-block %d: verification failed %v", i, result)
			}
		case <-time.After(time.Second):
			t.Fatalf("pre-block %d: verification timeout", i)
		}
		// Make sure no more data is returned
		select {
		case result := <-results:
			t.Fatalf("pre-block %d: unexpected result returned: %v", i, result)
		case <-time.After(25 * time.Millisecond):
		}
		chain.InsertChain(preBlocks[i : i+1])
	}

	// Make the transition
	merger.ReachTTD()
	merger.FinalizePoS()

	// Verify the blocks after the merging
	for i := 0; i < len(postBlocks); i++ {
		_, results := engine.VerifyHeaders(chain, []*types.Header{postHeaders[i]}, []bool{true})
		// Wait for the verification result
		select {
		case result := <-results:
			if result != nil {
				t.Errorf("post-block %d: verification failed %v", i, result)
			}
		case <-time.After(time.Second):
			t.Fatalf("test %d: verification timeout", i)
		}
		// Make sure no more data is returned
		select {
		case result := <-results:
			t.Fatalf("post-block %d: unexpected result returned: %v", i, result)
		case <-time.After(25 * time.Millisecond):
		}
		chain.InsertBlockWithoutSetHead(postBlocks[i])
	}

	// Verify the blocks with pre-merge blocks and post-merge blocks
	var (
		headers []*types.Header
		seals   []bool
	)
	for _, block := range preBlocks {
		headers = append(headers, block.Header())
		seals = append(seals, true)
	}
	for _, block := range postBlocks {
		headers = append(headers, block.Header())
		seals = append(seals, true)
	}
	_, results := engine.VerifyHeaders(chain, headers, seals)
	for i := 0; i < len(headers); i++ {
		select {
		case result := <-results:
			if result != nil {
				t.Errorf("test %d: verification failed %v", i, result)
			}
		case <-time.After(time.Second):
			t.Fatalf("test %d: verification timeout", i)
		}
	}
	// Make sure no more data is returned
	select {
	case result := <-results:
		t.Fatalf("unexpected result returned: %v", result)
	case <-time.After(25 * time.Millisecond):
	}
}

// Tests that concurrent header verification works, for both good and bad blocks.
func TestHeaderConcurrentVerification2(t *testing.T)  { testHeaderConcurrentVerification(t, 2) }
func TestHeaderConcurrentVerification8(t *testing.T)  { testHeaderConcurrentVerification(t, 8) }
func TestHeaderConcurrentVerification32(t *testing.T) { testHeaderConcurrentVerification(t, 32) }

func testHeaderConcurrentVerification(t *testing.T, threads int) {
	// Create a simple chain to verify
	var (
		gspec        = &Genesis{Config: params.TestChainConfig}
		_, blocks, _ = GenerateChainWithGenesis(gspec, ethash.NewFaker(), 8, nil)
	)
	headers := make([]*types.Header, len(blocks))
	seals := make([]bool, len(blocks))

	for i, block := range blocks {
		headers[i] = block.Header()
		seals[i] = true
	}
	// Set the number of threads to verify on
	old := runtime.GOMAXPROCS(threads)
	defer runtime.GOMAXPROCS(old)

	// Run the header checker for the entire block chain at once both for a valid and
	// also an invalid chain (enough if one arbitrary block is invalid).
	for i, valid := range []bool{true, false} {
		var results <-chan error

		if valid {
			chain, _ := NewBlockChain(rawdb.NewMemoryDatabase(), nil, gspec, nil, ethash.NewFaker(), vm.Config{}, nil, nil)
			_, results = chain.engine.VerifyHeaders(chain, headers, seals)
			chain.Stop()
		} else {
			chain, _ := NewBlockChain(rawdb.NewMemoryDatabase(), nil, gspec, nil, ethash.NewFakeFailer(uint64(len(headers)-1)), vm.Config{}, nil, nil)
			_, results = chain.engine.VerifyHeaders(chain, headers, seals)
			chain.Stop()
		}
		// Wait for all the verification results
		checks := make(map[int]error)
		for j := 0; j < len(blocks); j++ {
			select {
			case result := <-results:
				checks[j] = result

			case <-time.After(time.Second):
				t.Fatalf("test %d.%d: verification timeout", i, j)
			}
		}
		// Check nonce check validity
		for j := 0; j < len(blocks); j++ {
			want := valid || (j < len(blocks)-2) // We chose the last-but-one nonce in the chain to fail
			if (checks[j] == nil) != want {
				t.Errorf("test %d.%d: validity mismatch: have %v, want %v", i, j, checks[j], want)
			}
			if !want {
				// A few blocks after the first error may pass verification due to concurrent
				// workers. We don't care about those in this test, just that the correct block
				// errors out.
				break
			}
		}
		// Make sure no more data is returned
		select {
		case result := <-results:
			t.Fatalf("test %d: unexpected result returned: %v", i, result)
		case <-time.After(25 * time.Millisecond):
		}
	}
}

// Tests that aborting a header validation indeed prevents further checks from being
// run, as well as checks that no left-over goroutines are leaked.
func TestHeaderConcurrentAbortion2(t *testing.T)  { testHeaderConcurrentAbortion(t, 2) }
func TestHeaderConcurrentAbortion8(t *testing.T)  { testHeaderConcurrentAbortion(t, 8) }
func TestHeaderConcurrentAbortion32(t *testing.T) { testHeaderConcurrentAbortion(t, 32) }

func testHeaderConcurrentAbortion(t *testing.T, threads int) {
	// Create a simple chain to verify
	var (
		gspec        = &Genesis{Config: params.TestChainConfig}
		_, blocks, _ = GenerateChainWithGenesis(gspec, ethash.NewFaker(), 1024, nil)
	)
	headers := make([]*types.Header, len(blocks))
	seals := make([]bool, len(blocks))

	for i, block := range blocks {
		headers[i] = block.Header()
		seals[i] = true
	}
	// Set the number of threads to verify on
	old := runtime.GOMAXPROCS(threads)
	defer runtime.GOMAXPROCS(old)

	// Start the verifications and immediately abort
	chain, _ := NewBlockChain(rawdb.NewMemoryDatabase(), nil, gspec, nil, ethash.NewFakeDelayer(time.Millisecond), vm.Config{}, nil, nil)
	defer chain.Stop()

	abort, results := chain.engine.VerifyHeaders(chain, headers, seals)
	close(abort)

	// Deplete the results channel
	verified := 0
	for depleted := false; !depleted; {
		select {
		case result := <-results:
			if result != nil {
				t.Errorf("header %d: validation failed: %v", verified, result)
			}
			verified++
		case <-time.After(50 * time.Millisecond):
			depleted = true
		}
	}
	// Check that abortion was honored by not processing too many POWs
	if verified > 2*threads {
		t.Errorf("verification count too large: have %d, want below %d", verified, 2*threads)
	}
}

func TestCalcGasLimit(t *testing.T) {
	for i, tc := range []struct {
		pGasLimit uint64
		max       uint64
		min       uint64
	}{
		{20000000, 20019530, 19980470},
		{40000000, 40039061, 39960939},
	} {
		// Increase
		if have, want := CalcGasLimit(tc.pGasLimit, 2*tc.pGasLimit), tc.max; have != want {
			t.Errorf("test %d: have %d want <%d", i, have, want)
		}
		// Decrease
		if have, want := CalcGasLimit(tc.pGasLimit, 0), tc.min; have != want {
			t.Errorf("test %d: have %d want >%d", i, have, want)
		}
		// Small decrease
		if have, want := CalcGasLimit(tc.pGasLimit, tc.pGasLimit-1), tc.pGasLimit-1; have != want {
			t.Errorf("test %d: have %d want %d", i, have, want)
		}
		// Small increase
		if have, want := CalcGasLimit(tc.pGasLimit, tc.pGasLimit+1), tc.pGasLimit+1; have != want {
			t.Errorf("test %d: have %d want %d", i, have, want)
		}
		// No change
		if have, want := CalcGasLimit(tc.pGasLimit, tc.pGasLimit), tc.pGasLimit; have != want {
			t.Errorf("test %d: have %d want %d", i, have, want)
		}
	}
}
