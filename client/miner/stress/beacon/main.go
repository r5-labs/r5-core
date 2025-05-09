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

// This file contains a miner stress test for the eth1/2 transition
package main

import (
	"crypto/ecdsa"
	"errors"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/r5-labs/r5-core/client/accounts/keystore"
	"github.com/r5-labs/r5-core/client/beacon/engine"
	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/common/fdlimit"
	"github.com/r5-labs/r5-core/client/consensus/ethash"
	"github.com/r5-labs/r5-core/client/core"
	"github.com/r5-labs/r5-core/client/core/txpool"
	"github.com/r5-labs/r5-core/client/core/types"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/eth"
	ethcatalyst "github.com/r5-labs/r5-core/client/eth/catalyst"
	"github.com/r5-labs/r5-core/client/eth/downloader"
	"github.com/r5-labs/r5-core/client/eth/ethconfig"
	"github.com/r5-labs/r5-core/client/les"
	lescatalyst "github.com/r5-labs/r5-core/client/les/catalyst"
	"github.com/r5-labs/r5-core/client/log"
	"github.com/r5-labs/r5-core/client/miner"
	"github.com/r5-labs/r5-core/client/node"
	"github.com/r5-labs/r5-core/client/p2p"
	"github.com/r5-labs/r5-core/client/p2p/enode"
	"github.com/r5-labs/r5-core/client/params"
)

type nodetype int

const (
	legacyMiningNode nodetype = iota
	legacyNormalNode
	eth2MiningNode
	eth2NormalNode
	eth2LightClient
)

func (typ nodetype) String() string {
	switch typ {
	case legacyMiningNode:
		return "legacyMiningNode"
	case legacyNormalNode:
		return "legacyNormalNode"
	case eth2MiningNode:
		return "eth2MiningNode"
	case eth2NormalNode:
		return "eth2NormalNode"
	case eth2LightClient:
		return "eth2LightClient"
	default:
		return "undefined"
	}
}

var (
	// transitionDifficulty is the target total difficulty for transition
	transitionDifficulty = new(big.Int).Mul(big.NewInt(20), params.MinimumDifficulty)

	// blockInterval is the time interval for creating a new eth2 block
	blockIntervalInt = 3
	blockInterval    = time.Second * time.Duration(blockIntervalInt)

	// finalizationDist is the block distance for finalizing block
	finalizationDist = 10
)

type ethNode struct {
	typ        nodetype
	stack      *node.Node
	enode      *enode.Node
	api        *ethcatalyst.ConsensusAPI
	ethBackend *eth.Ethereum
	lapi       *lescatalyst.ConsensusAPI
	lesBackend *les.LightEthereum
}

func newNode(typ nodetype, genesis *core.Genesis, enodes []*enode.Node) *ethNode {
	var (
		err        error
		api        *ethcatalyst.ConsensusAPI
		lapi       *lescatalyst.ConsensusAPI
		stack      *node.Node
		ethBackend *eth.Ethereum
		lesBackend *les.LightEthereum
	)
	// Start the node and wait until it's up
	if typ == eth2LightClient {
		stack, lesBackend, lapi, err = makeLightNode(genesis)
	} else {
		stack, ethBackend, api, err = makeFullNode(genesis)
	}
	if err != nil {
		panic(err)
	}
	for stack.Server().NodeInfo().Ports.Listener == 0 {
		time.Sleep(250 * time.Millisecond)
	}
	// Connect the node to all the previous ones
	for _, n := range enodes {
		stack.Server().AddPeer(n)
	}
	enode := stack.Server().Self()

	// Inject the signer key and start sealing with it
	stack.AccountManager().AddBackend(keystore.NewPlaintextKeyStore("beacon-stress"))
	ks := stack.AccountManager().Backends(keystore.KeyStoreType)
	if len(ks) == 0 {
		panic("Keystore is not available")
	}
	store := ks[0].(*keystore.KeyStore)
	if _, err := store.NewAccount(""); err != nil {
		panic(err)
	}
	return &ethNode{
		typ:        typ,
		api:        api,
		ethBackend: ethBackend,
		lapi:       lapi,
		lesBackend: lesBackend,
		stack:      stack,
		enode:      enode,
	}
}

