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

package client

import (
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/r5-labs/r5-core/client/common/mclock"
	"github.com/r5-labs/r5-core/client/ethdb"
	"github.com/r5-labs/r5-core/client/ethdb/memorydb"
	"github.com/r5-labs/r5-core/client/p2p/enode"
	"github.com/r5-labs/r5-core/client/p2p/enr"
)

const (
	spTestNodes  = 1000
	spTestTarget = 5
	spTestLength = 10000
	spMinTotal   = 40000
	spMaxTotal   = 50000
)

func testNodeID(i int) enode.ID {
	return enode.ID{42, byte(i % 256), byte(i / 256)}
}

func testNodeIndex(id enode.ID) int {
	if id[0] != 42 {
		return -1
	}
	return int(id[1]) + int(id[2])*256
}

type ServerPoolTest struct {
	db                   ethdb.KeyValueStore
	clock                *mclock.Simulated
	quit                 chan chan struct{}
	preNeg, preNegFail   bool
	sp                   *ServerPool
	spi                  enode.Iterator
	input                enode.Iterator
	testNodes            []spTestNode
	trusted              []string
	waitCount, waitEnded int32

	// preNegLock protects the cycle counter, testNodes list and its connected field
	// (accessed from both the main thread and the preNeg callback)
	preNegLock sync.Mutex
	queryWg    *sync.WaitGroup // a new wait group is created each time the simulation is started
	stopping   bool            // stopping avoid calling queryWg.Add after queryWg.Wait

	cycle, conn, servedConn  int
	serviceCycles, dialCount int
	disconnect               map[int][]int
}

type spTestNode struct {
	connectCycles, waitCycles int
	nextConnCycle, totalConn  int
	connected, service        bool
	node                      *enode.Node
}

func newServerPoolTest(preNeg, preNegFail bool) *ServerPoolTest {
	nodes := make([]*enode.Node, spTestNodes)
	for i := range nodes {
		nodes[i] = enode.SignNull(&enr.Record{}, testNodeID(i))
	}
	return &ServerPoolTest{
		clock:      &mclock.Simulated{},
		db:         memorydb.New(),
		input:      enode.CycleNodes(nodes),
		testNodes:  make([]spTestNode, spTestNodes),
		preNeg:     preNeg,
		preNegFail: preNegFail,
	}
}

func (s *ServerPoolTest) beginWait() {
	// ensure that dialIterator and the maximal number of pre-neg queries are not all stuck in a waiting state
	for atomic.AddInt32(&s.waitCount, 1) > preNegLimit {
		atomic.AddInt32(&s.waitCount, -1)
		s.clock.Run(time.Second)
	}
}

func (s *ServerPoolTest) endWait() {
	atomic.AddInt32(&s.waitCount, -1)
	atomic.AddInt32(&s.waitEnded, 1)
}

func (s *ServerPoolTest) addTrusted(i int) {
	s.trusted = append(s.trusted, enode.SignNull(&enr.Record{}, testNodeID(i)).String())
}

