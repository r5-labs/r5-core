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

package gasprice

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/consensus/ethash"
	"github.com/r5-labs/r5-core/client/core"
	"github.com/r5-labs/r5-core/client/core/rawdb"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/core/vm"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/event"
	"github.com/r5-labs/r5-core/client/params"
	"github.com/r5-labs/r5-core/client/rpc"
)

const testHead = 32

type testBackend struct {
	chain   *core.BlockChain
	pending bool // pending block available
}

func (b *testBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	if number > testHead {
		return nil, nil
	}
	if number == rpc.EarliestBlockNumber {
		number = 0
	}
	if number == rpc.FinalizedBlockNumber {
		return b.chain.CurrentFinalBlock(), nil
	}
	if number == rpc.SafeBlockNumber {
		return b.chain.CurrentSafeBlock(), nil
	}
	if number == rpc.LatestBlockNumber {
		number = testHead
	}
	if number == rpc.PendingBlockNumber {
		if b.pending {
			number = testHead + 1
		} else {
			return nil, nil
		}
	}
	return b.chain.GetHeaderByNumber(uint64(number)), nil
}

func (b *testBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	if number > testHead {
		return nil, nil
	}
	if number == rpc.EarliestBlockNumber {
		number = 0
	}
	if number == rpc.FinalizedBlockNumber {
		number = rpc.BlockNumber(b.chain.CurrentFinalBlock().Number.Uint64())
	}
	if number == rpc.SafeBlockNumber {
		number = rpc.BlockNumber(b.chain.CurrentSafeBlock().Number.Uint64())
	}
	if number == rpc.LatestBlockNumber {
		number = testHead
	}
	if number == rpc.PendingBlockNumber {
		if b.pending {
			number = testHead + 1
		} else {
			return nil, nil
		}
	}
	return b.chain.GetBlockByNumber(uint64(number)), nil
}

func (b *testBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.chain.GetReceiptsByHash(hash), nil
}

func (b *testBackend) PendingBlockAndReceipts() (*types.Block, types.Receipts) {
	if b.pending {
		block := b.chain.GetBlockByNumber(testHead + 1)
		return block, b.chain.GetReceiptsByHash(block.Hash())
	}
	return nil, nil
}

func (b *testBackend) ChainConfig() *params.ChainConfig {
	return b.chain.Config()
}

func (b *testBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return nil
}

func (b *testBackend) teardown() {
	b.chain.Stop()
}

// newTestBackend creates a test backend. OBS: don't forget to invoke tearDown
// after use, otherwise the blockchain instance will mem-leak via goroutines.
func newTestBackend(t *testing.T, londonBlock *big.Int, pending bool) *testBackend {
	var (
		key, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		addr   = crypto.PubkeyToAddress(key.PublicKey)
		config = *params.TestChainConfig // needs copy because it is modified below
		gspec  = &core.Genesis{
			Config: &config,
			Alloc:  core.GenesisAlloc{addr: {Balance: big.NewInt(math.MaxInt64)}},
		}
		signer = types.LatestSigner(gspec.Config)
	)
	config.LondonBlock = londonBlock
	config.ArrowGlacierBlock = londonBlock
	config.GrayGlacierBlock = londonBlock
	config.TerminalTotalDifficulty = common.Big0
	engine := ethash.NewFaker()

	// Generate testing blocks
	_, blocks, _ := core.GenerateChainWithGenesis(gspec, engine, testHead+1, func(i int, b *core.BlockGen) {
		b.SetCoinbase(common.Address{1})

		var txdata types.TxData
		if londonBlock != nil && b.Number().Cmp(londonBlock) >= 0 {
			txdata = &types.DynamicFeeTx{
				ChainID:   gspec.Config.ChainID,
				Nonce:     b.TxNonce(addr),
				To:        &common.Address{},
				Gas:       30000,
				GasFeeCap: big.NewInt(100 * params.GWei),
				GasTipCap: big.NewInt(int64(i+1) * params.GWei),
				Data:      []byte{},
			}
		} else {
			txdata = &types.LegacyTx{
				Nonce:    b.TxNonce(addr),
				To:       &common.Address{},
				Gas:      21000,
				GasPrice: big.NewInt(int64(i+1) * params.GWei),
				Value:    big.NewInt(100),
				Data:     []byte{},
			}
		}
		b.AddTx(types.MustSignNewTx(key, signer, txdata))
	})
	// Construct testing chain
	chain, err := core.NewBlockChain(rawdb.NewMemoryDatabase(), &core.CacheConfig{TrieCleanNoPrefetch: true}, gspec, nil, engine, vm.Config{}, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create local chain, %v", err)
	}
	chain.InsertChain(blocks)
	chain.SetFinalized(chain.GetBlockByNumber(25).Header())
	chain.SetSafe(chain.GetBlockByNumber(25).Header())
	return &testBackend{chain: chain, pending: pending}
}

func (b *testBackend) CurrentHeader() *types.Header {
	return b.chain.CurrentHeader()
}

func (b *testBackend) GetBlockByNumber(number uint64) *types.Block {
	return b.chain.GetBlockByNumber(number)
}

func TestSuggestTipCap(t *testing.T) {
	config := Config{
		Blocks:     3,
		Percentile: 60,
		Default:    big.NewInt(params.GWei),
	}
	var cases = []struct {
		fork   *big.Int // London fork number
		expect *big.Int // Expected gasprice suggestion
	}{
		{nil, big.NewInt(params.GWei * int64(30))},
		{big.NewInt(0), big.NewInt(params.GWei * int64(30))},  // Fork point in genesis
		{big.NewInt(1), big.NewInt(params.GWei * int64(30))},  // Fork point in first block
		{big.NewInt(32), big.NewInt(params.GWei * int64(30))}, // Fork point in last block
		{big.NewInt(33), big.NewInt(params.GWei * int64(30))}, // Fork point in the future
	}
	for _, c := range cases {
		backend := newTestBackend(t, c.fork, false)
		oracle := NewOracle(backend, config)

		// The gas price sampled is: 32G, 31G, 30G, 29G, 28G, 27G
		got, err := oracle.SuggestTipCap(context.Background())
		backend.teardown()
		if err != nil {
			t.Fatalf("Failed to retrieve recommended gas price: %v", err)
		}
		if got.Cmp(c.expect) != 0 {
			t.Fatalf("Gas price mismatch, want %d, got %d", c.expect, got)
		}
	}
}
