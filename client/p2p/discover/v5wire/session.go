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

package v5wire

import (
	"crypto/ecdsa"
	crand "crypto/rand"
	"encoding/binary"
	"time"

	"github.com/r5-labs/r5-core/client/common/lru"
	"github.com/r5-labs/r5-core/client/common/mclock"
	"github.com/r5-labs/r5-core/client/crypto"
	"github.com/r5-labs/r5-core/client/p2p/enode"
)

const handshakeTimeout = time.Second

// The SessionCache keeps negotiated encryption keys and
// state for in-progress handshakes in the Discovery v5 wire protocol.
type SessionCache struct {
	sessions   lru.BasicLRU[sessionID, *session]
	handshakes map[sessionID]*Whoareyou
	clock      mclock.Clock

	// hooks for overriding randomness.
	nonceGen        func(uint32) (Nonce, error)
	maskingIVGen    func([]byte) error
	ephemeralKeyGen func() (*ecdsa.PrivateKey, error)
}

// sessionID identifies a session or handshake.
type sessionID struct {
	id   enode.ID
	addr string
}

// session contains session information
type session struct {
	writeKey     []byte
	readKey      []byte
	nonceCounter uint32
}

// keysFlipped returns a copy of s with the read and write keys flipped.
func (s *session) keysFlipped() *session {
	return &session{s.readKey, s.writeKey, s.nonceCounter}
}

func NewSessionCache(maxItems int, clock mclock.Clock) *SessionCache {
	return &SessionCache{
		sessions:        lru.NewBasicLRU[sessionID, *session](maxItems),
		handshakes:      make(map[sessionID]*Whoareyou),
		clock:           clock,
		nonceGen:        generateNonce,
		maskingIVGen:    generateMaskingIV,
		ephemeralKeyGen: crypto.GenerateKey,
	}
}

func generateNonce(counter uint32) (n Nonce, err error) {
	binary.BigEndian.PutUint32(n[:4], counter)
	_, err = crand.Read(n[4:])
	return n, err
}

func generateMaskingIV(buf []byte) error {
	_, err := crand.Read(buf)
	return err
}

// nextNonce creates a nonce for encrypting a message to the given session.
func (sc *SessionCache) nextNonce(s *session) (Nonce, error) {
	s.nonceCounter++
	return sc.nonceGen(s.nonceCounter)
}

// session returns the current session for the given node, if any.
func (sc *SessionCache) session(id enode.ID, addr string) *session {
	item, _ := sc.sessions.Get(sessionID{id, addr})
	return item
}

// readKey returns the current read key for the given node.
func (sc *SessionCache) readKey(id enode.ID, addr string) []byte {
	if s := sc.session(id, addr); s != nil {
		return s.readKey
	}
	return nil
}

// storeNewSession stores new encryption keys in the cache.
func (sc *SessionCache) storeNewSession(id enode.ID, addr string, s *session) {
	sc.sessions.Add(sessionID{id, addr}, s)
}

// getHandshake gets the handshake challenge we previously sent to the given remote node.
func (sc *SessionCache) getHandshake(id enode.ID, addr string) *Whoareyou {
	return sc.handshakes[sessionID{id, addr}]
}

// storeSentHandshake stores the handshake challenge sent to the given remote node.
func (sc *SessionCache) storeSentHandshake(id enode.ID, addr string, challenge *Whoareyou) {
	challenge.sent = sc.clock.Now()
	sc.handshakes[sessionID{id, addr}] = challenge
}

// deleteHandshake deletes handshake data for the given node.
func (sc *SessionCache) deleteHandshake(id enode.ID, addr string) {
	delete(sc.handshakes, sessionID{id, addr})
}

// handshakeGC deletes timed-out handshakes.
func (sc *SessionCache) handshakeGC() {
	deadline := sc.clock.Now().Add(-handshakeTimeout)
	for key, challenge := range sc.handshakes {
		if challenge.sent < deadline {
			delete(sc.handshakes, key)
		}
	}
}
