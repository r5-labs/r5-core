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

package rawdb

import (
	"bytes"
	"os"
	"testing"
)

func TestCopyFrom(t *testing.T) {
	var (
		content = []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8}
		prefix  = []byte{0x9, 0xa, 0xb, 0xc, 0xd, 0xf}
	)
	var cases = []struct {
		src, dest   string
		offset      uint64
		writePrefix bool
	}{
		{"foo", "bar", 0, false},
		{"foo", "bar", 1, false},
		{"foo", "bar", 8, false},
		{"foo", "foo", 0, false},
		{"foo", "foo", 1, false},
		{"foo", "foo", 8, false},
		{"foo", "bar", 0, true},
		{"foo", "bar", 1, true},
		{"foo", "bar", 8, true},
	}
	for _, c := range cases {
		os.WriteFile(c.src, content, 0600)

		if err := copyFrom(c.src, c.dest, c.offset, func(f *os.File) error {
			if !c.writePrefix {
				return nil
			}
			f.Write(prefix)
			return nil
		}); err != nil {
			os.Remove(c.src)
			t.Fatalf("Failed to copy %v", err)
		}

		blob, err := os.ReadFile(c.dest)
		if err != nil {
			os.Remove(c.src)
			os.Remove(c.dest)
			t.Fatalf("Failed to read %v", err)
		}
		want := content[c.offset:]
		if c.writePrefix {
			want = append(prefix, want...)
		}
		if !bytes.Equal(blob, want) {
			t.Fatal("Unexpected value")
		}
		os.Remove(c.src)
		os.Remove(c.dest)
	}
}
