// Copyright 2022 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>

package rawdb

import (
	"fmt"
	"sync"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/ethdb"
	"github.com/r5-labs/r5-core/client/log"
	"golang.org/x/crypto/sha3"
)

// HashScheme is the legacy hash-based state scheme with which trie nodes are
// stored in the disk with node hash as the database key. The advantage of this
// scheme is that different versions of trie nodes can be stored in disk, which
// is very beneficial for constructing archive nodes. The drawback is it will
// store different trie nodes on the same path to different locations on the disk
// with no data locality, and it's unfriendly for designing state pruning.
//
// Now this scheme is still kept for backward compatibility, and it will be used
// for archive node and some other tries(e.g. light trie).
const HashScheme = "hashScheme"

// PathScheme is the new path-based state scheme with which trie nodes are stored
// in the disk with node path as the database key. This scheme will only store one
// version of state data in the disk, which means that the state pruning operation
// is native. At the same time, this scheme will put adjacent trie nodes in the same
// area of the disk with good data locality property. But this scheme needs to rely
// on extra state diffs to survive deep reorg.
const PathScheme = "pathScheme"

// nodeHasher used to derive the hash of trie node.
type nodeHasher struct{ sha crypto.KeccakState }

var hasherPool = sync.Pool{
	New: func() interface{} { return &nodeHasher{sha: sha3.NewLegacyKeccak256().(crypto.KeccakState)} },
}

func newNodeHasher() *nodeHasher       { return hasherPool.Get().(*nodeHasher) }
func returnHasherToPool(h *nodeHasher) { hasherPool.Put(h) }

func (h *nodeHasher) hashData(data []byte) (n common.Hash) {
	h.sha.Reset()
	h.sha.Write(data)
	h.sha.Read(n[:])
	return n
}

// ReadAccountTrieNode retrieves the account trie node and the associated node
// hash with the specified node path.
func ReadAccountTrieNode(db ethdb.KeyValueReader, path []byte) ([]byte, common.Hash) {
	data, err := db.Get(accountTrieNodeKey(path))
	if err != nil {
		return nil, common.Hash{}
	}
	hasher := newNodeHasher()
	defer returnHasherToPool(hasher)
	return data, hasher.hashData(data)
}

// HasAccountTrieNode checks the account trie node presence with the specified
// node path and the associated node hash.
func HasAccountTrieNode(db ethdb.KeyValueReader, path []byte, hash common.Hash) bool {
	data, err := db.Get(accountTrieNodeKey(path))
	if err != nil {
		return false
	}
	hasher := newNodeHasher()
	defer returnHasherToPool(hasher)
	return hasher.hashData(data) == hash
}

// WriteAccountTrieNode writes the provided account trie node into database.
func WriteAccountTrieNode(db ethdb.KeyValueWriter, path []byte, node []byte) {
	if err := db.Put(accountTrieNodeKey(path), node); err != nil {
		log.Crit("Failed to store account trie node", "err", err)
	}
}

// DeleteAccountTrieNode deletes the specified account trie node from the database.
func DeleteAccountTrieNode(db ethdb.KeyValueWriter, path []byte) {
	if err := db.Delete(accountTrieNodeKey(path)); err != nil {
		log.Crit("Failed to delete account trie node", "err", err)
	}
}

// ReadStorageTrieNode retrieves the storage trie node and the associated node
// hash with the specified node path.
func ReadStorageTrieNode(db ethdb.KeyValueReader, accountHash common.Hash, path []byte) ([]byte, common.Hash) {
	data, err := db.Get(storageTrieNodeKey(accountHash, path))
	if err != nil {
		return nil, common.Hash{}
	}
	hasher := newNodeHasher()
	defer returnHasherToPool(hasher)
	return data, hasher.hashData(data)
}

// HasStorageTrieNode checks the storage trie node presence with the provided
// node path and the associated node hash.
func HasStorageTrieNode(db ethdb.KeyValueReader, accountHash common.Hash, path []byte, hash common.Hash) bool {
	data, err := db.Get(storageTrieNodeKey(accountHash, path))
	if err != nil {
		return false
	}
	hasher := newNodeHasher()
	defer returnHasherToPool(hasher)
	return hasher.hashData(data) == hash
}

// WriteStorageTrieNode writes the provided storage trie node into database.
func WriteStorageTrieNode(db ethdb.KeyValueWriter, accountHash common.Hash, path []byte, node []byte) {
	if err := db.Put(storageTrieNodeKey(accountHash, path), node); err != nil {
		log.Crit("Failed to store storage trie node", "err", err)
	}
}

