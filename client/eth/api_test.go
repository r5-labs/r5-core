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
	"bytes"
	"fmt"
	"math/big"
	"reflect"
	"sort"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/rawdb"
	"github.com/r5-labs/r5-core/client/core/state"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/trie"
)

var dumper = spew.ConfigState{Indent: "    "}

func accountRangeTest(t *testing.T, trie *state.Trie, statedb *state.StateDB, start common.Hash, requestedNum int, expectedNum int) state.IteratorDump {
	result := statedb.IteratorDump(&state.DumpConfig{
		SkipCode:          true,
		SkipStorage:       true,
		OnlyWithAddresses: false,
		Start:             start.Bytes(),
		Max:               uint64(requestedNum),
	})

	if len(result.Accounts) != expectedNum {
		t.Fatalf("expected %d results, got %d", expectedNum, len(result.Accounts))
	}
	for address := range result.Accounts {
		if address == (common.Address{}) {
			t.Fatalf("empty address returned")
		}
		if !statedb.Exist(address) {
			t.Fatalf("account not found in state %s", address.Hex())
		}
	}
	return result
}

type resultHash []common.Hash

func (h resultHash) Len() int           { return len(h) }
func (h resultHash) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h resultHash) Less(i, j int) bool { return bytes.Compare(h[i].Bytes(), h[j].Bytes()) < 0 }

func TestAccountRange(t *testing.T) {
	t.Parallel()

	var (
		statedb  = state.NewDatabaseWithConfig(rawdb.NewMemoryDatabase(), &trie.Config{Preimages: true})
		state, _ = state.New(common.Hash{}, statedb, nil)
		addrs    = [AccountRangeMaxResults * 2]common.Address{}
		m        = map[common.Address]bool{}
	)

	for i := range addrs {
		hash := common.HexToHash(fmt.Sprintf("%x", i))
		addr := common.BytesToAddress(crypto.Keccak256Hash(hash.Bytes()).Bytes())
		addrs[i] = addr
		state.SetBalance(addrs[i], big.NewInt(1))
		if _, ok := m[addr]; ok {
			t.Fatalf("bad")
		} else {
			m[addr] = true
		}
	}
	state.Commit(true)
	root := state.IntermediateRoot(true)

	trie, err := statedb.OpenTrie(root)
	if err != nil {
		t.Fatal(err)
	}
	accountRangeTest(t, &trie, state, common.Hash{}, AccountRangeMaxResults/2, AccountRangeMaxResults/2)
	// test pagination
	firstResult := accountRangeTest(t, &trie, state, common.Hash{}, AccountRangeMaxResults, AccountRangeMaxResults)
	secondResult := accountRangeTest(t, &trie, state, common.BytesToHash(firstResult.Next), AccountRangeMaxResults, AccountRangeMaxResults)

	hList := make(resultHash, 0)
	for addr1 := range firstResult.Accounts {
		// If address is empty, then it makes no sense to compare
		// them as they might be two different accounts.
		if addr1 == (common.Address{}) {
			continue
		}
		if _, duplicate := secondResult.Accounts[addr1]; duplicate {
			t.Fatalf("pagination test failed:  results should not overlap")
		}
		hList = append(hList, crypto.Keccak256Hash(addr1.Bytes()))
	}
	// Test to see if it's possible to recover from the middle of the previous
	// set and get an even split between the first and second sets.
	sort.Sort(hList)
	middleH := hList[AccountRangeMaxResults/2]
	middleResult := accountRangeTest(t, &trie, state, middleH, AccountRangeMaxResults, AccountRangeMaxResults)
	missing, infirst, insecond := 0, 0, 0
	for h := range middleResult.Accounts {
		if _, ok := firstResult.Accounts[h]; ok {
			infirst++
		} else if _, ok := secondResult.Accounts[h]; ok {
			insecond++
		} else {
			missing++
		}
	}
	if missing != 0 {
		t.Fatalf("%d hashes in the 'middle' set were neither in the first not the second set", missing)
	}
	if infirst != AccountRangeMaxResults/2 {
		t.Fatalf("Imbalance in the number of first-test results: %d != %d", infirst, AccountRangeMaxResults/2)
	}
	if insecond != AccountRangeMaxResults/2 {
		t.Fatalf("Imbalance in the number of second-test results: %d != %d", insecond, AccountRangeMaxResults/2)
	}
}

