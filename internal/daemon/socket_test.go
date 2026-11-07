package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestListenUnixAbsentPathBinds(t *testing.T) {
	// The parent dir does not exist yet.
	path := filepath.Join(t.TempDir(), "docker-secrets-engine", "engine.sock")
	l, err := ListenUnix(path)
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	conn, err := net.DialTimeout("unix", path, time.Second)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func TestListenUnixStaleSocketRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engine.sock")
	first, err := ListenUnix(path)
	require.NoError(t, err)
	// Closing leaves the socket file behind, like a crashed daemon.
	require.NoError(t, first.Close())

	second, err := ListenUnix(path)
	require.NoError(t, err)
	t.Cleanup(func() { second.Close() })

	conn, err := net.DialTimeout("unix", path, time.Second)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func TestListenUnixLiveSocketRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engine.sock")
	first, err := ListenUnix(path)
	require.NoError(t, err)
	t.Cleanup(func() { first.Close() })

	l, err := ListenUnix(path)
	require.ErrorIs(t, err, ErrSocketInUse)
	require.Nil(t, l)

	// The live listener must be untouched.
	conn, err := net.DialTimeout("unix", path, time.Second)
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	fi, err := os.Lstat(path)
	require.NoError(t, err)
	require.NotZero(t, fi.Mode()&os.ModeSocket)
}

func TestListenUnixNonSocketFileRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "engine.sock")
	require.NoError(t, os.WriteFile(path, []byte("not a socket"), 0o600))

	l, err := ListenUnix(path)
	require.Error(t, err)
	require.Nil(t, l)
	require.FileExists(t, path)
}

func TestListenUnixAbstractBindExclusive(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("abstract sockets are linux-only")
	}
	path := "@" + fmt.Sprintf("docker-secrets-engine-shim-test/%d-%d.sock", os.Getpid(), time.Now().UnixNano())

	first, err := ListenUnix(path)
	require.NoError(t, err)
	t.Cleanup(func() { first.Close() })

	// Abstract sockets cannot be stolen by unlinking.
	_, err = ListenUnix(path)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrSocketInUse)
}
