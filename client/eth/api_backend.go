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
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/r5-labs/r5-core/client"
	"github.com/r5-labs/r5-core/client/accounts"
	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/consensus"
	"github.com/r5-labs/r5-core/client/core"
	"github.com/r5-labs/r5-core/client/core/bloombits"
	"github.com/r5-labs/r5-core/client/core/rawdb"
	"github.com/r5-labs/r5-core/client/core/state"
	"github.com/r5-labs/r5-core/client/core/txpool"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/core/vm"
	"github.com/r5-labs/r5-core/client/eth/gasprice"
	"github.com/r5-labs/r5-core/client/eth/tracers"
	"github.com/r5-labs/r5-core/client/ethdb"
	"github.com/r5-labs/r5-core/client/event"
	"github.com/r5-labs/r5-core/client/miner"
	"github.com/r5-labs/r5-core/client/params"
	"github.com/r5-labs/r5-core/client/rpc"
)

// EthAPIBackend implements ethapi.Backend for full nodes
type EthAPIBackend struct {
	extRPCEnabled       bool
	allowUnprotectedTxs bool
	eth                 *Ethereum
	gpo                 *gasprice.Oracle
}

// ChainConfig returns the active chain configuration.
func (b *EthAPIBackend) ChainConfig() *params.ChainConfig {
	return b.eth.blockchain.Config()
}

func (b *EthAPIBackend) CurrentBlock() *types.Header {
	return b.eth.blockchain.CurrentBlock()
}

func (b *EthAPIBackend) SetHead(number uint64) {
	b.eth.handler.downloader.Cancel()
	b.eth.blockchain.SetHead(number)
}

func (b *EthAPIBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block := b.eth.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.eth.blockchain.CurrentBlock(), nil
	}
	if number == rpc.FinalizedBlockNumber {
		if !b.eth.Merger().TDDReached() {
			return nil, errors.New("'finalized' tag not supported on pre-merge network")
		}
		block := b.eth.blockchain.CurrentFinalBlock()
		if block != nil {
			return block, nil
		}
		return nil, errors.New("finalized block not found")
	}
	if number == rpc.SafeBlockNumber {
		if !b.eth.Merger().TDDReached() {
			return nil, errors.New("'safe' tag not supported on pre-merge network")
		}
		block := b.eth.blockchain.CurrentSafeBlock()
		if block != nil {
			return block, nil
		}
		return nil, errors.New("safe block not found")
	}
	return b.eth.blockchain.GetHeaderByNumber(uint64(number)), nil
}

func (b *EthAPIBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.HeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.eth.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.eth.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		return header, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *EthAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.eth.blockchain.GetHeaderByHash(hash), nil
}

func (b *EthAPIBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block := b.eth.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		header := b.eth.blockchain.CurrentBlock()
		return b.eth.blockchain.GetBlock(header.Hash(), header.Number.Uint64()), nil
	}
	if number == rpc.FinalizedBlockNumber {
		if !b.eth.Merger().TDDReached() {
			return nil, errors.New("'finalized' tag not supported on pre-merge network")
		}
		header := b.eth.blockchain.CurrentFinalBlock()
		return b.eth.blockchain.GetBlock(header.Hash(), header.Number.Uint64()), nil
	}
	if number == rpc.SafeBlockNumber {
		if !b.eth.Merger().TDDReached() {
			return nil, errors.New("'safe' tag not supported on pre-merge network")
		}
		header := b.eth.blockchain.CurrentSafeBlock()
		return b.eth.blockchain.GetBlock(header.Hash(), header.Number.Uint64()), nil
	}
	return b.eth.blockchain.GetBlockByNumber(uint64(number)), nil
}

func (b *EthAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.eth.blockchain.GetBlockByHash(hash), nil
}

// GetBody returns body of a block. It does not resolve special block numbers.
func (b *EthAPIBackend) GetBody(ctx context.Context, hash common.Hash, number rpc.BlockNumber) (*types.Body, error) {
	if number < 0 || hash == (common.Hash{}) {
		return nil, errors.New("invalid arguments; expect hash and no special block numbers")
	}
	if body := b.eth.blockchain.GetBody(hash); body != nil {
		return body, nil
	}
	return nil, errors.New("block body not found")
}

func (b *EthAPIBackend) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.eth.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.eth.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := b.eth.blockchain.GetBlock(hash, header.Number.Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *EthAPIBackend) PendingBlockAndReceipts() (*types.Block, types.Receipts) {
	return b.eth.miner.PendingBlockAndReceipts()
}

func (b *EthAPIBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if number == rpc.PendingBlockNumber {
		block, state := b.eth.miner.Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, number)
	if err != nil {
		return nil, nil, err
	}
	if header == nil {
		return nil, nil, errors.New("header not found")
	}
	stateDb, err := b.eth.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *EthAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, err := b.HeaderByHash(ctx, hash)
		if err != nil {
			return nil, nil, err
		}
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.eth.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := b.eth.BlockChain().StateAt(header.Root)
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *EthAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.eth.blockchain.GetReceiptsByHash(hash), nil
}

