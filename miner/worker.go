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

package miner

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/r5-codebase/r5-core/common"
	"github.com/r5-codebase/r5-core/consensus"
	"github.com/r5-codebase/r5-core/consensus/misc"
	"github.com/r5-codebase/r5-core/core"
	"github.com/r5-codebase/r5-core/core/state"
	"github.com/r5-codebase/r5-core/core/types"
	"github.com/r5-codebase/r5-core/event"
	"github.com/r5-codebase/r5-core/log"
	"github.com/r5-codebase/r5-core/params"
	"github.com/r5-codebase/r5-core/trie"
)

const (
	resultQueueSize          = 10
	txChanSize               = 4096
	chainHeadChanSize        = 10
	chainSideChanSize        = 10
	resubmitAdjustChanSize   = 10
	sealingLogAtDepth        = 7
	minRecommitInterval      = 1 * time.Second
	maxRecommitInterval      = 15 * time.Second
	intervalAdjustRatio      = 0.1
	intervalAdjustBias       = 200 * 1000.0 * 1000.0
	staleThreshold           = 7
)

var (
	errBlockInterruptedByNewHead  = errors.New("new head arrived while building block")
	errBlockInterruptedByRecommit = errors.New("recommit interrupt while building block")
	errBlockInterruptedByTimeout  = errors.New("timeout while building block")
)

type environment struct {
	signer    types.Signer
	state     *state.StateDB
	ancestors mapset.Set[common.Hash]
	family    mapset.Set[common.Hash]
	tcount    int
	gasPool   *core.GasPool
	coinbase  common.Address

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt
	uncles   map[common.Hash]*types.Header
}

func (env *environment) copy() *environment {
	cpy := &environment{
		signer:    env.signer,
		state:     env.state.Copy(),
		ancestors: env.ancestors.Clone(),
		family:    env.family.Clone(),
		tcount:    env.tcount,
		coinbase:  env.coinbase,
		header:    types.CopyHeader(env.header),
		receipts:  copyReceipts(env.receipts),
	}
	if env.gasPool != nil {
		gasPool := *env.gasPool
		cpy.gasPool = &gasPool
	}
	cpy.txs = make([]*types.Transaction, len(env.txs))
	copy(cpy.txs, env.txs)
	cpy.uncles = make(map[common.Hash]*types.Header)
	for hash, uncle := range env.uncles {
		cpy.uncles[hash] = uncle
	}
	return cpy
}

func (env *environment) unclelist() []*types.Header {
	var uncles []*types.Header
	for _, uncle := range env.uncles {
		uncles = append(uncles, uncle)
	}
	return uncles
}

func (env *environment) discard() {
	if env.state == nil {
		return
	}
	env.state.StopPrefetcher()
}

type task struct {
	receipts  []*types.Receipt
	state     *state.StateDB
	block     *types.Block
	createdAt time.Time
}

const (
	commitInterruptNone int32 = iota
	commitInterruptNewHead
	commitInterruptResubmit
	commitInterruptTimeout
)

type newWorkReq struct {
	interrupt *atomic.Int32
	noempty   bool
	timestamp int64
}

type newPayloadResult struct {
	err   error
	block *types.Block
	fees  *big.Int
}

type getWorkReq struct {
	params *generateParams
	result chan *newPayloadResult
}

type intervalAdjust struct {
	ratio float64
	inc   bool
}

