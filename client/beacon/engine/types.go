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

package engine

import (
	"fmt"
	"math/big"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/common/hexutil"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/trie"
)

//go:generate go run github.com/fjl/gencodec -type PayloadAttributes -field-override payloadAttributesMarshaling -out gen_blockparams.go

// PayloadAttributes describes the environment context in which a block should
// be built.
type PayloadAttributes struct {
	Timestamp             uint64              `json:"timestamp"             gencodec:"required"`
	Random                common.Hash         `json:"prevRandao"            gencodec:"required"`
	SuggestedFeeRecipient common.Address      `json:"suggestedFeeRecipient" gencodec:"required"`
	Withdrawals           []*types.Withdrawal `json:"withdrawals"`
}

// JSON type overrides for PayloadAttributes.
type payloadAttributesMarshaling struct {
	Timestamp hexutil.Uint64
}

//go:generate go run github.com/fjl/gencodec -type ExecutableData -field-override executableDataMarshaling -out gen_ed.go

// ExecutableData is the data necessary to execute an EL payload.
type ExecutableData struct {
	ParentHash    common.Hash         `json:"parentHash"    gencodec:"required"`
	FeeRecipient  common.Address      `json:"feeRecipient"  gencodec:"required"`
	StateRoot     common.Hash         `json:"stateRoot"     gencodec:"required"`
	ReceiptsRoot  common.Hash         `json:"receiptsRoot"  gencodec:"required"`
	LogsBloom     []byte              `json:"logsBloom"     gencodec:"required"`
	Random        common.Hash         `json:"prevRandao"    gencodec:"required"`
	Number        uint64              `json:"blockNumber"   gencodec:"required"`
	GasLimit      uint64              `json:"gasLimit"      gencodec:"required"`
	GasUsed       uint64              `json:"gasUsed"       gencodec:"required"`
	Timestamp     uint64              `json:"timestamp"     gencodec:"required"`
	ExtraData     []byte              `json:"extraData"     gencodec:"required"`
	BaseFeePerGas *big.Int            `json:"baseFeePerGas" gencodec:"required"`
	BlockHash     common.Hash         `json:"blockHash"     gencodec:"required"`
	Transactions  [][]byte            `json:"transactions"  gencodec:"required"`
	Withdrawals   []*types.Withdrawal `json:"withdrawals"`
}

// JSON type overrides for executableData.
type executableDataMarshaling struct {
	Number        hexutil.Uint64
	GasLimit      hexutil.Uint64
	GasUsed       hexutil.Uint64
	Timestamp     hexutil.Uint64
	BaseFeePerGas *hexutil.Big
	ExtraData     hexutil.Bytes
	LogsBloom     hexutil.Bytes
	Transactions  []hexutil.Bytes
}

//go:generate go run github.com/fjl/gencodec -type ExecutionPayloadEnvelope -field-override executionPayloadEnvelopeMarshaling -out gen_epe.go

type ExecutionPayloadEnvelope struct {
	ExecutionPayload *ExecutableData `json:"executionPayload"  gencodec:"required"`
	BlockValue       *big.Int        `json:"blockValue"  gencodec:"required"`
}

// JSON type overrides for ExecutionPayloadEnvelope.
type executionPayloadEnvelopeMarshaling struct {
	BlockValue *hexutil.Big
}

type PayloadStatusV1 struct {
	Status          string       `json:"status"`
	LatestValidHash *common.Hash `json:"latestValidHash"`
	ValidationError *string      `json:"validationError"`
}

type TransitionConfigurationV1 struct {
	TerminalTotalDifficulty *hexutil.Big   `json:"terminalTotalDifficulty"`
	TerminalBlockHash       common.Hash    `json:"terminalBlockHash"`
	TerminalBlockNumber     hexutil.Uint64 `json:"terminalBlockNumber"`
}

// PayloadID is an identifier of the payload build process
type PayloadID [8]byte

func (b PayloadID) String() string {
	return hexutil.Encode(b[:])
}

func (b PayloadID) MarshalText() ([]byte, error) {
	return hexutil.Bytes(b[:]).MarshalText()
}

func (b *PayloadID) UnmarshalText(input []byte) error {
	err := hexutil.UnmarshalFixedText("PayloadID", input, b[:])
	if err != nil {
		return fmt.Errorf("invalid payload id %q: %w", input, err)
	}
	return nil
}

type ForkChoiceResponse struct {
	PayloadStatus PayloadStatusV1 `json:"payloadStatus"`
	PayloadID     *PayloadID      `json:"payloadId"`
}

type ForkchoiceStateV1 struct {
	HeadBlockHash      common.Hash `json:"headBlockHash"`
	SafeBlockHash      common.Hash `json:"safeBlockHash"`
	FinalizedBlockHash common.Hash `json:"finalizedBlockHash"`
}