func (b *EthAPIBackend) GetLogs(ctx context.Context, hash common.Hash, number uint64) ([][]*types.Log, error) {
	return rawdb.ReadLogs(b.eth.chainDb, hash, number, b.ChainConfig()), nil
}

func (b *EthAPIBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int {
	if header := b.eth.blockchain.GetHeaderByHash(hash); header != nil {
		return b.eth.blockchain.GetTd(hash, header.Number.Uint64())
	}
	return nil
}

func (b *EthAPIBackend) GetEVM(ctx context.Context, msg *core.Message, state *state.StateDB, header *types.Header, vmConfig *vm.Config) (*vm.EVM, func() error, error) {
	if vmConfig == nil {
		vmConfig = b.eth.blockchain.GetVMConfig()
	}
	txContext := core.NewEVMTxContext(msg)
	context := core.NewEVMBlockContext(header, b.eth.BlockChain(), nil)
	return vm.NewEVM(context, txContext, state, b.eth.blockchain.Config(), *vmConfig), state.Error, nil
}

func (b *EthAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *EthAPIBackend) SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.eth.miner.SubscribePendingLogs(ch)
}

func (b *EthAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeChainEvent(ch)
}

func (b *EthAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *EthAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *EthAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.eth.BlockChain().SubscribeLogsEvent(ch)
}

func (b *EthAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.eth.txPool.AddLocal(signedTx)
}

func (b *EthAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending := b.eth.txPool.Pending(false)
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *EthAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.eth.txPool.Get(hash)
}

func (b *EthAPIBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(b.eth.ChainDb(), txHash)
	return tx, blockHash, blockNumber, index, nil
}

func (b *EthAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.eth.txPool.Nonce(addr), nil
}

func (b *EthAPIBackend) Stats() (pending int, queued int) {
	return b.eth.txPool.Stats()
}

func (b *EthAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.eth.TxPool().Content()
}

func (b *EthAPIBackend) TxPoolContentFrom(addr common.Address) (types.Transactions, types.Transactions) {
	return b.eth.TxPool().ContentFrom(addr)
}

func (b *EthAPIBackend) TxPool() *txpool.TxPool {
	return b.eth.TxPool()
}

func (b *EthAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.eth.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *EthAPIBackend) SyncProgress() ethereum.SyncProgress {
	return b.eth.Downloader().Progress()
}

func (b *EthAPIBackend) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestTipCap(ctx)
}

func (b *EthAPIBackend) FeeHistory(ctx context.Context, blockCount uint64, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (firstBlock *big.Int, reward [][]*big.Int, baseFee []*big.Int, gasUsedRatio []float64, err error) {
	return b.gpo.FeeHistory(ctx, blockCount, lastBlock, rewardPercentiles)
}

func (b *EthAPIBackend) ChainDb() ethdb.Database {
	return b.eth.ChainDb()
}

func (b *EthAPIBackend) EventMux() *event.TypeMux {
	return b.eth.EventMux()
}

func (b *EthAPIBackend) AccountManager() *accounts.Manager {
	return b.eth.AccountManager()
}

func (b *EthAPIBackend) ExtRPCEnabled() bool {
	return b.extRPCEnabled
}

func (b *EthAPIBackend) UnprotectedAllowed() bool {
	return b.allowUnprotectedTxs
}

func (b *EthAPIBackend) RPCGasCap() uint64 {
	return b.eth.config.RPCGasCap
}

func (b *EthAPIBackend) RPCEVMTimeout() time.Duration {
	return b.eth.config.RPCEVMTimeout
}

func (b *EthAPIBackend) RPCTxFeeCap() float64 {
	return b.eth.config.RPCTxFeeCap
}

func (b *EthAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.eth.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *EthAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.eth.bloomRequests)
	}
}

func (b *EthAPIBackend) Engine() consensus.Engine {
	return b.eth.engine
}

func (b *EthAPIBackend) CurrentHeader() *types.Header {
	return b.eth.blockchain.CurrentHeader()
}

func (b *EthAPIBackend) Miner() *miner.Miner {
	return b.eth.Miner()
}

func (b *EthAPIBackend) StartMining(threads int) error {
	return b.eth.StartMining(threads)
}

func (b *EthAPIBackend) StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, readOnly bool, preferDisk bool) (*state.StateDB, tracers.StateReleaseFunc, error) {
	return b.eth.StateAtBlock(ctx, block, reexec, base, readOnly, preferDisk)
}

func (b *EthAPIBackend) StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (*core.Message, vm.BlockContext, *state.StateDB, tracers.StateReleaseFunc, error) {
	return b.eth.stateAtTransaction(ctx, block, txIndex, reexec)
}