type worker struct {
	config      *Config
	chainConfig *params.ChainConfig
	engine      consensus.Engine
	eth         Backend
	chain       *core.BlockChain

	pendingLogsFeed event.Feed

	mux          *event.TypeMux
	txsCh        chan core.NewTxsEvent
	txsSub       event.Subscription
	chainHeadCh  chan core.ChainHeadEvent
	chainHeadSub event.Subscription
	chainSideCh  chan core.ChainSideEvent
	chainSideSub event.Subscription

	newWorkCh          chan *newWorkReq
	getWorkCh          chan *getWorkReq
	taskCh             chan *task
	resultCh           chan *types.Block
	startCh            chan struct{}
	exitCh             chan struct{}
	resubmitIntervalCh chan time.Duration
	resubmitAdjustCh   chan *intervalAdjust

	wg sync.WaitGroup

	current      *environment
	localUncles  map[common.Hash]*types.Block
	remoteUncles map[common.Hash]*types.Block
	unconfirmed  *unconfirmedBlocks

	mu       sync.RWMutex
	coinbase common.Address
	extra    []byte

	pendingMu    sync.RWMutex
	pendingTasks map[common.Hash]*task

	snapshotMu       sync.RWMutex
	snapshotBlock    *types.Block
	snapshotReceipts types.Receipts
	snapshotState    *state.StateDB

	running           atomic.Bool
	newTxs            atomic.Int32
	noempty           atomic.Bool
	newpayloadTimeout time.Duration
	recommit          time.Duration

	isLocalBlock func(header *types.Header) bool

	newTaskHook  func(*task)
	skipSealHook func(*task) bool
	fullTaskHook func()
	resubmitHook func(time.Duration, time.Duration)
}

func newWorker(config *Config, chainConfig *params.ChainConfig, engine consensus.Engine, eth Backend, mux *event.TypeMux, isLocalBlock func(header *types.Header) bool, init bool) *worker {
	worker := &worker{
		config:             config,
		chainConfig:        chainConfig,
		engine:             engine,
		eth:                eth,
		chain:              eth.BlockChain(),
		mux:                mux,
		isLocalBlock:       isLocalBlock,
		localUncles:        make(map[common.Hash]*types.Block),
		remoteUncles:       make(map[common.Hash]*types.Block),
		unconfirmed:        newUnconfirmedBlocks(eth.BlockChain(), sealingLogAtDepth),
		coinbase:           config.Etherbase,
		extra:              config.ExtraData,
		pendingTasks:       make(map[common.Hash]*task),
		txsCh:              make(chan core.NewTxsEvent, txChanSize),
		chainHeadCh:        make(chan core.ChainHeadEvent, chainHeadChanSize),
		chainSideCh:        make(chan core.ChainSideEvent, chainSideChanSize),
		newWorkCh:          make(chan *newWorkReq),
		getWorkCh:          make(chan *getWorkReq),
		taskCh:             make(chan *task),
		resultCh:           make(chan *types.Block, resultQueueSize),
		startCh:            make(chan struct{}, 1),
		exitCh:             make(chan struct{}),
		resubmitIntervalCh: make(chan time.Duration),
		resubmitAdjustCh:   make(chan *intervalAdjust, resubmitAdjustChanSize),
	}
	worker.txsSub = eth.TxPool().SubscribeNewTxsEvent(worker.txsCh)
	worker.chainHeadSub = eth.BlockChain().SubscribeChainHeadEvent(worker.chainHeadCh)
	worker.chainSideSub = eth.BlockChain().SubscribeChainSideEvent(worker.chainSideCh)

	recommit := worker.config.Recommit
	if recommit < minRecommitInterval {
		log.Warn("Sanitizing miner recommit interval", "provided", recommit, "updated", minRecommitInterval)
		recommit = minRecommitInterval
	}
	worker.recommit = recommit

	newpayloadTimeout := worker.config.NewPayloadTimeout
	if newpayloadTimeout == 0 {
		log.Warn("Sanitizing new payload timeout to default", "provided", newpayloadTimeout, "updated", DefaultConfig.NewPayloadTimeout)
		newpayloadTimeout = DefaultConfig.NewPayloadTimeout
	}
	if newpayloadTimeout < 100*time.Millisecond {
		log.Warn("Low payload timeout may cause high amount of non-full blocks", "provided", newpayloadTimeout, "default", DefaultConfig.NewPayloadTimeout)
	}
	worker.newpayloadTimeout = newpayloadTimeout

	worker.wg.Add(4)
	go worker.mainLoop()
	go worker.newWorkLoop(recommit)
	go worker.resultLoop()
	go worker.taskLoop()

	if init {
		worker.startCh <- struct{}{}
	}
	return worker
}

func (w *worker) setEtherbase(addr common.Address) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.coinbase = addr
}

func (w *worker) etherbase() common.Address {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.coinbase
}

func (w *worker) setGasCeil(ceil uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.config.GasCeil = ceil
}

