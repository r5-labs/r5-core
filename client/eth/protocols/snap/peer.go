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
	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/log"
	"github.com/r5-labs/r5-core/client/p2p"
)

// Peer is a collection of relevant information we have about a `snap` peer.
type Peer struct {
	id string // Unique ID for the peer, cached

	*p2p.Peer                   // The embedded P2P package peer
	rw        p2p.MsgReadWriter // Input/output streams for snap
	version   uint              // Protocol version negotiated

	logger log.Logger // Contextual logger with the peer id injected
}

// NewPeer create a wrapper for a network connection and negotiated  protocol
// version.
func NewPeer(version uint, p *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	id := p.ID().String()
	return &Peer{
		id:      id,
		Peer:    p,
		rw:      rw,
		version: version,
		logger:  log.New("peer", id[:8]),
	}
}

// NewFakePeer create a fake snap peer without a backing p2p peer, for testing purposes.
func NewFakePeer(version uint, id string, rw p2p.MsgReadWriter) *Peer {
	return &Peer{
		id:      id,
		rw:      rw,
		version: version,
		logger:  log.New("peer", id[:8]),
	}
}

// ID retrieves the peer's unique identifier.
func (p *Peer) ID() string {
	return p.id
}

// Version retrieves the peer's negotiated `snap` protocol version.
func (p *Peer) Version() uint {
	return p.version
}

// Log overrides the P2P logger with the higher level one containing only the id.
func (p *Peer) Log() log.Logger {
	return p.logger
}

// RequestAccountRange fetches a batch of accounts rooted in a specific account
// trie, starting with the origin.
func (p *Peer) RequestAccountRange(id uint64, root common.Hash, origin, limit common.Hash, bytes uint64) error {
	p.logger.Trace("Fetching range of accounts", "reqid", id, "root", root, "origin", origin, "limit", limit, "bytes", common.StorageSize(bytes))

	requestTracker.Track(p.id, p.version, GetAccountRangeMsg, AccountRangeMsg, id)
	return p2p.Send(p.rw, GetAccountRangeMsg, &GetAccountRangePacket{
		ID:     id,
		Root:   root,
		Origin: origin,
		Limit:  limit,
		Bytes:  bytes,
	})
}

// RequestStorageRanges fetches a batch of storage slots belonging to one or more
// accounts. If slots from only one account is requested, an origin marker may also
// be used to retrieve from there.
func (p *Peer) RequestStorageRanges(id uint64, root common.Hash, accounts []common.Hash, origin, limit []byte, bytes uint64) error {
	if len(accounts) == 1 && origin != nil {
		p.logger.Trace("Fetching range of large storage slots", "reqid", id, "root", root, "account", accounts[0], "origin", common.BytesToHash(origin), "limit", common.BytesToHash(limit), "bytes", common.StorageSize(bytes))
	} else {
		p.logger.Trace("Fetching ranges of small storage slots", "reqid", id, "root", root, "accounts", len(accounts), "first", accounts[0], "bytes", common.StorageSize(bytes))
	}
	requestTracker.Track(p.id, p.version, GetStorageRangesMsg, StorageRangesMsg, id)
	return p2p.Send(p.rw, GetStorageRangesMsg, &GetStorageRangesPacket{
		ID:       id,
		Root:     root,
		Accounts: accounts,
		Origin:   origin,
		Limit:    limit,
		Bytes:    bytes,
	})
}

// RequestByteCodes fetches a batch of bytecodes by hash.
func (p *Peer) RequestByteCodes(id uint64, hashes []common.Hash, bytes uint64) error {
	p.logger.Trace("Fetching set of byte codes", "reqid", id, "hashes", len(hashes), "bytes", common.StorageSize(bytes))

	requestTracker.Track(p.id, p.version, GetByteCodesMsg, ByteCodesMsg, id)
	return p2p.Send(p.rw, GetByteCodesMsg, &GetByteCodesPacket{
		ID:     id,
		Hashes: hashes,
		Bytes:  bytes,
	})
}

// RequestTrieNodes fetches a batch of account or storage trie nodes rooted in
// a specific state trie.
func (p *Peer) RequestTrieNodes(id uint64, root common.Hash, paths []TrieNodePathSet, bytes uint64) error {
	p.logger.Trace("Fetching set of trie nodes", "reqid", id, "root", root, "pathsets", len(paths), "bytes", common.StorageSize(bytes))

	requestTracker.Track(p.id, p.version, GetTrieNodesMsg, TrieNodesMsg, id)
	return p2p.Send(p.rw, GetTrieNodesMsg, &GetTrieNodesPacket{
		ID:    id,
		Root:  root,
		Paths: paths,
		Bytes: bytes,
	})
}
