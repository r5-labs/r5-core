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
	"fmt"
	"io"
	"math/big"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/forkid"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/rlp"
)

// Constants to match up protocol versions and messages
const (
	ETH66 = 66
	ETH67 = 67
	ETH68 = 68
)

// ProtocolName is the official short name of the `eth` protocol used during
// devp2p capability negotiation.
const ProtocolName = "eth"

// ProtocolVersions are the supported versions of the `eth` protocol (first
// is primary).
var ProtocolVersions = []uint{ETH68, ETH67, ETH66}

// protocolLengths are the number of implemented message corresponding to
// different protocol versions.
var protocolLengths = map[uint]uint64{ETH68: 17, ETH67: 17, ETH66: 17}

// maxMessageSize is the maximum cap on the size of a protocol message.
const maxMessageSize = 10 * 1024 * 1024

const (
	StatusMsg                     = 0x00
	NewBlockHashesMsg             = 0x01
	TransactionsMsg               = 0x02
	GetBlockHeadersMsg            = 0x03
	BlockHeadersMsg               = 0x04
	GetBlockBodiesMsg             = 0x05
	BlockBodiesMsg                = 0x06
	NewBlockMsg                   = 0x07
	GetNodeDataMsg                = 0x0d
	NodeDataMsg                   = 0x0e
	GetReceiptsMsg                = 0x0f
	ReceiptsMsg                   = 0x10
	NewPooledTransactionHashesMsg = 0x08
	GetPooledTransactionsMsg      = 0x09
	PooledTransactionsMsg         = 0x0a
)

var (
	errNoStatusMsg             = errors.New("no status message")
	errMsgTooLarge             = errors.New("message too long")
	errDecode                  = errors.New("invalid message")
	errInvalidMsgCode          = errors.New("invalid message code")
	errProtocolVersionMismatch = errors.New("protocol version mismatch")
	errNetworkIDMismatch       = errors.New("network ID mismatch")
	errGenesisMismatch         = errors.New("genesis mismatch")
	errForkIDRejected          = errors.New("fork ID rejected")
)

// Packet represents a p2p message in the `eth` protocol.
type Packet interface {
	Name() string // Name returns a string corresponding to the message type.
	Kind() byte   // Kind returns the message type.
}

// StatusPacket is the network packet for the status message for eth/64 and later.
type StatusPacket struct {
	ProtocolVersion uint32
	NetworkID       uint64
	TD              *big.Int
	Head            common.Hash
	Genesis         common.Hash
	ForkID          forkid.ID
}

// NewBlockHashesPacket is the network packet for the block announcements.
type NewBlockHashesPacket []struct {
	Hash   common.Hash // Hash of one particular block being announced
	Number uint64      // Number of one particular block being announced
}

// Unpack retrieves the block hashes and numbers from the announcement packet
// and returns them in a split flat format that's more consistent with the
// internal data structures.
func (p *NewBlockHashesPacket) Unpack() ([]common.Hash, []uint64) {
	var (
		hashes  = make([]common.Hash, len(*p))
		numbers = make([]uint64, len(*p))
	)
	for i, body := range *p {
		hashes[i], numbers[i] = body.Hash, body.Number
	}
	return hashes, numbers
}

// TransactionsPacket is the network packet for broadcasting new transactions.
type TransactionsPacket []*types.Transaction

// GetBlockHeadersPacket represents a block header query.
type GetBlockHeadersPacket struct {
	Origin  HashOrNumber // Block from which to retrieve headers
	Amount  uint64       // Maximum number of headers to retrieve
	Skip    uint64       // Blocks to skip between consecutive headers
	Reverse bool         // Query direction (false = rising towards latest, true = falling towards genesis)
}

// GetBlockHeadersPacket66 represents a block header query over eth/66
type GetBlockHeadersPacket66 struct {
	RequestId uint64
	*GetBlockHeadersPacket
}

// HashOrNumber is a combined field for specifying an origin block.
type HashOrNumber struct {
	Hash   common.Hash // Block hash from which to retrieve headers (excludes Number)
	Number uint64      // Block hash from which to retrieve headers (excludes Hash)
}

// EncodeRLP is a specialized encoder for HashOrNumber to encode only one of the
// two contained union fields.
func (hn *HashOrNumber) EncodeRLP(w io.Writer) error {
	if hn.Hash == (common.Hash{}) {
		return rlp.Encode(w, hn.Number)
	}
	if hn.Number != 0 {
		return fmt.Errorf("both origin hash (%x) and number (%d) provided", hn.Hash, hn.Number)
	}
	return rlp.Encode(w, hn.Hash)
}

