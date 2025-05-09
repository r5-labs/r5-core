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

package downloader

import (
	"context"
	"sync"

	"github.com/r5-labs/r5-core/client"
	"github.com/r5-labs/r5-core/client/event"
	"github.com/r5-labs/r5-core/client/rpc"
)

// DownloaderAPI provides an API which gives information about the current synchronisation status.
// It offers only methods that operates on data that can be available to anyone without security risks.
type DownloaderAPI struct {
	d                         *Downloader
	mux                       *event.TypeMux
	installSyncSubscription   chan chan interface{}
	uninstallSyncSubscription chan *uninstallSyncSubscriptionRequest
}

// NewDownloaderAPI create a new DownloaderAPI. The API has an internal event loop that
// listens for events from the downloader through the global event mux. In case it receives one of
// these events it broadcasts it to all syncing subscriptions that are installed through the
// installSyncSubscription channel.
func NewDownloaderAPI(d *Downloader, m *event.TypeMux) *DownloaderAPI {
	api := &DownloaderAPI{
		d:                         d,
		mux:                       m,
		installSyncSubscription:   make(chan chan interface{}),
		uninstallSyncSubscription: make(chan *uninstallSyncSubscriptionRequest),
	}

	go api.eventLoop()

	return api
}

// eventLoop runs a loop until the event mux closes. It will install and uninstall new
// sync subscriptions and broadcasts sync status updates to the installed sync subscriptions.
func (api *DownloaderAPI) eventLoop() {
	var (
		sub               = api.mux.Subscribe(StartEvent{}, DoneEvent{}, FailedEvent{})
		syncSubscriptions = make(map[chan interface{}]struct{})
	)

	for {
		select {
		case i := <-api.installSyncSubscription:
			syncSubscriptions[i] = struct{}{}
		case u := <-api.uninstallSyncSubscription:
			delete(syncSubscriptions, u.c)
			close(u.uninstalled)
		case event := <-sub.Chan():
			if event == nil {
				return
			}

			var notification interface{}
			switch event.Data.(type) {
			case StartEvent:
				notification = &SyncingResult{
					Syncing: true,
					Status:  api.d.Progress(),
				}
			case DoneEvent, FailedEvent:
				notification = false
			}
			// broadcast
			for c := range syncSubscriptions {
				c <- notification
			}
		}
	}
}

// Syncing provides information when this nodes starts synchronising with the Ethereum network and when it's finished.
func (api *DownloaderAPI) Syncing(ctx context.Context) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		statuses := make(chan interface{})
		sub := api.SubscribeSyncStatus(statuses)

		for {
			select {
			case status := <-statuses:
				notifier.Notify(rpcSub.ID, status)
			case <-rpcSub.Err():
				sub.Unsubscribe()
				return
			case <-notifier.Closed():
				sub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// SyncingResult provides information about the current synchronisation status for this node.
type SyncingResult struct {
	Syncing bool                  `json:"syncing"`
	Status  ethereum.SyncProgress `json:"status"`
}

// uninstallSyncSubscriptionRequest uninstalls a syncing subscription in the API event loop.
type uninstallSyncSubscriptionRequest struct {
	c           chan interface{}
	uninstalled chan interface{}
}

// SyncStatusSubscription represents a syncing subscription.
type SyncStatusSubscription struct {
	api       *DownloaderAPI   // register subscription in event loop of this api instance
	c         chan interface{} // channel where events are broadcasted to
	unsubOnce sync.Once        // make sure unsubscribe logic is executed once
}

// Unsubscribe uninstalls the subscription from the DownloadAPI event loop.
// The status channel that was passed to subscribeSyncStatus isn't used anymore
// after this method returns.
func (s *SyncStatusSubscription) Unsubscribe() {
	s.unsubOnce.Do(func() {
		req := uninstallSyncSubscriptionRequest{s.c, make(chan interface{})}
		s.api.uninstallSyncSubscription <- &req

		for {
			select {
			case <-s.c:
				// drop new status events until uninstall confirmation
				continue
			case <-req.uninstalled:
				return
			}
		}
	})
}

// SubscribeSyncStatus creates a subscription that will broadcast new synchronisation updates.
// The given channel must receive interface values, the result can either.
func (api *DownloaderAPI) SubscribeSyncStatus(status chan interface{}) *SyncStatusSubscription {
	api.installSyncSubscription <- status
	return &SyncStatusSubscription{api: api, c: status}
}