func (n *ethNode) assembleBlock(parentHash common.Hash, parentTimestamp uint64) (*engine.ExecutableData, error) {
	if n.typ != eth2MiningNode {
		return nil, errors.New("invalid node type")
	}
	timestamp := uint64(time.Now().Unix())
	if timestamp <= parentTimestamp {
		timestamp = parentTimestamp + 1
	}
	payloadAttribute := engine.PayloadAttributes{
		Timestamp:             timestamp,
		Random:                common.Hash{},
		SuggestedFeeRecipient: common.HexToAddress("0xdeadbeef"),
	}
	fcState := engine.ForkchoiceStateV1{
		HeadBlockHash:      parentHash,
		SafeBlockHash:      common.Hash{},
		FinalizedBlockHash: common.Hash{},
	}
	payload, err := n.api.ForkchoiceUpdatedV1(fcState, &payloadAttribute)
	if err != nil {
		return nil, err
	}
	time.Sleep(time.Second * 5) // give enough time for block creation
	return n.api.GetPayloadV1(*payload.PayloadID)
}

func (n *ethNode) insertBlock(eb engine.ExecutableData) error {
	if !eth2types(n.typ) {
		return errors.New("invalid node type")
	}
	switch n.typ {
	case eth2NormalNode, eth2MiningNode:
		newResp, err := n.api.NewPayloadV1(eb)
		if err != nil {
			return err
		} else if newResp.Status != "VALID" {
			return errors.New("failed to insert block")
		}
		return nil
	case eth2LightClient:
		newResp, err := n.lapi.ExecutePayloadV1(eb)
		if err != nil {
			return err
		} else if newResp.Status != "VALID" {
			return errors.New("failed to insert block")
		}
		return nil
	default:
		return errors.New("undefined node")
	}
}

func (n *ethNode) insertBlockAndSetHead(parent *types.Header, ed engine.ExecutableData) error {
	if !eth2types(n.typ) {
		return errors.New("invalid node type")
	}
	if err := n.insertBlock(ed); err != nil {
		return err
	}
	block, err := engine.ExecutableDataToBlock(ed)
	if err != nil {
		return err
	}
	fcState := engine.ForkchoiceStateV1{
		HeadBlockHash:      block.ParentHash(),
		SafeBlockHash:      common.Hash{},
		FinalizedBlockHash: common.Hash{},
	}
	switch n.typ {
	case eth2NormalNode, eth2MiningNode:
		if _, err := n.api.ForkchoiceUpdatedV1(fcState, nil); err != nil {
			return err
		}
		return nil
	case eth2LightClient:
		if _, err := n.lapi.ForkchoiceUpdatedV1(fcState, nil); err != nil {
			return err
		}
		return nil
	default:
		return errors.New("undefined node")
	}
}

type nodeManager struct {
	genesis      *core.Genesis
	genesisBlock *types.Block
	nodes        []*ethNode
	enodes       []*enode.Node
	close        chan struct{}
}

func newNodeManager(genesis *core.Genesis) *nodeManager {
	return &nodeManager{
		close:        make(chan struct{}),
		genesis:      genesis,
		genesisBlock: genesis.ToBlock(),
	}
}

func (mgr *nodeManager) createNode(typ nodetype) {
	node := newNode(typ, mgr.genesis, mgr.enodes)
	mgr.nodes = append(mgr.nodes, node)
	mgr.enodes = append(mgr.enodes, node.enode)
}

func (mgr *nodeManager) getNodes(typ nodetype) []*ethNode {
	var ret []*ethNode
	for _, node := range mgr.nodes {
		if node.typ == typ {
			ret = append(ret, node)
		}
	}
	return ret
}

func (mgr *nodeManager) startMining() {
	for _, node := range append(mgr.getNodes(eth2MiningNode), mgr.getNodes(legacyMiningNode)...) {
		if err := node.ethBackend.StartMining(1); err != nil {
			panic(err)
		}
	}
}

func (mgr *nodeManager) shutdown() {
	close(mgr.close)
	for _, node := range mgr.nodes {
		node.stack.Close()
	}
}

