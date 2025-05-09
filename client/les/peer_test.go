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

package les

import (
	"crypto/rand"
	"errors"
	"math/big"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core"
	"github.com/r5-labs/r5-core/client/core/forkid"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/p2p"
	"github.com/r5-labs/r5-core/client/p2p/enode"
	"github.com/r5-labs/r5-core/client/params"
)

type testServerPeerSub struct {
	regCh   chan *serverPeer
	unregCh chan *serverPeer
}

func newTestServerPeerSub() *testServerPeerSub {
	return &testServerPeerSub{
		regCh:   make(chan *serverPeer, 1),
		unregCh: make(chan *serverPeer, 1),
	}
}

func (t *testServerPeerSub) registerPeer(p *serverPeer)   { t.regCh <- p }
func (t *testServerPeerSub) unregisterPeer(p *serverPeer) { t.unregCh <- p }

func TestPeerSubscription(t *testing.T) {
	peers := newServerPeerSet()
	defer peers.close()

	checkIds := func(expect []string) {
		given := peers.ids()
		if len(given) == 0 && len(expect) == 0 {
			return
		}
		sort.Strings(given)
		sort.Strings(expect)
		if !reflect.DeepEqual(given, expect) {
			t.Fatalf("all peer ids mismatch, want %v, given %v", expect, given)
		}
	}
	checkPeers := func(peerCh chan *serverPeer) {
		select {
		case <-peerCh:
		case <-time.NewTimer(100 * time.Millisecond).C:
			t.Fatalf("timeout, no event received")
		}
		select {
		case <-peerCh:
			t.Fatalf("unexpected event received")
		case <-time.NewTimer(10 * time.Millisecond).C:
		}
	}
	checkIds([]string{})

	sub := newTestServerPeerSub()
	peers.subscribe(sub)

	// Generate a random id and create the peer
	var id enode.ID
	rand.Read(id[:])
	peer := newServerPeer(2, NetworkId, false, p2p.NewPeer(id, "name", nil), nil)
	peers.register(peer)

	checkIds([]string{peer.id})
	checkPeers(sub.regCh)

	peers.unregister(peer.id)
	checkIds([]string{})
	checkPeers(sub.unregCh)
}

type fakeChain struct{}

func (f *fakeChain) Config() *params.ChainConfig { return params.MainnetChainConfig }
func (f *fakeChain) Genesis() *types.Block {
	return core.DefaultGenesisBlock().ToBlock()
}
func (f *fakeChain) CurrentHeader() *types.Header { return &types.Header{Number: big.NewInt(10000000)} }

func TestHandshake(t *testing.T) {
	// Create a message pipe to communicate through
	app, net := p2p.MsgPipe()

	// Generate a random id and create the peer
	var id enode.ID
	rand.Read(id[:])

	peer1 := newClientPeer(2, NetworkId, p2p.NewPeer(id, "name", nil), net)
	peer2 := newServerPeer(2, NetworkId, true, p2p.NewPeer(id, "name", nil), app)

	var (
		errCh1 = make(chan error, 1)
		errCh2 = make(chan error, 1)

		td      = big.NewInt(100)
		head    = common.HexToHash("deadbeef")
		headNum = uint64(10)
		genesis = common.HexToHash("cafebabe")

		chain1, chain2   = &fakeChain{}, &fakeChain{}
		forkID1          = forkid.NewID(chain1.Config(), chain1.Genesis().Hash(), chain1.CurrentHeader().Number.Uint64(), chain1.CurrentHeader().Time)
		forkID2          = forkid.NewID(chain2.Config(), chain2.Genesis().Hash(), chain2.CurrentHeader().Number.Uint64(), chain2.CurrentHeader().Time)
		filter1, filter2 = forkid.NewFilter(chain1), forkid.NewFilter(chain2)
	)

	go func() {
		errCh1 <- peer1.handshake(td, head, headNum, genesis, forkID1, filter1, func(list *keyValueList) {
			var announceType uint64 = announceTypeSigned
			*list = (*list).add("announceType", announceType)
		}, nil)
	}()
	go func() {
		errCh2 <- peer2.handshake(td, head, headNum, genesis, forkID2, filter2, nil, func(recv keyValueMap) error {
			var reqType uint64
			err := recv.get("announceType", &reqType)
			if err != nil {
				return err
			}
			if reqType != announceTypeSigned {
				return errors.New("Expected announceTypeSigned")
			}
			return nil
		})
	}()

	for i := 0; i < 2; i++ {
		select {
		case err := <-errCh1:
			if err != nil {
				t.Fatalf("handshake failed, %v", err)
			}
		case err := <-errCh2:
			if err != nil {
				t.Fatalf("handshake failed, %v", err)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}
}
