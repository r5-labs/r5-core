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

// Contains the active peer-set of the downloader, maintaining both failures
// as well as reputation metrics to prioritize the block retrievals.

package downloader

import (
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/eth/protocols/eth"
	"github.com/r5-labs/r5-core/client/event"
	"github.com/r5-labs/r5-core/client/log"
	"github.com/r5-labs/r5-core/client/p2p/msgrate"
)

const (
	maxLackingHashes = 4096 // Maximum number of entries allowed on the list or lacking items
)

var (
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
)

// peerConnection represents an active peer from which hashes and blocks are retrieved.
type peerConnection struct {
	id string // Unique identifier of the peer

	rates   *msgrate.Tracker         // Tracker to hone in on the number of items retrievable per second
	lacking map[common.Hash]struct{} // Set of hashes not to request (didn't have previously)

	peer Peer

	version uint       // Eth protocol version number to switch strategies
	log     log.Logger // Contextual logger to add extra infos to peer logs
	lock    sync.RWMutex
}

// LightPeer encapsulates the methods required to synchronise with a remote light peer.
type LightPeer interface {
	Head() (common.Hash, *big.Int)
	RequestHeadersByHash(common.Hash, int, int, bool, chan *eth.Response) (*eth.Request, error)
	RequestHeadersByNumber(uint64, int, int, bool, chan *eth.Response) (*eth.Request, error)
}

// Peer encapsulates the methods required to synchronise with a remote full peer.
type Peer interface {
	LightPeer
	RequestBodies([]common.Hash, chan *eth.Response) (*eth.Request, error)
	RequestReceipts([]common.Hash, chan *eth.Response) (*eth.Request, error)
}

// lightPeerWrapper wraps a LightPeer struct, stubbing out the Peer-only methods.
type lightPeerWrapper struct {
	peer LightPeer
}

func (w *lightPeerWrapper) Head() (common.Hash, *big.Int) { return w.peer.Head() }
func (w *lightPeerWrapper) RequestHeadersByHash(h common.Hash, amount int, skip int, reverse bool, sink chan *eth.Response) (*eth.Request, error) {
	return w.peer.RequestHeadersByHash(h, amount, skip, reverse, sink)
}
func (w *lightPeerWrapper) RequestHeadersByNumber(i uint64, amount int, skip int, reverse bool, sink chan *eth.Response) (*eth.Request, error) {
	return w.peer.RequestHeadersByNumber(i, amount, skip, reverse, sink)
}
func (w *lightPeerWrapper) RequestBodies([]common.Hash, chan *eth.Response) (*eth.Request, error) {
	panic("RequestBodies not supported in light client mode sync")
}
func (w *lightPeerWrapper) RequestReceipts([]common.Hash, chan *eth.Response) (*eth.Request, error) {
	panic("RequestReceipts not supported in light client mode sync")
}

// newPeerConnection creates a new downloader peer.
func newPeerConnection(id string, version uint, peer Peer, logger log.Logger) *peerConnection {
	return &peerConnection{
		id:      id,
		lacking: make(map[common.Hash]struct{}),
		peer:    peer,
		version: version,
		log:     logger,
	}
}

// Reset clears the internal state of a peer entity.
func (p *peerConnection) Reset() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.lacking = make(map[common.Hash]struct{})
}

// UpdateHeaderRate updates the peer's estimated header retrieval throughput with
// the current measurement.
func (p *peerConnection) UpdateHeaderRate(delivered int, elapsed time.Duration) {
	p.rates.Update(eth.BlockHeadersMsg, elapsed, delivered)
}

// UpdateBodyRate updates the peer's estimated body retrieval throughput with the
// current measurement.
func (p *peerConnection) UpdateBodyRate(delivered int, elapsed time.Duration) {
	p.rates.Update(eth.BlockBodiesMsg, elapsed, delivered)
}

// UpdateReceiptRate updates the peer's estimated receipt retrieval throughput
// with the current measurement.
func (p *peerConnection) UpdateReceiptRate(delivered int, elapsed time.Duration) {
	p.rates.Update(eth.ReceiptsMsg, elapsed, delivered)
}

// HeaderCapacity retrieves the peer's header download allowance based on its
// previously discovered throughput.
func (p *peerConnection) HeaderCapacity(targetRTT time.Duration) int {
	cap := p.rates.Capacity(eth.BlockHeadersMsg, targetRTT)
	if cap > MaxHeaderFetch {
		cap = MaxHeaderFetch
	}
	return cap
}

// BodyCapacity retrieves the peer's body download allowance based on its
// previously discovered throughput.
func (p *peerConnection) BodyCapacity(targetRTT time.Duration) int {
	cap := p.rates.Capacity(eth.BlockBodiesMsg, targetRTT)
	if cap > MaxBlockFetch {
		cap = MaxBlockFetch
	}
	return cap
}