func (s *ServerPoolTest) start() {
	var testQuery QueryFunc
	s.queryWg = new(sync.WaitGroup)
	if s.preNeg {
		testQuery = func(node *enode.Node) int {
			s.preNegLock.Lock()
			if s.stopping {
				s.preNegLock.Unlock()
				return 0
			}
			s.queryWg.Add(1)
			idx := testNodeIndex(node.ID())
			n := &s.testNodes[idx]
			canConnect := !n.connected && n.connectCycles != 0 && s.cycle >= n.nextConnCycle
			s.preNegLock.Unlock()
			defer s.queryWg.Done()

			if s.preNegFail {
				// simulate a scenario where UDP queries never work
				s.beginWait()
				s.clock.Sleep(time.Second * 5)
				s.endWait()
				return -1
			}
			switch idx % 3 {
			case 0:
				// pre-neg returns true only if connection is possible
				if canConnect {
					return 1
				}
				return 0
			case 1:
				// pre-neg returns true but connection might still fail
				return 1
			case 2:
				// pre-neg returns true if connection is possible, otherwise timeout (node unresponsive)
				if canConnect {
					return 1
				}
				s.beginWait()
				s.clock.Sleep(time.Second * 5)
				s.endWait()
				return -1
			}
			return -1
		}
	}

	requestList := make([]RequestInfo, testReqTypes)
	for i := range requestList {
		requestList[i] = RequestInfo{Name: "testreq" + strconv.Itoa(i), InitAmount: 1, InitValue: 1}
	}

	s.sp, s.spi = NewServerPool(s.db, []byte("sp:"), 0, testQuery, s.clock, s.trusted, requestList)
	s.sp.AddSource(s.input)
	s.sp.validSchemes = enode.ValidSchemesForTesting
	s.sp.unixTime = func() int64 { return int64(s.clock.Now()) / int64(time.Second) }
	s.disconnect = make(map[int][]int)
	s.sp.Start()
	s.quit = make(chan chan struct{})
	go func() {
		last := int32(-1)
		for {
			select {
			case <-time.After(time.Millisecond * 100):
				c := atomic.LoadInt32(&s.waitEnded)
				if c == last {
					// advance clock if test is stuck (might happen in rare cases)
					s.clock.Run(time.Second)
				}
				last = c
			case quit := <-s.quit:
				close(quit)
				return
			}
		}
	}()
}

func (s *ServerPoolTest) stop() {
	// disable further queries and wait if one is currently running
	s.preNegLock.Lock()
	s.stopping = true
	s.preNegLock.Unlock()
	s.queryWg.Wait()

	quit := make(chan struct{})
	s.quit <- quit
	<-quit
	s.sp.Stop()
	s.spi.Close()
	s.preNegLock.Lock()
	s.stopping = false
	s.preNegLock.Unlock()
	for i := range s.testNodes {
		n := &s.testNodes[i]
		if n.connected {
			n.totalConn += s.cycle
		}
		n.connected = false
		n.node = nil
		n.nextConnCycle = 0
	}
	s.conn, s.servedConn = 0, 0
}

func (s *ServerPoolTest) run() {
	for count := spTestLength; count > 0; count-- {
		if dcList := s.disconnect[s.cycle]; dcList != nil {
			for _, idx := range dcList {
				n := &s.testNodes[idx]
				s.sp.UnregisterNode(n.node)
				n.totalConn += s.cycle
				s.preNegLock.Lock()
				n.connected = false
				s.preNegLock.Unlock()
				n.node = nil
				s.conn--
				if n.service {
					s.servedConn--
				}
				n.nextConnCycle = s.cycle + n.waitCycles
			}
			delete(s.disconnect, s.cycle)
		}
		if s.conn < spTestTarget {
			s.dialCount++
			s.beginWait()
			s.spi.Next()
			s.endWait()
			dial := s.spi.Node()
			id := dial.ID()
			idx := testNodeIndex(id)
			n := &s.testNodes[idx]
			if !n.connected && n.connectCycles != 0 && s.cycle >= n.nextConnCycle {
				s.conn++
				if n.service {
					s.servedConn++
				}
				n.totalConn -= s.cycle
				s.preNegLock.Lock()
				n.connected = true
				s.preNegLock.Unlock()
				dc := s.cycle + n.connectCycles
				s.disconnect[dc] = append(s.disconnect[dc], idx)
				n.node = dial
				nv, _ := s.sp.RegisterNode(n.node)
				if n.service {
					nv.Served([]ServedRequest{{ReqType: 0, Amount: 100}}, 0)
				}
			}
		}
		s.serviceCycles += s.servedConn
		s.clock.Run(time.Second)
		s.preNegLock.Lock()
		s.cycle++
		s.preNegLock.Unlock()
	}
}

func (s *ServerPoolTest) setNodes(count, conn, wait int, service, trusted bool) (res []int) {
	for ; count > 0; count-- {
		idx := rand.Intn(spTestNodes)
		for s.testNodes[idx].connectCycles != 0 || s.testNodes[idx].connected {
			idx = rand.Intn(spTestNodes)
		}
		res = append(res, idx)
		s.preNegLock.Lock()
		s.testNodes[idx] = spTestNode{
			connectCycles: conn,
			waitCycles:    wait,
			service:       service,
		}
		s.preNegLock.Unlock()
		if trusted {
			s.addTrusted(idx)
		}
	}
	return
}

