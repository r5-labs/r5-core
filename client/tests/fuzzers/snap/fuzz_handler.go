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

package snap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	"github.com/r5-labs/r5-core/common"
	"github.com/r5-labs/r5-core/consensus/ethash"
	"github.com/r5-labs/r5-core/core"
	"github.com/r5-labs/r5-core/core/rawdb"
	"github.com/r5-labs/r5-core/core/vm"
	"github.com/r5-labs/r5-core/eth/protocols/snap"
	"github.com/r5-labs/r5-core/p2p"
	"github.com/r5-labs/r5-core/p2p/enode"
	"github.com/r5-labs/r5-core/params"
	"github.com/r5-labs/r5-core/rlp"
	fuzz "github.com/google/gofuzz"
)

var trieRoot common.Hash

func getChain() *core.BlockChain {
	ga := make(core.GenesisAlloc, 1000)
	var a = make([]byte, 20)
	var mkStorage = func(k, v int) (common.Hash, common.Hash) {
		var kB = make([]byte, 32)
		var vB = make([]byte, 32)
		binary.LittleEndian.PutUint64(kB, uint64(k))
		binary.LittleEndian.PutUint64(vB, uint64(v))
		return common.BytesToHash(kB), common.BytesToHash(vB)
	}
	storage := make(map[common.Hash]common.Hash)
	for i := 0; i < 10; i++ {
		k, v := mkStorage(i, i)
		storage[k] = v
	}
	for i := 0; i < 1000; i++ {
		binary.LittleEndian.PutUint64(a, uint64(i+0xff))
		acc := core.GenesisAccount{Balance: big.NewInt(int64(i))}
		if i%2 == 1 {
			acc.Storage = storage
		}
		ga[common.BytesToAddress(a)] = acc
	}
	gspec := &core.Genesis{
		Config: params.TestChainConfig,
		Alloc:  ga,
	}
	_, blocks, _ := core.GenerateChainWithGenesis(gspec, ethash.NewFaker(), 2, func(i int, gen *core.BlockGen) {})
	cacheConf := &core.CacheConfig{
		TrieCleanLimit:      0,
		TrieDirtyLimit:      0,
		TrieTimeLimit:       5 * time.Minute,
		TrieCleanNoPrefetch: true,
		TrieCleanRejournal:  0,
		SnapshotLimit:       100,
		SnapshotWait:        true,
	}
	trieRoot = blocks[len(blocks)-1].Root()
	bc, _ := core.NewBlockChain(rawdb.NewMemoryDatabase(), cacheConf, gspec, nil, ethash.NewFaker(), vm.Config{}, nil, nil)
	if _, err := bc.InsertChain(blocks); err != nil {
		panic(err)
	}
	return bc
}

type dummyBackend struct {
	chain *core.BlockChain
}

func (d *dummyBackend) Chain() *core.BlockChain                { return d.chain }
func (d *dummyBackend) RunPeer(*snap.Peer, snap.Handler) error { return nil }
func (d *dummyBackend) PeerInfo(enode.ID) interface{}          { return "Foo" }
func (d *dummyBackend) Handle(*snap.Peer, snap.Packet) error   { return nil }

type dummyRW struct {
	code       uint64
	data       []byte
	writeCount int
}

func (d *dummyRW) ReadMsg() (p2p.Msg, error) {
	return p2p.Msg{
		Code:       d.code,
		Payload:    bytes.NewReader(d.data),
		ReceivedAt: time.Now(),
		Size:       uint32(len(d.data)),
	}, nil
}

func (d *dummyRW) WriteMsg(msg p2p.Msg) error {
	d.writeCount++
	return nil
}

func doFuzz(input []byte, obj interface{}, code int) int {
	if len(input) > 1024*4 {
		return -1
	}
	bc := getChain()
	defer bc.Stop()
	backend := &dummyBackend{bc}
	fuzz.NewFromGoFuzz(input).Fuzz(obj)
	var data []byte
	switch p := obj.(type) {
	case *snap.GetTrieNodesPacket:
		p.Root = trieRoot
		data, _ = rlp.EncodeToBytes(obj)
	default:
		data, _ = rlp.EncodeToBytes(obj)
	}
	cli := &dummyRW{
		code: uint64(code),
		data: data,
	}
	peer := snap.NewFakePeer(65, "gazonk01", cli)
	err := snap.HandleMessage(backend, peer)
	switch {
	case err == nil && cli.writeCount != 1:
		panic(fmt.Sprintf("Expected 1 response, got %d", cli.writeCount))
	case err != nil && cli.writeCount != 0:
		panic(fmt.Sprintf("Expected 0 response, got %d", cli.writeCount))
	}
	return 1
}

// To run a fuzzer, do
// $ CGO_ENABLED=0 go-fuzz-build -func FuzzTrieNodes
// $ go-fuzz

func FuzzARange(input []byte) int {
	return doFuzz(input, &snap.GetAccountRangePacket{}, snap.GetAccountRangeMsg)
}
func FuzzSRange(input []byte) int {
	return doFuzz(input, &snap.GetStorageRangesPacket{}, snap.GetStorageRangesMsg)
}
func FuzzByteCodes(input []byte) int {
	return doFuzz(input, &snap.GetByteCodesPacket{}, snap.GetByteCodesMsg)
}
func FuzzTrieNodes(input []byte) int {
	return doFuzz(input, &snap.GetTrieNodesPacket{}, snap.GetTrieNodesMsg)
}