func encodeTransactions(txs []*types.Transaction) [][]byte {
	var enc = make([][]byte, len(txs))
	for i, tx := range txs {
		enc[i], _ = tx.MarshalBinary()
	}
	return enc
}

func decodeTransactions(enc [][]byte) ([]*types.Transaction, error) {
	var txs = make([]*types.Transaction, len(enc))
	for i, encTx := range enc {
		var tx types.Transaction
		if err := tx.UnmarshalBinary(encTx); err != nil {
			return nil, fmt.Errorf("invalid transaction %d: %v", i, err)
		}
		txs[i] = &tx
	}
	return txs, nil
}

// ExecutableDataToBlock constructs a block from executable data.
// It verifies that the following fields:
//
//	len(extraData) <= 32
//	uncleHash = emptyUncleHash
//	difficulty = 0
//
// and that the blockhash of the constructed block matches the parameters. Nil
// Withdrawals value will propagate through the returned block. Empty
// Withdrawals value must be passed via non-nil, length 0 value in params.
func ExecutableDataToBlock(params ExecutableData) (*types.Block, error) {
	txs, err := decodeTransactions(params.Transactions)
	if err != nil {
		return nil, err
	}
	if len(params.ExtraData) > 32 {
		return nil, fmt.Errorf("invalid extradata length: %v", len(params.ExtraData))
	}
	if len(params.LogsBloom) != 256 {
		return nil, fmt.Errorf("invalid logsBloom length: %v", len(params.LogsBloom))
	}
	// Check that baseFeePerGas is not negative or too big
	if params.BaseFeePerGas != nil && (params.BaseFeePerGas.Sign() == -1 || params.BaseFeePerGas.BitLen() > 256) {
		return nil, fmt.Errorf("invalid baseFeePerGas: %v", params.BaseFeePerGas)
	}
	// Only set withdrawalsRoot if it is non-nil. This allows CLs to use
	// ExecutableData before withdrawals are enabled by marshaling
	// Withdrawals as the json null value.
	var withdrawalsRoot *common.Hash
	if params.Withdrawals != nil {
		h := types.DeriveSha(types.Withdrawals(params.Withdrawals), trie.NewStackTrie(nil))
		withdrawalsRoot = &h
	}
	header := &types.Header{
		ParentHash:      params.ParentHash,
		UncleHash:       types.EmptyUncleHash,
		Coinbase:        params.FeeRecipient,
		Root:            params.StateRoot,
		TxHash:          types.DeriveSha(types.Transactions(txs), trie.NewStackTrie(nil)),
		ReceiptHash:     params.ReceiptsRoot,
		Bloom:           types.BytesToBloom(params.LogsBloom),
		Difficulty:      common.Big0,
		Number:          new(big.Int).SetUint64(params.Number),
		GasLimit:        params.GasLimit,
		GasUsed:         params.GasUsed,
		Time:            params.Timestamp,
		BaseFee:         params.BaseFeePerGas,
		Extra:           params.ExtraData,
		MixDigest:       params.Random,
		WithdrawalsHash: withdrawalsRoot,
	}
	block := types.NewBlockWithHeader(header).WithBody(txs, nil /* uncles */).WithWithdrawals(params.Withdrawals)
	if block.Hash() != params.BlockHash {
		return nil, fmt.Errorf("blockhash mismatch, want %x, got %x", params.BlockHash, block.Hash())
	}
	return block, nil
}

// BlockToExecutableData constructs the ExecutableData structure by filling the
// fields from the given block. It assumes the given block is post-merge block.
func BlockToExecutableData(block *types.Block, fees *big.Int) *ExecutionPayloadEnvelope {
	data := &ExecutableData{
		BlockHash:     block.Hash(),
		ParentHash:    block.ParentHash(),
		FeeRecipient:  block.Coinbase(),
		StateRoot:     block.Root(),
		Number:        block.NumberU64(),
		GasLimit:      block.GasLimit(),
		GasUsed:       block.GasUsed(),
		BaseFeePerGas: block.BaseFee(),
		Timestamp:     block.Time(),
		ReceiptsRoot:  block.ReceiptHash(),
		LogsBloom:     block.Bloom().Bytes(),
		Transactions:  encodeTransactions(block.Transactions()),
		Random:        block.MixDigest(),
		ExtraData:     block.Extra(),
		Withdrawals:   block.Withdrawals(),
	}
	return &ExecutionPayloadEnvelope{ExecutionPayload: data, BlockValue: fees}
}

// ExecutionPayloadBodyV1 is used in the response to GetPayloadBodiesByHashV1 and GetPayloadBodiesByRangeV1
type ExecutionPayloadBodyV1 struct {
	TransactionData []hexutil.Bytes     `json:"transactions"`
	Withdrawals     []*types.Withdrawal `json:"withdrawals"`
}
