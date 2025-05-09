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

package les

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/r5-labs/r5-core/client/common/mclock"
)

type testDistReq struct {
	cost, procTime, order uint64
	canSendTo             map[*testDistPeer]struct{}
}

func (r *testDistReq) getCost(dp distPeer) uint64 {
	return r.cost
}

func (r *testDistReq) canSend(dp distPeer) bool {
	_, ok := r.canSendTo[dp.(*testDistPeer)]
	return ok
}

func (r *testDistReq) request(dp distPeer) func() {
	return func() { dp.(*testDistPeer).send(r) }
}

type testDistPeer struct {
	sent    []*testDistReq
	sumCost uint64
	lock    sync.RWMutex
}

func (p *testDistPeer) send(r *testDistReq) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.sent = append(p.sent, r)
	p.sumCost += r.cost
}

func (p *testDistPeer) worker(t *testing.T, checkOrder bool, stop chan struct{}) {
	var last uint64
	for {
		wait := time.Millisecond
		p.lock.Lock()
		if len(p.sent) > 0 {
			rq := p.sent[0]
			wait = time.Duration(rq.procTime)
			p.sumCost -= rq.cost
			if checkOrder {
				if rq.order <= last {
					t.Errorf("Requests processed in wrong order")
				}
				last = rq.order
			}
			p.sent = p.sent[1:]
		}
		p.lock.Unlock()
		select {
		case <-stop:
			return
		case <-time.After(wait):
		}
	}
}

const (
	testDistBufLimit       = 10000000
	testDistMaxCost        = 1000000
	testDistPeerCount      = 2
	testDistReqCount       = 10
	testDistMaxResendCount = 3
)

func (p *testDistPeer) waitBefore(cost uint64) (time.Duration, float64) {
	p.lock.RLock()
	sumCost := p.sumCost + cost
	p.lock.RUnlock()
	if sumCost < testDistBufLimit {
		return 0, float64(testDistBufLimit-sumCost) / float64(testDistBufLimit)
	}
	return time.Duration(sumCost - testDistBufLimit), 0
}

func (p *testDistPeer) canQueue() bool {
	return true
}

func (p *testDistPeer) queueSend(f func()) bool {
	f()
	return true
}

func TestRequestDistributor(t *testing.T) {
	testRequestDistributor(t, false)
}

func TestRequestDistributorResend(t *testing.T) {
	testRequestDistributor(t, true)
}

func testRequestDistributor(t *testing.T, resend bool) {
	stop := make(chan struct{})
	defer close(stop)

	dist := newRequestDistributor(nil, &mclock.System{})
	var peers [testDistPeerCount]*testDistPeer
	for i := range peers {
		peers[i] = &testDistPeer{}
		go peers[i].worker(t, !resend, stop)
		dist.registerTestPeer(peers[i])
	}
	// Disable the mechanism that we will wait a few time for request
	// even there is no suitable peer to send right now.
	waitForPeers = 0

	var wg sync.WaitGroup

	for i := 1; i <= testDistReqCount; i++ {
		cost := uint64(rand.Int63n(testDistMaxCost))
		procTime := uint64(rand.Int63n(int64(cost + 1)))
		rq := &testDistReq{
			cost:      cost,
			procTime:  procTime,
			order:     uint64(i),
			canSendTo: make(map[*testDistPeer]struct{}),
		}
		for _, peer := range peers {
			if rand.Intn(2) != 0 {
				rq.canSendTo[peer] = struct{}{}
			}
		}

		wg.Add(1)
		req := &distReq{
			getCost: rq.getCost,
			canSend: rq.canSend,
			request: rq.request,
		}
		chn := dist.queue(req)
		go func() {
			cnt := 1
			if resend && len(rq.canSendTo) != 0 {
				cnt = rand.Intn(testDistMaxResendCount) + 1
			}
			for i := 0; i < cnt; i++ {
				if i != 0 {
					chn = dist.queue(req)
				}
				p := <-chn
				if p == nil {
					if len(rq.canSendTo) != 0 {
						t.Errorf("Request that could have been sent was dropped")
					}
				} else {
					peer := p.(*testDistPeer)
					if _, ok := rq.canSendTo[peer]; !ok {
						t.Errorf("Request sent to wrong peer")
					}
				}
			}
			wg.Done()
		}()
		if rand.Intn(1000) == 0 {
			time.Sleep(time.Duration(rand.Intn(5000000)))
		}
	}

	wg.Wait()
}