func TestEmptyAccountRange(t *testing.T) {
	t.Parallel()

	var (
		statedb = state.NewDatabase(rawdb.NewMemoryDatabase())
		st, _   = state.New(common.Hash{}, statedb, nil)
	)
	st.Commit(true)
	st.IntermediateRoot(true)
	results := st.IteratorDump(&state.DumpConfig{
		SkipCode:          true,
		SkipStorage:       true,
		OnlyWithAddresses: true,
		Max:               uint64(AccountRangeMaxResults),
	})
	if bytes.Equal(results.Next, (common.Hash{}).Bytes()) {
		t.Fatalf("Empty results should not return a second page")
	}
	if len(results.Accounts) != 0 {
		t.Fatalf("Empty state should not return addresses: %v", results.Accounts)
	}
}

func TestStorageRangeAt(t *testing.T) {
	t.Parallel()

	// Create a state where account 0x010000... has a few storage entries.
	var (
		state, _ = state.New(common.Hash{}, state.NewDatabase(rawdb.NewMemoryDatabase()), nil)
		addr     = common.Address{0x01}
		keys     = []common.Hash{ // hashes of Keys of storage
			common.HexToHash("340dd630ad21bf010b4e676dbfa9ba9a02175262d1fa356232cfde6cb5b47ef2"),
			common.HexToHash("426fcb404ab2d5d8e61a3d918108006bbb0a9be65e92235bb10eefbdb6dcd053"),
			common.HexToHash("48078cfed56339ea54962e72c37c7f588fc4f8e5bc173827ba75cb10a63a96a5"),
			common.HexToHash("5723d2c3a83af9b735e3b7f21531e5623d183a9095a56604ead41f3582fdfb75"),
		}
		storage = storageMap{
			keys[0]: {Key: &common.Hash{0x02}, Value: common.Hash{0x01}},
			keys[1]: {Key: &common.Hash{0x04}, Value: common.Hash{0x02}},
			keys[2]: {Key: &common.Hash{0x01}, Value: common.Hash{0x03}},
			keys[3]: {Key: &common.Hash{0x03}, Value: common.Hash{0x04}},
		}
	)
	for _, entry := range storage {
		state.SetState(addr, *entry.Key, entry.Value)
	}

	// Check a few combinations of limit and start/end.
	tests := []struct {
		start []byte
		limit int
		want  StorageRangeResult
	}{
		{
			start: []byte{}, limit: 0,
			want: StorageRangeResult{storageMap{}, &keys[0]},
		},
		{
			start: []byte{}, limit: 100,
			want: StorageRangeResult{storage, nil},
		},
		{
			start: []byte{}, limit: 2,
			want: StorageRangeResult{storageMap{keys[0]: storage[keys[0]], keys[1]: storage[keys[1]]}, &keys[2]},
		},
		{
			start: []byte{0x00}, limit: 4,
			want: StorageRangeResult{storage, nil},
		},
		{
			start: []byte{0x40}, limit: 2,
			want: StorageRangeResult{storageMap{keys[1]: storage[keys[1]], keys[2]: storage[keys[2]]}, &keys[3]},
		},
	}
	for _, test := range tests {
		tr, err := state.StorageTrie(addr)
		if err != nil {
			t.Error(err)
		}
		result, err := storageRangeAt(tr, test.start, test.limit)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(result, test.want) {
			t.Fatalf("wrong result for range %#x.., limit %d:\ngot %s\nwant %s",
				test.start, test.limit, dumper.Sdump(result), dumper.Sdump(&test.want))
		}
	}
}
