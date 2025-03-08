// Copyright 2025 R5
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

package snapshot

import (
	"bytes"
	"math/big"

	"github.com/r5-labs/r5-core/common"
	"github.com/r5-labs/r5-core/core/types"
	"github.com/r5-labs/r5-core/rlp"
)

// Account is a modified version of a state.Account, where the root is replaced
// with a byte slice. This format can be used to represent full-consensus format
// or slim-snapshot format which replaces the empty root and code hash as nil
// byte slice.
type Account struct {
	Nonce    uint64
	Balance  *big.Int
	Root     []byte
	CodeHash []byte
}

// SlimAccount converts a state.Account content into a slim snapshot account
func SlimAccount(nonce uint64, balance *big.Int, root common.Hash, codehash []byte) Account {
	slim := Account{
		Nonce:   nonce,
		Balance: balance,
	}
	if root != types.EmptyRootHash {
		slim.Root = root[:]
	}
	if !bytes.Equal(codehash, types.EmptyCodeHash[:]) {
		slim.CodeHash = codehash
	}
	return slim
}

// SlimAccountRLP converts a state.Account content into a slim snapshot
// version RLP encoded.
func SlimAccountRLP(nonce uint64, balance *big.Int, root common.Hash, codehash []byte) []byte {
	data, err := rlp.EncodeToBytes(SlimAccount(nonce, balance, root, codehash))
	if err != nil {
		panic(err)
	}
	return data
}

// FullAccount decodes the data on the 'slim RLP' format and return
// the consensus format account.
func FullAccount(data []byte) (Account, error) {
	var account Account
	if err := rlp.DecodeBytes(data, &account); err != nil {
		return Account{}, err
	}
	if len(account.Root) == 0 {
		account.Root = types.EmptyRootHash[:]
	}
	if len(account.CodeHash) == 0 {
		account.CodeHash = types.EmptyCodeHash[:]
	}
	return account, nil
}

// FullAccountRLP converts data on the 'slim RLP' format into the full RLP-format.
func FullAccountRLP(data []byte) ([]byte, error) {
	account, err := FullAccount(data)
	if err != nil {
		return nil, err
	}
	return rlp.EncodeToBytes(account)
}