// DecodeRLP is a specialized decoder for HashOrNumber to decode the contents
// into either a block hash or a block number.
func (hn *HashOrNumber) DecodeRLP(s *rlp.Stream) error {
	_, size, err := s.Kind()
	switch {
	case err != nil:
		return err
	case size == 32:
		hn.Number = 0
		return s.Decode(&hn.Hash)
	case size <= 8:
		hn.Hash = common.Hash{}
		return s.Decode(&hn.Number)
	default:
		return fmt.Errorf("invalid input size %d for origin", size)
	}
}

// BlockHeadersPacket represents a block header response.
type BlockHeadersPacket []*types.Header

// BlockHeadersPacket66 represents a block header response over eth/66.
type BlockHeadersPacket66 struct {
	RequestId uint64
	BlockHeadersPacket
}

// BlockHeadersRLPPacket represents a block header response, to use when we already
// have the headers rlp encoded.
type BlockHeadersRLPPacket []rlp.RawValue

// BlockHeadersRLPPacket66 represents a block header response over eth/66.
type BlockHeadersRLPPacket66 struct {
	RequestId uint64
	BlockHeadersRLPPacket
}

// NewBlockPacket is the network packet for the block propagation message.
type NewBlockPacket struct {
	Block *types.Block
	TD    *big.Int
}

// sanityCheck verifies that the values are reasonable, as a DoS protection
func (request *NewBlockPacket) sanityCheck() error {
	if err := request.Block.SanityCheck(); err != nil {
		return err
	}
	//TD at mainnet block #7753254 is 76 bits. If it becomes 100 million times
	// larger, it will still fit within 100 bits
	if tdlen := request.TD.BitLen(); tdlen > 100 {
		return fmt.Errorf("too large block TD: bitlen %d", tdlen)
	}
	return nil
}

// GetBlockBodiesPacket represents a block body query.
type GetBlockBodiesPacket []common.Hash

// GetBlockBodiesPacket66 represents a block body query over eth/66.
type GetBlockBodiesPacket66 struct {
	RequestId uint64
	GetBlockBodiesPacket
}

// BlockBodiesPacket is the network packet for block content distribution.
type BlockBodiesPacket []*BlockBody

// BlockBodiesPacket66 is the network packet for block content distribution over eth/66.
type BlockBodiesPacket66 struct {
	RequestId uint64
	BlockBodiesPacket
}

// BlockBodiesRLPPacket is used for replying to block body requests, in cases
// where we already have them RLP-encoded, and thus can avoid the decode-encode
// roundtrip.
type BlockBodiesRLPPacket []rlp.RawValue

// BlockBodiesRLPPacket66 is the BlockBodiesRLPPacket over eth/66
type BlockBodiesRLPPacket66 struct {
	RequestId uint64
	BlockBodiesRLPPacket
}

// BlockBody represents the data content of a single block.
type BlockBody struct {
	Transactions []*types.Transaction // Transactions contained within a block
	Uncles       []*types.Header      // Uncles contained within a block
	Withdrawals  []*types.Withdrawal  `rlp:"optional"` // Withdrawals contained within a block
}

// Unpack retrieves the transactions and uncles from the range packet and returns
// them in a split flat format that's more consistent with the internal data structures.
func (p *BlockBodiesPacket) Unpack() ([][]*types.Transaction, [][]*types.Header, [][]*types.Withdrawal) {
	// TODO(matt): add support for withdrawals to fetchers
	var (
		txset         = make([][]*types.Transaction, len(*p))
		uncleset      = make([][]*types.Header, len(*p))
		withdrawalset = make([][]*types.Withdrawal, len(*p))
	)
	for i, body := range *p {
		txset[i], uncleset[i], withdrawalset[i] = body.Transactions, body.Uncles, body.Withdrawals
	}
	return txset, uncleset, withdrawalset
}

// GetNodeDataPacket represents a trie node data query.
type GetNodeDataPacket []common.Hash

// GetNodeDataPacket66 represents a trie node data query over eth/66.
type GetNodeDataPacket66 struct {
	RequestId uint64
	GetNodeDataPacket
}

// NodeDataPacket is the network packet for trie node data distribution.
type NodeDataPacket [][]byte

// NodeDataPacket66 is the network packet for trie node data distribution over eth/66.
type NodeDataPacket66 struct {
	RequestId uint64
	NodeDataPacket
}

// GetReceiptsPacket represents a block receipts query.
type GetReceiptsPacket []common.Hash

// GetReceiptsPacket66 represents a block receipts query over eth/66.
type GetReceiptsPacket66 struct {
	RequestId uint64
	GetReceiptsPacket
}

