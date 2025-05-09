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

package stacktrie

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"sort"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/rawdb"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/ethdb"
	"github.com/r5-labs/r5-core/client/trie"
	"golang.org/x/crypto/sha3"
)

type fuzzer struct {
	input     io.Reader
	exhausted bool
	debugging bool
}

func (f *fuzzer) read(size int) []byte {
	out := make([]byte, size)
	if _, err := f.input.Read(out); err != nil {
		f.exhausted = true
	}
	return out
}

func (f *fuzzer) readSlice(min, max int) []byte {
	var a uint16
	binary.Read(f.input, binary.LittleEndian, &a)
	size := min + int(a)%(max-min)
	out := make([]byte, size)
	if _, err := f.input.Read(out); err != nil {
		f.exhausted = true
	}
	return out
}

// spongeDb is a dummy db backend which accumulates writes in a sponge
type spongeDb struct {
	sponge hash.Hash
	debug  bool
}

func (s *spongeDb) Has(key []byte) (bool, error)             { panic("implement me") }
func (s *spongeDb) Get(key []byte) ([]byte, error)           { return nil, errors.New("no such elem") }
func (s *spongeDb) Delete(key []byte) error                  { panic("implement me") }
func (s *spongeDb) NewBatch() ethdb.Batch                    { return &spongeBatch{s} }
func (s *spongeDb) NewBatchWithSize(size int) ethdb.Batch    { return &spongeBatch{s} }
func (s *spongeDb) NewSnapshot() (ethdb.Snapshot, error)     { panic("implement me") }
func (s *spongeDb) Stat(property string) (string, error)     { panic("implement me") }
func (s *spongeDb) Compact(start []byte, limit []byte) error { panic("implement me") }
func (s *spongeDb) Close() error                             { return nil }

func (s *spongeDb) Put(key []byte, value []byte) error {
	if s.debug {
		fmt.Printf("db.Put %x : %x\n", key, value)
	}
	s.sponge.Write(key)
	s.sponge.Write(value)
	return nil
}
func (s *spongeDb) NewIterator(prefix []byte, start []byte) ethdb.Iterator { panic("implement me") }

// spongeBatch is a dummy batch which immediately writes to the underlying spongedb
type spongeBatch struct {
	db *spongeDb
}

func (b *spongeBatch) Put(key, value []byte) error {
	b.db.Put(key, value)
	return nil
}
func (b *spongeBatch) Delete(key []byte) error             { panic("implement me") }
func (b *spongeBatch) ValueSize() int                      { return 100 }
func (b *spongeBatch) Write() error                        { return nil }
func (b *spongeBatch) Reset()                              {}
func (b *spongeBatch) Replay(w ethdb.KeyValueWriter) error { return nil }

type kv struct {
	k, v []byte
}
type kvs []kv

func (k kvs) Len() int {
	return len(k)
}

func (k kvs) Less(i, j int) bool {
	return bytes.Compare(k[i].k, k[j].k) < 0
}

func (k kvs) Swap(i, j int) {
	k[j], k[i] = k[i], k[j]
}

// Fuzz is the fuzzing entry-point.
// The function must return
//
//   - 1 if the fuzzer should increase priority of the
//     given input during subsequent fuzzing (for example, the input is lexically
//     correct and was parsed successfully);
//   - -1 if the input must not be added to corpus even if gives new coverage; and
//   - 0 otherwise
//
// other values are reserved for future use.
func Fuzz(data []byte) int {
	f := fuzzer{
		input:     bytes.NewReader(data),
		exhausted: false,
	}
	return f.fuzz()
}

func Debug(data []byte) int {
	f := fuzzer{
		input:     bytes.NewReader(data),
		exhausted: false,
		debugging: true,
	}
	return f.fuzz()
}

func (f *fuzzer) fuzz() int {
	// This spongeDb is used to check the sequence of disk-db-writes
	var (
		spongeA = &spongeDb{sponge: sha3.NewLegacyKeccak256()}
		dbA     = trie.NewDatabase(rawdb.NewDatabase(spongeA))
		trieA   = trie.NewEmpty(dbA)
		spongeB = &spongeDb{sponge: sha3.NewLegacyKeccak256()}
		dbB     = trie.NewDatabase(rawdb.NewDatabase(spongeB))
		trieB   = trie.NewStackTrie(func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
			rawdb.WriteTrieNode(spongeB, owner, path, hash, blob, dbB.Scheme())
		})
		vals        kvs
		useful      bool
		maxElements = 10000
		// operate on unique keys only
		keys = make(map[string]struct{})
	)
	// Fill the trie with elements
	for i := 0; !f.exhausted && i < maxElements; i++ {
		k := f.read(32)
		v := f.readSlice(1, 500)
		if f.exhausted {
			// If it was exhausted while reading, the value may be all zeroes,
			// thus 'deletion' which is not supported on stacktrie
			break
		}
		if _, present := keys[string(k)]; present {
			// This key is a duplicate, ignore it
			continue
		}
		keys[string(k)] = struct{}{}
		vals = append(vals, kv{k: k, v: v})
		trieA.MustUpdate(k, v)
		useful = true
	}
	if !useful {
		return 0
	}
	// Flush trie -> database
	rootA, nodes := trieA.Commit(false)
	if nodes != nil {
		dbA.Update(trie.NewWithNodeSet(nodes))
	}
	// Flush memdb -> disk (sponge)
	dbA.Commit(rootA, false)

	// Stacktrie requires sorted insertion
	sort.Sort(vals)
	for _, kv := range vals {
		if f.debugging {
			fmt.Printf("{\"%#x\" , \"%#x\"} // stacktrie.Update\n", kv.k, kv.v)
		}
		trieB.MustUpdate(kv.k, kv.v)
	}
	rootB := trieB.Hash()
	trieB.Commit()
	if rootA != rootB {
		panic(fmt.Sprintf("roots differ: (trie) %x != %x (stacktrie)", rootA, rootB))
	}
	sumA := spongeA.sponge.Sum(nil)
	sumB := spongeB.sponge.Sum(nil)
	if !bytes.Equal(sumA, sumB) {
		panic(fmt.Sprintf("sequence differ: (trie) %x != %x (stacktrie)", sumA, sumB))
	}

	// Ensure all the nodes are persisted correctly
	var (
		nodeset = make(map[string][]byte) // path -> blob
		trieC   = trie.NewStackTrie(func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
			if crypto.Keccak256Hash(blob) != hash {
				panic("invalid node blob")
			}
			if owner != (common.Hash{}) {
				panic("invalid node owner")
			}
			nodeset[string(path)] = common.CopyBytes(blob)
		})
		checked int
	)
	for _, kv := range vals {
		trieC.MustUpdate(kv.k, kv.v)
	}
	rootC, _ := trieC.Commit()
	if rootA != rootC {
		panic(fmt.Sprintf("roots differ: (trie) %x != %x (stacktrie)", rootA, rootC))
	}
	trieA, _ = trie.New(trie.TrieID(rootA), dbA)
	iterA := trieA.NodeIterator(nil)
	for iterA.Next(true) {
		if iterA.Hash() == (common.Hash{}) {
			if _, present := nodeset[string(iterA.Path())]; present {
				panic("unexpected tiny node")
			}
			continue
		}
		nodeBlob, present := nodeset[string(iterA.Path())]
		if !present {
			panic("missing node")
		}
		if !bytes.Equal(nodeBlob, iterA.NodeBlob()) {
			panic("node blob is not matched")
		}
		checked += 1
	}
	if checked != len(nodeset) {
		panic("node number is not matched")
	}
	return 1
}
