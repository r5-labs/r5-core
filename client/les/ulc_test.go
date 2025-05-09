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
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/p2p"
	"github.com/r5-labs/r5-core/client/p2p/enode"
)

func TestULCAnnounceThresholdLes2(t *testing.T) { testULCAnnounceThreshold(t, 2) }
func TestULCAnnounceThresholdLes3(t *testing.T) { testULCAnnounceThreshold(t, 3) }

func testULCAnnounceThreshold(t *testing.T, protocol int) {
	// todo figure out why it takes fetcher so longer to fetcher the announced header.
	t.Skip("Sometimes it can failed")

	// newTestLightPeer creates node with light sync mode
	newTestLightPeer := func(t *testing.T, protocol int, ulcServers []string, ulcFraction int) (*testClient, func()) {
		netconfig := testnetConfig{
			protocol:    protocol,
			ulcServers:  ulcServers,
			ulcFraction: ulcFraction,
			nopruning:   true,
		}
		_, c, teardown := newClientServerEnv(t, netconfig)
		return c, teardown
	}

	var cases = []struct {
		height    []int
		threshold int
		expect    uint64
	}{
		{[]int{1}, 100, 1},
		{[]int{0, 0, 0}, 100, 0},
		{[]int{1, 2, 3}, 30, 3},
		{[]int{1, 2, 3}, 60, 2},
		{[]int{3, 2, 1}, 67, 1},
		{[]int{3, 2, 1}, 100, 1},
	}
	for _, testcase := range cases {
		var (
			servers   []*testServer
			teardowns []func()
			nodes     []*enode.Node
			ids       []string
		)
		for i := 0; i < len(testcase.height); i++ {
			s, n, teardown := newTestServerPeer(t, 0, protocol, nil)

			servers = append(servers, s)
			nodes = append(nodes, n)
			teardowns = append(teardowns, teardown)
			ids = append(ids, n.String())
		}
		c, teardown := newTestLightPeer(t, protocol, ids, testcase.threshold)

		// Connect all servers.
		for i := 0; i < len(servers); i++ {
			connect(servers[i].handler, nodes[i].ID(), c.handler, protocol, false)
		}
		for i := 0; i < len(servers); i++ {
			for j := 0; j < testcase.height[i]; j++ {
				servers[i].backend.Commit()
			}
		}
		time.Sleep(1500 * time.Millisecond) // Ensure the fetcher has done its work.
		head := c.handler.backend.blockchain.CurrentHeader().Number.Uint64()
		if head != testcase.expect {
			t.Fatalf("chain height mismatch, want %d, got %d", testcase.expect, head)
		}

		// Release all servers and client resources.
		teardown()
		for i := 0; i < len(teardowns); i++ {
			teardowns[i]()
		}
	}
}

func connect(server *serverHandler, serverId enode.ID, client *clientHandler, protocol int, noInitAnnounce bool) (*serverPeer, *clientPeer, error) {
	// Create a message pipe to communicate through
	app, net := p2p.MsgPipe()

	var id enode.ID
	rand.Read(id[:])

	peer1 := newServerPeer(protocol, NetworkId, true, p2p.NewPeer(serverId, "", nil), net) // Mark server as trusted
	peer2 := newClientPeer(protocol, NetworkId, p2p.NewPeer(id, "", nil), app)

	// Start the peerLight on a new thread
	errc1 := make(chan error, 1)
	errc2 := make(chan error, 1)
	go func() {
		select {
		case <-server.closeCh:
			errc1 <- p2p.DiscQuitting
		case errc1 <- server.handle(peer2):
		}
	}()
	go func() {
		select {
		case <-client.closeCh:
			errc1 <- p2p.DiscQuitting
		case errc1 <- client.handle(peer1, noInitAnnounce):
		}
	}()
	// Ensure the connection is established or exits when any error occurs
	for {
		select {
		case err := <-errc1:
			return nil, nil, fmt.Errorf("failed to establish protocol connection %v", err)
		case err := <-errc2:
			return nil, nil, fmt.Errorf("failed to establish protocol connection %v", err)
		default:
		}
		if atomic.LoadUint32(&peer1.serving) == 1 && atomic.LoadUint32(&peer2.serving) == 1 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return peer1, peer2, nil
}

// newTestServerPeer creates server peer.
func newTestServerPeer(t *testing.T, blocks int, protocol int, indexFn indexerCallback) (*testServer, *enode.Node, func()) {
	netconfig := testnetConfig{
		blocks:    blocks,
		protocol:  protocol,
		indexFn:   indexFn,
		nopruning: true,
	}
	s, _, teardown := newClientServerEnv(t, netconfig)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal("generate key err:", err)
	}
	s.handler.server.privateKey = key
	n := enode.NewV4(&key.PublicKey, net.ParseIP("127.0.0.1"), 35000, 35000)
	return s, n, teardown
}
