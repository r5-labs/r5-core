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

package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/consensus/ethash"
	"github.com/r5-labs/r5-core/client/core"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/core/vm"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/eth"
	"github.com/r5-labs/r5-core/client/eth/ethconfig"
	"github.com/r5-labs/r5-core/client/eth/filters"
	"github.com/r5-labs/r5-core/client/node"
	"github.com/r5-labs/r5-core/client/params"

	"github.com/stretchr/testify/assert"
)

func TestBuildSchema(t *testing.T) {
	ddir := t.TempDir()
	// Copy config
	conf := node.DefaultConfig
	conf.DataDir = ddir
	stack, err := node.New(&conf)
	if err != nil {
		t.Fatalf("could not create new node: %v", err)
	}
	defer stack.Close()
	// Make sure the schema can be parsed and matched up to the object model.
	if _, err := newHandler(stack, nil, nil, []string{}, []string{}); err != nil {
		t.Errorf("Could not construct GraphQL handler: %v", err)
	}
}

// Tests that a graphQL request is successfully handled when graphql is enabled on the specified endpoint
func TestGraphQLBlockSerialization(t *testing.T) {
	stack := createNode(t)
	defer stack.Close()
	genesis := &core.Genesis{
		Config:     params.AllEthashProtocolChanges,
		GasLimit:   11500000,
		Difficulty: big.NewInt(1048576),
	}
	newGQLService(t, stack, genesis, 10, func(i int, gen *core.BlockGen) {})
	// start node
	if err := stack.Start(); err != nil {
		t.Fatalf("could not start node: %v", err)
	}

	for i, tt := range []struct {
		body string
		want string
		code int
	}{
		{ // Should return latest block
			body: `{"query": "{block{number}}","variables": null}`,
			want: `{"data":{"block":{"number":10}}}`,
			code: 200,
		},
		{ // Should return info about latest block
			body: `{"query": "{block{number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":{"number":10,"gasUsed":0,"gasLimit":11500000}}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:0){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":{"number":0,"gasUsed":0,"gasLimit":11500000}}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:-1){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":null}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:-500){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":null}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:\"0\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":{"number":0,"gasUsed":0,"gasLimit":11500000}}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:\"-33\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":null}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:\"1337\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"data":{"block":null}}`,
			code: 200,
		},
		{
			body: `{"query": "{block(number:\"0xbad\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"errors":[{"message":"strconv.ParseInt: parsing \"0xbad\": invalid syntax"}],"data":{}}`,
			code: 400,
		},
		{ // hex strings are currently not supported. If that's added to the spec, this test will need to change
			body: `{"query": "{block(number:\"0x0\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"errors":[{"message":"strconv.ParseInt: parsing \"0x0\": invalid syntax"}],"data":{}}`,
			code: 400,
		},
		{
			body: `{"query": "{block(number:\"a\"){number,gasUsed,gasLimit}}","variables": null}`,
			want: `{"errors":[{"message":"strconv.ParseInt: parsing \"a\": invalid syntax"}],"data":{}}`,
			code: 400,
		},
		{
			body: `{"query": "{bleh{number}}","variables": null}"`,
			want: `{"errors":[{"message":"Cannot query field \"bleh\" on type \"Query\".","locations":[{"line":1,"column":2}]}]}`,
			code: 400,
		},
		// should return `estimateGas` as decimal
		{
			body: `{"query": "{block{ estimateGas(data:{}) }}"}`,
			want: `{"data":{"block":{"estimateGas":53000}}}`,
			code: 200,
		},
		// should return `status` as decimal
		{
			body: `{"query": "{block {number call (data : {from : \"0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b\", to: \"0x6295ee1b4f6dd65047762f924ecd367c17eabf8f\", data :\"0x12a7b914\"}){data status}}}"}`,
			want: `{"data":{"block":{"number":10,"call":{"data":"0x","status":1}}}}`,
			code: 200,
		},
	} {
		resp, err := http.Post(fmt.Sprintf("%s/graphql", stack.HTTPEndpoint()), "application/json", strings.NewReader(tt.body))
		if err != nil {
			t.Fatalf("could not post: %v", err)
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("could not read from response body: %v", err)
		}
		if have := string(bodyBytes); have != tt.want {
			t.Errorf("testcase %d %s,\nhave:\n%v\nwant:\n%v", i, tt.body, have, tt.want)
		}
		if tt.code != resp.StatusCode {
			t.Errorf("testcase %d %s,\nwrong statuscode, have: %v, want: %v", i, tt.body, resp.StatusCode, tt.code)
		}
	}
}

