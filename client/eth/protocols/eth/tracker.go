// Copyright 2025 R5
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

package eth

import (
	"time"

	"github.com/r5-codebase/r5-core/p2p/tracker"
)

// requestTracker is a singleton tracker for eth/66 and newer request times.
var requestTracker = tracker.New(ProtocolName, 5*time.Minute)
