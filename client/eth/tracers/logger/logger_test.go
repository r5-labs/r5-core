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

package logger

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/state"
	"github.com/r5-labs/r5-core/client/core/vm"
	"github.com/r5-labs/r5-core/client/params"
)

type dummyContractRef struct {
	calledForEach bool
}

func (dummyContractRef) Address() common.Address     { return common.Address{} }
func (dummyContractRef) Value() *big.Int             { return new(big.Int) }
func (dummyContractRef) SetCode(common.Hash, []byte) {}
func (d *dummyContractRef) ForEachStorage(callback func(key, value common.Hash) bool) {
	d.calledForEach = true
}
func (d *dummyContractRef) SubBalance(amount *big.Int) {}
func (d *dummyContractRef) AddBalance(amount *big.Int) {}
func (d *dummyContractRef) SetBalance(*big.Int)        {}
func (d *dummyContractRef) SetNonce(uint64)            {}
func (d *dummyContractRef) Balance() *big.Int          { return new(big.Int) }

type dummyStatedb struct {
	state.StateDB
}

func (*dummyStatedb) GetRefund() uint64                                       { return 1337 }
func (*dummyStatedb) GetState(_ common.Address, _ common.Hash) common.Hash    { return common.Hash{} }
func (*dummyStatedb) SetState(_ common.Address, _ common.Hash, _ common.Hash) {}

func TestStoreCapture(t *testing.T) {
	var (
		logger   = NewStructLogger(nil)
		env      = vm.NewEVM(vm.BlockContext{}, vm.TxContext{}, &dummyStatedb{}, params.TestChainConfig, vm.Config{Tracer: logger})
		contract = vm.NewContract(&dummyContractRef{}, &dummyContractRef{}, new(big.Int), 100000)
	)
	contract.Code = []byte{byte(vm.PUSH1), 0x1, byte(vm.PUSH1), 0x0, byte(vm.SSTORE)}
	var index common.Hash
	logger.CaptureStart(env, common.Address{}, contract.Address(), false, nil, 0, nil)
	_, err := env.Interpreter().Run(contract, []byte{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(logger.storage[contract.Address()]) == 0 {
		t.Fatalf("expected exactly 1 changed value on address %x, got %d", contract.Address(),
			len(logger.storage[contract.Address()]))
	}
	exp := common.BigToHash(big.NewInt(1))
	if logger.storage[contract.Address()][index] != exp {
		t.Errorf("expected %x, got %x", exp, logger.storage[contract.Address()][index])
	}
}

// Tests that blank fields don't appear in logs when JSON marshalled, to reduce
// logs bloat and confusion. See https://github.com/r5-labs/r5-core/client/issues/24487
func TestStructLogMarshalingOmitEmpty(t *testing.T) {
	tests := []struct {
		name string
		log  *StructLog
		want string
	}{
		{"empty err and no fields", &StructLog{},
			`{"pc":0,"op":0,"gas":"0x0","gasCost":"0x0","memSize":0,"stack":null,"depth":0,"refund":0,"opName":"STOP"}`},
		{"with err", &StructLog{Err: fmt.Errorf("this failed")},
			`{"pc":0,"op":0,"gas":"0x0","gasCost":"0x0","memSize":0,"stack":null,"depth":0,"refund":0,"opName":"STOP","error":"this failed"}`},
		{"with mem", &StructLog{Memory: make([]byte, 2), MemorySize: 2},
			`{"pc":0,"op":0,"gas":"0x0","gasCost":"0x0","memory":"0x0000","memSize":2,"stack":null,"depth":0,"refund":0,"opName":"STOP"}`},
		{"with 0-size mem", &StructLog{Memory: make([]byte, 0)},
			`{"pc":0,"op":0,"gas":"0x0","gasCost":"0x0","memSize":0,"stack":null,"depth":0,"refund":0,"opName":"STOP"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blob, err := json.Marshal(tt.log)
			if err != nil {
				t.Fatal(err)
			}
			if have, want := string(blob), tt.want; have != want {
				t.Fatalf("mismatched results\n\thave: %v\n\twant: %v", have, want)
			}
		})
	}
}