func TestGraphQLBlockSerializationEIP2718(t *testing.T) {
	// Account for signing txes
	var (
		key, _  = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		address = crypto.PubkeyToAddress(key.PublicKey)
		funds   = big.NewInt(1000000000000000)
		dad     = common.HexToAddress("0x0000000000000000000000000000000000000dad")
	)
	stack := createNode(t)
	defer stack.Close()
	genesis := &core.Genesis{
		Config:     params.AllEthashProtocolChanges,
		GasLimit:   11500000,
		Difficulty: big.NewInt(1048576),
		Alloc: core.GenesisAlloc{
			address: {Balance: funds},
			// The address 0xdad sloads 0x00 and 0x01
			dad: {
				Code:    []byte{byte(vm.PC), byte(vm.PC), byte(vm.SLOAD), byte(vm.SLOAD)},
				Nonce:   0,
				Balance: big.NewInt(0),
			},
		},
		BaseFee: big.NewInt(params.InitialBaseFee),
	}
	signer := types.LatestSigner(genesis.Config)
	newGQLService(t, stack, genesis, 1, func(i int, gen *core.BlockGen) {
		gen.SetCoinbase(common.Address{1})
		tx, _ := types.SignNewTx(key, signer, &types.LegacyTx{
			Nonce:    uint64(0),
			To:       &dad,
			Value:    big.NewInt(100),
			Gas:      50000,
			GasPrice: big.NewInt(params.InitialBaseFee),
		})
		gen.AddTx(tx)
		tx, _ = types.SignNewTx(key, signer, &types.AccessListTx{
			ChainID:  genesis.Config.ChainID,
			Nonce:    uint64(1),
			To:       &dad,
			Gas:      30000,
			GasPrice: big.NewInt(params.InitialBaseFee),
			Value:    big.NewInt(50),
			AccessList: types.AccessList{{
				Address:     dad,
				StorageKeys: []common.Hash{{0}},
			}},
		})
		gen.AddTx(tx)
	})
	// start node
	if err := stack.Start(); err != nil {
		t.Fatalf("could not start node: %v", err)
	}

	for i, tt := range []struct {
		body string
		want string
		code int
	}{
		{
			body: `{"query": "{block {number transactions { from { address } to { address } value hash type accessList { address storageKeys } index}}}"}`,
			want: `{"data":{"block":{"number":1,"transactions":[{"from":{"address":"0x71562b71999873db5b286df957af199ec94617f7"},"to":{"address":"0x0000000000000000000000000000000000000dad"},"value":"0x64","hash":"0xd864c9d7d37fade6b70164740540c06dd58bb9c3f6b46101908d6339db6a6a7b","type":0,"accessList":[],"index":0},{"from":{"address":"0x71562b71999873db5b286df957af199ec94617f7"},"to":{"address":"0x0000000000000000000000000000000000000dad"},"value":"0x32","hash":"0x19b35f8187b4e15fb59a9af469dca5dfa3cd363c11d372058c12f6482477b474","type":1,"accessList":[{"address":"0x0000000000000000000000000000000000000dad","storageKeys":["0x0000000000000000000000000000000000000000000000000000000000000000"]}],"index":1}]}}}`,
			code: 200,
		},
	} {
		resp, err := http.Post(fmt.Sprintf("%s/graphql", stack.HTTPEndpoint()), "application/json", strings.NewReader(tt.body))
		if err != nil {
			t.Fatalf("could not post: %v", err)
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("could not read from response body: %v", err)
		}
		if have := string(bodyBytes); have != tt.want {
			t.Errorf("testcase %d %s,\nhave:\n%v\nwant:\n%v", i, tt.body, have, tt.want)
		}
		if tt.code != resp.StatusCode {
			t.Errorf("testcase %d %s,\nwrong statuscode, have: %v, want: %v", i, tt.body, resp.StatusCode, tt.code)
		}
	}
}