func (s *ServerPoolTest) resetNodes() {
	for i, n := range s.testNodes {
		if n.connected {
			n.totalConn += s.cycle
			s.sp.UnregisterNode(n.node)
		}
		s.preNegLock.Lock()
		s.testNodes[i] = spTestNode{totalConn: n.totalConn}
		s.preNegLock.Unlock()
	}
	s.conn, s.servedConn = 0, 0
	s.disconnect = make(map[int][]int)
	s.trusted = nil
}

func (s *ServerPoolTest) checkNodes(t *testing.T, nodes []int) {
	var sum int
	for _, idx := range nodes {
		n := &s.testNodes[idx]
		if n.connected {
			n.totalConn += s.cycle
		}
		sum += n.totalConn
		n.totalConn = 0
		if n.connected {
			n.totalConn -= s.cycle
		}
	}
	if sum < spMinTotal || sum > spMaxTotal {
		t.Errorf("Total connection amount %d outside expected range %d to %d", sum, spMinTotal, spMaxTotal)
	}
}

func TestServerPool(t *testing.T)               { testServerPool(t, false, false) }
func TestServerPoolWithPreNeg(t *testing.T)     { testServerPool(t, true, false) }
func TestServerPoolWithPreNegFail(t *testing.T) { testServerPool(t, true, true) }
func testServerPool(t *testing.T, preNeg, fail bool) {
	s := newServerPoolTest(preNeg, fail)
	nodes := s.setNodes(100, 200, 200, true, false)
	s.setNodes(100, 20, 20, false, false)
	s.start()
	s.run()
	s.stop()
	s.checkNodes(t, nodes)
}

func TestServerPoolChangedNodes(t *testing.T)           { testServerPoolChangedNodes(t, false) }
func TestServerPoolChangedNodesWithPreNeg(t *testing.T) { testServerPoolChangedNodes(t, true) }
func testServerPoolChangedNodes(t *testing.T, preNeg bool) {
	s := newServerPoolTest(preNeg, false)
	nodes := s.setNodes(100, 200, 200, true, false)
	s.setNodes(100, 20, 20, false, false)
	s.start()
	s.run()
	s.checkNodes(t, nodes)
	for i := 0; i < 3; i++ {
		s.resetNodes()
		nodes := s.setNodes(100, 200, 200, true, false)
		s.setNodes(100, 20, 20, false, false)
		s.run()
		s.checkNodes(t, nodes)
	}
	s.stop()
}

func TestServerPoolRestartNoDiscovery(t *testing.T) { testServerPoolRestartNoDiscovery(t, false) }
func TestServerPoolRestartNoDiscoveryWithPreNeg(t *testing.T) {
	testServerPoolRestartNoDiscovery(t, true)
}
func testServerPoolRestartNoDiscovery(t *testing.T, preNeg bool) {
	s := newServerPoolTest(preNeg, false)
	nodes := s.setNodes(100, 200, 200, true, false)
	s.setNodes(100, 20, 20, false, false)
	s.start()
	s.run()
	s.stop()
	s.checkNodes(t, nodes)
	s.input = nil
	s.start()
	s.run()
	s.stop()
	s.checkNodes(t, nodes)
}

func TestServerPoolTrustedNoDiscovery(t *testing.T) { testServerPoolTrustedNoDiscovery(t, false) }
func TestServerPoolTrustedNoDiscoveryWithPreNeg(t *testing.T) {
	testServerPoolTrustedNoDiscovery(t, true)
}
func testServerPoolTrustedNoDiscovery(t *testing.T, preNeg bool) {
	s := newServerPoolTest(preNeg, false)
	trusted := s.setNodes(200, 200, 200, true, true)
	s.input = nil
	s.start()
	s.run()
	s.stop()
	s.checkNodes(t, trusted)
}
