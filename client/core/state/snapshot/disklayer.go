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

package snapshot

import (
	"bytes"
	"sync"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/rawdb"
	"github.com/r5-labs/r5-core/client/ethdb"
	"github.com/r5-labs/r5-core/client/rlp"
	"github.com/r5-labs/r5-core/client/trie"
)

// diskLayer is a low level persistent snapshot built on top of a key-value store.
type diskLayer struct {
	diskdb ethdb.KeyValueStore // Key-value store containing the base snapshot
	triedb *trie.Database      // Trie node cache for reconstruction purposes
	cache  *fastcache.Cache    // Cache to avoid hitting the disk for direct access

	root  common.Hash // Root hash of the base snapshot
	stale bool        // Signals that the layer became stale (state progressed)

	genMarker  []byte                    // Marker for the state that's indexed during initial layer generation
	genPending chan struct{}             // Notification channel when generation is done (test synchronicity)
	genAbort   chan chan *generatorStats // Notification channel to abort generating the snapshot in this layer

	lock sync.RWMutex
}

// Root returns  root hash for which this snapshot was made.
func (dl *diskLayer) Root() common.Hash {
	return dl.root
}

// Parent always returns nil as there's no layer below the disk.
func (dl *diskLayer) Parent() snapshot {
	return nil
}

// Stale return whether this layer has become stale (was flattened across) or if
// it's still live.
func (dl *diskLayer) Stale() bool {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	return dl.stale
}

// Account directly retrieves the account associated with a particular hash in
// the snapshot slim data format.
func (dl *diskLayer) Account(hash common.Hash) (*Account, error) {
	data, err := dl.AccountRLP(hash)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 { // can be both nil and []byte{}
		return nil, nil
	}
	account := new(Account)
	if err := rlp.DecodeBytes(data, account); err != nil {
		panic(err)
	}
	return account, nil
}

// AccountRLP directly retrieves the account RLP associated with a particular
// hash in the snapshot slim data format.
func (dl *diskLayer) AccountRLP(hash common.Hash) ([]byte, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	// If the layer was flattened into, consider it invalid (any live reference to
	// the original should be marked as unusable).
	if dl.stale {
		return nil, ErrSnapshotStale
	}
	// If the layer is being generated, ensure the requested hash has already been
	// covered by the generator.
	if dl.genMarker != nil && bytes.Compare(hash[:], dl.genMarker) > 0 {
		return nil, ErrNotCoveredYet
	}
	// If we're in the disk layer, all diff layers missed
	snapshotDirtyAccountMissMeter.Mark(1)

	// Try to retrieve the account from the memory cache
	if blob, found := dl.cache.HasGet(nil, hash[:]); found {
		snapshotCleanAccountHitMeter.Mark(1)
		snapshotCleanAccountReadMeter.Mark(int64(len(blob)))
		return blob, nil
	}
	// Cache doesn't contain account, pull from disk and cache for later
	blob := rawdb.ReadAccountSnapshot(dl.diskdb, hash)
	dl.cache.Set(hash[:], blob)

	snapshotCleanAccountMissMeter.Mark(1)
	if n := len(blob); n > 0 {
		snapshotCleanAccountWriteMeter.Mark(int64(n))
	} else {
		snapshotCleanAccountInexMeter.Mark(1)
	}
	return blob, nil
}

// Storage directly retrieves the storage data associated with a particular hash,
// within a particular account.
func (dl *diskLayer) Storage(accountHash, storageHash common.Hash) ([]byte, error) {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	// If the layer was flattened into, consider it invalid (any live reference to
	// the original should be marked as unusable).
	if dl.stale {
		return nil, ErrSnapshotStale
	}
	key := append(accountHash[:], storageHash[:]...)

	// If the layer is being generated, ensure the requested hash has already been
	// covered by the generator.
	if dl.genMarker != nil && bytes.Compare(key, dl.genMarker) > 0 {
		return nil, ErrNotCoveredYet
	}
	// If we're in the disk layer, all diff layers missed
	snapshotDirtyStorageMissMeter.Mark(1)

	// Try to retrieve the storage slot from the memory cache
	if blob, found := dl.cache.HasGet(nil, key); found {
		snapshotCleanStorageHitMeter.Mark(1)
		snapshotCleanStorageReadMeter.Mark(int64(len(blob)))
		return blob, nil
	}
	// Cache doesn't contain storage slot, pull from disk and cache for later
	blob := rawdb.ReadStorageSnapshot(dl.diskdb, accountHash, storageHash)
	dl.cache.Set(key, blob)

	snapshotCleanStorageMissMeter.Mark(1)
	if n := len(blob); n > 0 {
		snapshotCleanStorageWriteMeter.Mark(int64(n))
	} else {
		snapshotCleanStorageInexMeter.Mark(1)
	}
	return blob, nil
}

// Update creates a new layer on top of the existing snapshot diff tree with
// the specified data items. Note, the maps are retained by the method to avoid
// copying everything.
func (dl *diskLayer) Update(blockHash common.Hash, destructs map[common.Hash]struct{}, accounts map[common.Hash][]byte, storage map[common.Hash]map[common.Hash][]byte) *diffLayer {
	return newDiffLayer(dl, blockHash, destructs, accounts, storage)
}
