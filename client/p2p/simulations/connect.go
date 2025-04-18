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

package simulations

import (
	"errors"
	"strings"

	"github.com/r5-labs/r5-core/client/p2p/enode"
)

var (
	ErrNodeNotFound = errors.New("node not found")
)

// ConnectToLastNode connects the node with provided NodeID
// to the last node that is up, and avoiding connection to self.
// It is useful when constructing a chain network topology
// when Network adds and removes nodes dynamically.
func (net *Network) ConnectToLastNode(id enode.ID) (err error) {
	net.lock.Lock()
	defer net.lock.Unlock()

	ids := net.getUpNodeIDs()
	l := len(ids)
	if l < 2 {
		return nil
	}
	last := ids[l-1]
	if last == id {
		last = ids[l-2]
	}
	return net.connectNotConnected(last, id)
}

// ConnectToRandomNode connects the node with provided NodeID
// to a random node that is up.
func (net *Network) ConnectToRandomNode(id enode.ID) (err error) {
	net.lock.Lock()
	defer net.lock.Unlock()

	selected := net.getRandomUpNode(id)
	if selected == nil {
		return ErrNodeNotFound
	}
	return net.connectNotConnected(selected.ID(), id)
}

// ConnectNodesFull connects all nodes one to another.
// It provides a complete connectivity in the network
// which should be rarely needed.
func (net *Network) ConnectNodesFull(ids []enode.ID) (err error) {
	net.lock.Lock()
	defer net.lock.Unlock()

	if ids == nil {
		ids = net.getUpNodeIDs()
	}
	for i, lid := range ids {
		for _, rid := range ids[i+1:] {
			if err = net.connectNotConnected(lid, rid); err != nil {
				return err
			}
		}
	}
	return nil
}

// ConnectNodesChain connects all nodes in a chain topology.
// If ids argument is nil, all nodes that are up will be connected.
func (net *Network) ConnectNodesChain(ids []enode.ID) (err error) {
	net.lock.Lock()
	defer net.lock.Unlock()

	return net.connectNodesChain(ids)
}

func (net *Network) connectNodesChain(ids []enode.ID) (err error) {
	if ids == nil {
		ids = net.getUpNodeIDs()
	}
	l := len(ids)
	for i := 0; i < l-1; i++ {
		if err := net.connectNotConnected(ids[i], ids[i+1]); err != nil {
			return err
		}
	}
	return nil
}

// ConnectNodesRing connects all nodes in a ring topology.
// If ids argument is nil, all nodes that are up will be connected.
func (net *Network) ConnectNodesRing(ids []enode.ID) (err error) {
	net.lock.Lock()
	defer net.lock.Unlock()

	if ids == nil {
		ids = net.getUpNodeIDs()
	}
	l := len(ids)
	if l < 2 {
		return nil
	}
	if err := net.connectNodesChain(ids); err != nil {
		return err
	}
	return net.connectNotConnected(ids[l-1], ids[0])
}

// ConnectNodesStar connects all nodes into a star topology
// If ids argument is nil, all nodes that are up will be connected.
func (net *Network) ConnectNodesStar(ids []enode.ID, center enode.ID) (err error) {
	net.lock.Lock()
	defer net.lock.Unlock()

	if ids == nil {
		ids = net.getUpNodeIDs()
	}
	for _, id := range ids {
		if center == id {
			continue
		}
		if err := net.connectNotConnected(center, id); err != nil {
			return err
		}
	}
	return nil
}

func (net *Network) connectNotConnected(oneID, otherID enode.ID) error {
	return ignoreAlreadyConnectedErr(net.connect(oneID, otherID))
}

func ignoreAlreadyConnectedErr(err error) error {
	if err == nil || strings.Contains(err.Error(), "already connected") {
		return nil
	}
	return err
}
