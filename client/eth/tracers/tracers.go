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

// Package tracers is a manager for transaction tracing engines.
package tracers

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/vm"
)

// Context contains some contextual infos for a transaction execution that is not
// available from within the EVM object.
type Context struct {
	BlockHash   common.Hash // Hash of the block the tx is contained within (zero if dangling tx or call)
	BlockNumber *big.Int    // Number of the block the tx is contained within (zero if dangling tx or call)
	TxIndex     int         // Index of the transaction within a block (zero if dangling tx or call)
	TxHash      common.Hash // Hash of the transaction being traced (zero if dangling call)
}

// Tracer interface extends vm.EVMLogger and additionally
// allows collecting the tracing result.
type Tracer interface {
	vm.EVMLogger
	GetResult() (json.RawMessage, error)
	// Stop terminates execution of the tracer at the first opportune moment.
	Stop(err error)
}

type ctorFn func(*Context, json.RawMessage) (Tracer, error)
type jsCtorFn func(string, *Context, json.RawMessage) (Tracer, error)

type elem struct {
	ctor ctorFn
	isJS bool
}

// DefaultDirectory is the collection of tracers bundled by default.
var DefaultDirectory = directory{elems: make(map[string]elem)}

// directory provides functionality to lookup a tracer by name
// and a function to instantiate it. It falls back to a JS code evaluator
// if no tracer of the given name exists.
type directory struct {
	elems  map[string]elem
	jsEval jsCtorFn
}

// Register registers a method as a lookup for tracers, meaning that
// users can invoke a named tracer through that lookup.
func (d *directory) Register(name string, f ctorFn, isJS bool) {
	d.elems[name] = elem{ctor: f, isJS: isJS}
}

// RegisterJSEval registers a tracer that is able to parse
// dynamic user-provided JS code.
func (d *directory) RegisterJSEval(f jsCtorFn) {
	d.jsEval = f
}

// New returns a new instance of a tracer, by iterating through the
// registered lookups. Name is either name of an existing tracer
// or an arbitrary JS code.
func (d *directory) New(name string, ctx *Context, cfg json.RawMessage) (Tracer, error) {
	if elem, ok := d.elems[name]; ok {
		return elem.ctor(ctx, cfg)
	}
	// Assume JS code
	return d.jsEval(name, ctx, cfg)
}

// IsJS will return true if the given tracer will evaluate
// JS code. Because code evaluation has high overhead, this
// info will be used in determining fast and slow code paths.
func (d *directory) IsJS(name string) bool {
	if elem, ok := d.elems[name]; ok {
		return elem.isJS
	}
	// JS eval will execute JS code
	return true
}

const (
	memoryPadLimit = 1024 * 1024
)

// GetMemoryCopyPadded returns offset + size as a new slice.
// It zero-pads the slice if it extends beyond memory bounds.
func GetMemoryCopyPadded(m *vm.Memory, offset, size int64) ([]byte, error) {
	if offset < 0 || size < 0 {
		return nil, fmt.Errorf("offset or size must not be negative")
	}
	if int(offset+size) < m.Len() { // slice fully inside memory
		return m.GetCopy(offset, size), nil
	}
	paddingNeeded := int(offset+size) - m.Len()
	if paddingNeeded > memoryPadLimit {
		return nil, fmt.Errorf("reached limit for padding memory slice: %d", paddingNeeded)
	}
	cpy := make([]byte, size)
	if overlap := int64(m.Len()) - offset; overlap > 0 {
		copy(cpy, m.GetPtr(offset, overlap))
	}
	return cpy, nil
}
