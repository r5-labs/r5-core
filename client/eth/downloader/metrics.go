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

// Contains the metrics collected by the downloader.

package downloader

import (
	"github.com/r5-labs/r5-core/client/metrics"
)

var (
	headerInMeter      = metrics.NewRegisteredMeter("eth/downloader/headers/in", nil)
	headerReqTimer     = metrics.NewRegisteredTimer("eth/downloader/headers/req", nil)
	headerDropMeter    = metrics.NewRegisteredMeter("eth/downloader/headers/drop", nil)
	headerTimeoutMeter = metrics.NewRegisteredMeter("eth/downloader/headers/timeout", nil)

	bodyInMeter      = metrics.NewRegisteredMeter("eth/downloader/bodies/in", nil)
	bodyReqTimer     = metrics.NewRegisteredTimer("eth/downloader/bodies/req", nil)
	bodyDropMeter    = metrics.NewRegisteredMeter("eth/downloader/bodies/drop", nil)
	bodyTimeoutMeter = metrics.NewRegisteredMeter("eth/downloader/bodies/timeout", nil)

	receiptInMeter      = metrics.NewRegisteredMeter("eth/downloader/receipts/in", nil)
	receiptReqTimer     = metrics.NewRegisteredTimer("eth/downloader/receipts/req", nil)
	receiptDropMeter    = metrics.NewRegisteredMeter("eth/downloader/receipts/drop", nil)
	receiptTimeoutMeter = metrics.NewRegisteredMeter("eth/downloader/receipts/timeout", nil)

	throttleCounter = metrics.NewRegisteredCounter("eth/downloader/throttle", nil)
)
