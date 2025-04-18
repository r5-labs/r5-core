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

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/r5-labs/r5-core/client/p2p"
	"github.com/r5-labs/r5-core/client/rpc"
)

type gethrpc struct {
	name     string
	rpc      *rpc.Client
	geth     *testgeth
	nodeInfo *p2p.NodeInfo
}

func (g *gethrpc) killAndWait() {
	g.geth.Kill()
	g.geth.WaitExit()
}

func (g *gethrpc) callRPC(result interface{}, method string, args ...interface{}) {
	if err := g.rpc.Call(&result, method, args...); err != nil {
		g.geth.Fatalf("callRPC %v: %v", method, err)
	}
}

func (g *gethrpc) addPeer(peer *gethrpc) {
	g.geth.Logf("%v.addPeer(%v)", g.name, peer.name)
	enode := peer.getNodeInfo().Enode
	peerCh := make(chan *p2p.PeerEvent)
	sub, err := g.rpc.Subscribe(context.Background(), "admin", peerCh, "peerEvents")
	if err != nil {
		g.geth.Fatalf("subscribe %v: %v", g.name, err)
	}
	defer sub.Unsubscribe()
	g.callRPC(nil, "admin_addPeer", enode)
	dur := 14 * time.Second
	timeout := time.After(dur)
	select {
	case ev := <-peerCh:
		g.geth.Logf("%v received event: type=%v, peer=%v", g.name, ev.Type, ev.Peer)
	case err := <-sub.Err():
		g.geth.Fatalf("%v sub error: %v", g.name, err)
	case <-timeout:
		g.geth.Error("timeout adding peer after", dur)
	}
}

// Use this function instead of `g.nodeInfo` directly
func (g *gethrpc) getNodeInfo() *p2p.NodeInfo {
	if g.nodeInfo != nil {
		return g.nodeInfo
	}
	g.nodeInfo = &p2p.NodeInfo{}
	g.callRPC(&g.nodeInfo, "admin_nodeInfo")
	return g.nodeInfo
}

// ipcEndpoint resolves an IPC endpoint based on a configured value, taking into
// account the set data folders as well as the designated platform we're currently
// running on.
func ipcEndpoint(ipcPath, datadir string) string {
	// On windows we can only use plain top-level pipes
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(ipcPath, `\\.\pipe\`) {
			return ipcPath
		}
		return `\\.\pipe\` + ipcPath
	}
	// Resolve names into the data directory full paths otherwise
	if filepath.Base(ipcPath) == ipcPath {
		if datadir == "" {
			return filepath.Join(os.TempDir(), ipcPath)
		}
		return filepath.Join(datadir, ipcPath)
	}
	return ipcPath
}

// nextIPC ensures that each ipc pipe gets a unique name.
// On linux, it works well to use ipc pipes all over the filesystem (in datadirs),
// but windows require pipes to sit in "\\.\pipe\". Therefore, to run several
// nodes simultaneously, we need to distinguish between them, which we do by
// the pipe filename instead of folder.
var nextIPC = uint32(0)

func startGethWithIpc(t *testing.T, name string, args ...string) *gethrpc {
	ipcName := fmt.Sprintf("geth-%d.ipc", atomic.AddUint32(&nextIPC, 1))
	args = append([]string{"--networkid=42", "--port=0", "--authrpc.port", "0", "--ipcpath", ipcName}, args...)
	t.Logf("Starting %v with rpc: %v", name, args)

	g := &gethrpc{
		name: name,
		geth: runGeth(t, args...),
	}
	ipcpath := ipcEndpoint(ipcName, g.geth.Datadir)
	// We can't know exactly how long geth will take to start, so we try 10
	// times over a 5 second period.
	var err error
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if g.rpc, err = rpc.Dial(ipcpath); err == nil {
			return g
		}
	}
	t.Fatalf("%v rpc connect to %v: %v", name, ipcpath, err)
	return nil
}

func initGeth(t *testing.T) string {
	args := []string{"--networkid=42", "init", "./testdata/clique.json"}
	t.Logf("Initializing geth: %v ", args)
	g := runGeth(t, args...)
	datadir := g.Datadir
	g.WaitExit()
	return datadir
}

func startLightServer(t *testing.T) *gethrpc {
	datadir := initGeth(t)
	t.Logf("Importing keys to geth")
	runGeth(t, "account", "import", "--datadir", datadir, "--password", "./testdata/password.txt", "--lightkdf", "./testdata/key.prv").WaitExit()
	account := "0x02f0d131f1f97aef08aec6e3291b957d9efe7105"
	server := startGethWithIpc(t, "lightserver", "--allow-insecure-unlock", "--datadir", datadir, "--password", "./testdata/password.txt", "--unlock", account, "--miner.etherbase=0x02f0d131f1f97aef08aec6e3291b957d9efe7105", "--mine", "--light.serve=100", "--light.maxpeers=1", "--nodiscover", "--nat=extip:127.0.0.1", "--verbosity=4")
	return server
}

func startClient(t *testing.T, name string) *gethrpc {
	datadir := initGeth(t)
	return startGethWithIpc(t, name, "--datadir", datadir, "--nodiscover", "--syncmode=light", "--nat=extip:127.0.0.1", "--verbosity=4")
}

func TestPriorityClient(t *testing.T) {
	lightServer := startLightServer(t)
	defer lightServer.killAndWait()

	// Start client and add lightServer as peer
	freeCli := startClient(t, "freeCli")
	defer freeCli.killAndWait()
	freeCli.addPeer(lightServer)

	var peers []*p2p.PeerInfo
	freeCli.callRPC(&peers, "admin_peers")
	if len(peers) != 1 {
		t.Errorf("Expected: # of client peers == 1, actual: %v", len(peers))
		return
	}

	// Set up priority client, get its nodeID, increase its balance on the lightServer
	prioCli := startClient(t, "prioCli")
	defer prioCli.killAndWait()
	// 3_000_000_000 once we move to Go 1.13
	tokens := uint64(3000000000)
	lightServer.callRPC(nil, "les_addBalance", prioCli.getNodeInfo().ID, tokens)
	prioCli.addPeer(lightServer)

	// Check if priority client is actually syncing and the regular client got kicked out
	prioCli.callRPC(&peers, "admin_peers")
	if len(peers) != 1 {
		t.Errorf("Expected: # of prio peers == 1, actual: %v", len(peers))
	}

	nodes := map[string]*gethrpc{
		lightServer.getNodeInfo().ID: lightServer,
		freeCli.getNodeInfo().ID:     freeCli,
		prioCli.getNodeInfo().ID:     prioCli,
	}
	time.Sleep(1 * time.Second)
	lightServer.callRPC(&peers, "admin_peers")
	peersWithNames := make(map[string]string)
	for _, p := range peers {
		peersWithNames[nodes[p.ID].name] = p.ID
	}
	if _, freeClientFound := peersWithNames[freeCli.name]; freeClientFound {
		t.Error("client is still a peer of lightServer", peersWithNames)
	}
	if _, prioClientFound := peersWithNames[prioCli.name]; !prioClientFound {
		t.Error("prio client is not among lightServer peers", peersWithNames)
	}
}
