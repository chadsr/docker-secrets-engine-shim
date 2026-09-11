//go:build !linux

package daemon

import "net"

func NewPeerCredListener(l net.Listener) net.Listener { return l }