func (w *worker) setExtra(extra []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.extra = extra
}

func (w *worker) setRecommitInterval(interval time.Duration) {
	select {
	case w.resubmitIntervalCh <- interval:
	case <-w.exitCh:
	}
}

func (w *worker) disablePreseal() {
	w.noempty.Store(true)
}

func (w *worker) enablePreseal() {
	w.noempty.Store(false)
}

func (w *worker) pending() (*types.Block, *state.StateDB) {
	w.snapshotMu.RLock()
	defer w.snapshotMu.RUnlock()
	if w.snapshotState == nil {
		return nil, nil
	}
	return w.snapshotBlock, w.snapshotState.Copy()
}

func (w *worker) pendingBlock() *types.Block {
	w.snapshotMu.RLock()
	defer w.snapshotMu.RUnlock()
	return w.snapshotBlock
}

func (w *worker) pendingBlockAndReceipts() (*types.Block, types.Receipts) {
	w.snapshotMu.RLock()
	defer w.snapshotMu.RUnlock()
	return w.snapshotBlock, w.snapshotReceipts
}

func (w *worker) start() {
	w.running.Store(true)
	w.startCh <- struct{}{}
}

func (w *worker) stop() {
	w.running.Store(false)
}

func (w *worker) isRunning() bool {
	return w.running.Load()
}

func (w *worker) close() {
	w.running.Store(false)
	close(w.exitCh)
	w.wg.Wait()
}

func recalcRecommit(minRecommit, prev time.Duration, target float64, inc bool) time.Duration {
	prevF := float64(prev.Nanoseconds())
	var next float64
	if inc {
		next = prevF*(1-intervalAdjustRatio) + intervalAdjustRatio*(target+intervalAdjustBias)
		max := float64(maxRecommitInterval.Nanoseconds())
		if next > max {
			next = max
		}
	} else {
		next = prevF*(1-intervalAdjustRatio) + intervalAdjustRatio*(target-intervalAdjustBias)
		min := float64(minRecommit.Nanoseconds())
		if next < min {
			next = min
		}
	}
	return time.Duration(int64(next))
}

func (w *worker) newWorkLoop(recommit time.Duration) {
	defer w.wg.Done()
	var (
		interrupt   *atomic.Int32
		minRecommit = recommit
		timestamp   int64
	)

	timer := time.NewTimer(0)
	defer timer.Stop()
	<-timer.C

	commit := func(noempty bool, s int32) {
		if interrupt != nil {
			interrupt.Store(s)
		}
		interrupt = new(atomic.Int32)
		select {
		case w.newWorkCh <- &newWorkReq{interrupt: interrupt, noempty: noempty, timestamp: timestamp}:
		case <-w.exitCh:
			return
		}
		timer.Reset(recommit)
		w.newTxs.Store(0)
	}
	clearPending := func(number uint64) {
		w.pendingMu.Lock()
		for h, t := range w.pendingTasks {
			if t.block.NumberU64()+staleThreshold <= number {
				delete(w.pendingTasks, h)
			}
		}
		w.pendingMu.Unlock()
	}

	for {
		select {
		case <-w.startCh:
			clearPending(w.chain.CurrentBlock().Number.Uint64())
			timestamp = time.Now().Unix()
			commit(false, commitInterruptNewHead)
		case head := <-w.chainHeadCh:
			clearPending(head.Block.NumberU64())
			timestamp = time.Now().Unix()
			commit(false, commitInterruptNewHead)
		case <-timer.C:
			if w.isRunning() && (w.chainConfig.Clique == nil || w.chainConfig.Clique.Period > 0) {
				if w.newTxs.Load() == 0 {
					timer.Reset(recommit)
					continue
				}
				commit(true, commitInterruptResubmit)
			}
		case interval := <-w.resubmitIntervalCh:
			if interval < minRecommit {
				log.Warn("Sanitizing miner recommit interval", "provided", interval, "updated", minRecommitInterval)
				interval = minRecommit
			}
			log.Info("Miner recommit interval update", "from", minRecommit, "to", interval)
			minRecommit, recommit = interval, interval
			if w.resubmitHook != nil {
				w.resubmitHook(minRecommit, recommit)
			}
		case adjust := <-w.resubmitAdjustCh:
			if adjust.inc {
				before := recommit
				target := float64(recommit.Nanoseconds()) / adjust.ratio
				recommit = recalcRecommit(minRecommit, recommit, target, true)
				log.Trace("Increase miner recommit interval", "from", before, "to", recommit)
			} else {
				before := recommit
				recommit = recalcRecommit(minRecommit, recommit, float64(minRecommit.Nanoseconds()), false)
				log.Trace("Decrease miner recommit interval", "from", before, "to", recommit)
			}
			if w.resubmitHook != nil {
				w.resubmitHook(minRecommit, recommit)
			}
		case <-w.exitCh:
			return
		}
	}
}

