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

package rpc

import (
	"context"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"time"
)

func newTestServer() *Server {
	server := NewServer()
	server.idgen = sequentialIDGenerator()
	if err := server.RegisterName("test", new(testService)); err != nil {
		panic(err)
	}
	if err := server.RegisterName("nftest", new(notificationTestService)); err != nil {
		panic(err)
	}
	return server
}

func sequentialIDGenerator() func() ID {
	var (
		mu      sync.Mutex
		counter uint64
	)
	return func() ID {
		mu.Lock()
		defer mu.Unlock()
		counter++
		id := make([]byte, 8)
		binary.BigEndian.PutUint64(id, counter)
		return encodeID(id)
	}
}

type testService struct{}

type echoArgs struct {
	S string
}

type echoResult struct {
	String string
	Int    int
	Args   *echoArgs
}

type testError struct{}

func (testError) Error() string          { return "testError" }
func (testError) ErrorCode() int         { return 444 }
func (testError) ErrorData() interface{} { return "testError data" }

type MarshalErrObj struct{}

func (o *MarshalErrObj) MarshalText() ([]byte, error) {
	return nil, errors.New("marshal error")
}

func (s *testService) NoArgsRets() {}

func (s *testService) Null() any {
	return nil
}

func (s *testService) Echo(str string, i int, args *echoArgs) echoResult {
	return echoResult{str, i, args}
}

func (s *testService) EchoWithCtx(ctx context.Context, str string, i int, args *echoArgs) echoResult {
	return echoResult{str, i, args}
}

func (s *testService) PeerInfo(ctx context.Context) PeerInfo {
	return PeerInfoFromContext(ctx)
}

func (s *testService) Sleep(ctx context.Context, duration time.Duration) {
	time.Sleep(duration)
}

func (s *testService) Block(ctx context.Context) error {
	<-ctx.Done()
	return errors.New("context canceled in testservice_block")
}

func (s *testService) Rets() (string, error) {
	return "", nil
}

//lint:ignore ST1008 returns error first on purpose.
func (s *testService) InvalidRets1() (error, string) {
	return nil, ""
}

func (s *testService) InvalidRets2() (string, string) {
	return "", ""
}

func (s *testService) InvalidRets3() (string, string, error) {
	return "", "", nil
}

func (s *testService) ReturnError() error {
	return testError{}
}

func (s *testService) MarshalError() *MarshalErrObj {
	return &MarshalErrObj{}
}

func (s *testService) Panic() string {
	panic("service panic")
}

func (s *testService) CallMeBack(ctx context.Context, method string, args []interface{}) (interface{}, error) {
	c, ok := ClientFromContext(ctx)
	if !ok {
		return nil, errors.New("no client")
	}
	var result interface{}
	err := c.Call(&result, method, args...)
	return result, err
}

func (s *testService) CallMeBackLater(ctx context.Context, method string, args []interface{}) error {
	c, ok := ClientFromContext(ctx)
	if !ok {
		return errors.New("no client")
	}
	go func() {
		<-ctx.Done()
		var result interface{}
		c.Call(&result, method, args...)
	}()
	return nil
}

func (s *testService) Subscription(ctx context.Context) (*Subscription, error) {
	return nil, nil
}

type notificationTestService struct {
	unsubscribed            chan string
	gotHangSubscriptionReq  chan struct{}
	unblockHangSubscription chan struct{}
}

func (s *notificationTestService) Echo(i int) int {
	return i
}

func (s *notificationTestService) Unsubscribe(subid string) {
	if s.unsubscribed != nil {
		s.unsubscribed <- subid
	}
}

func (s *notificationTestService) SomeSubscription(ctx context.Context, n, val int) (*Subscription, error) {
	notifier, supported := NotifierFromContext(ctx)
	if !supported {
		return nil, ErrNotificationsUnsupported
	}

	// By explicitly creating an subscription we make sure that the subscription id is send
	// back to the client before the first subscription.Notify is called. Otherwise the
	// events might be send before the response for the *_subscribe method.
	subscription := notifier.CreateSubscription()
	go func() {
		for i := 0; i < n; i++ {
			if err := notifier.Notify(subscription.ID, val+i); err != nil {
				return
			}
		}
		select {
		case <-notifier.Closed():
		case <-subscription.Err():
		}
		if s.unsubscribed != nil {
			s.unsubscribed <- string(subscription.ID)
		}
	}()
	return subscription, nil
}

// HangSubscription blocks on s.unblockHangSubscription before sending anything.
func (s *notificationTestService) HangSubscription(ctx context.Context, val int) (*Subscription, error) {
	notifier, supported := NotifierFromContext(ctx)
	if !supported {
		return nil, ErrNotificationsUnsupported
	}
	s.gotHangSubscriptionReq <- struct{}{}
	<-s.unblockHangSubscription
	subscription := notifier.CreateSubscription()

	go func() {
		notifier.Notify(subscription.ID, val)
	}()
	return subscription, nil
}

// largeRespService generates arbitrary-size JSON responses.
type largeRespService struct {
	length int
}

func (x largeRespService) LargeResp() string {
	return strings.Repeat("x", x.length)
}