// DeleteStorageTrieNode deletes the specified storage trie node from the database.
func DeleteStorageTrieNode(db ethdb.KeyValueWriter, accountHash common.Hash, path []byte) {
	if err := db.Delete(storageTrieNodeKey(accountHash, path)); err != nil {
		log.Crit("Failed to delete storage trie node", "err", err)
	}
}

// ReadLegacyTrieNode retrieves the legacy trie node with the given
// associated node hash.
func ReadLegacyTrieNode(db ethdb.KeyValueReader, hash common.Hash) []byte {
	data, err := db.Get(hash.Bytes())
	if err != nil {
		return nil
	}
	return data
}

// HasLegacyTrieNode checks if the trie node with the provided hash is present in db.
func HasLegacyTrieNode(db ethdb.KeyValueReader, hash common.Hash) bool {
	ok, _ := db.Has(hash.Bytes())
	return ok
}

// WriteLegacyTrieNode writes the provided legacy trie node to database.
func WriteLegacyTrieNode(db ethdb.KeyValueWriter, hash common.Hash, node []byte) {
	if err := db.Put(hash.Bytes(), node); err != nil {
		log.Crit("Failed to store legacy trie node", "err", err)
	}
}

// DeleteLegacyTrieNode deletes the specified legacy trie node from database.
func DeleteLegacyTrieNode(db ethdb.KeyValueWriter, hash common.Hash) {
	if err := db.Delete(hash.Bytes()); err != nil {
		log.Crit("Failed to delete legacy trie node", "err", err)
	}
}

// HasTrieNode checks the trie node presence with the provided node info and
// the associated node hash.
func HasTrieNode(db ethdb.KeyValueReader, owner common.Hash, path []byte, hash common.Hash, scheme string) bool {
	switch scheme {
	case HashScheme:
		return HasLegacyTrieNode(db, hash)
	case PathScheme:
		if owner == (common.Hash{}) {
			return HasAccountTrieNode(db, path, hash)
		}
		return HasStorageTrieNode(db, owner, path, hash)
	default:
		panic(fmt.Sprintf("Unknown scheme %v", scheme))
	}
}

// ReadTrieNode retrieves the trie node from database with the provided node info
// and associated node hash.
// hashScheme-based lookup requires the following:
//   - hash
//
// pathScheme-based lookup requires the following:
//   - owner
//   - path
func ReadTrieNode(db ethdb.KeyValueReader, owner common.Hash, path []byte, hash common.Hash, scheme string) []byte {
	switch scheme {
	case HashScheme:
		return ReadLegacyTrieNode(db, hash)
	case PathScheme:
		var (
			blob  []byte
			nHash common.Hash
		)
		if owner == (common.Hash{}) {
			blob, nHash = ReadAccountTrieNode(db, path)
		} else {
			blob, nHash = ReadStorageTrieNode(db, owner, path)
		}
		if nHash != hash {
			return nil
		}
		return blob
	default:
		panic(fmt.Sprintf("Unknown scheme %v", scheme))
	}
}

// WriteTrieNode writes the trie node into database with the provided node info
// and associated node hash.
// hashScheme-based lookup requires the following:
//   - hash
//
// pathScheme-based lookup requires the following:
//   - owner
//   - path
func WriteTrieNode(db ethdb.KeyValueWriter, owner common.Hash, path []byte, hash common.Hash, node []byte, scheme string) {
	switch scheme {
	case HashScheme:
		WriteLegacyTrieNode(db, hash, node)
	case PathScheme:
		if owner == (common.Hash{}) {
			WriteAccountTrieNode(db, path, node)
		} else {
			WriteStorageTrieNode(db, owner, path, node)
		}
	default:
		panic(fmt.Sprintf("Unknown scheme %v", scheme))
	}
}

// DeleteTrieNode deletes the trie node from database with the provided node info
// and associated node hash.
// hashScheme-based lookup requires the following:
//   - hash
//
// pathScheme-based lookup requires the following:
//   - owner
//   - path
func DeleteTrieNode(db ethdb.KeyValueWriter, owner common.Hash, path []byte, hash common.Hash, scheme string) {
	switch scheme {
	case HashScheme:
		DeleteLegacyTrieNode(db, hash)
	case PathScheme:
		if owner == (common.Hash{}) {
			DeleteAccountTrieNode(db, path)
		} else {
			DeleteStorageTrieNode(db, owner, path)
		}
	default:
		panic(fmt.Sprintf("Unknown scheme %v", scheme))
	}
}