func (w *worker) mainLoop() {
	defer w.wg.Done()
	defer w.txsSub.Unsubscribe()
	defer w.chainHeadSub.Unsubscribe()
	defer w.chainSideSub.Unsubscribe()
	defer func() {
		if w.current != nil {
			w.current.discard()
		}
	}()

	cleanTicker := time.NewTicker(10 * time.Second)
	defer cleanTicker.Stop()

	for {
		select {
		case req := <-w.newWorkCh:
			w.commitWork(req.interrupt, req.noempty, req.timestamp)
		case req := <-w.getWorkCh:
			block, fees, err := w.generateWork(req.params)
			req.result <- &newPayloadResult{
				err:   err,
				block: block,
				fees:  fees,
			}
		case ev := <-w.chainSideCh:
			if _, exist := w.localUncles[ev.Block.Hash()]; exist {
				continue
			}
			if _, exist := w.remoteUncles[ev.Block.Hash()]; exist {
				continue
			}
			if w.isLocalBlock != nil && w.isLocalBlock(ev.Block.Header()) {
				w.localUncles[ev.Block.Hash()] = ev.Block
			} else {
				w.remoteUncles[ev.Block.Hash()] = ev.Block
			}
			if w.isRunning() && w.current != nil && len(w.current.uncles) < 2 {
				start := time.Now()
				if err := w.commitUncle(w.current, ev.Block.Header()); err == nil {
					w.commit(w.current.copy(), nil, true, start)
				}
			}
		case <-cleanTicker.C:
			chainHead := w.chain.CurrentBlock()
			for hash, uncle := range w.localUncles {
				if uncle.NumberU64()+staleThreshold <= chainHead.Number.Uint64() {
					delete(w.localUncles, hash)
				}
			}
			for hash, uncle := range w.remoteUncles {
				if uncle.NumberU64()+staleThreshold <= chainHead.Number.Uint64() {
					delete(w.remoteUncles, hash)
				}
			}
		case ev := <-w.txsCh:
			if !w.isRunning() && w.current != nil {
				if gp := w.current.gasPool; gp != nil && gp.Gas() < params.TxGas {
					continue
				}
				txs := make(map[common.Address]types.Transactions)
				for _, tx := range ev.Txs {
					acc, _ := types.Sender(w.current.signer, tx)
					txs[acc] = append(txs[acc], tx)
				}
				txset := types.NewTransactionsByPriceAndNonce(w.current.signer, txs, w.current.header.BaseFee)
				tcount := w.current.tcount
				w.commitTransactions(w.current, txset, nil)
				if tcount != w.current.tcount {
					w.updateSnapshot(w.current)
				}
			} else {
				if w.chainConfig.Clique != nil && w.chainConfig.Clique.Period == 0 {
					w.commitWork(nil, true, time.Now().Unix())
				}
			}
			w.newTxs.Add(int32(len(ev.Txs)))
		case <-w.exitCh:
			return
		case <-w.txsSub.Err():
			return
		case <-w.chainHeadSub.Err():
			return
		case <-w.chainSideSub.Err():
			return
		}
	}
}

