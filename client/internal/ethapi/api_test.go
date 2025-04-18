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

package ethapi

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/params"
)

func TestTransaction_RoundTripRpcJSON(t *testing.T) {
	var (
		config = params.AllEthashProtocolChanges
		signer = types.LatestSigner(config)
		key, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		tests  = allTransactionTypes(common.Address{0xde, 0xad}, config)
	)
	t.Parallel()
	for i, tt := range tests {
		var tx2 types.Transaction
		tx, err := types.SignNewTx(key, signer, tt)
		if err != nil {
			t.Fatalf("test %d: signing failed: %v", i, err)
		}
		// Regular transaction
		if data, err := json.Marshal(tx); err != nil {
			t.Fatalf("test %d: marshalling failed; %v", i, err)
		} else if err = tx2.UnmarshalJSON(data); err != nil {
			t.Fatalf("test %d: sunmarshal failed: %v", i, err)
		} else if want, have := tx.Hash(), tx2.Hash(); want != have {
			t.Fatalf("test %d: stx changed, want %x have %x", i, want, have)
		}

		//  rpcTransaction
		rpcTx := newRPCTransaction(tx, common.Hash{}, 0, 0, nil, config)
		if data, err := json.Marshal(rpcTx); err != nil {
			t.Fatalf("test %d: marshalling failed; %v", i, err)
		} else if err = tx2.UnmarshalJSON(data); err != nil {
			t.Fatalf("test %d: unmarshal failed: %v", i, err)
		} else if want, have := tx.Hash(), tx2.Hash(); want != have {
			t.Fatalf("test %d: tx changed, want %x have %x", i, want, have)
		}
	}
}

func allTransactionTypes(addr common.Address, config *params.ChainConfig) []types.TxData {
	return []types.TxData{
		&types.LegacyTx{
			Nonce:    5,
			GasPrice: big.NewInt(6),
			Gas:      7,
			To:       &addr,
			Value:    big.NewInt(8),
			Data:     []byte{0, 1, 2, 3, 4},
			V:        big.NewInt(9),
			R:        big.NewInt(10),
			S:        big.NewInt(11),
		},
		&types.LegacyTx{
			Nonce:    5,
			GasPrice: big.NewInt(6),
			Gas:      7,
			To:       nil,
			Value:    big.NewInt(8),
			Data:     []byte{0, 1, 2, 3, 4},
			V:        big.NewInt(32),
			R:        big.NewInt(10),
			S:        big.NewInt(11),
		},
		&types.AccessListTx{
			ChainID:  config.ChainID,
			Nonce:    5,
			GasPrice: big.NewInt(6),
			Gas:      7,
			To:       &addr,
			Value:    big.NewInt(8),
			Data:     []byte{0, 1, 2, 3, 4},
			AccessList: types.AccessList{
				types.AccessTuple{
					Address:     common.Address{0x2},
					StorageKeys: []common.Hash{types.EmptyRootHash},
				},
			},
			V: big.NewInt(32),
			R: big.NewInt(10),
			S: big.NewInt(11),
		},
		&types.AccessListTx{
			ChainID:  config.ChainID,
			Nonce:    5,
			GasPrice: big.NewInt(6),
			Gas:      7,
			To:       nil,
			Value:    big.NewInt(8),
			Data:     []byte{0, 1, 2, 3, 4},
			AccessList: types.AccessList{
				types.AccessTuple{
					Address:     common.Address{0x2},
					StorageKeys: []common.Hash{types.EmptyRootHash},
				},
			},
			V: big.NewInt(32),
			R: big.NewInt(10),
			S: big.NewInt(11),
		},
		&types.DynamicFeeTx{
			ChainID:   config.ChainID,
			Nonce:     5,
			GasTipCap: big.NewInt(6),
			GasFeeCap: big.NewInt(9),
			Gas:       7,
			To:        &addr,
			Value:     big.NewInt(8),
			Data:      []byte{0, 1, 2, 3, 4},
			AccessList: types.AccessList{
				types.AccessTuple{
					Address:     common.Address{0x2},
					StorageKeys: []common.Hash{types.EmptyRootHash},
				},
			},
			V: big.NewInt(32),
			R: big.NewInt(10),
			S: big.NewInt(11),
		},
		&types.DynamicFeeTx{
			ChainID:    config.ChainID,
			Nonce:      5,
			GasTipCap:  big.NewInt(6),
			GasFeeCap:  big.NewInt(9),
			Gas:        7,
			To:         nil,
			Value:      big.NewInt(8),
			Data:       []byte{0, 1, 2, 3, 4},
			AccessList: types.AccessList{},
			V:          big.NewInt(32),
			R:          big.NewInt(10),
			S:          big.NewInt(11),
		},
	}
}
