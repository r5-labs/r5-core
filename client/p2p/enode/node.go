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

package enode

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"net"
	"strings"

	"github.com/r5-labs/r5-core/client/p2p/enr"
	"github.com/r5-labs/r5-core/client/rlp"
)

var errMissingPrefix = errors.New("missing 'enr:' prefix for base64-encoded record")

// Node represents a host on the network.
type Node struct {
	r  enr.Record
	id ID
}

// New wraps a node record. The record must be valid according to the given
// identity scheme.
func New(validSchemes enr.IdentityScheme, r *enr.Record) (*Node, error) {
	if err := r.VerifySignature(validSchemes); err != nil {
		return nil, err
	}
	node := &Node{r: *r}
	if n := copy(node.id[:], validSchemes.NodeAddr(&node.r)); n != len(ID{}) {
		return nil, fmt.Errorf("invalid node ID length %d, need %d", n, len(ID{}))
	}
	return node, nil
}

// MustParse parses a node record or enode:// URL. It panics if the input is invalid.
func MustParse(rawurl string) *Node {
	n, err := Parse(ValidSchemes, rawurl)
	if err != nil {
		panic("invalid node: " + err.Error())
	}
	return n
}

// Parse decodes and verifies a base64-encoded node record.
func Parse(validSchemes enr.IdentityScheme, input string) (*Node, error) {
	if strings.HasPrefix(input, "enode://") {
		return ParseV4(input)
	}
	if !strings.HasPrefix(input, "enr:") {
		return nil, errMissingPrefix
	}
	bin, err := base64.RawURLEncoding.DecodeString(input[4:])
	if err != nil {
		return nil, err
	}
	var r enr.Record
	if err := rlp.DecodeBytes(bin, &r); err != nil {
		return nil, err
	}
	return New(validSchemes, &r)
}

// ID returns the node identifier.
func (n *Node) ID() ID {
	return n.id
}

// Seq returns the sequence number of the underlying record.
func (n *Node) Seq() uint64 {
	return n.r.Seq()
}

// Incomplete returns true for nodes with no IP address.
func (n *Node) Incomplete() bool {
	return n.IP() == nil
}

// Load retrieves an entry from the underlying record.
func (n *Node) Load(k enr.Entry) error {
	return n.r.Load(k)
}

// IP returns the IP address of the node. This prefers IPv4 addresses.
func (n *Node) IP() net.IP {
	var (
		ip4 enr.IPv4
		ip6 enr.IPv6
	)
	if n.Load(&ip4) == nil {
		return net.IP(ip4)
	}
	if n.Load(&ip6) == nil {
		return net.IP(ip6)
	}
	return nil
}

// UDP returns the UDP port of the node.
func (n *Node) UDP() int {
	var port enr.UDP
	n.Load(&port)
	return int(port)
}

// TCP returns the TCP port of the node.
func (n *Node) TCP() int {
	var port enr.TCP
	n.Load(&port)
	return int(port)
}

// Pubkey returns the secp256k1 public key of the node, if present.
func (n *Node) Pubkey() *ecdsa.PublicKey {
	var key ecdsa.PublicKey
	if n.Load((*Secp256k1)(&key)) != nil {
		return nil
	}
	return &key
}

// Record returns the node's record. The return value is a copy and may
// be modified by the caller.
func (n *Node) Record() *enr.Record {
	cpy := n.r
	return &cpy
}

// ValidateComplete checks whether n has a valid IP and UDP port.
// Deprecated: don't use this method.
func (n *Node) ValidateComplete() error {
	if n.Incomplete() {
		return errors.New("missing IP address")
	}
	if n.UDP() == 0 {
		return errors.New("missing UDP port")
	}
	ip := n.IP()
	if ip.IsMulticast() || ip.IsUnspecified() {
		return errors.New("invalid IP (multicast/unspecified)")
	}
	// Validate the node key (on curve, etc.).
	var key Secp256k1
	return n.Load(&key)
}

// String returns the text representation of the record.
func (n *Node) String() string {
	if isNewV4(n) {
		return n.URLv4() // backwards-compatibility glue for NewV4 nodes
	}
	enc, _ := rlp.EncodeToBytes(&n.r) // always succeeds because record is valid
	b64 := base64.RawURLEncoding.EncodeToString(enc)
	return "enr:" + b64
}

// MarshalText implements encoding.TextMarshaler.
func (n *Node) MarshalText() ([]byte, error) {
	return []byte(n.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *Node) UnmarshalText(text []byte) error {
	dec, err := Parse(ValidSchemes, string(text))
	if err == nil {
		*n = *dec
	}
	return err
}

// ID is a unique identifier for each node.
type ID [32]byte

// Bytes returns a byte slice representation of the ID
func (n ID) Bytes() []byte {
	return n[:]
}

// ID prints as a long hexadecimal number.
func (n ID) String() string {
	return fmt.Sprintf("%x", n[:])
}

// GoString returns the Go syntax representation of a ID is a call to HexID.
func (n ID) GoString() string {
	return fmt.Sprintf("enode.HexID(\"%x\")", n[:])
}

// TerminalString returns a shortened hex string for terminal logging.
func (n ID) TerminalString() string {
	return hex.EncodeToString(n[:8])
}

// MarshalText implements the encoding.TextMarshaler interface.
func (n ID) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(n[:])), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (n *ID) UnmarshalText(text []byte) error {
	id, err := ParseID(string(text))
	if err != nil {
		return err
	}
	*n = id
	return nil
}

// HexID converts a hex string to an ID.
// The string may be prefixed with 0x.
// It panics if the string is not a valid ID.
func HexID(in string) ID {
	id, err := ParseID(in)
	if err != nil {
		panic(err)
	}
	return id
}

func ParseID(in string) (ID, error) {
	var id ID
	b, err := hex.DecodeString(strings.TrimPrefix(in, "0x"))
	if err != nil {
		return id, err
	} else if len(b) != len(id) {
		return id, fmt.Errorf("wrong length, want %d hex chars", len(id)*2)
	}
	copy(id[:], b)
	return id, nil
}

// DistCmp compares the distances a->target and b->target.
// Returns -1 if a is closer to target, 1 if b is closer to target
// and 0 if they are equal.
func DistCmp(target, a, b ID) int {
	for i := range target {
		da := a[i] ^ target[i]
		db := b[i] ^ target[i]
		if da > db {
			return 1
		} else if da < db {
			return -1
		}
	}
	return 0
}

// LogDist returns the logarithmic distance between a and b, log2(a ^ b).
func LogDist(a, b ID) int {
	lz := 0
	for i := range a {
		x := a[i] ^ b[i]
		if x == 0 {
			lz += 8
		} else {
			lz += bits.LeadingZeros8(x)
			break
		}
	}
	return len(a)*8 - lz
}
