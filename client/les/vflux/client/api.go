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

package client

import (
	"time"

	"github.com/r5-labs/r5-core/client/common/mclock"
	"github.com/r5-labs/r5-core/client/les/utils"
	"github.com/r5-labs/r5-core/client/p2p/enode"
)

// PrivateClientAPI implements the vflux client side API
type PrivateClientAPI struct {
	vt *ValueTracker
}

// NewPrivateClientAPI creates a PrivateClientAPI
func NewPrivateClientAPI(vt *ValueTracker) *PrivateClientAPI {
	return &PrivateClientAPI{vt}
}

// parseNodeStr converts either an enode address or a plain hex node id to enode.ID
func parseNodeStr(nodeStr string) (enode.ID, error) {
	if id, err := enode.ParseID(nodeStr); err == nil {
		return id, nil
	}
	if node, err := enode.Parse(enode.ValidSchemes, nodeStr); err == nil {
		return node.ID(), nil
	} else {
		return enode.ID{}, err
	}
}

// RequestStats returns the current contents of the reference request basket, with
// request values meaning average per request rather than total.
func (api *PrivateClientAPI) RequestStats() []RequestStatsItem {
	return api.vt.RequestStats()
}

// Distribution returns a distribution as a series of (X, Y) chart coordinates,
// where the X axis is the response time in seconds while the Y axis is the amount of
// service value received with a response time close to the X coordinate.
// The distribution is optionally normalized to a sum of 1.
// If nodeStr == "" then the global distribution is returned, otherwise the individual
// distribution of the specified server node.
func (api *PrivateClientAPI) Distribution(nodeStr string, normalized bool) (RtDistribution, error) {
	var expFactor utils.ExpirationFactor
	if !normalized {
		expFactor = utils.ExpFactor(api.vt.StatsExpirer().LogOffset(mclock.Now()))
	}
	if nodeStr == "" {
		return api.vt.RtStats().Distribution(normalized, expFactor), nil
	}
	if id, err := parseNodeStr(nodeStr); err == nil {
		return api.vt.GetNode(id).RtStats().Distribution(normalized, expFactor), nil
	} else {
		return RtDistribution{}, err
	}
}

// Timeout suggests a timeout value based on either the global distribution or the
// distribution of the specified node. The parameter is the desired rate of timeouts
// assuming a similar distribution in the future.
// Note that the actual timeout should have a sensible minimum bound so that operating
// under ideal working conditions for a long time (for example, using a local server
// with very low response times) will not make it very hard for the system to accommodate
// longer response times in the future.
func (api *PrivateClientAPI) Timeout(nodeStr string, failRate float64) (float64, error) {
	if nodeStr == "" {
		return float64(api.vt.RtStats().Timeout(failRate)) / float64(time.Second), nil
	}
	if id, err := parseNodeStr(nodeStr); err == nil {
		return float64(api.vt.GetNode(id).RtStats().Timeout(failRate)) / float64(time.Second), nil
	} else {
		return 0, err
	}
}

// Value calculates the total service value provided either globally or by the specified
// server node, using a weight function based on the given timeout.
func (api *PrivateClientAPI) Value(nodeStr string, timeout float64) (float64, error) {
	wt := TimeoutWeights(time.Duration(timeout * float64(time.Second)))
	expFactor := utils.ExpFactor(api.vt.StatsExpirer().LogOffset(mclock.Now()))
	if nodeStr == "" {
		return api.vt.RtStats().Value(wt, expFactor), nil
	}
	if id, err := parseNodeStr(nodeStr); err == nil {
		return api.vt.GetNode(id).RtStats().Value(wt, expFactor), nil
	} else {
		return 0, err
	}
}
