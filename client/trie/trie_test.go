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

package trie

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"math/big"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/davecgh/go-spew/spew"
	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/rawdb"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/ethdb"
	"github.com/r5-labs/r5-core/client/rlp"
	"golang.org/x/crypto/sha3"
)

func init() {
	spew.Config.Indent = "    "
	spew.Config.DisableMethods = false
}

func TestEmptyTrie(t *testing.T) {
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	res := trie.Hash()
	exp := types.EmptyRootHash
	if res != exp {
		t.Errorf("expected %x got %x", exp, res)
	}
}

func TestNull(t *testing.T) {
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	key := make([]byte, 32)
	value := []byte("test")
	trie.MustUpdate(key, value)
	if !bytes.Equal(trie.MustGet(key), value) {
		t.Fatal("wrong value")
	}
}

func TestMissingRoot(t *testing.T) {
	root := common.HexToHash("0beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33")
	trie, err := New(TrieID(root), NewDatabase(rawdb.NewMemoryDatabase()))
	if trie != nil {
		t.Error("New returned non-nil trie for invalid root")
	}
	if _, ok := err.(*MissingNodeError); !ok {
		t.Errorf("New returned wrong error: %v", err)
	}
}

func TestMissingNodeDisk(t *testing.T)    { testMissingNode(t, false) }
func TestMissingNodeMemonly(t *testing.T) { testMissingNode(t, true) }

