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

package node

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/r5-labs/r5-core/client/rpc"
	"github.com/stretchr/testify/assert"
)

// This test uses the admin_startRPC and admin_startWS APIs,
// checking whether the HTTP server is started correctly.
func TestStartRPC(t *testing.T) {
	type test struct {
		name string
		cfg  Config
		fn   func(*testing.T, *Node, *adminAPI)

		// Checks. These run after the node is configured and all API calls have been made.
		wantReachable bool // whether the HTTP server should be reachable at all
		wantHandlers  bool // whether RegisterHandler handlers should be accessible
		wantRPC       bool // whether JSON-RPC/HTTP should be accessible
		wantWS        bool // whether JSON-RPC/WS should be accessible
	}

	tests := []test{
		{
			name: "all off",
			cfg:  Config{},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
			},
			wantReachable: false,
			wantHandlers:  false,
			wantRPC:       false,
			wantWS:        false,
		},
		{
			name: "rpc enabled through config",
			cfg:  Config{HTTPHost: "127.0.0.1"},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
			},
			wantReachable: true,
			wantHandlers:  true,
			wantRPC:       true,
			wantWS:        false,
		},
		{
			name: "rpc enabled through API",
			cfg:  Config{},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				_, err := api.StartHTTP(sp("127.0.0.1"), ip(0), nil, nil, nil)
				assert.NoError(t, err)
			},
			wantReachable: true,
			wantHandlers:  true,
			wantRPC:       true,
			wantWS:        false,
		},
		{
			name: "rpc start again after failure",
			cfg:  Config{},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				// Listen on a random port.
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal("can't listen:", err)
				}
				defer listener.Close()
				port := listener.Addr().(*net.TCPAddr).Port

				// Now try to start RPC on that port. This should fail.
				_, err = api.StartHTTP(sp("127.0.0.1"), ip(port), nil, nil, nil)
				if err == nil {
					t.Fatal("StartHTTP should have failed on port", port)
				}

				// Try again after unblocking the port. It should work this time.
				listener.Close()
				_, err = api.StartHTTP(sp("127.0.0.1"), ip(port), nil, nil, nil)
				assert.NoError(t, err)
			},
			wantReachable: true,
			wantHandlers:  true,
			wantRPC:       true,
			wantWS:        false,
		},
		{
			name: "rpc stopped through API",
			cfg:  Config{HTTPHost: "127.0.0.1"},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				_, err := api.StopHTTP()
				assert.NoError(t, err)
			},
			wantReachable: false,
			wantHandlers:  false,
			wantRPC:       false,
			wantWS:        false,
		},
		{
			name: "rpc stopped twice",
			cfg:  Config{HTTPHost: "127.0.0.1"},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				_, err := api.StopHTTP()
				assert.NoError(t, err)

				_, err = api.StopHTTP()
				assert.NoError(t, err)
			},
			wantReachable: false,
			wantHandlers:  false,
			wantRPC:       false,
			wantWS:        false,
		},
		{
			name:          "ws enabled through config",
			cfg:           Config{WSHost: "127.0.0.1"},
			wantReachable: true,
			wantHandlers:  false,
			wantRPC:       false,
			wantWS:        true,
		},
		{
			name: "ws enabled through API",
			cfg:  Config{},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				_, err := api.StartWS(sp("127.0.0.1"), ip(0), nil, nil)
				assert.NoError(t, err)
			},
			wantReachable: true,
			wantHandlers:  false,
			wantRPC:       false,
			wantWS:        true,
		},
		{
			name: "ws stopped through API",
			cfg:  Config{WSHost: "127.0.0.1"},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				_, err := api.StopWS()
				assert.NoError(t, err)
			},
			wantReachable: false,
			wantHandlers:  false,
			wantRPC:       false,
			wantWS:        false,
		},
		{
			name: "ws stopped twice",
			cfg:  Config{WSHost: "127.0.0.1"},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				_, err := api.StopWS()
				assert.NoError(t, err)

				_, err = api.StopWS()
				assert.NoError(t, err)
			},
			wantReachable: false,
			wantHandlers:  false,
			wantRPC:       false,
			wantWS:        false,
		},
		{
			name: "ws enabled after RPC",
			cfg:  Config{HTTPHost: "127.0.0.1"},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				wsport := n.http.port
				_, err := api.StartWS(sp("127.0.0.1"), ip(wsport), nil, nil)
				assert.NoError(t, err)
			},
			wantReachable: true,
			wantHandlers:  true,
			wantRPC:       true,
			wantWS:        true,
		},
		{
			name: "ws enabled after RPC then stopped",
			cfg:  Config{HTTPHost: "127.0.0.1"},
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				wsport := n.http.port
				_, err := api.StartWS(sp("127.0.0.1"), ip(wsport), nil, nil)
				assert.NoError(t, err)

				_, err = api.StopWS()
				assert.NoError(t, err)
			},
			wantReachable: true,
			wantHandlers:  true,
			wantRPC:       true,
			wantWS:        false,
		},
		{
			name: "rpc stopped with ws enabled",
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				_, err := api.StartHTTP(sp("127.0.0.1"), ip(0), nil, nil, nil)
				assert.NoError(t, err)

				wsport := n.http.port
				_, err = api.StartWS(sp("127.0.0.1"), ip(wsport), nil, nil)
				assert.NoError(t, err)

				_, err = api.StopHTTP()
				assert.NoError(t, err)
			},
			wantReachable: false,
			wantHandlers:  false,
			wantRPC:       false,
			wantWS:        false,
		},
		{
			name: "rpc enabled after ws",
			fn: func(t *testing.T, n *Node, api *adminAPI) {
				_, err := api.StartWS(sp("127.0.0.1"), ip(0), nil, nil)
				assert.NoError(t, err)

				wsport := n.http.port
				_, err = api.StartHTTP(sp("127.0.0.1"), ip(wsport), nil, nil, nil)
				assert.NoError(t, err)
			},
			wantReachable: true,
			wantHandlers:  true,
			wantRPC:       true,
			wantWS:        true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// Apply some sane defaults.
			config := test.cfg
			// config.Logger = testlog.Logger(t, log.LvlDebug)
			config.P2P.NoDiscovery = true
			if config.HTTPTimeouts == (rpc.HTTPTimeouts{}) {
				config.HTTPTimeouts = rpc.DefaultHTTPTimeouts
			}

			// Create Node.
			stack, err := New(&config)
			if err != nil {
				t.Fatal("can't create node:", err)
			}
			defer stack.Close()

			// Register the test handler.
			stack.RegisterHandler("test", "/test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("OK"))
			}))

			if err := stack.Start(); err != nil {
				t.Fatal("can't start node:", err)
			}

			// Run the API call hook.
			if test.fn != nil {
				test.fn(t, stack, &adminAPI{stack})
			}

			// Check if the HTTP endpoints are available.
			baseURL := stack.HTTPEndpoint()
			reachable := checkReachable(baseURL)
			handlersAvailable := checkBodyOK(baseURL + "/test")
			rpcAvailable := checkRPC(baseURL)
			wsAvailable := checkRPC(strings.Replace(baseURL, "http://", "ws://", 1))
			if reachable != test.wantReachable {
				t.Errorf("HTTP server is %sreachable, want it %sreachable", not(reachable), not(test.wantReachable))
			}
			if handlersAvailable != test.wantHandlers {
				t.Errorf("RegisterHandler handlers %savailable, want them %savailable", not(handlersAvailable), not(test.wantHandlers))
			}
			if rpcAvailable != test.wantRPC {
				t.Errorf("HTTP RPC %savailable, want it %savailable", not(rpcAvailable), not(test.wantRPC))
			}
			if wsAvailable != test.wantWS {
				t.Errorf("WS RPC %savailable, want it %savailable", not(wsAvailable), not(test.wantWS))
			}
		})
	}
}

// checkReachable checks if the TCP endpoint in rawurl is open.
func checkReachable(rawurl string) bool {
	u, err := url.Parse(rawurl)
	if err != nil {
		panic(err)
	}
	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// checkBodyOK checks whether the given HTTP URL responds with 200 OK and body "OK".
func checkBodyOK(url string) bool {
	resp, err := http.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false
	}
	buf := make([]byte, 2)
	if _, err = io.ReadFull(resp.Body, buf); err != nil {
		return false
	}
	return bytes.Equal(buf, []byte("OK"))
}

// checkRPC checks whether JSON-RPC works against the given URL.
func checkRPC(url string) bool {
	c, err := rpc.Dial(url)
	if err != nil {
		return false
	}
	defer c.Close()

	_, err = c.SupportedModules()
	return err == nil
}

// string/int pointer helpers.
func sp(s string) *string { return &s }
func ip(i int) *int       { return &i }

func not(ok bool) string {
	if ok {
		return ""
	}
	return "not "
}
