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

// This file contains a miner stress test for eip 1559.
package main

import (
	"crypto/ecdsa"
	crand "crypto/rand"
	"math/big"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/common/fdlimit"
	"github.com/r5-labs/r5-core/client/consensus/ethash"
	"github.com/r5-labs/r5-core/client/core"
	"github.com/r5-labs/r5-core/client/core/txpool"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/eth"
	"github.com/r5-labs/r5-core/client/eth/downloader"
	"github.com/r5-labs/r5-core/client/eth/ethconfig"
	"github.com/r5-labs/r5-core/client/log"
	"github.com/r5-labs/r5-core/client/miner"
	"github.com/r5-labs/r5-core/client/node"
	"github.com/r5-labs/r5-core/client/p2p"
	"github.com/r5-labs/r5-core/client/p2p/enode"
	"github.com/r5-labs/r5-core/client/params"
)

var (
	londonBlock = big.NewInt(30) // Predefined london fork block for activating eip 1559.
)

func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
	fdlimit.Raise(2048)

	// Generate a batch of accounts to seal and fund with
	faucets := make([]*ecdsa.PrivateKey, 128)
	for i := 0; i < len(faucets); i++ {
		faucets[i], _ = crypto.GenerateKey()
	}
	// Pre-generate the ethash mining DAG so we don't race
	ethash.MakeDataset(1, ethconfig.Defaults.Ethash.DatasetDir)

	// Create an Ethash network
	genesis := makeGenesis(faucets)

	// Handle interrupts.
	interruptCh := make(chan os.Signal, 5)
	signal.Notify(interruptCh, os.Interrupt)

	var (
		stacks []*node.Node
		nodes  []*eth.Ethereum
		enodes []*enode.Node
	)
	for i := 0; i < 4; i++ {
		// Start the node and wait until it's up
		stack, ethBackend, err := makeMiner(genesis)
		if err != nil {
			panic(err)
		}
		defer stack.Close()

		for stack.Server().NodeInfo().Ports.Listener == 0 {
			time.Sleep(250 * time.Millisecond)
		}
		// Connect the node to all the previous ones
		for _, n := range enodes {
			stack.Server().AddPeer(n)
		}
		// Start tracking the node and its enode
		nodes = append(nodes, ethBackend)
		enodes = append(enodes, stack.Server().Self())
	}

	// Iterate over all the nodes and start mining
	time.Sleep(3 * time.Second)
	for _, node := range nodes {
		if err := node.StartMining(1); err != nil {
			panic(err)
		}
	}
	time.Sleep(3 * time.Second)

	// Start injecting transactions from the faucets like crazy
	var (
		nonces = make([]uint64, len(faucets))

		// The signer activates the 1559 features even before the fork,
		// so the new 1559 txs can be created with this signer.
		signer = types.LatestSignerForChainID(genesis.Config.ChainID)
	)
	for {
		// Stop when interrupted.
		select {
		case <-interruptCh:
			for _, node := range stacks {
				node.Close()
			}
			return
		default:
		}

		// Pick a random mining node
		index := rand.Intn(len(faucets))
		backend := nodes[index%len(nodes)]

		headHeader := backend.BlockChain().CurrentHeader()
		baseFee := headHeader.BaseFee

		// Create a self transaction and inject into the pool. The legacy
		// and 1559 transactions can all be created by random even if the
		// fork is not happened.
		tx := makeTransaction(nonces[index], faucets[index], signer, baseFee)
		if err := backend.TxPool().AddLocal(tx); err != nil {
			continue
		}
		nonces[index]++

		// Wait if we're too saturated
		if pend, _ := backend.TxPool().Stats(); pend > 4192 {
			time.Sleep(100 * time.Millisecond)
		}

		// Wait if the basefee is raised too fast
		if baseFee != nil && baseFee.Cmp(new(big.Int).Mul(big.NewInt(100), big.NewInt(params.GWei))) > 0 {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func makeTransaction(nonce uint64, privKey *ecdsa.PrivateKey, signer types.Signer, baseFee *big.Int) *types.Transaction {
	// Generate legacy transaction
	if rand.Intn(2) == 0 {
		tx, err := types.SignTx(types.NewTransaction(nonce, crypto.PubkeyToAddress(privKey.PublicKey), new(big.Int), 21000, big.NewInt(100000000000+rand.Int63n(65536)), nil), signer, privKey)
		if err != nil {
			panic(err)
		}
		return tx
	}
	// Generate eip 1559 transaction
	recipient := crypto.PubkeyToAddress(privKey.PublicKey)

	// Feecap and feetip are limited to 32 bytes. Offer a sightly
	// larger buffer for creating both valid and invalid transactions.
	var buf = make([]byte, 32+5)
	crand.Read(buf)
	gasTipCap := new(big.Int).SetBytes(buf)

	// If the given base fee is nil(the 1559 is still not available),
	// generate a fake base fee in order to create 1559 tx forcibly.
	if baseFee == nil {
		baseFee = new(big.Int).SetInt64(int64(rand.Int31()))
	}
	// Generate the feecap, 75% valid feecap and 25% unguaranteed.
	var gasFeeCap *big.Int
	if rand.Intn(4) == 0 {
		crand.Read(buf)
		gasFeeCap = new(big.Int).SetBytes(buf)
	} else {
		gasFeeCap = new(big.Int).Add(baseFee, gasTipCap)
	}
	return types.MustSignNewTx(privKey, signer, &types.DynamicFeeTx{
		ChainID:    signer.ChainID(),
		Nonce:      nonce,
		GasTipCap:  gasTipCap,
		GasFeeCap:  gasFeeCap,
		Gas:        21000,
		To:         &recipient,
		Value:      big.NewInt(100),
		Data:       nil,
		AccessList: nil,
	})
}

// makeGenesis creates a custom Ethash genesis block based on some pre-defined
// faucet accounts.
func makeGenesis(faucets []*ecdsa.PrivateKey) *core.Genesis {
	genesis := core.DefaultGenesisBlock()

	genesis.Config = params.AllEthashProtocolChanges
	genesis.Config.LondonBlock = londonBlock
	genesis.Difficulty = params.MinimumDifficulty

	// Small gaslimit for easier basefee moving testing.
	genesis.GasLimit = 8_000_000

	genesis.Config.ChainID = big.NewInt(18)

	genesis.Alloc = core.GenesisAlloc{}
	for _, faucet := range faucets {
		genesis.Alloc[crypto.PubkeyToAddress(faucet.PublicKey)] = core.GenesisAccount{
			Balance: new(big.Int).Exp(big.NewInt(2), big.NewInt(128), nil),
		}
	}
	if londonBlock.Sign() == 0 {
		log.Info("Enabled the eip 1559 by default")
	} else {
		log.Info("Registered the london fork", "number", londonBlock)
	}
	return genesis
}

func makeMiner(genesis *core.Genesis) (*node.Node, *eth.Ethereum, error) {
	// Define the basic configurations for the Ethereum node
	datadir, _ := os.MkdirTemp("", "")

	config := &node.Config{
		Name:    "geth",
		Version: params.Version,
		DataDir: datadir,
		P2P: p2p.Config{
			ListenAddr:  "0.0.0.0:0",
			NoDiscovery: true,
			MaxPeers:    25,
		},
		UseLightweightKDF: true,
	}
	// Create the node and configure a full Ethereum node on it
	stack, err := node.New(config)
	if err != nil {
		return nil, nil, err
	}
	ethBackend, err := eth.New(stack, &ethconfig.Config{
		Genesis:         genesis,
		NetworkId:       genesis.Config.ChainID.Uint64(),
		SyncMode:        downloader.FullSync,
		DatabaseCache:   256,
		DatabaseHandles: 256,
		TxPool:          txpool.DefaultConfig,
		GPO:             ethconfig.Defaults.GPO,
		Ethash:          ethconfig.Defaults.Ethash,
		Miner: miner.Config{
			Etherbase: common.Address{1},
			GasCeil:   genesis.GasLimit * 11 / 10,
			GasPrice:  big.NewInt(1),
			Recommit:  time.Second,
		},
	})
	if err != nil {
		return nil, nil, err
	}
	err = stack.Start()
	return stack, ethBackend, err
}