func (w *worker) taskLoop() {
	defer w.wg.Done()
	var (
		stopCh chan struct{}
		prev   common.Hash
	)

	interrupt := func() {
		if stopCh != nil {
			close(stopCh)
			stopCh = nil
		}
	}
	for {
		select {
		case task := <-w.taskCh:
			if w.newTaskHook != nil {
				w.newTaskHook(task)
			}
			// Now, call SealHash which returns a single common.Hash.
			sealHash := w.engine.SealHash(task.block.Header())
			if sealHash == prev {
				continue
			}
			interrupt()
			stopCh, prev = make(chan struct{}), sealHash

			if w.skipSealHook != nil && w.skipSealHook(task) {
				continue
			}
			w.pendingMu.Lock()
			w.pendingTasks[sealHash] = task
			w.pendingMu.Unlock()

			if err := w.engine.Seal(w.chain, task.block, w.resultCh, stopCh); err != nil {
				log.Warn("Block sealing failed", "err", err)
				w.pendingMu.Lock()
				delete(w.pendingTasks, sealHash)
				w.pendingMu.Unlock()
			}
		case <-w.exitCh:
			interrupt()
			return
		}
	}
}

func (w *worker) resultLoop() {
	defer w.wg.Done()
	for {
		select {
		case block := <-w.resultCh:
			if block == nil {
				continue
			}
			if w.chain.HasBlock(block.Hash(), block.NumberU64()) {
				continue
			}
			sealhash := w.engine.SealHash(block.Header())
			w.pendingMu.RLock()
			task, exist := w.pendingTasks[sealhash]
			w.pendingMu.RUnlock()
			if !exist {
				log.Error("Block found but no relative pending task", "number", block.Number(), "sealhash", sealhash, "hash", block.Hash())
				continue
			}
			receipts := make([]*types.Receipt, len(task.receipts))
			var logs []*types.Log
			for i, taskReceipt := range task.receipts {
				receipt := new(types.Receipt)
				receipts[i] = receipt
				*receipt = *taskReceipt
				receipt.BlockHash = block.Hash()
				receipt.BlockNumber = block.Number()
				receipt.TransactionIndex = uint(i)
				receipt.Logs = make([]*types.Log, len(taskReceipt.Logs))
				for i, taskLog := range taskReceipt.Logs {
					l := new(types.Log)
					receipt.Logs[i] = l
					*l = *taskLog
					l.BlockHash = block.Hash()
				}
				logs = append(logs, receipt.Logs...)
			}
			_, err := w.chain.WriteBlockAndSetHead(block, receipts, logs, task.state, true)
			if err != nil {
				log.Error("Failed writing block to chain", "err", err)
				continue
			}
			log.Info("Successfully Sealed Block", "number", block.Number(), "sealhash", sealhash, "hash", block.Hash(),
				"elapsed", common.PrettyDuration(time.Since(task.createdAt)))
			w.mux.Post(core.NewMinedBlockEvent{Block: block})
			w.unconfirmed.Insert(block.NumberU64(), block.Hash())
		case <-w.exitCh:
			return
		}
	}
}

func (w *worker) makeEnv(parent *types.Header, header *types.Header, coinbase common.Address) (*environment, error) {
	state, err := w.chain.StateAt(parent.Root)
	if err != nil {
		return nil, err
	}
	state.StartPrefetcher("miner")
	env := &environment{
		signer:    types.MakeSigner(w.chainConfig, header.Number),
		state:     state,
		coinbase:  coinbase,
		ancestors: mapset.NewSet[common.Hash](),
		family:    mapset.NewSet[common.Hash](),
		header:    header,
		uncles:    make(map[common.Hash]*types.Header),
	}
	for _, ancestor := range w.chain.GetBlocksFromHash(parent.Hash(), 7) {
		for _, uncle := range ancestor.Uncles() {
			env.family.Add(uncle.Hash())
		}
		env.family.Add(ancestor.Hash())
		env.ancestors.Add(ancestor.Hash())
	}
	env.tcount = 0
	return env, nil
}

func (w *worker) commitUncle(env *environment, uncle *types.Header) error {
	if w.isTTDReached(env.header) {
		return errors.New("ignore uncle for beacon block")
	}
	hash := uncle.Hash()
	if _, exist := env.uncles[hash]; exist {
		return errors.New("uncle not unique")
	}
	if env.header.ParentHash == uncle.ParentHash {
		return errors.New("uncle is sibling")
	}
	if !env.ancestors.Contains(uncle.ParentHash) {
		return errors.New("uncle's parent unknown")
	}
	if env.family.Contains(hash) {
		return errors.New("uncle already included")
	}
	env.uncles[hash] = uncle
	return nil
}

