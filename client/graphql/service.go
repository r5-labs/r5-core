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
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/r5-labs/r5-core/client/eth/filters"
	"github.com/r5-labs/r5-core/client/internal/ethapi"
	"github.com/r5-labs/r5-core/client/node"
	"github.com/r5-labs/r5-core/client/rpc"
	"github.com/graph-gophers/graphql-go"
	gqlErrors "github.com/graph-gophers/graphql-go/errors"
)

type handler struct {
	Schema *graphql.Schema
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var params struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var (
		ctx       = r.Context()
		responded sync.Once
		timer     *time.Timer
		cancel    context.CancelFunc
	)
	ctx, cancel = context.WithCancel(ctx)
	defer cancel()

	if timeout, ok := rpc.ContextRequestTimeout(ctx); ok {
		timer = time.AfterFunc(timeout, func() {
			responded.Do(func() {
				// Cancel request handling.
				cancel()

				// Create the timeout response.
				response := &graphql.Response{
					Errors: []*gqlErrors.QueryError{{Message: "request timed out"}},
				}
				responseJSON, err := json.Marshal(response)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				// Setting this disables gzip compression in package node.
				w.Header().Set("transfer-encoding", "identity")

				// Flush the response. Since we are writing close to the response timeout,
				// chunked transfer encoding must be disabled by setting content-length.
				w.Header().Set("content-type", "application/json")
				w.Header().Set("content-length", strconv.Itoa(len(responseJSON)))
				w.Write(responseJSON)
				if flush, ok := w.(http.Flusher); ok {
					flush.Flush()
				}
			})
		})
	}

	response := h.Schema.Exec(ctx, params.Query, params.OperationName, params.Variables)
	timer.Stop()
	responded.Do(func() {
		responseJSON, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(response.Errors) > 0 {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(responseJSON)
	})
}

// New constructs a new GraphQL service instance.
func New(stack *node.Node, backend ethapi.Backend, filterSystem *filters.FilterSystem, cors, vhosts []string) error {
	_, err := newHandler(stack, backend, filterSystem, cors, vhosts)
	return err
}

// newHandler returns a new `http.Handler` that will answer GraphQL queries.
// It additionally exports an interactive query browser on the / endpoint.
func newHandler(stack *node.Node, backend ethapi.Backend, filterSystem *filters.FilterSystem, cors, vhosts []string) (*handler, error) {
	q := Resolver{backend, filterSystem}

	s, err := graphql.ParseSchema(schema, &q)
	if err != nil {
		return nil, err
	}
	h := handler{Schema: s}
	handler := node.NewHTTPHandlerStack(h, cors, vhosts, nil)

	stack.RegisterHandler("GraphQL UI", "/graphql/ui", GraphiQL{})
	stack.RegisterHandler("GraphQL", "/graphql", handler)
	stack.RegisterHandler("GraphQL", "/graphql/", handler)

	return &h, nil
}
