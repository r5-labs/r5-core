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

package v5wire

import (
	"fmt"
	"net"

	"github.com/r5-labs/r5-core/client/common/hexutil"
	"github.com/r5-labs/r5-core/client/common/mclock"
	"github.com/r5-labs/r5-core/client/p2p/enode"
	"github.com/r5-labs/r5-core/client/p2p/enr"
	"github.com/r5-labs/r5-core/client/rlp"
)

// Packet is implemented by all message types.
type Packet interface {
	Name() string        // Name returns a string corresponding to the message type.
	Kind() byte          // Kind returns the message type.
	RequestID() []byte   // Returns the request ID.
	SetRequestID([]byte) // Sets the request ID.

	// AppendLogInfo returns its argument 'ctx' with additional fields
	// appended for logging purposes.
	AppendLogInfo(ctx []interface{}) []interface{}
}

// Message types.
const (
	PingMsg byte = iota + 1
	PongMsg
	FindnodeMsg
	NodesMsg
	TalkRequestMsg
	TalkResponseMsg
	RequestTicketMsg
	TicketMsg

	UnknownPacket   = byte(255) // any non-decryptable packet
	WhoareyouPacket = byte(254) // the WHOAREYOU packet
)

// Protocol messages.
type (
	// Unknown represents any packet that can't be decrypted.
	Unknown struct {
		Nonce Nonce
	}

	// WHOAREYOU contains the handshake challenge.
	Whoareyou struct {
		ChallengeData []byte   // Encoded challenge
		Nonce         Nonce    // Nonce of request packet
		IDNonce       [16]byte // Identity proof data
		RecordSeq     uint64   // ENR sequence number of recipient

		// Node is the locally known node record of recipient.
		// This must be set by the caller of Encode.
		Node *enode.Node

		sent mclock.AbsTime // for handshake GC.
	}

	// PING is sent during liveness checks.
	Ping struct {
		ReqID  []byte
		ENRSeq uint64
	}

	// PONG is the reply to PING.
	Pong struct {
		ReqID  []byte
		ENRSeq uint64
		ToIP   net.IP // These fields should mirror the UDP envelope address of the ping
		ToPort uint16 // packet, which provides a way to discover the external address (after NAT).
	}

	// FINDNODE is a query for nodes in the given bucket.
	Findnode struct {
		ReqID     []byte
		Distances []uint

		// OpID is for debugging purposes and is not part of the packet encoding.
		// It identifies the 'operation' on behalf of which the request was sent.
		OpID uint64 `rlp:"-"`
	}

	// NODES is a response to FINDNODE.
	Nodes struct {
		ReqID     []byte
		RespCount uint8 // total number of responses to the request
		Nodes     []*enr.Record
	}

	// TALKREQ is an application-level request.
	TalkRequest struct {
		ReqID    []byte
		Protocol string
		Message  []byte
	}

	// TALKRESP is the reply to TALKREQ.
	TalkResponse struct {
		ReqID   []byte
		Message []byte
	}
)

// DecodeMessage decodes the message body of a packet.
func DecodeMessage(ptype byte, body []byte) (Packet, error) {
	var dec Packet
	switch ptype {
	case PingMsg:
		dec = new(Ping)
	case PongMsg:
		dec = new(Pong)
	case FindnodeMsg:
		dec = new(Findnode)
	case NodesMsg:
		dec = new(Nodes)
	case TalkRequestMsg:
		dec = new(TalkRequest)
	case TalkResponseMsg:
		dec = new(TalkResponse)
	default:
		return nil, fmt.Errorf("unknown packet type %d", ptype)
	}
	if err := rlp.DecodeBytes(body, dec); err != nil {
		return nil, err
	}
	if dec.RequestID() != nil && len(dec.RequestID()) > 8 {
		return nil, ErrInvalidReqID
	}
	return dec, nil
}

func (*Whoareyou) Name() string        { return "WHOAREYOU/v5" }
func (*Whoareyou) Kind() byte          { return WhoareyouPacket }
func (*Whoareyou) RequestID() []byte   { return nil }
func (*Whoareyou) SetRequestID([]byte) {}

func (*Whoareyou) AppendLogInfo(ctx []interface{}) []interface{} {
	return ctx
}

func (*Unknown) Name() string        { return "UNKNOWN/v5" }
func (*Unknown) Kind() byte          { return UnknownPacket }
func (*Unknown) RequestID() []byte   { return nil }
func (*Unknown) SetRequestID([]byte) {}

func (*Unknown) AppendLogInfo(ctx []interface{}) []interface{} {
	return ctx
}

func (*Ping) Name() string             { return "PING/v5" }
func (*Ping) Kind() byte               { return PingMsg }
func (p *Ping) RequestID() []byte      { return p.ReqID }
func (p *Ping) SetRequestID(id []byte) { p.ReqID = id }

func (p *Ping) AppendLogInfo(ctx []interface{}) []interface{} {
	return append(ctx, "req", hexutil.Bytes(p.ReqID), "enrseq", p.ENRSeq)
}

func (*Pong) Name() string             { return "PONG/v5" }
func (*Pong) Kind() byte               { return PongMsg }
func (p *Pong) RequestID() []byte      { return p.ReqID }
func (p *Pong) SetRequestID(id []byte) { p.ReqID = id }

func (p *Pong) AppendLogInfo(ctx []interface{}) []interface{} {
	return append(ctx, "req", hexutil.Bytes(p.ReqID), "enrseq", p.ENRSeq)
}

func (p *Findnode) Name() string           { return "FINDNODE/v5" }
func (p *Findnode) Kind() byte             { return FindnodeMsg }
func (p *Findnode) RequestID() []byte      { return p.ReqID }
func (p *Findnode) SetRequestID(id []byte) { p.ReqID = id }

func (p *Findnode) AppendLogInfo(ctx []interface{}) []interface{} {
	ctx = append(ctx, "req", hexutil.Bytes(p.ReqID))
	if p.OpID != 0 {
		ctx = append(ctx, "opid", p.OpID)
	}
	return ctx
}

func (*Nodes) Name() string             { return "NODES/v5" }
func (*Nodes) Kind() byte               { return NodesMsg }
func (p *Nodes) RequestID() []byte      { return p.ReqID }
func (p *Nodes) SetRequestID(id []byte) { p.ReqID = id }

func (p *Nodes) AppendLogInfo(ctx []interface{}) []interface{} {
	return append(ctx,
		"req", hexutil.Bytes(p.ReqID),
		"tot", p.RespCount,
		"n", len(p.Nodes),
	)
}

func (*TalkRequest) Name() string             { return "TALKREQ/v5" }
func (*TalkRequest) Kind() byte               { return TalkRequestMsg }
func (p *TalkRequest) RequestID() []byte      { return p.ReqID }
func (p *TalkRequest) SetRequestID(id []byte) { p.ReqID = id }

func (p *TalkRequest) AppendLogInfo(ctx []interface{}) []interface{} {
	return append(ctx, "proto", p.Protocol, "reqid", hexutil.Bytes(p.ReqID), "len", len(p.Message))
}

func (*TalkResponse) Name() string             { return "TALKRESP/v5" }
func (*TalkResponse) Kind() byte               { return TalkResponseMsg }
func (p *TalkResponse) RequestID() []byte      { return p.ReqID }
func (p *TalkResponse) SetRequestID(id []byte) { p.ReqID = id }

func (p *TalkResponse) AppendLogInfo(ctx []interface{}) []interface{} {
	return append(ctx, "req", p.ReqID, "len", len(p.Message))
}
