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

package eth

import (
	"errors"
	"testing"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/forkid"
	"github.com/r5-labs/r5-core/client/p2p"
	"github.com/r5-labs/r5-core/client/p2p/enode"
)

// Tests that handshake failures are detected and reported correctly.
func TestHandshake66(t *testing.T) { testHandshake(t, ETH66) }

func testHandshake(t *testing.T, protocol uint) {
	t.Parallel()

	// Create a test backend only to have some valid genesis chain
	backend := newTestBackend(3)
	defer backend.close()

	var (
		genesis = backend.chain.Genesis()
		head    = backend.chain.CurrentBlock()
		td      = backend.chain.GetTd(head.Hash(), head.Number.Uint64())
		forkID  = forkid.NewID(backend.chain.Config(), backend.chain.Genesis().Hash(), backend.chain.CurrentHeader().Number.Uint64(), backend.chain.CurrentHeader().Time)
	)
	tests := []struct {
		code uint64
		data interface{}
		want error
	}{
		{
			code: TransactionsMsg, data: []interface{}{},
			want: errNoStatusMsg,
		},
		{
			code: StatusMsg, data: StatusPacket{10, 1, td, head.Hash(), genesis.Hash(), forkID},
			want: errProtocolVersionMismatch,
		},
		{
			code: StatusMsg, data: StatusPacket{uint32(protocol), 999, td, head.Hash(), genesis.Hash(), forkID},
			want: errNetworkIDMismatch,
		},
		{
			code: StatusMsg, data: StatusPacket{uint32(protocol), 1, td, head.Hash(), common.Hash{3}, forkID},
			want: errGenesisMismatch,
		},
		{
			code: StatusMsg, data: StatusPacket{uint32(protocol), 1, td, head.Hash(), genesis.Hash(), forkid.ID{Hash: [4]byte{0x00, 0x01, 0x02, 0x03}}},
			want: errForkIDRejected,
		},
	}
	for i, test := range tests {
		// Create the two peers to shake with each other
		app, net := p2p.MsgPipe()
		defer app.Close()
		defer net.Close()

		peer := NewPeer(protocol, p2p.NewPeer(enode.ID{}, "peer", nil), net, nil)
		defer peer.Close()

		// Send the junk test with one peer, check the handshake failure
		go p2p.Send(app, test.code, test.data)

		err := peer.Handshake(1, td, head.Hash(), genesis.Hash(), forkID, forkid.NewFilter(backend.chain))
		if err == nil {
			t.Errorf("test %d: protocol returned nil error, want %q", i, test.want)
		} else if !errors.Is(err, test.want) {
			t.Errorf("test %d: wrong error: got %q, want %q", i, err, test.want)
		}
	}
}