func (w *worker) updateSnapshot(env *environment) {
	w.snapshotMu.Lock()
	defer w.snapshotMu.Unlock()

	w.snapshotBlock = types.NewBlock(
		env.header,
		env.txs,
		env.unclelist(),
		env.receipts,
		trie.NewStackTrie(nil),
	)
	w.snapshotReceipts = copyReceipts(env.receipts)
	w.snapshotState = env.state.Copy()
}

func (w *worker) commitTransaction(env *environment, tx *types.Transaction) ([]*types.Log, error) {
	snap := env.state.Snapshot()
	gp := env.gasPool.Gas()
	receipt, err := core.ApplyTransaction(w.chainConfig, w.chain, &env.coinbase, env.gasPool, env.state, env.header, tx, &env.header.GasUsed, *w.chain.GetVMConfig())
	if err != nil {
		env.state.RevertToSnapshot(snap)
		env.gasPool.SetGas(gp)
		return nil, err
	}
	env.txs = append(env.txs, tx)
	env.receipts = append(env.receipts, receipt)
	return receipt.Logs, nil
}

func (w *worker) commitTransactions(env *environment, txs *types.TransactionsByPriceAndNonce, interrupt *atomic.Int32) error {
	gasLimit := env.header.GasLimit
	if env.gasPool == nil {
		env.gasPool = new(core.GasPool).AddGas(gasLimit)
	}
	var coalescedLogs []*types.Log

	for {
		if interrupt != nil {
			if signal := interrupt.Load(); signal != commitInterruptNone {
				return signalToErr(signal)
			}
		}
		if env.gasPool.Gas() < params.TxGas {
			log.Trace("Not enough gas for further transactions", "have", env.gasPool, "want", params.TxGas)
			break
		}
		tx := txs.Peek()
		if tx == nil {
			break
		}
		from, _ := types.Sender(env.signer, tx)
		if tx.Protected() && !w.chainConfig.IsEIP155(env.header.Number) {
			log.Trace("Ignoring replay protected transaction", "hash", tx.Hash(), "eip155", w.chainConfig.EIP155Block)
			txs.Pop()
			continue
		}
		env.state.SetTxContext(tx.Hash(), env.tcount)
		logs, err := w.commitTransaction(env, tx)
		switch {
		case errors.Is(err, core.ErrNonceTooLow):
			log.Trace("Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())
			txs.Shift()
		case err == nil:
			coalescedLogs = append(coalescedLogs, logs...)
			env.tcount++
			txs.Shift()
		default:
			log.Debug("Transaction failed, account skipped", "hash", tx.Hash(), "err", err)
			txs.Pop()
		}
	}
	if !w.isRunning() && len(coalescedLogs) > 0 {
		cpy := make([]*types.Log, len(coalescedLogs))
		for i, l := range coalescedLogs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l
		}
		w.pendingLogsFeed.Send(cpy)
	}
	return nil
}

type generateParams struct {
	timestamp   uint64
	forceTime   bool
	parentHash  common.Hash
	coinbase    common.Address
	random      common.Hash
	withdrawals types.Withdrawals
	noUncle     bool
	noTxs       bool
}

