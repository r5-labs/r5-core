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
	"fmt"
	"math/big"
	"testing"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/consensus/ethash"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/params"
	"github.com/r5-labs/r5-core/client/rlp"
	"golang.org/x/crypto/sha3"
)

func getBlock(transactions int, uncles int, dataSize int) *types.Block {
	var (
		aa     = common.HexToAddress("0x000000000000000000000000000000000000aaaa")
		engine = ethash.NewFaker()

		// A sender who makes transactions, has some funds
		key, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		address = crypto.PubkeyToAddress(key.PublicKey)
		funds   = big.NewInt(1_000_000_000_000_000_000)
		gspec   = &Genesis{
			Config: params.TestChainConfig,
			Alloc:  GenesisAlloc{address: {Balance: funds}},
		}
	)
	// We need to generate as many blocks +1 as uncles
	_, blocks, _ := GenerateChainWithGenesis(gspec, engine, uncles+1,
		func(n int, b *BlockGen) {
			if n == uncles {
				// Add transactions and stuff on the last block
				for i := 0; i < transactions; i++ {
					tx, _ := types.SignTx(types.NewTransaction(uint64(i), aa,
						big.NewInt(0), 50000, b.header.BaseFee, make([]byte, dataSize)), types.HomesteadSigner{}, key)
					b.AddTx(tx)
				}
				for i := 0; i < uncles; i++ {
					b.AddUncle(&types.Header{ParentHash: b.PrevBlock(n - 1 - i).Hash(), Number: big.NewInt(int64(n - i))})
				}
			}
		})
	block := blocks[len(blocks)-1]
	return block
}

// TestRlpIterator tests that individual transactions can be picked out
// from blocks without full unmarshalling/marshalling
func TestRlpIterator(t *testing.T) {
	for _, tt := range []struct {
		txs      int
		uncles   int
		datasize int
	}{
		{0, 0, 0},
		{0, 2, 0},
		{10, 0, 0},
		{10, 2, 0},
		{10, 2, 50},
	} {
		testRlpIterator(t, tt.txs, tt.uncles, tt.datasize)
	}
}

func testRlpIterator(t *testing.T, txs, uncles, datasize int) {
	desc := fmt.Sprintf("%d txs [%d datasize] and %d uncles", txs, datasize, uncles)
	bodyRlp, _ := rlp.EncodeToBytes(getBlock(txs, uncles, datasize).Body())
	it, err := rlp.NewListIterator(bodyRlp)
	if err != nil {
		t.Fatal(err)
	}
	// Check that txs exist
	if !it.Next() {
		t.Fatal("expected two elems, got zero")
	}
	txdata := it.Value()
	// Check that uncles exist
	if !it.Next() {
		t.Fatal("expected two elems, got one")
	}
	// No more after that
	if it.Next() {
		t.Fatal("expected only two elems, got more")
	}
	txIt, err := rlp.NewListIterator(txdata)
	if err != nil {
		t.Fatal(err)
	}
	var gotHashes []common.Hash
	var expHashes []common.Hash
	for txIt.Next() {
		gotHashes = append(gotHashes, crypto.Keccak256Hash(txIt.Value()))
	}

	var expBody types.Body
	err = rlp.DecodeBytes(bodyRlp, &expBody)
	if err != nil {
		t.Fatal(err)
	}
	for _, tx := range expBody.Transactions {
		expHashes = append(expHashes, tx.Hash())
	}
	if gotLen, expLen := len(gotHashes), len(expHashes); gotLen != expLen {
		t.Fatalf("testcase %v: length wrong, got %d exp %d", desc, gotLen, expLen)
	}
	// also sanity check against input
	if gotLen := len(gotHashes); gotLen != txs {
		t.Fatalf("testcase %v: length wrong, got %d exp %d", desc, gotLen, txs)
	}
	for i, got := range gotHashes {
		if exp := expHashes[i]; got != exp {
			t.Errorf("testcase %v: hash wrong, got %x, exp %x", desc, got, exp)
		}
	}
}

// BenchmarkHashing compares the speeds of hashing a rlp raw data directly
// without the unmarshalling/marshalling step
func BenchmarkHashing(b *testing.B) {
	// Make a pretty fat block
	var (
		bodyRlp  []byte
		blockRlp []byte
	)
	{
		block := getBlock(200, 2, 50)
		bodyRlp, _ = rlp.EncodeToBytes(block.Body())
		blockRlp, _ = rlp.EncodeToBytes(block)
	}
	var got common.Hash
	var hasher = sha3.NewLegacyKeccak256()
	b.Run("iteratorhashing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var hash common.Hash
			it, err := rlp.NewListIterator(bodyRlp)
			if err != nil {
				b.Fatal(err)
			}
			it.Next()
			txs := it.Value()
			txIt, err := rlp.NewListIterator(txs)
			if err != nil {
				b.Fatal(err)
			}
			for txIt.Next() {
				hasher.Reset()
				hasher.Write(txIt.Value())
				hasher.Sum(hash[:0])
				got = hash
			}
		}
	})
	var exp common.Hash
	b.Run("fullbodyhashing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var body types.Body
			rlp.DecodeBytes(bodyRlp, &body)
			for _, tx := range body.Transactions {
				exp = tx.Hash()
			}
		}
	})
	b.Run("fullblockhashing", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var block types.Block
			rlp.DecodeBytes(blockRlp, &block)
			for _, tx := range block.Transactions() {
				tx.Hash()
			}
		}
	})
	if got != exp {
		b.Fatalf("hash wrong, got %x exp %x", got, exp)
	}
}
