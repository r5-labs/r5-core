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

package trie

import (
	"fmt"

	"github.com/r5-labs/r5-core/client/common"
)

// leaf represents a trie leaf node
type leaf struct {
	blob   []byte      // raw blob of leaf
	parent common.Hash // the hash of parent node
}

// committer is the tool used for the trie Commit operation. The committer will
// capture all dirty nodes during the commit process and keep them cached in
// insertion order.
type committer struct {
	nodes       *NodeSet
	collectLeaf bool
}

// newCommitter creates a new committer or picks one from the pool.
func newCommitter(nodeset *NodeSet, collectLeaf bool) *committer {
	return &committer{
		nodes:       nodeset,
		collectLeaf: collectLeaf,
	}
}

// Commit collapses a node down into a hash node.
func (c *committer) Commit(n node) hashNode {
	return c.commit(nil, n).(hashNode)
}

// commit collapses a node down into a hash node and returns it.
func (c *committer) commit(path []byte, n node) node {
	// if this path is clean, use available cached data
	hash, dirty := n.cache()
	if hash != nil && !dirty {
		return hash
	}
	// Commit children, then parent, and remove the dirty flag.
	switch cn := n.(type) {
	case *shortNode:
		// Commit child
		collapsed := cn.copy()

		// If the child is fullNode, recursively commit,
		// otherwise it can only be hashNode or valueNode.
		if _, ok := cn.Val.(*fullNode); ok {
			collapsed.Val = c.commit(append(path, cn.Key...), cn.Val)
		}
		// The key needs to be copied, since we're adding it to the
		// modified nodeset.
		collapsed.Key = hexToCompact(cn.Key)
		hashedNode := c.store(path, collapsed)
		if hn, ok := hashedNode.(hashNode); ok {
			return hn
		}
		return collapsed
	case *fullNode:
		hashedKids := c.commitChildren(path, cn)
		collapsed := cn.copy()
		collapsed.Children = hashedKids

		hashedNode := c.store(path, collapsed)
		if hn, ok := hashedNode.(hashNode); ok {
			return hn
		}
		return collapsed
	case hashNode:
		return cn
	default:
		// nil, valuenode shouldn't be committed
		panic(fmt.Sprintf("%T: invalid node: %v", n, n))
	}
}

// commitChildren commits the children of the given fullnode
func (c *committer) commitChildren(path []byte, n *fullNode) [17]node {
	var children [17]node
	for i := 0; i < 16; i++ {
		child := n.Children[i]
		if child == nil {
			continue
		}
		// If it's the hashed child, save the hash value directly.
		// Note: it's impossible that the child in range [0, 15]
		// is a valueNode.
		if hn, ok := child.(hashNode); ok {
			children[i] = hn
			continue
		}
		// Commit the child recursively and store the "hashed" value.
		// Note the returned node can be some embedded nodes, so it's
		// possible the type is not hashNode.
		children[i] = c.commit(append(path, byte(i)), child)
	}
	// For the 17th child, it's possible the type is valuenode.
	if n.Children[16] != nil {
		children[16] = n.Children[16]
	}
	return children
}

// store hashes the node n and adds it to the modified nodeset. If leaf collection
// is enabled, leaf nodes will be tracked in the modified nodeset as well.
func (c *committer) store(path []byte, n node) node {
	// Larger nodes are replaced by their hash and stored in the database.
	var hash, _ = n.cache()

	// This was not generated - must be a small node stored in the parent.
	// In theory, we should check if the node is leaf here (embedded node
	// usually is leaf node). But small value (less than 32bytes) is not
	// our target (leaves in account trie only).
	if hash == nil {
		// The node is embedded in its parent, in other words, this node
		// will not be stored in the database independently, mark it as
		// deleted only if the node was existent in database before.
		if _, ok := c.nodes.accessList[string(path)]; ok {
			c.nodes.markDeleted(path)
		}
		return n
	}
	// We have the hash already, estimate the RLP encoding-size of the node.
	// The size is used for mem tracking, does not need to be exact
	var (
		size  = estimateSize(n)
		nhash = common.BytesToHash(hash)
		mnode = &memoryNode{
			hash: nhash,
			node: simplifyNode(n),
			size: uint16(size),
		}
	)
	// Collect the dirty node to nodeset for return.
	c.nodes.markUpdated(path, mnode)

	// Collect the corresponding leaf node if it's required. We don't check
	// full node since it's impossible to store value in fullNode. The key
	// length of leaves should be exactly same.
	if c.collectLeaf {
		if sn, ok := n.(*shortNode); ok {
			if val, ok := sn.Val.(valueNode); ok {
				c.nodes.addLeaf(&leaf{blob: val, parent: nhash})
			}
		}
	}
	return hash
}

// estimateSize estimates the size of an rlp-encoded node, without actually
// rlp-encoding it (zero allocs). This method has been experimentally tried, and with a trie
// with 1000 leaves, the only errors above 1% are on small shortnodes, where this
// method overestimates by 2 or 3 bytes (e.g. 37 instead of 35)
func estimateSize(n node) int {
	switch n := n.(type) {
	case *shortNode:
		// A short node contains a compacted key, and a value.
		return 3 + len(n.Key) + estimateSize(n.Val)
	case *fullNode:
		// A full node contains up to 16 hashes (some nils), and a key
		s := 3
		for i := 0; i < 16; i++ {
			if child := n.Children[i]; child != nil {
				s += estimateSize(child)
			} else {
				s++
			}
		}
		return s
	case valueNode:
		return 1 + len(n)
	case hashNode:
		return 1 + len(n)
	default:
		panic(fmt.Sprintf("node type %T", n))
	}
}
