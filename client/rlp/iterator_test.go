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

package rlp

import (
	"testing"

	"github.com/r5-labs/r5-core/client/common/hexutil"
)

// TestIterator tests some basic things about the ListIterator. A more
// comprehensive test can be found in core/rlp_test.go, where we can
// use both types and rlp without dependency cycles
func TestIterator(t *testing.T) {
	bodyRlpHex := "0xf902cbf8d6f869800182c35094000000000000000000000000000000000000aaaa808a000000000000000000001ba01025c66fad28b4ce3370222624d952c35529e602af7cbe04f667371f61b0e3b3a00ab8813514d1217059748fd903288ace1b4001a4bc5fbde2790debdc8167de2ff869010182c35094000000000000000000000000000000000000aaaa808a000000000000000000001ca05ac4cf1d19be06f3742c21df6c49a7e929ceb3dbaf6a09f3cfb56ff6828bd9a7a06875970133a35e63ac06d360aa166d228cc013e9b96e0a2cae7f55b22e1ee2e8f901f0f901eda0c75448377c0e426b8017b23c5f77379ecf69abc1d5c224284ad3ba1c46c59adaa00000000000000000000000000000000000000000000000000000000000000000940000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000000b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000808080808080a00000000000000000000000000000000000000000000000000000000000000000880000000000000000"
	bodyRlp := hexutil.MustDecode(bodyRlpHex)

	it, err := NewListIterator(bodyRlp)
	if err != nil {
		t.Fatal(err)
	}
	// Check that txs exist
	if !it.Next() {
		t.Fatal("expected two elems, got zero")
	}
	txs := it.Value()
	// Check that uncles exist
	if !it.Next() {
		t.Fatal("expected two elems, got one")
	}
	txit, err := NewListIterator(txs)
	if err != nil {
		t.Fatal(err)
	}
	var i = 0
	for txit.Next() {
		if txit.err != nil {
			t.Fatal(txit.err)
		}
		i++
	}
	if exp := 2; i != exp {
		t.Errorf("count wrong, expected %d got %d", i, exp)
	}
}
