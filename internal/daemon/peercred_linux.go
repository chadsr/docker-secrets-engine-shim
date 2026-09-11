//go:build linux

package daemon

import (
	"log"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// allowedPeerUID reports whether a connection from uid peer is accepted:
// the daemon owner's own uid, or root (uid 0) for dockerd's NRI plugin.
func allowedPeerUID(peer, owner uint32) bool {
	return peer == owner || peer == 0
}

type peerCredListener struct {
	net.Listener
	owner uint32
	logf  func(format string, args ...any)
}

// NewPeerCredListener gates a unix listener by peer credentials: connections
// from uids other than the daemon owner's (or root) are closed before any
// HTTP traffic is served. Abstract sockets carry no filesystem permissions,
// so this is the only access control on them.
func NewPeerCredListener(l net.Listener) net.Listener {
	return &peerCredListener{
		Listener: l,
		owner:    uint32(os.Getuid()),
		logf:     log.Printf,
	}
}

func (l *peerCredListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		cred := peerCred(conn)
		if cred == nil {
			l.logf("daemon: closing connection with unknown peer credentials")
			conn.Close()
			continue
		}
		if !allowedPeerUID(cred.Uid, l.owner) {
			l.logf("daemon: closing untrusted connection from uid %d (pid %d)", cred.Uid, cred.Pid)
			conn.Close()
			continue
		}
		return conn, nil
	}
}

func peerCred(conn net.Conn) *unix.Ucred {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return nil
	}
	var cred *unix.Ucred
	raw, err := uc.SyscallConn()
	if err != nil {
		return nil
	}
	_ = raw.Control(func(fd uintptr) {
		cred, _ = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	return cred
}
