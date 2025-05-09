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

// Package simulations simulates p2p networks.
// A mocker simulates starting and stopping real nodes in a network.
package simulations

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/r5-labs/r5-core/client/log"
	"github.com/r5-labs/r5-core/client/p2p/enode"
	"github.com/r5-labs/r5-core/client/p2p/simulations/adapters"
)

// a map of mocker names to its function
var mockerList = map[string]func(net *Network, quit chan struct{}, nodeCount int){
	"startStop":     startStop,
	"probabilistic": probabilistic,
	"boot":          boot,
}

// LookupMocker looks a mocker by its name, returns the mockerFn
func LookupMocker(mockerType string) func(net *Network, quit chan struct{}, nodeCount int) {
	return mockerList[mockerType]
}

// GetMockerList returns a list of mockers (keys of the map)
// Useful for frontend to build available mocker selection
func GetMockerList() []string {
	list := make([]string, 0, len(mockerList))
	for k := range mockerList {
		list = append(list, k)
	}
	return list
}

// The boot mockerFn only connects the node in a ring and doesn't do anything else
func boot(net *Network, quit chan struct{}, nodeCount int) {
	_, err := connectNodesInRing(net, nodeCount)
	if err != nil {
		panic("Could not startup node network for mocker")
	}
}

// The startStop mockerFn stops and starts nodes in a defined period (ticker)
func startStop(net *Network, quit chan struct{}, nodeCount int) {
	nodes, err := connectNodesInRing(net, nodeCount)
	if err != nil {
		panic("Could not startup node network for mocker")
	}
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-quit:
			log.Info("Terminating simulation loop")
			return
		case <-tick.C:
			id := nodes[rand.Intn(len(nodes))]
			log.Info("stopping node", "id", id)
			if err := net.Stop(id); err != nil {
				log.Error("error stopping node", "id", id, "err", err)
				return
			}

			select {
			case <-quit:
				log.Info("Terminating simulation loop")
				return
			case <-time.After(3 * time.Second):
			}

			log.Debug("starting node", "id", id)
			if err := net.Start(id); err != nil {
				log.Error("error starting node", "id", id, "err", err)
				return
			}
		}
	}
}

// The probabilistic mocker func has a more probabilistic pattern
// (the implementation could probably be improved):
// nodes are connected in a ring, then a varying number of random nodes is selected,
// mocker then stops and starts them in random intervals, and continues the loop
func probabilistic(net *Network, quit chan struct{}, nodeCount int) {
	nodes, err := connectNodesInRing(net, nodeCount)
	if err != nil {
		select {
		case <-quit:
			//error may be due to abortion of mocking; so the quit channel is closed
			return
		default:
			panic("Could not startup node network for mocker")
		}
	}
	for {
		select {
		case <-quit:
			log.Info("Terminating simulation loop")
			return
		default:
		}
		var lowid, highid int
		var wg sync.WaitGroup
		randWait := time.Duration(rand.Intn(5000)+1000) * time.Millisecond
		rand1 := rand.Intn(nodeCount - 1)
		rand2 := rand.Intn(nodeCount - 1)
		if rand1 <= rand2 {
			lowid = rand1
			highid = rand2
		} else if rand1 > rand2 {
			highid = rand1
			lowid = rand2
		}
		var steps = highid - lowid
		wg.Add(steps)
		for i := lowid; i < highid; i++ {
			select {
			case <-quit:
				log.Info("Terminating simulation loop")
				return
			case <-time.After(randWait):
			}
			log.Debug(fmt.Sprintf("node %v shutting down", nodes[i]))
			err := net.Stop(nodes[i])
			if err != nil {
				log.Error("Error stopping node", "node", nodes[i])
				wg.Done()
				continue
			}
			go func(id enode.ID) {
				time.Sleep(randWait)
				err := net.Start(id)
				if err != nil {
					log.Error("Error starting node", "node", id)
				}
				wg.Done()
			}(nodes[i])
		}
		wg.Wait()
	}
}

// connect nodeCount number of nodes in a ring
func connectNodesInRing(net *Network, nodeCount int) ([]enode.ID, error) {
	ids := make([]enode.ID, nodeCount)
	for i := 0; i < nodeCount; i++ {
		conf := adapters.RandomNodeConfig()
		node, err := net.NewNodeWithConfig(conf)
		if err != nil {
			log.Error("Error creating a node!", "err", err)
			return nil, err
		}
		ids[i] = node.ID()
	}

	for _, id := range ids {
		if err := net.Start(id); err != nil {
			log.Error("Error starting a node!", "err", err)
			return nil, err
		}
		log.Debug(fmt.Sprintf("node %v starting up", id))
	}
	for i, id := range ids {
		peerID := ids[(i+1)%len(ids)]
		if err := net.Connect(id, peerID); err != nil {
			log.Error("Error connecting a node to a peer!", "err", err)
			return nil, err
		}
	}

	return ids, nil
}
