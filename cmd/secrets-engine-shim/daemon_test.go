package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chadsr/docker-secrets-engine-shim/internal/daemon"
)

// isolateCache points the socket dir at a temp dir so tests can assert nothing is created.
func isolateCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	return filepath.Join(dir, "docker-secrets-engine")
}

// Regression: --help used to fall through into daemon startup, unlinking the live daemon's socket.
func TestDaemonCommandHelp(t *testing.T) {
	socketDir := isolateCache(t)

	var out bytes.Buffer
	root := newDaemonCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "Usage:")
	require.Contains(t, out.String(), "docker-secrets-engine-shim")
	require.NoDirExists(t, socketDir)
}

func TestDaemonCommandShortHelp(t *testing.T) {
	socketDir := isolateCache(t)

	root := newDaemonCommand()
	root.SetArgs([]string{"-h"})
	require.NoError(t, root.Execute())
	require.NoDirExists(t, socketDir)
}

func TestDaemonCommandVersion(t *testing.T) {
	socketDir := isolateCache(t)

	var out bytes.Buffer
	root := newDaemonCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--version"})
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), version)
	require.NoDirExists(t, socketDir)
}

func TestDaemonCommandUnknownFlagFails(t *testing.T) {
	socketDir := isolateCache(t)

	root := newDaemonCommand()
	root.SetArgs([]string{"--frobnicate"})
	require.Error(t, root.Execute())
	require.NoDirExists(t, socketDir)
}

func TestDaemonCommandRejectsPositionalArgs(t *testing.T) {
	socketDir := isolateCache(t)

	root := newDaemonCommand()
	root.SetArgs([]string{"oops"})
	require.Error(t, root.Execute())
	require.NoDirExists(t, socketDir)
}

func TestSecondStartDoesNotDisturbLiveDaemon(t *testing.T) {
	fsSock := filepath.Join(t.TempDir(), "engine.sock")

	srv := daemon.NewServer(fsSock, engineName, version, commit, date)
	go srv.ListenAndServe()
	t.Cleanup(func() { srv.Close() })

	require.Eventually(t, func() bool {
		_, err := os.Stat(fsSock)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)

	_, err := daemon.ListenUnix(fsSock)
	require.ErrorIs(t, err, daemon.ErrSocketInUse)

	require.FileExists(t, fsSock)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", fsSock)
			},
		},
	}
	resp, err := client.Get("http://daemon/health")
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
