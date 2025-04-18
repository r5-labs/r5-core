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

package trie

import (
	"testing"

	"github.com/r5-labs/r5-core/client/common"
	"github.com/r5-labs/r5-core/client/core/rawdb"
)

// Tests that the trie database returns a missing trie node error if attempting
// to retrieve the meta root.
func TestDatabaseMetarootFetch(t *testing.T) {
	db := NewDatabase(rawdb.NewMemoryDatabase())
	if _, err := db.Node(common.Hash{}); err == nil {
		t.Fatalf("metaroot retrieval succeeded")
	}
}
