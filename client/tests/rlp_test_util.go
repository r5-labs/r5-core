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

package tests

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/r5-labs/r5-core/client/rlp"
)

// RLPTest is the JSON structure of a single RLP test.
type RLPTest struct {
	// If the value of In is "INVALID" or "VALID", the test
	// checks whether Out can be decoded into a value of
	// type interface{}.
	//
	// For other JSON values, In is treated as a driver for
	// calls to rlp.Stream. The test also verifies that encoding
	// In produces the bytes in Out.
	In interface{}

	// Out is a hex-encoded RLP value.
	Out string
}

// FromHex returns the bytes represented by the hexadecimal string s.
// s may be prefixed with "0x".
// This is copy-pasted from bytes.go, which does not return the error
func FromHex(s string) ([]byte, error) {
	if len(s) > 1 && (s[0:2] == "0x" || s[0:2] == "0X") {
		s = s[2:]
	}
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return hex.DecodeString(s)
}

// Run executes the test.
func (t *RLPTest) Run() error {
	outb, err := FromHex(t.Out)
	if err != nil {
		return fmt.Errorf("invalid hex in Out")
	}

	// Handle simple decoding tests with no actual In value.
	if t.In == "VALID" || t.In == "INVALID" {
		return checkDecodeInterface(outb, t.In == "VALID")
	}

	// Check whether encoding the value produces the same bytes.
	in := translateJSON(t.In)
	b, err := rlp.EncodeToBytes(in)
	if err != nil {
		return fmt.Errorf("encode failed: %v", err)
	}
	if !bytes.Equal(b, outb) {
		return fmt.Errorf("encode produced %x, want %x", b, outb)
	}
	// Test stream decoding.
	s := rlp.NewStream(bytes.NewReader(outb), 0)
	return checkDecodeFromJSON(s, in)
}

func checkDecodeInterface(b []byte, isValid bool) error {
	err := rlp.DecodeBytes(b, new(interface{}))
	switch {
	case isValid && err != nil:
		return fmt.Errorf("decoding failed: %v", err)
	case !isValid && err == nil:
		return fmt.Errorf("decoding of invalid value succeeded")
	}
	return nil
}

// translateJSON makes test json values encodable with RLP.
func translateJSON(v interface{}) interface{} {
	switch v := v.(type) {
	case float64:
		return uint64(v)
	case string:
		if len(v) > 0 && v[0] == '#' { // # starts a faux big int.
			big, ok := new(big.Int).SetString(v[1:], 10)
			if !ok {
				panic(fmt.Errorf("bad test: bad big int: %q", v))
			}
			return big
		}
		return []byte(v)
	case []interface{}:
		new := make([]interface{}, len(v))
		for i := range v {
			new[i] = translateJSON(v[i])
		}
		return new
	default:
		panic(fmt.Errorf("can't handle %T", v))
	}
}

// checkDecodeFromJSON decodes from s guided by exp. exp drives the
// Stream by invoking decoding operations (Uint, Big, List, ...) based
// on the type of each value. The value decoded from the RLP stream
// must match the JSON value.
func checkDecodeFromJSON(s *rlp.Stream, exp interface{}) error {
	switch exp := exp.(type) {
	case uint64:
		i, err := s.Uint64()
		if err != nil {
			return addStack("Uint", exp, err)
		}
		if i != exp {
			return addStack("Uint", exp, fmt.Errorf("result mismatch: got %d", i))
		}
	case *big.Int:
		big := new(big.Int)
		if err := s.Decode(&big); err != nil {
			return addStack("Big", exp, err)
		}
		if big.Cmp(exp) != 0 {
			return addStack("Big", exp, fmt.Errorf("result mismatch: got %d", big))
		}
	case []byte:
		b, err := s.Bytes()
		if err != nil {
			return addStack("Bytes", exp, err)
		}
		if !bytes.Equal(b, exp) {
			return addStack("Bytes", exp, fmt.Errorf("result mismatch: got %x", b))
		}
	case []interface{}:
		if _, err := s.List(); err != nil {
			return addStack("List", exp, err)
		}
		for i, v := range exp {
			if err := checkDecodeFromJSON(s, v); err != nil {
				return addStack(fmt.Sprintf("[%d]", i), exp, err)
			}
		}
		if err := s.ListEnd(); err != nil {
			return addStack("ListEnd", exp, err)
		}
	default:
		panic(fmt.Errorf("unhandled type: %T", exp))
	}
	return nil
}

func addStack(op string, val interface{}, err error) error {
	lines := strings.Split(err.Error(), "\n")
	lines = append(lines, fmt.Sprintf("\t%s: %v", op, val))
	return errors.New(strings.Join(lines, "\n"))
}