func (mgr *nodeManager) run() {
	if len(mgr.nodes) == 0 {
		return
	}
	chain := mgr.nodes[0].ethBackend.BlockChain()
	sink := make(chan core.ChainHeadEvent, 1024)
	sub := chain.SubscribeChainHeadEvent(sink)
	defer sub.Unsubscribe()

	var (
		transitioned bool
		parentBlock  *types.Block
		waitFinalise []*types.Block
	)
	timer := time.NewTimer(0)
	defer timer.Stop()
	<-timer.C // discard the initial tick

	// Handle the by default transition.
	if transitionDifficulty.Sign() == 0 {
		transitioned = true
		parentBlock = mgr.genesisBlock
		timer.Reset(blockInterval)
		log.Info("Enable the transition by default")
	}

	// Handle the block finalization.
	checkFinalise := func() {
		if parentBlock == nil {
			return
		}
		if len(waitFinalise) == 0 {
			return
		}
		oldest := waitFinalise[0]
		if oldest.NumberU64() > parentBlock.NumberU64() {
			return
		}
		distance := parentBlock.NumberU64() - oldest.NumberU64()
		if int(distance) < finalizationDist {
			return
		}
		nodes := mgr.getNodes(eth2MiningNode)
		nodes = append(nodes, mgr.getNodes(eth2NormalNode)...)
		//nodes = append(nodes, mgr.getNodes(eth2LightClient)...)
		for _, node := range nodes {
			fcState := engine.ForkchoiceStateV1{
				HeadBlockHash:      parentBlock.Hash(),
				SafeBlockHash:      oldest.Hash(),
				FinalizedBlockHash: oldest.Hash(),
			}
			node.api.ForkchoiceUpdatedV1(fcState, nil)
		}
		log.Info("Finalised eth2 block", "number", oldest.NumberU64(), "hash", oldest.Hash())
		waitFinalise = waitFinalise[1:]
	}

	for {
		checkFinalise()
		select {
		case <-mgr.close:
			return

		case ev := <-sink:
			if transitioned {
				continue
			}
			td := chain.GetTd(ev.Block.Hash(), ev.Block.NumberU64())
			if td.Cmp(transitionDifficulty) < 0 {
				continue
			}
			transitioned, parentBlock = true, ev.Block
			timer.Reset(blockInterval)
			log.Info("Transition difficulty reached", "td", td, "target", transitionDifficulty, "number", ev.Block.NumberU64(), "hash", ev.Block.Hash())

		case <-timer.C:
			producers := mgr.getNodes(eth2MiningNode)
			if len(producers) == 0 {
				continue
			}
			hash, timestamp := parentBlock.Hash(), parentBlock.Time()
			if parentBlock.NumberU64() == 0 {
				timestamp = uint64(time.Now().Unix()) - uint64(blockIntervalInt)
			}
			ed, err := producers[0].assembleBlock(hash, timestamp)
			if err != nil {
				log.Error("Failed to assemble the block", "err", err)
				continue
			}
			block, _ := engine.ExecutableDataToBlock(*ed)

			nodes := mgr.getNodes(eth2MiningNode)
			nodes = append(nodes, mgr.getNodes(eth2NormalNode)...)
			nodes = append(nodes, mgr.getNodes(eth2LightClient)...)
			for _, node := range nodes {
				if err := node.insertBlockAndSetHead(parentBlock.Header(), *ed); err != nil {
					log.Error("Failed to insert block", "type", node.typ, "err", err)
				}
			}
			log.Info("Create and insert eth2 block", "number", ed.Number)
			parentBlock = block
			waitFinalise = append(waitFinalise, block)
			timer.Reset(blockInterval)
		}
	}
}

