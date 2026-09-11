//go:build linux

package daemon

import (
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowedPeerUID(t *testing.T) {
	tests := []struct {
		peer, owner uint32
		want        bool
	}{
		{1000, 1000, true},
		{0, 1000, true},
		{1001, 1000, false},
		{65534, 1000, false},
		{1000, 0, false},
		{1001, 0, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, allowedPeerUID(tt.peer, tt.owner), "peer=%d owner=%d", tt.peer, tt.owner)
	}
}

func TestPeerCredListener_accepts_same_uid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.sock")
	l, err := net.Listen("unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	gated := NewPeerCredListener(l)
	accepted := make(chan net.Conn, 1)
	go func() {
		conn, err := gated.Accept()
		if err == nil {
			accepted <- conn
		}
	}()

	client, err := net.Dial("unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	_, err = client.Write([]byte("ping"))
	require.NoError(t, err)

	var conn net.Conn
	select {
	case conn = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("same-uid connection was not accepted")
	}
	t.Cleanup(func() { conn.Close() })

	buf := make([]byte, 4)
	_, err = conn.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "ping", string(buf))
}
