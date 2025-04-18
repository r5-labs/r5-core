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

// FillSet tries to read nodes from an input iterator and add them to a node set by
// setting the specified node state flag(s) until the size of the set reaches the target.
// Note that other mechanisms (like other FillSet instances reading from different inputs)
// can also set the same flag(s) and FillSet will always care about the total number of
// nodes having those flags.
type FillSet struct {
	lock          sync.Mutex
	cond          *sync.Cond
	ns            *nodestate.NodeStateMachine
	input         enode.Iterator
	closed        bool
	flags         nodestate.Flags
	count, target int
}

// NewFillSet creates a new FillSet
func NewFillSet(ns *nodestate.NodeStateMachine, input enode.Iterator, flags nodestate.Flags) *FillSet {
	fs := &FillSet{
		ns:    ns,
		input: input,
		flags: flags,
	}
	fs.cond = sync.NewCond(&fs.lock)

	ns.SubscribeState(flags, func(n *enode.Node, oldState, newState nodestate.Flags) {
		fs.lock.Lock()
		if oldState.Equals(flags) {
			fs.count--
		}
		if newState.Equals(flags) {
			fs.count++
		}
		if fs.target > fs.count {
			fs.cond.Signal()
		}
		fs.lock.Unlock()
	})

	go fs.readLoop()
	return fs
}

// readLoop keeps reading nodes from the input and setting the specified flags for them
// whenever the node set size is under the current target
func (fs *FillSet) readLoop() {
	for {
		fs.lock.Lock()
		for fs.target <= fs.count && !fs.closed {
			fs.cond.Wait()
		}

		fs.lock.Unlock()
		if !fs.input.Next() {
			return
		}
		fs.ns.SetState(fs.input.Node(), fs.flags, nodestate.Flags{}, 0)
	}
}

// SetTarget sets the current target for node set size. If the previous target was not
// reached and FillSet was still waiting for the next node from the input then the next
// incoming node will be added to the set regardless of the target. This ensures that
// all nodes coming from the input are eventually added to the set.
func (fs *FillSet) SetTarget(target int) {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	fs.target = target
	if fs.target > fs.count {
		fs.cond.Signal()
	}
}

// Close shuts FillSet down and closes the input iterator
func (fs *FillSet) Close() {
	fs.lock.Lock()
	defer fs.lock.Unlock()

	fs.closed = true
	fs.input.Close()
	fs.cond.Signal()
}
