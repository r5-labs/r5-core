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
	"io"
	"os"

	"github.com/r5-labs/r5-core/client/log"
	"github.com/r5-labs/r5-core/client/rlp"
)

const freezerVersion = 1 // The initial version tag of freezer table metadata

// freezerTableMeta wraps all the metadata of the freezer table.
type freezerTableMeta struct {
	// Version is the versioning descriptor of the freezer table.
	Version uint16

	// VirtualTail indicates how many items have been marked as deleted.
	// Its value is equal to the number of items removed from the table
	// plus the number of items hidden in the table, so it should never
	// be lower than the "actual tail".
	VirtualTail uint64
}

// newMetadata initializes the metadata object with the given virtual tail.
func newMetadata(tail uint64) *freezerTableMeta {
	return &freezerTableMeta{
		Version:     freezerVersion,
		VirtualTail: tail,
	}
}

// readMetadata reads the metadata of the freezer table from the
// given metadata file.
func readMetadata(file *os.File) (*freezerTableMeta, error) {
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	var meta freezerTableMeta
	if err := rlp.Decode(file, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// writeMetadata writes the metadata of the freezer table into the
// given metadata file.
func writeMetadata(file *os.File, meta *freezerTableMeta) error {
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	return rlp.Encode(file, meta)
}

// loadMetadata loads the metadata from the given metadata file.
// Initializes the metadata file with the given "actual tail" if
// it's empty.
func loadMetadata(file *os.File, tail uint64) (*freezerTableMeta, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	// Write the metadata with the given actual tail into metadata file
	// if it's non-existent. There are two possible scenarios here:
	// - the freezer table is empty
	// - the freezer table is legacy
	// In both cases, write the meta into the file with the actual tail
	// as the virtual tail.
	if stat.Size() == 0 {
		m := newMetadata(tail)
		if err := writeMetadata(file, m); err != nil {
			return nil, err
		}
		return m, nil
	}
	m, err := readMetadata(file)
	if err != nil {
		return nil, err
	}
	// Update the virtual tail with the given actual tail if it's even
	// lower than it. Theoretically it shouldn't happen at all, print
	// a warning here.
	if m.VirtualTail < tail {
		log.Warn("Updated virtual tail", "have", m.VirtualTail, "now", tail)
		m.VirtualTail = tail
		if err := writeMetadata(file, m); err != nil {
			return nil, err
		}
	}
	return m, nil
}
