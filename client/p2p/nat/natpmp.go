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

package nat

import (
	"fmt"
	"net"
	"strings"
	"time"

	natpmp "github.com/jackpal/go-nat-pmp"
)

// natPMPClient adapts the NAT-PMP protocol implementation so it conforms to
// the common interface.
type pmp struct {
	gw net.IP
	c  *natpmp.Client
}

func (n *pmp) String() string {
	return fmt.Sprintf("NAT-PMP(%v)", n.gw)
}

func (n *pmp) ExternalIP() (net.IP, error) {
	response, err := n.c.GetExternalAddress()
	if err != nil {
		return nil, err
	}
	return response.ExternalIPAddress[:], nil
}

func (n *pmp) AddMapping(protocol string, extport, intport int, name string, lifetime time.Duration) error {
	if lifetime <= 0 {
		return fmt.Errorf("lifetime must not be <= 0")
	}
	// Note order of port arguments is switched between our
	// AddMapping and the client's AddPortMapping.
	res, err := n.c.AddPortMapping(strings.ToLower(protocol), intport, extport, int(lifetime/time.Second))
	if err != nil {
		return err
	}

	// NAT-PMP maps an alternative available port number if the requested
	// port is already mapped to another address and returns success. In this
	// case, we return an error because there is no way to return the new port
	// to the caller.
	if uint16(extport) != res.MappedExternalPort {
		// Destroy the mapping in NAT device.
		n.c.AddPortMapping(strings.ToLower(protocol), intport, 0, 0)
		return fmt.Errorf("port %d already mapped to another address (%s)", extport, protocol)
	}

	return nil
}

func (n *pmp) DeleteMapping(protocol string, extport, intport int) (err error) {
	// To destroy a mapping, send an add-port with an internalPort of
	// the internal port to destroy, an external port of zero and a
	// time of zero.
	_, err = n.c.AddPortMapping(strings.ToLower(protocol), intport, 0, 0)
	return err
}

func discoverPMP() Interface {
	// run external address lookups on all potential gateways
	gws := potentialGateways()
	found := make(chan *pmp, len(gws))
	for i := range gws {
		gw := gws[i]
		go func() {
			c := natpmp.NewClient(gw)
			if _, err := c.GetExternalAddress(); err != nil {
				found <- nil
			} else {
				found <- &pmp{gw, c}
			}
		}()
	}
	// return the one that responds first.
	// discovery needs to be quick, so we stop caring about
	// any responses after a very short timeout.
	timeout := time.NewTimer(1 * time.Second)
	defer timeout.Stop()
	for range gws {
		select {
		case c := <-found:
			if c != nil {
				return c
			}
		case <-timeout.C:
			return nil
		}
	}
	return nil
}

// TODO: improve this. We currently assume that (on most networks)
// the router is X.X.X.1 in a local LAN range.
func potentialGateways() (gws []net.IP) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		ifaddrs, err := iface.Addrs()
		if err != nil {
			return gws
		}
		for _, addr := range ifaddrs {
			if x, ok := addr.(*net.IPNet); ok {
				if x.IP.IsPrivate() {
					ip := x.IP.Mask(x.Mask).To4()
					if ip != nil {
						ip[3] = ip[3] | 0x01
						gws = append(gws, ip)
					}
				}
			}
		}
	}
	return gws
}