func (w *worker) prepareWork(genParams *generateParams) (*environment, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	parent := w.chain.CurrentBlock()
	if genParams.parentHash != (common.Hash{}) {
		block := w.chain.GetBlockByHash(genParams.parentHash)
		if block == nil {
			return nil, fmt.Errorf("missing parent")
		}
		parent = block.Header()
	}
	timestamp := genParams.timestamp
	if parent.Time >= timestamp {
		if genParams.forceTime {
			return nil, fmt.Errorf("invalid timestamp, parent %d given %d", parent.Time, timestamp)
		}
		timestamp = parent.Time + 1
	}
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     new(big.Int).Add(parent.Number, common.Big1),
		GasLimit:   core.CalcGasLimit(parent.GasLimit, w.config.GasCeil),
		Time:       timestamp,
		Coinbase:   genParams.coinbase,
	}
	if len(w.extra) != 0 {
		header.Extra = w.extra
	}
	if genParams.random != (common.Hash{}) {
		header.MixDigest = genParams.random
	}
	if w.chainConfig.IsLondon(header.Number) {
		header.BaseFee = misc.CalcBaseFee(w.chainConfig, parent)
		if !w.chainConfig.IsLondon(parent.Number) {
			parentGasLimit := parent.GasLimit * w.chainConfig.ElasticityMultiplier()
			header.GasLimit = core.CalcGasLimit(parentGasLimit, w.config.GasCeil)
		}
	}
	if err := w.engine.Prepare(w.chain, header); err != nil {
		log.Error("Failed to prepare header for sealing", "err", err)
		return nil, err
	}
	env, err := w.makeEnv(parent, header, genParams.coinbase)
	if err != nil {
		log.Error("Failed to create sealing context", "err", err)
		return nil, err
	}
	if !genParams.noUncle {
		commitUncles := func(blocks map[common.Hash]*types.Block) {
			for hash, uncle := range blocks {
				if len(env.uncles) == 2 {
					break
				}
				if err := w.commitUncle(env, uncle.Header()); err != nil {
					log.Trace("Possible uncle rejected", "hash", hash, "reason", err)
				} else {
					log.Debug("Committing new uncle to block", "hash", hash)
				}
			}
		}
		commitUncles(w.localUncles)
		commitUncles(w.remoteUncles)
	}
	return env, nil
}

func (w *worker) fillTransactions(interrupt *atomic.Int32, env *environment) error {
	pending := w.eth.TxPool().Pending(true)
	localTxs, remoteTxs := make(map[common.Address]types.Transactions), pending
	for _, account := range w.eth.TxPool().Locals() {
		if txs := remoteTxs[account]; len(txs) > 0 {
			delete(remoteTxs, account)
			localTxs[account] = txs
		}
	}
	if len(localTxs) > 0 {
		txs := types.NewTransactionsByPriceAndNonce(env.signer, localTxs, env.header.BaseFee)
		if err := w.commitTransactions(env, txs, interrupt); err != nil {
			return err
		}
	}
	if len(remoteTxs) > 0 {
		txs := types.NewTransactionsByPriceAndNonce(env.signer, remoteTxs, env.header.BaseFee)
		if err := w.commitTransactions(env, txs, interrupt); err != nil {
			return err
		}
	}
	return nil
}

func (w *worker) generateWork(params *generateParams) (*types.Block, *big.Int, error) {
	work, err := w.prepareWork(params)
	if err != nil {
		return nil, nil, err
	}
	defer work.discard()

	if !params.noTxs {
		interrupt := new(atomic.Int32)
		timer := time.AfterFunc(w.newpayloadTimeout, func() {
			interrupt.Store(commitInterruptTimeout)
		})
		defer timer.Stop()

		err := w.fillTransactions(interrupt, work)
		if errors.Is(err, errBlockInterruptedByTimeout) {
			log.Warn("Block building is interrupted", "allowance", w.newpayloadTimeout)
		}
	}
	block, err := w.engine.FinalizeAndAssemble(w.chain, work.header, work.state, work.txs, work.unclelist(), work.receipts, params.withdrawals)
	if err != nil {
		return nil, nil, err
	}
	return block, totalFees(block, work.receipts), nil
}

func (w *worker) commitWork(interrupt *atomic.Int32, noempty bool, timestamp int64) {
	start := time.Now()

	var coinbase common.Address
	if w.isRunning() {
		coinbase = w.etherbase()
		if coinbase == (common.Address{}) {
			log.Error("Refusing to mine without etherbase")
			return
		}
	}
	work, err := w.prepareWork(&generateParams{
		timestamp: uint64(timestamp),
		coinbase:  coinbase,
	})
	if err != nil {
		return
	}
	if !noempty && !w.noempty.Load() {
		w.commit(work.copy(), nil, false, start)
	}
	err = w.fillTransactions(interrupt, work)
	switch {
	case err == nil:
		w.resubmitAdjustCh <- &intervalAdjust{inc: false}
	case errors.Is(err, errBlockInterruptedByRecommit):
		gaslimit := work.header.GasLimit
		ratio := float64(gaslimit-work.gasPool.Gas()) / float64(gaslimit)
		if ratio < 0.1 {
			ratio = 0.1
		}
		w.resubmitAdjustCh <- &intervalAdjust{
			ratio: ratio,
			inc:   true,
		}
	case errors.Is(err, errBlockInterruptedByNewHead):
		work.discard()
		return
	}
	w.commit(work.copy(), w.fullTaskHook, true, start)
	if w.current != nil {
		w.current.discard()
	}
	w.current = work
}

