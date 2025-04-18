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

package enode

import (
	"crypto/rand"
	"net"
	"testing"

	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/p2p/enr"
	"github.com/stretchr/testify/assert"
)

func newLocalNodeForTesting() (*LocalNode, *DB) {
	db, _ := OpenDB("")
	key, _ := crypto.GenerateKey()
	return NewLocalNode(db, key), db
}

func TestLocalNode(t *testing.T) {
	ln, db := newLocalNodeForTesting()
	defer db.Close()

	if ln.Node().ID() != ln.ID() {
		t.Fatal("inconsistent ID")
	}

	ln.Set(enr.WithEntry("x", uint(3)))
	var x uint
	if err := ln.Node().Load(enr.WithEntry("x", &x)); err != nil {
		t.Fatal("can't load entry 'x':", err)
	} else if x != 3 {
		t.Fatal("wrong value for entry 'x':", x)
	}
}

// This test checks that the sequence number is persisted between restarts.
func TestLocalNodeSeqPersist(t *testing.T) {
	timestamp := nowMilliseconds()

	ln, db := newLocalNodeForTesting()
	defer db.Close()

	initialSeq := ln.Node().Seq()
	if initialSeq < timestamp {
		t.Fatalf("wrong initial seq %d, want at least %d", initialSeq, timestamp)
	}

	ln.Set(enr.WithEntry("x", uint(1)))
	if s := ln.Node().Seq(); s != initialSeq+1 {
		t.Fatalf("wrong seq %d after set, want %d", s, initialSeq+1)
	}

	// Create a new instance, it should reload the sequence number.
	// The number increases just after that because a new record is
	// created without the "x" entry.
	ln2 := NewLocalNode(db, ln.key)
	if s := ln2.Node().Seq(); s != initialSeq+2 {
		t.Fatalf("wrong seq %d on new instance, want %d", s, initialSeq+2)
	}

	finalSeq := ln2.Node().Seq()

	// Create a new instance with a different node key on the same database.
	// This should reset the sequence number.
	key, _ := crypto.GenerateKey()
	ln3 := NewLocalNode(db, key)
	if s := ln3.Node().Seq(); s < finalSeq {
		t.Fatalf("wrong seq %d on instance with changed key, want >= %d", s, finalSeq)
	}
}

// This test checks behavior of the endpoint predictor.
func TestLocalNodeEndpoint(t *testing.T) {
	var (
		fallback  = &net.UDPAddr{IP: net.IP{127, 0, 0, 1}, Port: 80}
		predicted = &net.UDPAddr{IP: net.IP{127, 0, 1, 2}, Port: 81}
		staticIP  = net.IP{127, 0, 1, 2}
	)
	ln, db := newLocalNodeForTesting()
	defer db.Close()

	// Nothing is set initially.
	assert.Equal(t, net.IP(nil), ln.Node().IP())
	assert.Equal(t, 0, ln.Node().UDP())
	initialSeq := ln.Node().Seq()

	// Set up fallback address.
	ln.SetFallbackIP(fallback.IP)
	ln.SetFallbackUDP(fallback.Port)
	assert.Equal(t, fallback.IP, ln.Node().IP())
	assert.Equal(t, fallback.Port, ln.Node().UDP())
	assert.Equal(t, initialSeq+1, ln.Node().Seq())

	// Add endpoint statements from random hosts.
	for i := 0; i < iptrackMinStatements; i++ {
		assert.Equal(t, fallback.IP, ln.Node().IP())
		assert.Equal(t, fallback.Port, ln.Node().UDP())
		assert.Equal(t, initialSeq+1, ln.Node().Seq())

		from := &net.UDPAddr{IP: make(net.IP, 4), Port: 90}
		rand.Read(from.IP)
		ln.UDPEndpointStatement(from, predicted)
	}
	assert.Equal(t, predicted.IP, ln.Node().IP())
	assert.Equal(t, predicted.Port, ln.Node().UDP())
	assert.Equal(t, initialSeq+2, ln.Node().Seq())

	// Static IP overrides prediction.
	ln.SetStaticIP(staticIP)
	assert.Equal(t, staticIP, ln.Node().IP())
	assert.Equal(t, fallback.Port, ln.Node().UDP())
	assert.Equal(t, initialSeq+3, ln.Node().Seq())
}