func main() {
	log.Root().SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
	fdlimit.Raise(2048)

	// Generate a batch of accounts to seal and fund with
	faucets := make([]*ecdsa.PrivateKey, 16)
	for i := 0; i < len(faucets); i++ {
		faucets[i], _ = crypto.GenerateKey()
	}
	// Pre-generate the ethash mining DAG so we don't race
	ethash.MakeDataset(1, filepath.Join(os.Getenv("HOME"), ".ethash"))

	// Create an Ethash network
	genesis := makeGenesis(faucets)
	manager := newNodeManager(genesis)
	defer manager.shutdown()

	manager.createNode(eth2NormalNode)
	manager.createNode(eth2MiningNode)
	manager.createNode(legacyMiningNode)
	manager.createNode(legacyNormalNode)
	manager.createNode(eth2LightClient)

	// Iterate over all the nodes and start mining
	time.Sleep(3 * time.Second)
	if transitionDifficulty.Sign() != 0 {
		manager.startMining()
	}
	go manager.run()

	// Start injecting transactions from the faucets like crazy
	time.Sleep(3 * time.Second)
	nonces := make([]uint64, len(faucets))
	for {
		// Pick a random mining node
		nodes := manager.getNodes(eth2MiningNode)

		index := rand.Intn(len(faucets))
		node := nodes[index%len(nodes)]

		// Create a self transaction and inject into the pool
		tx, err := types.SignTx(types.NewTransaction(nonces[index], crypto.PubkeyToAddress(faucets[index].PublicKey), new(big.Int), 21000, big.NewInt(10_000_000_000+rand.Int63n(6_553_600_000)), nil), types.HomesteadSigner{}, faucets[index])
		if err != nil {
			panic(err)
		}
		if err := node.ethBackend.TxPool().AddLocal(tx); err != nil {
			panic(err)
		}
		nonces[index]++

		// Wait if we're too saturated
		if pend, _ := node.ethBackend.TxPool().Stats(); pend > 2048 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// makeGenesis creates a custom Ethash genesis block based on some pre-defined
// faucet accounts.
func makeGenesis(faucets []*ecdsa.PrivateKey) *core.Genesis {
	genesis := core.DefaultGenesisBlock()
	genesis.Difficulty = params.MinimumDifficulty
	genesis.GasLimit = 25000000

	genesis.BaseFee = big.NewInt(params.InitialBaseFee)
	genesis.Config = params.AllEthashProtocolChanges
	genesis.Config.TerminalTotalDifficulty = transitionDifficulty

	genesis.Alloc = core.GenesisAlloc{}
	for _, faucet := range faucets {
		genesis.Alloc[crypto.PubkeyToAddress(faucet.PublicKey)] = core.GenesisAccount{
			Balance: new(big.Int).Exp(big.NewInt(2), big.NewInt(128), nil),
		}
	}
	return genesis
}

func makeFullNode(genesis *core.Genesis) (*node.Node, *eth.Ethereum, *ethcatalyst.ConsensusAPI, error) {
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
		return nil, nil, nil, err
	}
	econfig := &ethconfig.Config{
		Genesis:         genesis,
		NetworkId:       genesis.Config.ChainID.Uint64(),
		SyncMode:        downloader.FullSync,
		DatabaseCache:   256,
		DatabaseHandles: 256,
		TxPool:          txpool.DefaultConfig,
		GPO:             ethconfig.Defaults.GPO,
		Ethash:          ethconfig.Defaults.Ethash,
		Miner: miner.Config{
			GasFloor: genesis.GasLimit * 9 / 10,
			GasCeil:  genesis.GasLimit * 11 / 10,
			GasPrice: big.NewInt(1),
			Recommit: 1 * time.Second,
		},
		LightServ:        100,
		LightPeers:       10,
		LightNoSyncServe: true,
	}
	ethBackend, err := eth.New(stack, econfig)
	if err != nil {
		return nil, nil, nil, err
	}
	_, err = les.NewLesServer(stack, ethBackend, econfig)
	if err != nil {
		log.Crit("Failed to create the LES server", "err", err)
	}
	err = stack.Start()
	return stack, ethBackend, ethcatalyst.NewConsensusAPI(ethBackend), err
}

func makeLightNode(genesis *core.Genesis) (*node.Node, *les.LightEthereum, *lescatalyst.ConsensusAPI, error) {
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
		return nil, nil, nil, err
	}
	lesBackend, err := les.New(stack, &ethconfig.Config{
		Genesis:         genesis,
		NetworkId:       genesis.Config.ChainID.Uint64(),
		SyncMode:        downloader.LightSync,
		DatabaseCache:   256,
		DatabaseHandles: 256,
		TxPool:          txpool.DefaultConfig,
		GPO:             ethconfig.Defaults.GPO,
		Ethash:          ethconfig.Defaults.Ethash,
		LightPeers:      10,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	err = stack.Start()
	return stack, lesBackend, lescatalyst.NewConsensusAPI(lesBackend), err
}

func eth2types(typ nodetype) bool {
	if typ == eth2LightClient || typ == eth2NormalNode || typ == eth2MiningNode {
		return true
	}
	return false
}