// ReceiptCapacity retrieves the peers receipt download allowance based on its
// previously discovered throughput.
func (p *peerConnection) ReceiptCapacity(targetRTT time.Duration) int {
	cap := p.rates.Capacity(eth.ReceiptsMsg, targetRTT)
	if cap > MaxReceiptFetch {
		cap = MaxReceiptFetch
	}
	return cap
}

// MarkLacking appends a new entity to the set of items (blocks, receipts, states)
// that a peer is known not to have (i.e. have been requested before). If the
// set reaches its maximum allowed capacity, items are randomly dropped off.
func (p *peerConnection) MarkLacking(hash common.Hash) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for len(p.lacking) >= maxLackingHashes {
		for drop := range p.lacking {
			delete(p.lacking, drop)
			break
		}
	}
	p.lacking[hash] = struct{}{}
}

// Lacks retrieves whether the hash of a blockchain item is on the peers lacking
// list (i.e. whether we know that the peer does not have it).
func (p *peerConnection) Lacks(hash common.Hash) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	_, ok := p.lacking[hash]
	return ok
}

// peeringEvent is sent on the peer event feed when a remote peer connects or
// disconnects.
type peeringEvent struct {
	peer *peerConnection
	join bool
}

// peerSet represents the collection of active peer participating in the chain
// download procedure.
type peerSet struct {
	peers  map[string]*peerConnection
	rates  *msgrate.Trackers // Set of rate trackers to give the sync a common beat
	events event.Feed        // Feed to publish peer lifecycle events on

	lock sync.RWMutex
}

// newPeerSet creates a new peer set top track the active download sources.
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*peerConnection),
		rates: msgrate.NewTrackers(log.New("proto", "eth")),
	}
}

// SubscribeEvents subscribes to peer arrival and departure events.
func (ps *peerSet) SubscribeEvents(ch chan<- *peeringEvent) event.Subscription {
	return ps.events.Subscribe(ch)
}

// Reset iterates over the current peer set, and resets each of the known peers
// to prepare for a next batch of block retrieval.
func (ps *peerSet) Reset() {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	for _, peer := range ps.peers {
		peer.Reset()
	}
}

// Register injects a new peer into the working set, or returns an error if the
// peer is already known.
//
// The method also sets the starting throughput values of the new peer to the
// average of all existing peers, to give it a realistic chance of being used
// for data retrievals.
func (ps *peerSet) Register(p *peerConnection) error {
	// Register the new peer with some meaningful defaults
	ps.lock.Lock()
	if _, ok := ps.peers[p.id]; ok {
		ps.lock.Unlock()
		return errAlreadyRegistered
	}
	p.rates = msgrate.NewTracker(ps.rates.MeanCapacities(), ps.rates.MedianRoundTrip())
	if err := ps.rates.Track(p.id, p.rates); err != nil {
		ps.lock.Unlock()
		return err
	}
	ps.peers[p.id] = p
	ps.lock.Unlock()

	ps.events.Send(&peeringEvent{peer: p, join: true})
	return nil
}

// Unregister removes a remote peer from the active set, disabling any further
// actions to/from that particular entity.
func (ps *peerSet) Unregister(id string) error {
	ps.lock.Lock()
	p, ok := ps.peers[id]
	if !ok {
		ps.lock.Unlock()
		return errNotRegistered
	}
	delete(ps.peers, id)
	ps.rates.Untrack(id)
	ps.lock.Unlock()

	ps.events.Send(&peeringEvent{peer: p, join: false})
	return nil
}

// Peer retrieves the registered peer with the given id.
func (ps *peerSet) Peer(id string) *peerConnection {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.peers[id]
}

// Len returns if the current number of peers in the set.
func (ps *peerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

// AllPeers retrieves a flat list of all the peers within the set.
func (ps *peerSet) AllPeers() []*peerConnection {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peerConnection, 0, len(ps.peers))
	for _, p := range ps.peers {
		list = append(list, p)
	}
	return list
}

// peerCapacitySort implements sort.Interface.
// It sorts peer connections by capacity (descending).
type peerCapacitySort struct {
	peers []*peerConnection
	caps  []int
}

func (ps *peerCapacitySort) Len() int {
	return len(ps.peers)
}

func (ps *peerCapacitySort) Less(i, j int) bool {
	return ps.caps[i] > ps.caps[j]
}

func (ps *peerCapacitySort) Swap(i, j int) {
	ps.peers[i], ps.peers[j] = ps.peers[j], ps.peers[i]
	ps.caps[i], ps.caps[j] = ps.caps[j], ps.caps[i]
}
