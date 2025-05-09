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

package light

import (
	"context"
	"errors"
	"math/big"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core"
	"github.com/r5-labs/r5-core/client/core/rawdb"
	"github.com/r5-labs/r5-core/client/core/txpool"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/ethdb"
)

// NoOdr is the default context passed to an ODR capable function when the ODR
// service is not required.
var NoOdr = context.Background()

// ErrNoPeers is returned if no peers capable of serving a queued request are available
var ErrNoPeers = errors.New("no suitable peers available")

// OdrBackend is an interface to a backend service that handles ODR retrievals type
type OdrBackend interface {
	Database() ethdb.Database
	ChtIndexer() *core.ChainIndexer
	BloomTrieIndexer() *core.ChainIndexer
	BloomIndexer() *core.ChainIndexer
	Retrieve(ctx context.Context, req OdrRequest) error
	RetrieveTxStatus(ctx context.Context, req *TxStatusRequest) error
	IndexerConfig() *IndexerConfig
}

// OdrRequest is an interface for retrieval requests
type OdrRequest interface {
	StoreResult(db ethdb.Database)
}

// TrieID identifies a state or account storage trie
type TrieID struct {
	BlockHash   common.Hash
	BlockNumber uint64
	StateRoot   common.Hash
	Root        common.Hash
	AccKey      []byte
}

// StateTrieID returns a TrieID for a state trie belonging to a certain block
// header.
func StateTrieID(header *types.Header) *TrieID {
	return &TrieID{
		BlockHash:   header.Hash(),
		BlockNumber: header.Number.Uint64(),
		StateRoot:   header.Root,
		Root:        header.Root,
		AccKey:      nil,
	}
}

// StorageTrieID returns a TrieID for a contract storage trie at a given account
// of a given state trie. It also requires the root hash of the trie for
// checking Merkle proofs.
func StorageTrieID(state *TrieID, addrHash, root common.Hash) *TrieID {
	return &TrieID{
		BlockHash:   state.BlockHash,
		BlockNumber: state.BlockNumber,
		StateRoot:   state.StateRoot,
		AccKey:      addrHash[:],
		Root:        root,
	}
}

// TrieRequest is the ODR request type for state/storage trie entries
type TrieRequest struct {
	Id    *TrieID
	Key   []byte
	Proof *NodeSet
}

// StoreResult stores the retrieved data in local database
func (req *TrieRequest) StoreResult(db ethdb.Database) {
	req.Proof.Store(db)
}

// CodeRequest is the ODR request type for retrieving contract code
type CodeRequest struct {
	Id   *TrieID // references storage trie of the account
	Hash common.Hash
	Data []byte
}

// StoreResult stores the retrieved data in local database
func (req *CodeRequest) StoreResult(db ethdb.Database) {
	rawdb.WriteCode(db, req.Hash, req.Data)
}

// BlockRequest is the ODR request type for retrieving block bodies
type BlockRequest struct {
	Hash   common.Hash
	Number uint64
	Header *types.Header
	Rlp    []byte
}

// StoreResult stores the retrieved data in local database
func (req *BlockRequest) StoreResult(db ethdb.Database) {
	rawdb.WriteBodyRLP(db, req.Hash, req.Number, req.Rlp)
}

// ReceiptsRequest is the ODR request type for retrieving receipts.
type ReceiptsRequest struct {
	Untrusted bool // Indicator whether the result retrieved is trusted or not
	Hash      common.Hash
	Number    uint64
	Header    *types.Header
	Receipts  types.Receipts
}

// StoreResult stores the retrieved data in local database
func (req *ReceiptsRequest) StoreResult(db ethdb.Database) {
	if !req.Untrusted {
		rawdb.WriteReceipts(db, req.Hash, req.Number, req.Receipts)
	}
}

// ChtRequest is the ODR request type for retrieving header by Canonical Hash Trie
type ChtRequest struct {
	Config           *IndexerConfig
	ChtNum, BlockNum uint64
	ChtRoot          common.Hash
	Header           *types.Header
	Td               *big.Int
	Proof            *NodeSet
}

// StoreResult stores the retrieved data in local database
func (req *ChtRequest) StoreResult(db ethdb.Database) {
	hash, num := req.Header.Hash(), req.Header.Number.Uint64()
	rawdb.WriteHeader(db, req.Header)
	rawdb.WriteTd(db, hash, num, req.Td)
	rawdb.WriteCanonicalHash(db, hash, num)
}

// BloomRequest is the ODR request type for retrieving bloom filters from a CHT structure
type BloomRequest struct {
	OdrRequest
	Config           *IndexerConfig
	BloomTrieNum     uint64
	BitIdx           uint
	SectionIndexList []uint64
	BloomTrieRoot    common.Hash
	BloomBits        [][]byte
	Proofs           *NodeSet
}

// StoreResult stores the retrieved data in local database
func (req *BloomRequest) StoreResult(db ethdb.Database) {
	for i, sectionIdx := range req.SectionIndexList {
		sectionHead := rawdb.ReadCanonicalHash(db, (sectionIdx+1)*req.Config.BloomTrieSize-1)
		// if we don't have the canonical hash stored for this section head number, we'll still store it under
		// a key with a zero sectionHead. GetBloomBits will look there too if we still don't have the canonical
		// hash. In the unlikely case we've retrieved the section head hash since then, we'll just retrieve the
		// bit vector again from the network.
		rawdb.WriteBloomBits(db, req.BitIdx, sectionIdx, sectionHead, req.BloomBits[i])
	}
}

// TxStatus describes the status of a transaction
type TxStatus struct {
	Status txpool.TxStatus
	Lookup *rawdb.LegacyTxLookupEntry `rlp:"nil"`
	Error  string
}

// TxStatusRequest is the ODR request type for retrieving transaction status
type TxStatusRequest struct {
	Hashes []common.Hash
	Status []TxStatus
}

// StoreResult stores the retrieved data in local database
func (req *TxStatusRequest) StoreResult(db ethdb.Database) {}
