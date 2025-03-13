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

// Package miner implements Ethereum block creation and mining.

package miner

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/r5-labs/r5-core/common"
	"github.com/r5-labs/r5-core/common/hexutil"
	"github.com/r5-labs/r5-core/consensus"
	"github.com/r5-labs/r5-core/core"
	"github.com/r5-labs/r5-core/core/state"
	"github.com/r5-labs/r5-core/core/txpool"
	"github.com/r5-labs/r5-core/core/types"
	"github.com/r5-labs/r5-core/eth/downloader"
	"github.com/r5-labs/r5-core/event"
	"github.com/r5-labs/r5-core/log"
	"github.com/r5-labs/r5-core/params"
)

type Backend interface {
	BlockChain() *core.BlockChain
	TxPool() *txpool.TxPool
}

type Config struct {
	Etherbase  common.Address `toml:",omitempty"` // Public address for block mining rewards
	Notify     []string       `toml:",omitempty"` // HTTP URL list to be notified of new work packages (only useful in ethash).
	NotifyFull bool           `toml:",omitempty"` // Notify with pending block headers instead of work packages
	ExtraData  hexutil.Bytes  `toml:",omitempty"` // Block extra data set by the miner
	GasFloor   uint64         // Target gas floor for mined blocks.
	GasCeil    uint64         // Target gas ceiling for mined blocks.
	GasPrice   *big.Int       // Minimum gas price for mining a transaction
	Recommit   time.Duration  // The time interval for miner to re-create mining work.
	Noverify   bool           // Disable remote mining solution verification (only useful in ethash).

	NewPayloadTimeout time.Duration // The maximum time allowance for creating a new payload
}

var DefaultConfig = Config{
	GasCeil:  250000000,
	GasPrice: big.NewInt(params.GWei),
	Recommit:          2 * time.Second,
	NewPayloadTimeout: 2 * time.Second,
}

type Miner struct {
	mux     *event.TypeMux
	eth     Backend
	engine  consensus.Engine
	exitCh  chan struct{}
	startCh chan struct{}
	stopCh  chan struct{}
	worker  *worker

	wg sync.WaitGroup
}

func New(eth Backend, config *Config, chainConfig *params.ChainConfig, mux *event.TypeMux, engine consensus.Engine, isLocalBlock func(header *types.Header) bool) *Miner {
	miner := &Miner{
		mux:     mux,
		eth:     eth,
		engine:  engine,
		exitCh:  make(chan struct{}),
		startCh: make(chan struct{}),
		stopCh:  make(chan struct{}),
		worker:  newWorker(config, chainConfig, engine, eth, mux, isLocalBlock, true),
	}
	miner.wg.Add(1)
	go miner.update()
	return miner
}

func (miner *Miner) update() {
	defer miner.wg.Done()

	events := miner.mux.Subscribe(downloader.StartEvent{}, downloader.DoneEvent{}, downloader.FailedEvent{})
	defer func() {
		if !events.Closed() {
			events.Unsubscribe()
		}
	}()

	shouldStart := false
	canStart := true
	dlEventCh := events.Chan()
	for {
		select {
		case ev := <-dlEventCh:
			if ev == nil {
				dlEventCh = nil
				continue
			}
			switch ev.Data.(type) {
			case downloader.StartEvent:
				wasMining := miner.Mining()
				miner.worker.stop()
				canStart = false
				if wasMining {
					shouldStart = true
					log.Info("Mining aborted due to sync")
				}
			case downloader.FailedEvent:
				canStart = true
				if shouldStart {
					miner.worker.start()
				}
			case downloader.DoneEvent:
				canStart = true
				if shouldStart {
					miner.worker.start()
				}
				events.Unsubscribe()
			}
		case <-miner.startCh:
			if canStart {
				miner.worker.start()
			}
			shouldStart = true
		case <-miner.stopCh:
			shouldStart = false
			miner.worker.stop()
		case <-miner.exitCh:
			miner.worker.close()
			return
		}
	}
}

func (miner *Miner) Start() {
	miner.startCh <- struct{}{}
}

func (miner *Miner) Stop() {
	miner.stopCh <- struct{}{}
}

func (miner *Miner) Close() {
	close(miner.exitCh)
	miner.wg.Wait()
}

func (miner *Miner) Mining() bool {
	return miner.worker.isRunning()
}

func (miner *Miner) Hashrate() uint64 {
	if pow, ok := miner.engine.(consensus.PoW); ok {
		return uint64(pow.Hashrate())
	}
	return 0
}

func (miner *Miner) SetExtra(extra []byte) error {
	if uint64(len(extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra exceeds max length. %d > %v", len(extra), params.MaximumExtraDataSize)
	}
	miner.worker.setExtra(extra)
	return nil
}

func (miner *Miner) SetRecommitInterval(interval time.Duration) {
	miner.worker.setRecommitInterval(interval)
}

func (miner *Miner) Pending() (*types.Block, *state.StateDB) {
	return miner.worker.pending()
}

func (miner *Miner) PendingBlock() *types.Block {
	return miner.worker.pendingBlock()
}

func (miner *Miner) PendingBlockAndReceipts() (*types.Block, types.Receipts) {
	return miner.worker.pendingBlockAndReceipts()
}

func (miner *Miner) SetEtherbase(addr common.Address) {
	miner.worker.setEtherbase(addr)
}

func (miner *Miner) SetGasCeil(ceil uint64) {
	miner.worker.setGasCeil(ceil)
}

func (miner *Miner) EnablePreseal() {
	miner.worker.enablePreseal()
}

func (miner *Miner) DisablePreseal() {
	miner.worker.disablePreseal()
}

func (miner *Miner) SubscribePendingLogs(ch chan<- []*types.Log) event.Subscription {
	return miner.worker.pendingLogsFeed.Subscribe(ch)
}

func (miner *Miner) BuildPayload(args *BuildPayloadArgs) (*Payload, error) {
	return miner.worker.buildPayload(args)
}