func (w *worker) commit(env *environment, interval func(), update bool, start time.Time) error {
	if w.isRunning() {
		if interval != nil {
			interval()
		}
		env = env.copy()
		block, err := w.engine.FinalizeAndAssemble(w.chain, env.header, env.state, env.txs, env.unclelist(), env.receipts, nil)
		if err != nil {
			return err
		}
		// Now SealHash returns a single common.Hash.
		sealhash := w.engine.SealHash(block.Header())
		if !w.isTTDReached(block.Header()) {
			select {
			case w.taskCh <- &task{receipts: env.receipts, state: env.state, block: block, createdAt: time.Now()}:
				w.unconfirmed.Shift(block.NumberU64() - 1)
				fees := totalFees(block, env.receipts)
				feesInEther := new(big.Float).Quo(new(big.Float).SetInt(fees), big.NewFloat(params.Ether))
				log.Info("Mining R5...", "number", block.Number(), "sealhash", sealhash, "hash", block.Hash(),
					"uncles", len(env.uncles), "txs", env.tcount,
					"gas", block.GasUsed(), "fees", feesInEther,
					"elapsed", common.PrettyDuration(time.Since(start)))
			case <-w.exitCh:
				log.Info("Worker has exited")
			}
		}
	}
	if update {
		w.updateSnapshot(env)
	}
	return nil
}

func (w *worker) getSealingBlock(parent common.Hash, timestamp uint64, coinbase common.Address, random common.Hash, withdrawals types.Withdrawals, noTxs bool) (*types.Block, *big.Int, error) {
	req := &getWorkReq{
		params: &generateParams{
			timestamp:   timestamp,
			forceTime:   true,
			parentHash:  parent,
			coinbase:    coinbase,
			random:      random,
			withdrawals: withdrawals,
			noUncle:     true,
			noTxs:       noTxs,
		},
		result: make(chan *newPayloadResult, 1),
	}
	select {
	case w.getWorkCh <- req:
		result := <-req.result
		if result.err != nil {
			return nil, nil, result.err
		}
		return result.block, result.fees, nil
	case <-w.exitCh:
		return nil, nil, errors.New("miner closed")
	}
}

func (w *worker) isTTDReached(header *types.Header) bool {
	td, ttd := w.chain.GetTd(header.ParentHash, header.Number.Uint64()-1), w.chain.Config().TerminalTotalDifficulty
	return td != nil && ttd != nil && td.Cmp(ttd) >= 0
}

func copyReceipts(receipts []*types.Receipt) []*types.Receipt {
	result := make([]*types.Receipt, len(receipts))
	for i, l := range receipts {
		cpy := *l
		result[i] = &cpy
	}
	return result
}

func (w *worker) postSideBlock(event core.ChainSideEvent) {
	select {
	case w.chainSideCh <- event:
	case <-w.exitCh:
	}
}

func totalFees(block *types.Block, receipts []*types.Receipt) *big.Int {
	feesWei := new(big.Int)
	for i, tx := range block.Transactions() {
		minerFee, _ := tx.EffectiveGasTip(block.BaseFee())
		feesWei.Add(feesWei, new(big.Int).Mul(new(big.Int).SetUint64(receipts[i].GasUsed), minerFee))
	}
	return feesWei
}

func signalToErr(signal int32) error {
	switch signal {
	case commitInterruptNewHead:
		return errBlockInterruptedByNewHead
	case commitInterruptResubmit:
		return errBlockInterruptedByRecommit
	case commitInterruptTimeout:
		return errBlockInterruptedByTimeout
	default:
		panic(fmt.Errorf("undefined signal %d", signal))
	}
}
