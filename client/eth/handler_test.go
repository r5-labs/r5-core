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

package eth

import (
	"math/big"
	"sort"
	"sync"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/consensus"
	"github.com/r5-labs/r5-core/client/consensus/ethash"
	"github.com/r5-labs/r5-core/client/core"
	"github.com/r5-labs/r5-core/client/core/rawdb"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/core/vm"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/eth/downloader"
	"github.com/r5-labs/r5-core/client/ethdb"
	"github.com/r5-labs/r5-core/client/event"
	"github.com/r5-labs/r5-core/client/params"
)

var (
	// testKey is a private key to use for funding a tester account.
	testKey, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")

	// testAddr is the Ethereum address of the tester account.
	testAddr = crypto.PubkeyToAddress(testKey.PublicKey)
)

// testTxPool is a mock transaction pool that blindly accepts all transactions.
// Its goal is to get around setting up a valid statedb for the balance and nonce
// checks.
type testTxPool struct {
	pool map[common.Hash]*types.Transaction // Hash map of collected transactions

	txFeed event.Feed   // Notification feed to allow waiting for inclusion
	lock   sync.RWMutex // Protects the transaction pool
}

// newTestTxPool creates a mock transaction pool.
func newTestTxPool() *testTxPool {
	return &testTxPool{
		pool: make(map[common.Hash]*types.Transaction),
	}
}

// Has returns an indicator whether txpool has a transaction
// cached with the given hash.
func (p *testTxPool) Has(hash common.Hash) bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.pool[hash] != nil
}

// Get retrieves the transaction from local txpool with given
// tx hash.
func (p *testTxPool) Get(hash common.Hash) *types.Transaction {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.pool[hash]
}

// AddRemotes appends a batch of transactions to the pool, and notifies any
// listeners if the addition channel is non nil
func (p *testTxPool) AddRemotes(txs []*types.Transaction) []error {
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, tx := range txs {
		p.pool[tx.Hash()] = tx
	}
	p.txFeed.Send(core.NewTxsEvent{Txs: txs})
	return make([]error, len(txs))
}

// Pending returns all the transactions known to the pool
func (p *testTxPool) Pending(enforceTips bool) map[common.Address]types.Transactions {
	p.lock.RLock()
	defer p.lock.RUnlock()

	batches := make(map[common.Address]types.Transactions)
	for _, tx := range p.pool {
		from, _ := types.Sender(types.HomesteadSigner{}, tx)
		batches[from] = append(batches[from], tx)
	}
	for _, batch := range batches {
		sort.Sort(types.TxByNonce(batch))
	}
	return batches
}

// SubscribeNewTxsEvent should return an event subscription of NewTxsEvent and
// send events to the given channel.
func (p *testTxPool) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return p.txFeed.Subscribe(ch)
}

// testHandler is a live implementation of the Ethereum protocol handler, just
// preinitialized with some sane testing defaults and the transaction pool mocked
// out.
type testHandler struct {
	db      ethdb.Database
	chain   *core.BlockChain
	txpool  *testTxPool
	handler *handler
}

// newTestHandler creates a new handler for testing purposes with no blocks.
func newTestHandler() *testHandler {
	return newTestHandlerWithBlocks(0)
}

// newTestHandlerWithBlocks creates a new handler for testing purposes, with a
// given number of initial blocks.
func newTestHandlerWithBlocks(blocks int) *testHandler {
	// Create a database pre-initialize with a genesis block
	db := rawdb.NewMemoryDatabase()
	gspec := &core.Genesis{
		Config: params.TestChainConfig,
		Alloc:  core.GenesisAlloc{testAddr: {Balance: big.NewInt(1000000)}},
	}
	chain, _ := core.NewBlockChain(db, nil, gspec, nil, ethash.NewFaker(), vm.Config{}, nil, nil)

	_, bs, _ := core.GenerateChainWithGenesis(gspec, ethash.NewFaker(), blocks, nil)
	if _, err := chain.InsertChain(bs); err != nil {
		panic(err)
	}
	txpool := newTestTxPool()

	handler, _ := newHandler(&handlerConfig{
		Database:   db,
		Chain:      chain,
		TxPool:     txpool,
		Merger:     consensus.NewMerger(rawdb.NewMemoryDatabase()),
		Network:    1,
		Sync:       downloader.SnapSync,
		BloomCache: 1,
	})
	handler.Start(1000)

	return &testHandler{
		db:      db,
		chain:   chain,
		txpool:  txpool,
		handler: handler,
	}
}

// close tears down the handler and all its internal constructs.
func (b *testHandler) close() {
	b.handler.Stop()
	b.chain.Stop()
}
