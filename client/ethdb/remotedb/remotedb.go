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

// Package remotedb implements the key-value database layer based on a remote geth
// node. Under the hood, it utilises the `debug_dbGet` method to implement a
// read-only database.
// There really are no guarantees in this database, since the local geth does not
// exclusive access, but it can be used for basic diagnostics of a remote node.
package remotedb

import (
	"github.com/r5-labs/r5-core/client/common/hexutil"
	"github.com/r5-labs/r5-core/client/ethdb"
	"github.com/r5-labs/r5-core/client/rpc"
)

// Database is a key-value lookup for a remote database via debug_dbGet.
type Database struct {
	remote *rpc.Client
}

func (db *Database) Has(key []byte) (bool, error) {
	if _, err := db.Get(key); err != nil {
		return false, nil
	}
	return true, nil
}

func (db *Database) Get(key []byte) ([]byte, error) {
	var resp hexutil.Bytes
	err := db.remote.Call(&resp, "debug_dbGet", hexutil.Bytes(key))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (db *Database) HasAncient(kind string, number uint64) (bool, error) {
	if _, err := db.Ancient(kind, number); err != nil {
		return false, nil
	}
	return true, nil
}

func (db *Database) Ancient(kind string, number uint64) ([]byte, error) {
	var resp hexutil.Bytes
	err := db.remote.Call(&resp, "debug_dbAncient", kind, number)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (db *Database) AncientRange(kind string, start, count, maxBytes uint64) ([][]byte, error) {
	panic("not supported")
}

func (db *Database) Ancients() (uint64, error) {
	var resp uint64
	err := db.remote.Call(&resp, "debug_dbAncients")
	return resp, err
}

func (db *Database) Tail() (uint64, error) {
	panic("not supported")
}

func (db *Database) AncientSize(kind string) (uint64, error) {
	panic("not supported")
}

func (db *Database) ReadAncients(fn func(op ethdb.AncientReaderOp) error) (err error) {
	return fn(db)
}

func (db *Database) Put(key []byte, value []byte) error {
	panic("not supported")
}

func (db *Database) Delete(key []byte) error {
	panic("not supported")
}

func (db *Database) ModifyAncients(f func(ethdb.AncientWriteOp) error) (int64, error) {
	panic("not supported")
}

func (db *Database) TruncateHead(n uint64) error {
	panic("not supported")
}

func (db *Database) TruncateTail(n uint64) error {
	panic("not supported")
}

func (db *Database) Sync() error {
	return nil
}

func (db *Database) MigrateTable(s string, f func([]byte) ([]byte, error)) error {
	panic("not supported")
}

func (db *Database) NewBatch() ethdb.Batch {
	panic("not supported")
}

func (db *Database) NewBatchWithSize(size int) ethdb.Batch {
	panic("not supported")
}

func (db *Database) NewIterator(prefix []byte, start []byte) ethdb.Iterator {
	panic("not supported")
}

func (db *Database) Stat(property string) (string, error) {
	panic("not supported")
}

func (db *Database) AncientDatadir() (string, error) {
	panic("not supported")
}

func (db *Database) Compact(start []byte, limit []byte) error {
	return nil
}

func (db *Database) NewSnapshot() (ethdb.Snapshot, error) {
	panic("not supported")
}

func (db *Database) Close() error {
	db.remote.Close()
	return nil
}

func New(client *rpc.Client) ethdb.Database {
	return &Database{
		remote: client,
	}
}
