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

// Package v4wire implements the Discovery v4 Wire Protocol.
package v4wire

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/r5-labs/r5-core/client/common/math"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/p2p/enode"
	"github.com/r5-labs/r5-core/client/p2p/enr"
	"github.com/r5-labs/r5-core/client/rlp"
)

// RPC packet types
const (
	PingPacket = iota + 1 // zero is 'reserved'
	PongPacket
	FindnodePacket
	NeighborsPacket
	ENRRequestPacket
	ENRResponsePacket
)

// RPC request structures
type (
	Ping struct {
		Version    uint
		From, To   Endpoint
		Expiration uint64
		ENRSeq     uint64 `rlp:"optional"` // Sequence number of local record, added by EIP-868.

		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// Pong is the reply to ping.
	Pong struct {
		// This field should mirror the UDP envelope address
		// of the ping packet, which provides a way to discover the
		// external address (after NAT).
		To         Endpoint
		ReplyTok   []byte // This contains the hash of the ping packet.
		Expiration uint64 // Absolute timestamp at which the packet becomes invalid.
		ENRSeq     uint64 `rlp:"optional"` // Sequence number of local record, added by EIP-868.

		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// Findnode is a query for nodes close to the given target.
	Findnode struct {
		Target     Pubkey
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// Neighbors is the reply to findnode.
	Neighbors struct {
		Nodes      []Node
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// ENRRequest queries for the remote node's record.
	ENRRequest struct {
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// ENRResponse is the reply to ENRRequest.
	ENRResponse struct {
		ReplyTok []byte // Hash of the ENRRequest packet.
		Record   enr.Record
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}
)

// MaxNeighbors is the maximum number of neighbor nodes in a Neighbors packet.
const MaxNeighbors = 12

// This code computes the MaxNeighbors constant value.

// func init() {
// 	var maxNeighbors int
// 	p := Neighbors{Expiration: ^uint64(0)}
// 	maxSizeNode := Node{IP: make(net.IP, 16), UDP: ^uint16(0), TCP: ^uint16(0)}
// 	for n := 0; ; n++ {
// 		p.Nodes = append(p.Nodes, maxSizeNode)
// 		size, _, err := rlp.EncodeToReader(p)
// 		if err != nil {
// 			// If this ever happens, it will be caught by the unit tests.
// 			panic("cannot encode: " + err.Error())
// 		}
// 		if headSize+size+1 >= 1280 {
// 			maxNeighbors = n
// 			break
// 		}
// 	}
// 	fmt.Println("maxNeighbors", maxNeighbors)
// }

// Pubkey represents an encoded 64-byte secp256k1 public key.
type Pubkey [64]byte

// ID returns the node ID corresponding to the public key.
func (e Pubkey) ID() enode.ID {
	return enode.ID(crypto.Keccak256Hash(e[:]))
}

// Node represents information about a node.
type Node struct {
	IP  net.IP // len 4 for IPv4 or 16 for IPv6
	UDP uint16 // for discovery protocol
	TCP uint16 // for RLPx protocol
	ID  Pubkey
}

// Endpoint represents a network endpoint.
type Endpoint struct {
	IP  net.IP // len 4 for IPv4 or 16 for IPv6
	UDP uint16 // for discovery protocol
	TCP uint16 // for RLPx protocol
}

// NewEndpoint creates an endpoint.
func NewEndpoint(addr *net.UDPAddr, tcpPort uint16) Endpoint {
	ip := net.IP{}
	if ip4 := addr.IP.To4(); ip4 != nil {
		ip = ip4
	} else if ip6 := addr.IP.To16(); ip6 != nil {
		ip = ip6
	}
	return Endpoint{IP: ip, UDP: uint16(addr.Port), TCP: tcpPort}
}

type Packet interface {
	// Name is the name of the package, for logging purposes.
	Name() string
	// Kind is the packet type, for logging purposes.
	Kind() byte
}

func (req *Ping) Name() string { return "PING/v4" }
func (req *Ping) Kind() byte   { return PingPacket }

func (req *Pong) Name() string { return "PONG/v4" }
func (req *Pong) Kind() byte   { return PongPacket }

func (req *Findnode) Name() string { return "FINDNODE/v4" }
func (req *Findnode) Kind() byte   { return FindnodePacket }

func (req *Neighbors) Name() string { return "NEIGHBORS/v4" }
func (req *Neighbors) Kind() byte   { return NeighborsPacket }

func (req *ENRRequest) Name() string { return "ENRREQUEST/v4" }
func (req *ENRRequest) Kind() byte   { return ENRRequestPacket }

func (req *ENRResponse) Name() string { return "ENRRESPONSE/v4" }
func (req *ENRResponse) Kind() byte   { return ENRResponsePacket }

// Expired checks whether the given UNIX time stamp is in the past.
func Expired(ts uint64) bool {
	return time.Unix(int64(ts), 0).Before(time.Now())
}

// Encoder/decoder.

const (
	macSize  = 32
	sigSize  = crypto.SignatureLength
	headSize = macSize + sigSize // space of packet frame data
)

var (
	ErrPacketTooSmall = errors.New("too small")
	ErrBadHash        = errors.New("bad hash")
	ErrBadPoint       = errors.New("invalid curve point")
)

var headSpace = make([]byte, headSize)

// Decode reads a discovery v4 packet.
func Decode(input []byte) (Packet, Pubkey, []byte, error) {
	if len(input) < headSize+1 {
		return nil, Pubkey{}, nil, ErrPacketTooSmall
	}
	hash, sig, sigdata := input[:macSize], input[macSize:headSize], input[headSize:]
	shouldhash := crypto.Keccak256(input[macSize:])
	if !bytes.Equal(hash, shouldhash) {
		return nil, Pubkey{}, nil, ErrBadHash
	}
	fromKey, err := recoverNodeKey(crypto.Keccak256(input[headSize:]), sig)
	if err != nil {
		return nil, fromKey, hash, err
	}

	var req Packet
	switch ptype := sigdata[0]; ptype {
	case PingPacket:
		req = new(Ping)
	case PongPacket:
		req = new(Pong)
	case FindnodePacket:
		req = new(Findnode)
	case NeighborsPacket:
		req = new(Neighbors)
	case ENRRequestPacket:
		req = new(ENRRequest)
	case ENRResponsePacket:
		req = new(ENRResponse)
	default:
		return nil, fromKey, hash, fmt.Errorf("unknown type: %d", ptype)
	}
	s := rlp.NewStream(bytes.NewReader(sigdata[1:]), 0)
	err = s.Decode(req)
	return req, fromKey, hash, err
}

// Encode encodes a discovery packet.
func Encode(priv *ecdsa.PrivateKey, req Packet) (packet, hash []byte, err error) {
	b := new(bytes.Buffer)
	b.Write(headSpace)
	b.WriteByte(req.Kind())
	if err := rlp.Encode(b, req); err != nil {
		return nil, nil, err
	}
	packet = b.Bytes()
	sig, err := crypto.Sign(crypto.Keccak256(packet[headSize:]), priv)
	if err != nil {
		return nil, nil, err
	}
	copy(packet[macSize:], sig)
	// Add the hash to the front. Note: this doesn't protect the packet in any way.
	hash = crypto.Keccak256(packet[macSize:])
	copy(packet, hash)
	return packet, hash, nil
}

// recoverNodeKey computes the public key used to sign the given hash from the signature.
func recoverNodeKey(hash, sig []byte) (key Pubkey, err error) {
	pubkey, err := crypto.Ecrecover(hash, sig)
	if err != nil {
		return key, err
	}
	copy(key[:], pubkey[1:])
	return key, nil
}

// EncodePubkey encodes a secp256k1 public key.
func EncodePubkey(key *ecdsa.PublicKey) Pubkey {
	var e Pubkey
	math.ReadBits(key.X, e[:len(e)/2])
	math.ReadBits(key.Y, e[len(e)/2:])
	return e
}

// DecodePubkey reads an encoded secp256k1 public key.
func DecodePubkey(curve elliptic.Curve, e Pubkey) (*ecdsa.PublicKey, error) {
	p := &ecdsa.PublicKey{Curve: curve, X: new(big.Int), Y: new(big.Int)}
	half := len(e) / 2
	p.X.SetBytes(e[:half])
	p.Y.SetBytes(e[half:])
	if !p.Curve.IsOnCurve(p.X, p.Y) {
		return nil, ErrBadPoint
	}
	return p, nil
}
