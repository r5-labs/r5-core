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

package beacon

import (
	"math/big"

	"github.com/r5-labs/r5-core/consensus"
	"github.com/r5-labs/r5-core/core/types"
)

// NewFaker creates a fake consensus engine for testing.
// The fake engine simulates a merged network.
// It can not be used to test the merge transition.
// This type is needed since the fakeChainReader can not be used with
// a normal beacon consensus engine.
func NewFaker() consensus.Engine {
	return new(faker)
}

type faker struct {
	Beacon
}

func (f *faker) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return beaconDifficulty
}
