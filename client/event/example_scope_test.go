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

package event_test

import (
	"fmt"
	"sync"

	"github.com/r5-labs/r5-core/client/event"
)

// This example demonstrates how SubscriptionScope can be used to control the lifetime of
// subscriptions.
//
// Our example program consists of two servers, each of which performs a calculation when
// requested. The servers also allow subscribing to results of all computations.
type divServer struct{ results event.Feed }
type mulServer struct{ results event.Feed }

func (s *divServer) do(a, b int) int {
	r := a / b
	s.results.Send(r)
	return r
}

func (s *mulServer) do(a, b int) int {
	r := a * b
	s.results.Send(r)
	return r
}

// The servers are contained in an App. The app controls the servers and exposes them
// through its API.
type App struct {
	divServer
	mulServer
	scope event.SubscriptionScope
}

func (s *App) Calc(op byte, a, b int) int {
	switch op {
	case '/':
		return s.divServer.do(a, b)
	case '*':
		return s.mulServer.do(a, b)
	default:
		panic("invalid op")
	}
}

// The app's SubscribeResults method starts sending calculation results to the given
// channel. Subscriptions created through this method are tied to the lifetime of the App
// because they are registered in the scope.
func (s *App) SubscribeResults(op byte, ch chan<- int) event.Subscription {
	switch op {
	case '/':
		return s.scope.Track(s.divServer.results.Subscribe(ch))
	case '*':
		return s.scope.Track(s.mulServer.results.Subscribe(ch))
	default:
		panic("invalid op")
	}
}

// Stop stops the App, closing all subscriptions created through SubscribeResults.
func (s *App) Stop() {
	s.scope.Close()
}

func ExampleSubscriptionScope() {
	// Create the app.
	var (
		app  App
		wg   sync.WaitGroup
		divs = make(chan int)
		muls = make(chan int)
	)

	// Run a subscriber in the background.
	divsub := app.SubscribeResults('/', divs)
	mulsub := app.SubscribeResults('*', muls)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer fmt.Println("subscriber exited")
		defer divsub.Unsubscribe()
		defer mulsub.Unsubscribe()
		for {
			select {
			case result := <-divs:
				fmt.Println("division happened:", result)
			case result := <-muls:
				fmt.Println("multiplication happened:", result)
			case <-divsub.Err():
				return
			case <-mulsub.Err():
				return
			}
		}
	}()

	// Interact with the app.
	app.Calc('/', 22, 11)
	app.Calc('*', 3, 4)

	// Stop the app. This shuts down the subscriptions, causing the subscriber to exit.
	app.Stop()
	wg.Wait()

	// Output:
	// division happened: 2
	// multiplication happened: 12
	// subscriber exited
}