// Tests that a graphQL request is not handled successfully when graphql is not enabled on the specified endpoint
func TestGraphQLHTTPOnSamePort_GQLRequest_Unsuccessful(t *testing.T) {
	stack := createNode(t)
	defer stack.Close()
	if err := stack.Start(); err != nil {
		t.Fatalf("could not start node: %v", err)
	}
	body := strings.NewReader(`{"query": "{block{number}}","variables": null}`)
	resp, err := http.Post(fmt.Sprintf("%s/graphql", stack.HTTPEndpoint()), "application/json", body)
	if err != nil {
		t.Fatalf("could not post: %v", err)
	}
	resp.Body.Close()
	// make sure the request is not handled successfully
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGraphQLConcurrentResolvers(t *testing.T) {
	var (
		key, _  = crypto.GenerateKey()
		addr    = crypto.PubkeyToAddress(key.PublicKey)
		dadStr  = "0x0000000000000000000000000000000000000dad"
		dad     = common.HexToAddress(dadStr)
		genesis = &core.Genesis{
			Config:     params.AllEthashProtocolChanges,
			GasLimit:   11500000,
			Difficulty: big.NewInt(1048576),
			Alloc: core.GenesisAlloc{
				addr: {Balance: big.NewInt(params.Ether)},
				dad: {
					// LOG0(0, 0), LOG0(0, 0), RETURN(0, 0)
					Code:    common.Hex2Bytes("60006000a060006000a060006000f3"),
					Nonce:   0,
					Balance: big.NewInt(0),
				},
			},
		}
		signer = types.LatestSigner(genesis.Config)
		stack  = createNode(t)
	)
	defer stack.Close()

	var tx *types.Transaction
	handler, chain := newGQLService(t, stack, genesis, 1, func(i int, gen *core.BlockGen) {
		tx, _ = types.SignNewTx(key, signer, &types.LegacyTx{To: &dad, Gas: 100000, GasPrice: big.NewInt(params.InitialBaseFee)})
		gen.AddTx(tx)
		tx, _ = types.SignNewTx(key, signer, &types.LegacyTx{To: &dad, Nonce: 1, Gas: 100000, GasPrice: big.NewInt(params.InitialBaseFee)})
		gen.AddTx(tx)
		tx, _ = types.SignNewTx(key, signer, &types.LegacyTx{To: &dad, Nonce: 2, Gas: 100000, GasPrice: big.NewInt(params.InitialBaseFee)})
		gen.AddTx(tx)
	})
	// start node
	if err := stack.Start(); err != nil {
		t.Fatalf("could not start node: %v", err)
	}

	for i, tt := range []struct {
		body string
		want string
	}{
		// Multiple txes race to get/set the block hash.
		{
			body: "{block { transactions { logs { account { address } } } } }",
			want: fmt.Sprintf(`{"block":{"transactions":[{"logs":[{"account":{"address":"%s"}},{"account":{"address":"%s"}}]},{"logs":[{"account":{"address":"%s"}},{"account":{"address":"%s"}}]},{"logs":[{"account":{"address":"%s"}},{"account":{"address":"%s"}}]}]}}`, dadStr, dadStr, dadStr, dadStr, dadStr, dadStr),
		},
		// Multiple fields of a tx race to resolve it. Happens in this case
		// because resolving the tx body belonging to a log is delayed.
		{
			body: `{block { logs(filter: {}) { transaction { nonce value gasPrice }}}}`,
			want: `{"block":{"logs":[{"transaction":{"nonce":"0x0","value":"0x0","gasPrice":"0x3b9aca00"}},{"transaction":{"nonce":"0x0","value":"0x0","gasPrice":"0x3b9aca00"}},{"transaction":{"nonce":"0x1","value":"0x0","gasPrice":"0x3b9aca00"}},{"transaction":{"nonce":"0x1","value":"0x0","gasPrice":"0x3b9aca00"}},{"transaction":{"nonce":"0x2","value":"0x0","gasPrice":"0x3b9aca00"}},{"transaction":{"nonce":"0x2","value":"0x0","gasPrice":"0x3b9aca00"}}]}}`,
		},
		// Multiple txes of a block race to set/retrieve receipts of a block.
		{
			body: "{block { transactions { status gasUsed } } }",
			want: `{"block":{"transactions":[{"status":1,"gasUsed":21768},{"status":1,"gasUsed":21768},{"status":1,"gasUsed":21768}]}}`,
		},
		// Multiple fields of block race to resolve header and body.
		{
			body: "{ block { number hash gasLimit ommerCount transactionCount totalDifficulty } }",
			want: fmt.Sprintf(`{"block":{"number":1,"hash":"%s","gasLimit":11500000,"ommerCount":0,"transactionCount":3,"totalDifficulty":"0x200000"}}`, chain[len(chain)-1].Hash()),
		},
		// Multiple fields of a block race to resolve the header and body.
		{
			body: fmt.Sprintf(`{ transaction(hash: "%s") { block { number hash gasLimit ommerCount transactionCount } } }`, tx.Hash()),
			want: fmt.Sprintf(`{"transaction":{"block":{"number":1,"hash":"%s","gasLimit":11500000,"ommerCount":0,"transactionCount":3}}}`, chain[len(chain)-1].Hash()),
		},
		// Account fields race the resolve the state object.
		{
			body: fmt.Sprintf(`{ block { account(address: "%s") { balance transactionCount code } } }`, dadStr),
			want: `{"block":{"account":{"balance":"0x0","transactionCount":"0x0","code":"0x60006000a060006000a060006000f3"}}}`,
		},
		// Test values for a non-existent account.
		{
			body: fmt.Sprintf(`{ block { account(address: "%s") { balance transactionCount code } } }`, "0x1111111111111111111111111111111111111111"),
			want: `{"block":{"account":{"balance":"0x0","transactionCount":"0x0","code":"0x"}}}`,
		},
	} {
		res := handler.Schema.Exec(context.Background(), tt.body, "", map[string]interface{}{})
		if res.Errors != nil {
			t.Fatalf("failed to execute query for testcase #%d: %v", i, res.Errors)
		}
		have, err := json.Marshal(res.Data)
		if err != nil {
			t.Fatalf("failed to encode graphql response for testcase #%d: %s", i, err)
		}
		if string(have) != tt.want {
			t.Errorf("response unmatch for testcase #%d.\nExpected:\n%s\nGot:\n%s\n", i, tt.want, have)
		}
	}
}

func createNode(t *testing.T) *node.Node {
	stack, err := node.New(&node.Config{
		HTTPHost:     "127.0.0.1",
		HTTPPort:     0,
		WSHost:       "127.0.0.1",
		WSPort:       0,
		HTTPTimeouts: node.DefaultConfig.HTTPTimeouts,
	})
	if err != nil {
		t.Fatalf("could not create node: %v", err)
	}
	return stack
}

func newGQLService(t *testing.T, stack *node.Node, gspec *core.Genesis, genBlocks int, genfunc func(i int, gen *core.BlockGen)) (*handler, []*types.Block) {
	ethConf := &ethconfig.Config{
		Genesis: gspec,
		Ethash: ethash.Config{
			PowMode: ethash.ModeFake,
		},
		NetworkId:               1337,
		TrieCleanCache:          5,
		TrieCleanCacheJournal:   "triecache",
		TrieCleanCacheRejournal: 60 * time.Minute,
		TrieDirtyCache:          5,
		TrieTimeout:             60 * time.Minute,
		SnapshotCache:           5,
	}
	ethBackend, err := eth.New(stack, ethConf)
	if err != nil {
		t.Fatalf("could not create eth backend: %v", err)
	}
	// Create some blocks and import them
	chain, _ := core.GenerateChain(params.AllEthashProtocolChanges, ethBackend.BlockChain().Genesis(),
		ethash.NewFaker(), ethBackend.ChainDb(), genBlocks, genfunc)
	_, err = ethBackend.BlockChain().InsertChain(chain)
	if err != nil {
		t.Fatalf("could not create import blocks: %v", err)
	}
	// Set up handler
	filterSystem := filters.NewFilterSystem(ethBackend.APIBackend, filters.Config{})
	handler, err := newHandler(stack, ethBackend.APIBackend, filterSystem, []string{}, []string{})
	if err != nil {
		t.Fatalf("could not create graphql service: %v", err)
	}
	return handler, chain
}
