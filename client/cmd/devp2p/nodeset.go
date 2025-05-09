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

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/p2p/enode"
)

const jsonIndent = "    "

// nodeSet is the nodes.json file format. It holds a set of node records
// as a JSON object.
type nodeSet map[enode.ID]nodeJSON

type nodeJSON struct {
	Seq uint64      `json:"seq"`
	N   *enode.Node `json:"record"`

	// The score tracks how many liveness checks were performed. It is incremented by one
	// every time the node passes a check, and halved every time it doesn't.
	Score int `json:"score,omitempty"`
	// These two track the time of last successful contact.
	FirstResponse time.Time `json:"firstResponse,omitempty"`
	LastResponse  time.Time `json:"lastResponse,omitempty"`
	// This one tracks the time of our last attempt to contact the node.
	LastCheck time.Time `json:"lastCheck,omitempty"`
}

func loadNodesJSON(file string) nodeSet {
	var nodes nodeSet
	if err := common.LoadJSON(file, &nodes); err != nil {
		exit(err)
	}
	return nodes
}

func writeNodesJSON(file string, nodes nodeSet) {
	nodesJSON, err := json.MarshalIndent(nodes, "", jsonIndent)
	if err != nil {
		exit(err)
	}
	if file == "-" {
		os.Stdout.Write(nodesJSON)
		return
	}
	if err := os.WriteFile(file, nodesJSON, 0644); err != nil {
		exit(err)
	}
}

// nodes returns the node records contained in the set.
func (ns nodeSet) nodes() []*enode.Node {
	result := make([]*enode.Node, 0, len(ns))
	for _, n := range ns {
		result = append(result, n.N)
	}
	// Sort by ID.
	sort.Slice(result, func(i, j int) bool {
		return bytes.Compare(result[i].ID().Bytes(), result[j].ID().Bytes()) < 0
	})
	return result
}

// add ensures the given nodes are present in the set.
func (ns nodeSet) add(nodes ...*enode.Node) {
	for _, n := range nodes {
		v := ns[n.ID()]
		v.N = n
		v.Seq = n.Seq()
		ns[n.ID()] = v
	}
}

// topN returns the top n nodes by score as a new set.
func (ns nodeSet) topN(n int) nodeSet {
	if n >= len(ns) {
		return ns
	}

	byscore := make([]nodeJSON, 0, len(ns))
	for _, v := range ns {
		byscore = append(byscore, v)
	}
	sort.Slice(byscore, func(i, j int) bool {
		return byscore[i].Score >= byscore[j].Score
	})
	result := make(nodeSet, n)
	for _, v := range byscore[:n] {
		result[v.N.ID()] = v
	}
	return result
}

// verify performs integrity checks on the node set.
func (ns nodeSet) verify() error {
	for id, n := range ns {
		if n.N.ID() != id {
			return fmt.Errorf("invalid node %v: ID does not match ID %v in record", id, n.N.ID())
		}
		if n.N.Seq() != n.Seq {
			return fmt.Errorf("invalid node %v: 'seq' does not match seq %d from record", id, n.N.Seq())
		}
	}
	return nil
}