func testMissingNode(t *testing.T, memonly bool) {
	diskdb := rawdb.NewMemoryDatabase()
	triedb := NewDatabase(diskdb)

	trie := NewEmpty(triedb)
	updateString(trie, "120000", "qwerqwerqwerqwerqwerqwerqwerqwer")
	updateString(trie, "123456", "asdfasdfasdfasdfasdfasdfasdfasdf")
	root, nodes := trie.Commit(false)
	triedb.Update(NewWithNodeSet(nodes))
	if !memonly {
		triedb.Commit(root, false)
	}

	trie, _ = New(TrieID(root), triedb)
	_, err := trie.Get([]byte("120000"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	trie, _ = New(TrieID(root), triedb)
	_, err = trie.Get([]byte("120099"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	trie, _ = New(TrieID(root), triedb)
	_, err = trie.Get([]byte("123456"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	trie, _ = New(TrieID(root), triedb)
	err = trie.Update([]byte("120099"), []byte("zxcvzxcvzxcvzxcvzxcvzxcvzxcvzxcv"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	trie, _ = New(TrieID(root), triedb)
	err = trie.Delete([]byte("123456"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	hash := common.HexToHash("0xe1d943cc8f061a0c0b98162830b970395ac9315654824bf21b73b891365262f9")
	if memonly {
		delete(triedb.dirties, hash)
	} else {
		diskdb.Delete(hash[:])
	}

	trie, _ = New(TrieID(root), triedb)
	_, err = trie.Get([]byte("120000"))
	if _, ok := err.(*MissingNodeError); !ok {
		t.Errorf("Wrong error: %v", err)
	}
	trie, _ = New(TrieID(root), triedb)
	_, err = trie.Get([]byte("120099"))
	if _, ok := err.(*MissingNodeError); !ok {
		t.Errorf("Wrong error: %v", err)
	}
	trie, _ = New(TrieID(root), triedb)
	_, err = trie.Get([]byte("123456"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	trie, _ = New(TrieID(root), triedb)
	err = trie.Update([]byte("120099"), []byte("zxcv"))
	if _, ok := err.(*MissingNodeError); !ok {
		t.Errorf("Wrong error: %v", err)
	}
	trie, _ = New(TrieID(root), triedb)
	err = trie.Delete([]byte("123456"))
	if _, ok := err.(*MissingNodeError); !ok {
		t.Errorf("Wrong error: %v", err)
	}
}

func TestInsert(t *testing.T) {
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))

	updateString(trie, "doe", "reindeer")
	updateString(trie, "dog", "puppy")
	updateString(trie, "dogglesworth", "cat")

	exp := common.HexToHash("8aad789dff2f538bca5d8ea56e8abe10f4c7ba3a5dea95fea4cd6e7c3a1168d3")
	root := trie.Hash()
	if root != exp {
		t.Errorf("case 1: exp %x got %x", exp, root)
	}

	trie = NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	updateString(trie, "A", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")

	exp = common.HexToHash("d23786fb4a010da3ce639d66d5e904a11dbc02746d1ce25029e53290cabf28ab")
	root, _ = trie.Commit(false)
	if root != exp {
		t.Errorf("case 2: exp %x got %x", exp, root)
	}
}

func TestGet(t *testing.T) {
	db := NewDatabase(rawdb.NewMemoryDatabase())
	trie := NewEmpty(db)
	updateString(trie, "doe", "reindeer")
	updateString(trie, "dog", "puppy")
	updateString(trie, "dogglesworth", "cat")

	for i := 0; i < 2; i++ {
		res := getString(trie, "dog")
		if !bytes.Equal(res, []byte("puppy")) {
			t.Errorf("expected puppy got %x", res)
		}
		unknown := getString(trie, "unknown")
		if unknown != nil {
			t.Errorf("expected nil got %x", unknown)
		}
		if i == 1 {
			return
		}
		root, nodes := trie.Commit(false)
		db.Update(NewWithNodeSet(nodes))
		trie, _ = New(TrieID(root), db)
	}
}

func TestDelete(t *testing.T) {
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	vals := []struct{ k, v string }{
		{"do", "verb"},
		{"ether", "wookiedoo"},
		{"horse", "stallion"},
		{"shaman", "horse"},
		{"doge", "coin"},
		{"ether", ""},
		{"dog", "puppy"},
		{"shaman", ""},
	}
	for _, val := range vals {
		if val.v != "" {
			updateString(trie, val.k, val.v)
		} else {
			deleteString(trie, val.k)
		}
	}

	hash := trie.Hash()
	exp := common.HexToHash("5991bb8c6514148a29db676a14ac506cd2cd5775ace63c30a4fe457715e9ac84")
	if hash != exp {
		t.Errorf("expected %x got %x", exp, hash)
	}
}

func TestEmptyValues(t *testing.T) {
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))

	vals := []struct{ k, v string }{
		{"do", "verb"},
		{"ether", "wookiedoo"},
		{"horse", "stallion"},
		{"shaman", "horse"},
		{"doge", "coin"},
		{"ether", ""},
		{"dog", "puppy"},
		{"shaman", ""},
	}
	for _, val := range vals {
		updateString(trie, val.k, val.v)
	}

	hash := trie.Hash()
	exp := common.HexToHash("5991bb8c6514148a29db676a14ac506cd2cd5775ace63c30a4fe457715e9ac84")
	if hash != exp {
		t.Errorf("expected %x got %x", exp, hash)
	}
}

func TestReplication(t *testing.T) {
	triedb := NewDatabase(rawdb.NewMemoryDatabase())
	trie := NewEmpty(triedb)
	vals := []struct{ k, v string }{
		{"do", "verb"},
		{"ether", "wookiedoo"},
		{"horse", "stallion"},
		{"shaman", "horse"},
		{"doge", "coin"},
		{"dog", "puppy"},
		{"somethingveryoddindeedthis is", "myothernodedata"},
	}
	for _, val := range vals {
		updateString(trie, val.k, val.v)
	}
	exp, nodes := trie.Commit(false)
	triedb.Update(NewWithNodeSet(nodes))

	// create a new trie on top of the database and check that lookups work.
	trie2, err := New(TrieID(exp), triedb)
	if err != nil {
		t.Fatalf("can't recreate trie at %x: %v", exp, err)
	}
	for _, kv := range vals {
		if string(getString(trie2, kv.k)) != kv.v {
			t.Errorf("trie2 doesn't have %q => %q", kv.k, kv.v)
		}
	}
	hash, nodes := trie2.Commit(false)
	if hash != exp {
		t.Errorf("root failure. expected %x got %x", exp, hash)
	}

	// recreate the trie after commit
	if nodes != nil {
		triedb.Update(NewWithNodeSet(nodes))
	}
	trie2, err = New(TrieID(hash), triedb)
	if err != nil {
		t.Fatalf("can't recreate trie at %x: %v", exp, err)
	}
	// perform some insertions on the new trie.
	vals2 := []struct{ k, v string }{
		{"do", "verb"},
		{"ether", "wookiedoo"},
		{"horse", "stallion"},
		// {"shaman", "horse"},
		// {"doge", "coin"},
		// {"ether", ""},
		// {"dog", "puppy"},
		// {"somethingveryoddindeedthis is", "myothernodedata"},
		// {"shaman", ""},
	}
	for _, val := range vals2 {
		updateString(trie2, val.k, val.v)
	}
	if hash := trie2.Hash(); hash != exp {
		t.Errorf("root failure. expected %x got %x", exp, hash)
	}
}

func TestLargeValue(t *testing.T) {
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	trie.MustUpdate([]byte("key1"), []byte{99, 99, 99, 99})
	trie.MustUpdate([]byte("key2"), bytes.Repeat([]byte{1}, 32))
	trie.Hash()
}

// TestRandomCases tests som cases that were found via random fuzzing
func TestRandomCases(t *testing.T) {
	var rt = []randTestStep{
		{op: 6, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 0
		{op: 6, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 1
		{op: 0, key: common.Hex2Bytes("d51b182b95d677e5f1c82508c0228de96b73092d78ce78b2230cd948674f66fd1483bd"), value: common.Hex2Bytes("0000000000000002")},           // step 2
		{op: 2, key: common.Hex2Bytes("c2a38512b83107d665c65235b0250002882ac2022eb00711552354832c5f1d030d0e408e"), value: common.Hex2Bytes("")},                         // step 3
		{op: 3, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 4
		{op: 3, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 5
		{op: 6, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 6
		{op: 3, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 7
		{op: 0, key: common.Hex2Bytes("c2a38512b83107d665c65235b0250002882ac2022eb00711552354832c5f1d030d0e408e"), value: common.Hex2Bytes("0000000000000008")},         // step 8
		{op: 0, key: common.Hex2Bytes("d51b182b95d677e5f1c82508c0228de96b73092d78ce78b2230cd948674f66fd1483bd"), value: common.Hex2Bytes("0000000000000009")},           // step 9
		{op: 2, key: common.Hex2Bytes("fd"), value: common.Hex2Bytes("")},                                                                                               // step 10
		{op: 6, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 11
		{op: 6, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 12
		{op: 0, key: common.Hex2Bytes("fd"), value: common.Hex2Bytes("000000000000000d")},                                                                               // step 13
		{op: 6, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 14
		{op: 1, key: common.Hex2Bytes("c2a38512b83107d665c65235b0250002882ac2022eb00711552354832c5f1d030d0e408e"), value: common.Hex2Bytes("")},                         // step 15
		{op: 3, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 16
		{op: 0, key: common.Hex2Bytes("c2a38512b83107d665c65235b0250002882ac2022eb00711552354832c5f1d030d0e408e"), value: common.Hex2Bytes("0000000000000011")},         // step 17
		{op: 5, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 18
		{op: 3, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 19
		{op: 0, key: common.Hex2Bytes("d51b182b95d677e5f1c82508c0228de96b73092d78ce78b2230cd948674f66fd1483bd"), value: common.Hex2Bytes("0000000000000014")},           // step 20
		{op: 0, key: common.Hex2Bytes("d51b182b95d677e5f1c82508c0228de96b73092d78ce78b2230cd948674f66fd1483bd"), value: common.Hex2Bytes("0000000000000015")},           // step 21
		{op: 0, key: common.Hex2Bytes("c2a38512b83107d665c65235b0250002882ac2022eb00711552354832c5f1d030d0e408e"), value: common.Hex2Bytes("0000000000000016")},         // step 22
		{op: 5, key: common.Hex2Bytes(""), value: common.Hex2Bytes("")},                                                                                                 // step 23
		{op: 1, key: common.Hex2Bytes("980c393656413a15c8da01978ed9f89feb80b502f58f2d640e3a2f5f7a99a7018f1b573befd92053ac6f78fca4a87268"), value: common.Hex2Bytes("")}, // step 24
		{op: 1, key: common.Hex2Bytes("fd"), value: common.Hex2Bytes("")},                                                                                               // step 25
	}
	runRandTest(rt)
}

// randTest performs random trie operations.
// Instances of this test are created by Generate.
type randTest []randTestStep

type randTestStep struct {
	op    int
	key   []byte // for opUpdate, opDelete, opGet
	value []byte // for opUpdate
	err   error  // for debugging
}

const (
	opUpdate = iota
	opDelete
	opGet
	opHash
	opCommit
	opItercheckhash
	opNodeDiff
	opProve
	opMax // boundary value, not an actual op
)

func (randTest) Generate(r *rand.Rand, size int) reflect.Value {
	var allKeys [][]byte
	genKey := func() []byte {
		if len(allKeys) < 2 || r.Intn(100) < 10 {
			// new key
			key := make([]byte, r.Intn(50))
			r.Read(key)
			allKeys = append(allKeys, key)
			return key
		}
		// use existing key
		return allKeys[r.Intn(len(allKeys))]
	}

	var steps randTest
	for i := 0; i < size; i++ {
		step := randTestStep{op: r.Intn(opMax)}
		switch step.op {
		case opUpdate:
			step.key = genKey()
			step.value = make([]byte, 8)
			binary.BigEndian.PutUint64(step.value, uint64(i))
		case opGet, opDelete, opProve:
			step.key = genKey()
		}
		steps = append(steps, step)
	}
	return reflect.ValueOf(steps)
}

func verifyAccessList(old *Trie, new *Trie, set *NodeSet) error {
	deletes, inserts, updates := diffTries(old, new)

	// Check insertion set
	for path := range inserts {
		n, ok := set.nodes[path]
		if !ok || n.isDeleted() {
			return errors.New("expect new node")
		}
		_, ok = set.accessList[path]
		if ok {
			return errors.New("unexpected origin value")
		}
	}
	// Check deletion set
	for path, blob := range deletes {
		n, ok := set.nodes[path]
		if !ok || !n.isDeleted() {
			return errors.New("expect deleted node")
		}
		v, ok := set.accessList[path]
		if !ok {
			return errors.New("expect origin value")
		}
		if !bytes.Equal(v, blob) {
			return errors.New("invalid origin value")
		}
	}
	// Check update set
	for path, blob := range updates {
		n, ok := set.nodes[path]
		if !ok || n.isDeleted() {
			return errors.New("expect updated node")
		}
		v, ok := set.accessList[path]
		if !ok {
			return errors.New("expect origin value")
		}
		if !bytes.Equal(v, blob) {
			return errors.New("invalid origin value")
		}
	}
	return nil
}

func runRandTest(rt randTest) bool {
	var (
		triedb   = NewDatabase(rawdb.NewMemoryDatabase())
		tr       = NewEmpty(triedb)
		values   = make(map[string]string) // tracks content of the trie
		origTrie = NewEmpty(triedb)
	)
	for i, step := range rt {
		// fmt.Printf("{op: %d, key: common.Hex2Bytes(\"%x\"), value: common.Hex2Bytes(\"%x\")}, // step %d\n",
		// 	step.op, step.key, step.value, i)

		switch step.op {
		case opUpdate:
			tr.MustUpdate(step.key, step.value)
			values[string(step.key)] = string(step.value)
		case opDelete:
			tr.MustDelete(step.key)
			delete(values, string(step.key))
		case opGet:
			v := tr.MustGet(step.key)
			want := values[string(step.key)]
			if string(v) != want {
				rt[i].err = fmt.Errorf("mismatch for key %#x, got %#x want %#x", step.key, v, want)
			}
		case opProve:
			hash := tr.Hash()
			if hash == types.EmptyRootHash {
				continue
			}
			proofDb := rawdb.NewMemoryDatabase()
			err := tr.Prove(step.key, 0, proofDb)
			if err != nil {
				rt[i].err = fmt.Errorf("failed for proving key %#x, %v", step.key, err)
			}
			_, err = VerifyProof(hash, step.key, proofDb)
			if err != nil {
				rt[i].err = fmt.Errorf("failed for verifying key %#x, %v", step.key, err)
			}
		case opHash:
			tr.Hash()
		case opCommit:
			root, nodes := tr.Commit(true)
			if nodes != nil {
				triedb.Update(NewWithNodeSet(nodes))
			}
			newtr, err := New(TrieID(root), triedb)
			if err != nil {
				rt[i].err = err
				return false
			}
			if nodes != nil {
				if err := verifyAccessList(origTrie, newtr, nodes); err != nil {
					rt[i].err = err
					return false
				}
			}
			tr = newtr
			origTrie = tr.Copy()
		case opItercheckhash:
			checktr := NewEmpty(triedb)
			it := NewIterator(tr.NodeIterator(nil))
			for it.Next() {
				checktr.MustUpdate(it.Key, it.Value)
			}
			if tr.Hash() != checktr.Hash() {
				rt[i].err = fmt.Errorf("hash mismatch in opItercheckhash")
			}
		case opNodeDiff:
			var (
				origIter = origTrie.NodeIterator(nil)
				curIter  = tr.NodeIterator(nil)
				origSeen = make(map[string]struct{})
				curSeen  = make(map[string]struct{})
			)
			for origIter.Next(true) {
				if origIter.Leaf() {
					continue
				}
				origSeen[string(origIter.Path())] = struct{}{}
			}
			for curIter.Next(true) {
				if curIter.Leaf() {
					continue
				}
				curSeen[string(curIter.Path())] = struct{}{}
			}
			var (
				insertExp = make(map[string]struct{})
				deleteExp = make(map[string]struct{})
			)
			for path := range curSeen {
				_, present := origSeen[path]
				if !present {
					insertExp[path] = struct{}{}
				}
			}
			for path := range origSeen {
				_, present := curSeen[path]
				if !present {
					deleteExp[path] = struct{}{}
				}
			}
			if len(insertExp) != len(tr.tracer.inserts) {
				rt[i].err = fmt.Errorf("insert set mismatch")
			}
			if len(deleteExp) != len(tr.tracer.deletes) {
				rt[i].err = fmt.Errorf("delete set mismatch")
			}
			for insert := range tr.tracer.inserts {
				if _, present := insertExp[insert]; !present {
					rt[i].err = fmt.Errorf("missing inserted node")
				}
			}
			for del := range tr.tracer.deletes {
				if _, present := deleteExp[del]; !present {
					rt[i].err = fmt.Errorf("missing deleted node")
				}
			}
		}
		// Abort the test on error.
		if rt[i].err != nil {
			return false
		}
	}
	return true
}

func TestRandom(t *testing.T) {
	if err := quick.Check(runRandTest, nil); err != nil {
		if cerr, ok := err.(*quick.CheckError); ok {
			t.Fatalf("random test iteration %d failed: %s", cerr.Count, spew.Sdump(cerr.In))
		}
		t.Fatal(err)
	}
}

func BenchmarkGet(b *testing.B)      { benchGet(b) }
func BenchmarkUpdateBE(b *testing.B) { benchUpdate(b, binary.BigEndian) }
func BenchmarkUpdateLE(b *testing.B) { benchUpdate(b, binary.LittleEndian) }

const benchElemCount = 20000

func benchGet(b *testing.B) {
	triedb := NewDatabase(rawdb.NewMemoryDatabase())
	trie := NewEmpty(triedb)
	k := make([]byte, 32)
	for i := 0; i < benchElemCount; i++ {
		binary.LittleEndian.PutUint64(k, uint64(i))
		trie.MustUpdate(k, k)
	}
	binary.LittleEndian.PutUint64(k, benchElemCount/2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trie.MustGet(k)
	}
	b.StopTimer()
}

func benchUpdate(b *testing.B, e binary.ByteOrder) *Trie {
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	k := make([]byte, 32)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		e.PutUint64(k, uint64(i))
		trie.MustUpdate(k, k)
	}
	return trie
}

// Benchmarks the trie hashing. Since the trie caches the result of any operation,
// we cannot use b.N as the number of hashing rounds, since all rounds apart from
// the first one will be NOOP. As such, we'll use b.N as the number of account to
// insert into the trie before measuring the hashing.
// BenchmarkHash-6   	  288680	      4561 ns/op	     682 B/op	       9 allocs/op
// BenchmarkHash-6   	  275095	      4800 ns/op	     685 B/op	       9 allocs/op
// pure hasher:
// BenchmarkHash-6   	  319362	      4230 ns/op	     675 B/op	       9 allocs/op
// BenchmarkHash-6   	  257460	      4674 ns/op	     689 B/op	       9 allocs/op
// With hashing in-between and pure hasher:
// BenchmarkHash-6   	  225417	      7150 ns/op	     982 B/op	      12 allocs/op
// BenchmarkHash-6   	  220378	      6197 ns/op	     983 B/op	      12 allocs/op
// same with old hasher
// BenchmarkHash-6   	  229758	      6437 ns/op	     981 B/op	      12 allocs/op
// BenchmarkHash-6   	  212610	      7137 ns/op	     986 B/op	      12 allocs/op
func BenchmarkHash(b *testing.B) {
	// Create a realistic account trie to hash. We're first adding and hashing N
	// entries, then adding N more.
	addresses, accounts := makeAccounts(2 * b.N)
	// Insert the accounts into the trie and hash it
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	i := 0
	for ; i < len(addresses)/2; i++ {
		trie.MustUpdate(crypto.Keccak256(addresses[i][:]), accounts[i])
	}
	trie.Hash()
	for ; i < len(addresses); i++ {
		trie.MustUpdate(crypto.Keccak256(addresses[i][:]), accounts[i])
	}
	b.ResetTimer()
	b.ReportAllocs()
	//trie.hashRoot(nil, nil)
	trie.Hash()
}

// Benchmarks the trie Commit following a Hash. Since the trie caches the result of any operation,
// we cannot use b.N as the number of hashing rounds, since all rounds apart from
// the first one will be NOOP. As such, we'll use b.N as the number of account to
// insert into the trie before measuring the hashing.
func BenchmarkCommitAfterHash(b *testing.B) {
	b.Run("no-onleaf", func(b *testing.B) {
		benchmarkCommitAfterHash(b, false)
	})
	b.Run("with-onleaf", func(b *testing.B) {
		benchmarkCommitAfterHash(b, true)
	})
}

func benchmarkCommitAfterHash(b *testing.B, collectLeaf bool) {
	// Make the random benchmark deterministic
	addresses, accounts := makeAccounts(b.N)
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	for i := 0; i < len(addresses); i++ {
		trie.MustUpdate(crypto.Keccak256(addresses[i][:]), accounts[i])
	}
	// Insert the accounts into the trie and hash it
	trie.Hash()
	b.ResetTimer()
	b.ReportAllocs()
	trie.Commit(collectLeaf)
}

func TestTinyTrie(t *testing.T) {
	// Create a realistic account trie to hash
	_, accounts := makeAccounts(5)
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	trie.MustUpdate(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000001337"), accounts[3])
	if exp, root := common.HexToHash("8c6a85a4d9fda98feff88450299e574e5378e32391f75a055d470ac0653f1005"), trie.Hash(); exp != root {
		t.Errorf("1: got %x, exp %x", root, exp)
	}
	trie.MustUpdate(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000001338"), accounts[4])
	if exp, root := common.HexToHash("ec63b967e98a5720e7f720482151963982890d82c9093c0d486b7eb8883a66b1"), trie.Hash(); exp != root {
		t.Errorf("2: got %x, exp %x", root, exp)
	}
	trie.MustUpdate(common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000001339"), accounts[4])
	if exp, root := common.HexToHash("0608c1d1dc3905fa22204c7a0e43644831c3b6d3def0f274be623a948197e64a"), trie.Hash(); exp != root {
		t.Errorf("3: got %x, exp %x", root, exp)
	}
	checktr := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	it := NewIterator(trie.NodeIterator(nil))
	for it.Next() {
		checktr.MustUpdate(it.Key, it.Value)
	}
	if troot, itroot := trie.Hash(), checktr.Hash(); troot != itroot {
		t.Fatalf("hash mismatch in opItercheckhash, trie: %x, check: %x", troot, itroot)
	}
}

func TestCommitAfterHash(t *testing.T) {
	// Create a realistic account trie to hash
	addresses, accounts := makeAccounts(1000)
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	for i := 0; i < len(addresses); i++ {
		trie.MustUpdate(crypto.Keccak256(addresses[i][:]), accounts[i])
	}
	// Insert the accounts into the trie and hash it
	trie.Hash()
	trie.Commit(false)
	root := trie.Hash()
	exp := common.HexToHash("72f9d3f3fe1e1dd7b8936442e7642aef76371472d94319900790053c493f3fe6")
	if exp != root {
		t.Errorf("got %x, exp %x", root, exp)
	}
	root, _ = trie.Commit(false)
	if exp != root {
		t.Errorf("got %x, exp %x", root, exp)
	}
}

func makeAccounts(size int) (addresses [][20]byte, accounts [][]byte) {
	// Make the random benchmark deterministic
	random := rand.New(rand.NewSource(0))
	// Create a realistic account trie to hash
	addresses = make([][20]byte, size)
	for i := 0; i < len(addresses); i++ {
		data := make([]byte, 20)
		random.Read(data)
		copy(addresses[i][:], data)
	}
	accounts = make([][]byte, len(addresses))
	for i := 0; i < len(accounts); i++ {
		var (
			nonce = uint64(random.Int63())
			root  = types.EmptyRootHash
			code  = crypto.Keccak256(nil)
		)
		// The big.Rand function is not deterministic with regards to 64 vs 32 bit systems,
		// and will consume different amount of data from the rand source.
		//balance = new(big.Int).Rand(random, new(big.Int).Exp(common.Big2, common.Big256, nil))
		// Therefore, we instead just read via byte buffer
		numBytes := random.Uint32() % 33 // [0, 32] bytes
		balanceBytes := make([]byte, numBytes)
		random.Read(balanceBytes)
		balance := new(big.Int).SetBytes(balanceBytes)
		data, _ := rlp.EncodeToBytes(&types.StateAccount{Nonce: nonce, Balance: balance, Root: root, CodeHash: code})
		accounts[i] = data
	}
	return addresses, accounts
}

// spongeDb is a dummy db backend which accumulates writes in a sponge
type spongeDb struct {
	sponge  hash.Hash
	id      string
	journal []string
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
	valbrief := value
	if len(valbrief) > 8 {
		valbrief = valbrief[:8]
	}
	s.journal = append(s.journal, fmt.Sprintf("%v: PUT([%x...], [%d bytes] %x...)\n", s.id, key[:8], len(value), valbrief))
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

// TestCommitSequence tests that the trie.Commit operation writes the elements of the trie
// in the expected order.
// The test data was based on the 'master' code, and is basically random. It can be used
// to check whether changes to the trie modifies the write order or data in any way.
func TestCommitSequence(t *testing.T) {
	for i, tc := range []struct {
		count           int
		expWriteSeqHash []byte
	}{
		{20, common.FromHex("873c78df73d60e59d4a2bcf3716e8bfe14554549fea2fc147cb54129382a8066")},
		{200, common.FromHex("ba03d891bb15408c940eea5ee3d54d419595102648d02774a0268d892add9c8e")},
		{2000, common.FromHex("f7a184f20df01c94f09537401d11e68d97ad0c00115233107f51b9c287ce60c7")},
	} {
		addresses, accounts := makeAccounts(tc.count)
		// This spongeDb is used to check the sequence of disk-db-writes
		s := &spongeDb{sponge: sha3.NewLegacyKeccak256()}
		db := NewDatabase(rawdb.NewDatabase(s))
		trie := NewEmpty(db)
		// Fill the trie with elements
		for i := 0; i < tc.count; i++ {
			trie.MustUpdate(crypto.Keccak256(addresses[i][:]), accounts[i])
		}
		// Flush trie -> database
		root, nodes := trie.Commit(false)
		db.Update(NewWithNodeSet(nodes))
		// Flush memdb -> disk (sponge)
		db.Commit(root, false)
		if got, exp := s.sponge.Sum(nil), tc.expWriteSeqHash; !bytes.Equal(got, exp) {
			t.Errorf("test %d, disk write sequence wrong:\ngot %x exp %x\n", i, got, exp)
		}
	}
}

// TestCommitSequenceRandomBlobs is identical to TestCommitSequence
// but uses random blobs instead of 'accounts'
func TestCommitSequenceRandomBlobs(t *testing.T) {
	for i, tc := range []struct {
		count           int
		expWriteSeqHash []byte
	}{
		{20, common.FromHex("8e4a01548551d139fa9e833ebc4e66fc1ba40a4b9b7259d80db32cff7b64ebbc")},
		{200, common.FromHex("6869b4e7b95f3097a19ddb30ff735f922b915314047e041614df06958fc50554")},
		{2000, common.FromHex("444200e6f4e2df49f77752f629a96ccf7445d4698c164f962bbd85a0526ef424")},
	} {
		prng := rand.New(rand.NewSource(int64(i)))
		// This spongeDb is used to check the sequence of disk-db-writes
		s := &spongeDb{sponge: sha3.NewLegacyKeccak256()}
		db := NewDatabase(rawdb.NewDatabase(s))
		trie := NewEmpty(db)
		// Fill the trie with elements
		for i := 0; i < tc.count; i++ {
			key := make([]byte, 32)
			var val []byte
			// 50% short elements, 50% large elements
			if prng.Intn(2) == 0 {
				val = make([]byte, 1+prng.Intn(32))
			} else {
				val = make([]byte, 1+prng.Intn(4096))
			}
			prng.Read(key)
			prng.Read(val)
			trie.MustUpdate(key, val)
		}
		// Flush trie -> database
		root, nodes := trie.Commit(false)
		db.Update(NewWithNodeSet(nodes))
		// Flush memdb -> disk (sponge)
		db.Commit(root, false)
		if got, exp := s.sponge.Sum(nil), tc.expWriteSeqHash; !bytes.Equal(got, exp) {
			t.Fatalf("test %d, disk write sequence wrong:\ngot %x exp %x\n", i, got, exp)
		}
	}
}

func TestCommitSequenceStackTrie(t *testing.T) {
	for count := 1; count < 200; count++ {
		prng := rand.New(rand.NewSource(int64(count)))
		// This spongeDb is used to check the sequence of disk-db-writes
		s := &spongeDb{sponge: sha3.NewLegacyKeccak256(), id: "a"}
		db := NewDatabase(rawdb.NewDatabase(s))
		trie := NewEmpty(db)
		// Another sponge is used for the stacktrie commits
		stackTrieSponge := &spongeDb{sponge: sha3.NewLegacyKeccak256(), id: "b"}
		stTrie := NewStackTrie(func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
			rawdb.WriteTrieNode(stackTrieSponge, owner, path, hash, blob, db.Scheme())
		})
		// Fill the trie with elements
		for i := 0; i < count; i++ {
			// For the stack trie, we need to do inserts in proper order
			key := make([]byte, 32)
			binary.BigEndian.PutUint64(key, uint64(i))
			var val []byte
			// 50% short elements, 50% large elements
			if prng.Intn(2) == 0 {
				val = make([]byte, 1+prng.Intn(32))
			} else {
				val = make([]byte, 1+prng.Intn(1024))
			}
			prng.Read(val)
			trie.Update(key, val)
			stTrie.Update(key, val)
		}
		// Flush trie -> database
		root, nodes := trie.Commit(false)
		// Flush memdb -> disk (sponge)
		db.Update(NewWithNodeSet(nodes))
		db.Commit(root, false)
		// And flush stacktrie -> disk
		stRoot, err := stTrie.Commit()
		if err != nil {
			t.Fatalf("Failed to commit stack trie %v", err)
		}
		if stRoot != root {
			t.Fatalf("root wrong, got %x exp %x", stRoot, root)
		}
		if got, exp := stackTrieSponge.sponge.Sum(nil), s.sponge.Sum(nil); !bytes.Equal(got, exp) {
			// Show the journal
			t.Logf("Expected:")
			for i, v := range s.journal {
				t.Logf("op %d: %v", i, v)
			}
			t.Logf("Stacktrie:")
			for i, v := range stackTrieSponge.journal {
				t.Logf("op %d: %v", i, v)
			}
			t.Fatalf("test %d, disk write sequence wrong:\ngot %x exp %x\n", count, got, exp)
		}
	}
}

// TestCommitSequenceSmallRoot tests that a trie which is essentially only a
// small (<32 byte) shortnode with an included value is properly committed to a
// database.
// This case might not matter, since in practice, all keys are 32 bytes, which means
// that even a small trie which contains a leaf will have an extension making it
// not fit into 32 bytes, rlp-encoded. However, it's still the correct thing to do.
func TestCommitSequenceSmallRoot(t *testing.T) {
	s := &spongeDb{sponge: sha3.NewLegacyKeccak256(), id: "a"}
	db := NewDatabase(rawdb.NewDatabase(s))
	trie := NewEmpty(db)
	// Another sponge is used for the stacktrie commits
	stackTrieSponge := &spongeDb{sponge: sha3.NewLegacyKeccak256(), id: "b"}
	stTrie := NewStackTrie(func(owner common.Hash, path []byte, hash common.Hash, blob []byte) {
		rawdb.WriteTrieNode(stackTrieSponge, owner, path, hash, blob, db.Scheme())
	})
	// Add a single small-element to the trie(s)
	key := make([]byte, 5)
	key[0] = 1
	trie.Update(key, []byte{0x1})
	stTrie.Update(key, []byte{0x1})
	// Flush trie -> database
	root, nodes := trie.Commit(false)
	// Flush memdb -> disk (sponge)
	db.Update(NewWithNodeSet(nodes))
	db.Commit(root, false)
	// And flush stacktrie -> disk
	stRoot, err := stTrie.Commit()
	if err != nil {
		t.Fatalf("Failed to commit stack trie %v", err)
	}
	if stRoot != root {
		t.Fatalf("root wrong, got %x exp %x", stRoot, root)
	}

	t.Logf("root: %x\n", stRoot)
	if got, exp := stackTrieSponge.sponge.Sum(nil), s.sponge.Sum(nil); !bytes.Equal(got, exp) {
		t.Fatalf("test, disk write sequence wrong:\ngot %x exp %x\n", got, exp)
	}
}

// BenchmarkCommitAfterHashFixedSize benchmarks the Commit (after Hash) of a fixed number of updates to a trie.
// This benchmark is meant to capture the difference on efficiency of small versus large changes. Typically,
// storage tries are small (a couple of entries), whereas the full post-block account trie update is large (a couple
// of thousand entries)
func BenchmarkHashFixedSize(b *testing.B) {
	b.Run("10", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(20)
		for i := 0; i < b.N; i++ {
			benchmarkHashFixedSize(b, acc, add)
		}
	})
	b.Run("100", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(100)
		for i := 0; i < b.N; i++ {
			benchmarkHashFixedSize(b, acc, add)
		}
	})

	b.Run("1K", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(1000)
		for i := 0; i < b.N; i++ {
			benchmarkHashFixedSize(b, acc, add)
		}
	})
	b.Run("10K", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(10000)
		for i := 0; i < b.N; i++ {
			benchmarkHashFixedSize(b, acc, add)
		}
	})
	b.Run("100K", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(100000)
		for i := 0; i < b.N; i++ {
			benchmarkHashFixedSize(b, acc, add)
		}
	})
}

func benchmarkHashFixedSize(b *testing.B, addresses [][20]byte, accounts [][]byte) {
	b.ReportAllocs()
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	for i := 0; i < len(addresses); i++ {
		trie.MustUpdate(crypto.Keccak256(addresses[i][:]), accounts[i])
	}
	// Insert the accounts into the trie and hash it
	b.StartTimer()
	trie.Hash()
	b.StopTimer()
}

func BenchmarkCommitAfterHashFixedSize(b *testing.B) {
	b.Run("10", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(20)
		for i := 0; i < b.N; i++ {
			benchmarkCommitAfterHashFixedSize(b, acc, add)
		}
	})
	b.Run("100", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(100)
		for i := 0; i < b.N; i++ {
			benchmarkCommitAfterHashFixedSize(b, acc, add)
		}
	})

	b.Run("1K", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(1000)
		for i := 0; i < b.N; i++ {
			benchmarkCommitAfterHashFixedSize(b, acc, add)
		}
	})
	b.Run("10K", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(10000)
		for i := 0; i < b.N; i++ {
			benchmarkCommitAfterHashFixedSize(b, acc, add)
		}
	})
	b.Run("100K", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(100000)
		for i := 0; i < b.N; i++ {
			benchmarkCommitAfterHashFixedSize(b, acc, add)
		}
	})
}

func benchmarkCommitAfterHashFixedSize(b *testing.B, addresses [][20]byte, accounts [][]byte) {
	b.ReportAllocs()
	trie := NewEmpty(NewDatabase(rawdb.NewMemoryDatabase()))
	for i := 0; i < len(addresses); i++ {
		trie.MustUpdate(crypto.Keccak256(addresses[i][:]), accounts[i])
	}
	// Insert the accounts into the trie and hash it
	trie.Hash()
	b.StartTimer()
	trie.Commit(false)
	b.StopTimer()
}

func BenchmarkDerefRootFixedSize(b *testing.B) {
	b.Run("10", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(20)
		for i := 0; i < b.N; i++ {
			benchmarkDerefRootFixedSize(b, acc, add)
		}
	})
	b.Run("100", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(100)
		for i := 0; i < b.N; i++ {
			benchmarkDerefRootFixedSize(b, acc, add)
		}
	})

	b.Run("1K", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(1000)
		for i := 0; i < b.N; i++ {
			benchmarkDerefRootFixedSize(b, acc, add)
		}
	})
	b.Run("10K", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(10000)
		for i := 0; i < b.N; i++ {
			benchmarkDerefRootFixedSize(b, acc, add)
		}
	})
	b.Run("100K", func(b *testing.B) {
		b.StopTimer()
		acc, add := makeAccounts(100000)
		for i := 0; i < b.N; i++ {
			benchmarkDerefRootFixedSize(b, acc, add)
		}
	})
}

func benchmarkDerefRootFixedSize(b *testing.B, addresses [][20]byte, accounts [][]byte) {
	b.ReportAllocs()
	triedb := NewDatabase(rawdb.NewMemoryDatabase())
	trie := NewEmpty(triedb)
	for i := 0; i < len(addresses); i++ {
		trie.MustUpdate(crypto.Keccak256(addresses[i][:]), accounts[i])
	}
	h := trie.Hash()
	_, nodes := trie.Commit(false)
	triedb.Update(NewWithNodeSet(nodes))
	b.StartTimer()
	triedb.Dereference(h)
	b.StopTimer()
}

func getString(trie *Trie, k string) []byte {
	return trie.MustGet([]byte(k))
}

func updateString(trie *Trie, k, v string) {
	trie.MustUpdate([]byte(k), []byte(v))
}

func deleteString(trie *Trie, k string) {
	trie.MustDelete([]byte(k))
}

func TestDecodeNode(t *testing.T) {
	t.Parallel()

	var (
		hash  = make([]byte, 20)
		elems = make([]byte, 20)
	)
	for i := 0; i < 5000000; i++ {
		prng.Read(hash)
		prng.Read(elems)
		decodeNode(hash, elems)
	}
}
