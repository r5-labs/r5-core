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

package client

import (
	"sync"

	"github.com/r5-labs/r5-core/client/p2p/enode"
	"github.com/r5-labs/r5-core/client/p2p/nodestate"
)

// QueueIterator returns nodes from the specified selectable set in the same order as
// they entered the set.
type QueueIterator struct {
	lock sync.Mutex
	cond *sync.Cond

	ns           *nodestate.NodeStateMachine
	queue        []*enode.Node
	nextNode     *enode.Node
	waitCallback func(bool)
	fifo, closed bool
}

// NewQueueIterator creates a new QueueIterator. Nodes are selectable if they have all the required
// and none of the disabled flags set. When a node is selected the selectedFlag is set which also
// disables further selectability until it is removed or times out.
func NewQueueIterator(ns *nodestate.NodeStateMachine, requireFlags, disableFlags nodestate.Flags, fifo bool, waitCallback func(bool)) *QueueIterator {
	qi := &QueueIterator{
		ns:           ns,
		fifo:         fifo,
		waitCallback: waitCallback,
	}
	qi.cond = sync.NewCond(&qi.lock)

	ns.SubscribeState(requireFlags.Or(disableFlags), func(n *enode.Node, oldState, newState nodestate.Flags) {
		oldMatch := oldState.HasAll(requireFlags) && oldState.HasNone(disableFlags)
		newMatch := newState.HasAll(requireFlags) && newState.HasNone(disableFlags)
		if newMatch == oldMatch {
			return
		}

		qi.lock.Lock()
		defer qi.lock.Unlock()

		if newMatch {
			qi.queue = append(qi.queue, n)
		} else {
			id := n.ID()
			for i, qn := range qi.queue {
				if qn.ID() == id {
					copy(qi.queue[i:len(qi.queue)-1], qi.queue[i+1:])
					qi.queue = qi.queue[:len(qi.queue)-1]
					break
				}
			}
		}
		qi.cond.Signal()
	})
	return qi
}

// Next moves to the next selectable node.
func (qi *QueueIterator) Next() bool {
	qi.lock.Lock()
	if !qi.closed && len(qi.queue) == 0 {
		if qi.waitCallback != nil {
			qi.waitCallback(true)
		}
		for !qi.closed && len(qi.queue) == 0 {
			qi.cond.Wait()
		}
		if qi.waitCallback != nil {
			qi.waitCallback(false)
		}
	}
	if qi.closed {
		qi.nextNode = nil
		qi.lock.Unlock()
		return false
	}
	// Move to the next node in queue.
	if qi.fifo {
		qi.nextNode = qi.queue[0]
		copy(qi.queue[:len(qi.queue)-1], qi.queue[1:])
		qi.queue = qi.queue[:len(qi.queue)-1]
	} else {
		qi.nextNode = qi.queue[len(qi.queue)-1]
		qi.queue = qi.queue[:len(qi.queue)-1]
	}
	qi.lock.Unlock()
	return true
}

// Close ends the iterator.
func (qi *QueueIterator) Close() {
	qi.lock.Lock()
	qi.closed = true
	qi.lock.Unlock()
	qi.cond.Signal()
}

// Node returns the current node.
func (qi *QueueIterator) Node() *enode.Node {
	qi.lock.Lock()
	defer qi.lock.Unlock()

	return qi.nextNode
}