// ReceiptsPacket is the network packet for block receipts distribution.
type ReceiptsPacket [][]*types.Receipt

// ReceiptsPacket66 is the network packet for block receipts distribution over eth/66.
type ReceiptsPacket66 struct {
	RequestId uint64
	ReceiptsPacket
}

// ReceiptsRLPPacket is used for receipts, when we already have it encoded
type ReceiptsRLPPacket []rlp.RawValue

// ReceiptsRLPPacket66 is the eth-66 version of ReceiptsRLPPacket
type ReceiptsRLPPacket66 struct {
	RequestId uint64
	ReceiptsRLPPacket
}

// NewPooledTransactionHashesPacket66 represents a transaction announcement packet on eth/66 and eth/67.
type NewPooledTransactionHashesPacket66 []common.Hash

// NewPooledTransactionHashesPacket68 represents a transaction announcement packet on eth/68 and newer.
type NewPooledTransactionHashesPacket68 struct {
	Types  []byte
	Sizes  []uint32
	Hashes []common.Hash
}

// GetPooledTransactionsPacket represents a transaction query.
type GetPooledTransactionsPacket []common.Hash

type GetPooledTransactionsPacket66 struct {
	RequestId uint64
	GetPooledTransactionsPacket
}

// PooledTransactionsPacket is the network packet for transaction distribution.
type PooledTransactionsPacket []*types.Transaction

// PooledTransactionsPacket66 is the network packet for transaction distribution over eth/66.
type PooledTransactionsPacket66 struct {
	RequestId uint64
	PooledTransactionsPacket
}

// PooledTransactionsRLPPacket is the network packet for transaction distribution, used
// in the cases we already have them in rlp-encoded form
type PooledTransactionsRLPPacket []rlp.RawValue

// PooledTransactionsRLPPacket66 is the eth/66 form of PooledTransactionsRLPPacket
type PooledTransactionsRLPPacket66 struct {
	RequestId uint64
	PooledTransactionsRLPPacket
}

func (*StatusPacket) Name() string { return "Status" }
func (*StatusPacket) Kind() byte   { return StatusMsg }

func (*NewBlockHashesPacket) Name() string { return "NewBlockHashes" }
func (*NewBlockHashesPacket) Kind() byte   { return NewBlockHashesMsg }

func (*TransactionsPacket) Name() string { return "Transactions" }
func (*TransactionsPacket) Kind() byte   { return TransactionsMsg }

func (*GetBlockHeadersPacket) Name() string { return "GetBlockHeaders" }
func (*GetBlockHeadersPacket) Kind() byte   { return GetBlockHeadersMsg }

func (*BlockHeadersPacket) Name() string { return "BlockHeaders" }
func (*BlockHeadersPacket) Kind() byte   { return BlockHeadersMsg }

func (*GetBlockBodiesPacket) Name() string { return "GetBlockBodies" }
func (*GetBlockBodiesPacket) Kind() byte   { return GetBlockBodiesMsg }

func (*BlockBodiesPacket) Name() string { return "BlockBodies" }
func (*BlockBodiesPacket) Kind() byte   { return BlockBodiesMsg }

func (*NewBlockPacket) Name() string { return "NewBlock" }
func (*NewBlockPacket) Kind() byte   { return NewBlockMsg }

func (*GetNodeDataPacket) Name() string { return "GetNodeData" }
func (*GetNodeDataPacket) Kind() byte   { return GetNodeDataMsg }

func (*NodeDataPacket) Name() string { return "NodeData" }
func (*NodeDataPacket) Kind() byte   { return NodeDataMsg }

func (*GetReceiptsPacket) Name() string { return "GetReceipts" }
func (*GetReceiptsPacket) Kind() byte   { return GetReceiptsMsg }

func (*ReceiptsPacket) Name() string { return "Receipts" }
func (*ReceiptsPacket) Kind() byte   { return ReceiptsMsg }

func (*NewPooledTransactionHashesPacket66) Name() string { return "NewPooledTransactionHashes" }
func (*NewPooledTransactionHashesPacket66) Kind() byte   { return NewPooledTransactionHashesMsg }

func (*NewPooledTransactionHashesPacket68) Name() string { return "NewPooledTransactionHashes" }
func (*NewPooledTransactionHashesPacket68) Kind() byte   { return NewPooledTransactionHashesMsg }

func (*GetPooledTransactionsPacket) Name() string { return "GetPooledTransactions" }
func (*GetPooledTransactionsPacket) Kind() byte   { return GetPooledTransactionsMsg }

func (*PooledTransactionsPacket) Name() string { return "PooledTransactions" }
func (*PooledTransactionsPacket) Kind() byte   { return PooledTransactionsMsg }
