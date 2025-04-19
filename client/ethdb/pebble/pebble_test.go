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

//go:build (arm64 || amd64) && !openbsd

package pebble

import (
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/r5-labs/r5-core/client/ethdb"
	"github.com/r5-labs/r5-core/client/ethdb/dbtest"
)

func TestPebbleDB(t *testing.T) {
	t.Run("DatabaseSuite", func(t *testing.T) {
		dbtest.TestDatabaseSuite(t, func() ethdb.KeyValueStore {
			db, err := pebble.Open("", &pebble.Options{
				FS: vfs.NewMem(),
			})
			if err != nil {
				t.Fatal(err)
			}
			return &Database{
				db: db,
			}
		})
	})
}

func BenchmarkPebbleDB(b *testing.B) {
	dbtest.BenchDatabaseSuite(b, func() ethdb.KeyValueStore {
		db, err := pebble.Open("", &pebble.Options{
			FS: vfs.NewMem(),
		})
		if err != nil {
			b.Fatal(err)
		}
		return &Database{
			db: db,
		}
	})
}
