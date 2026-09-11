package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/chadsr/docker-secrets-engine-shim/internal/daemon"
	"github.com/docker/secrets-engine/x/logging"
)

// The plugin child is this test binary re-exec'd; TestMain branches into plugin mode.
const testPluginChild = "SHIM_TEST_PLUGIN_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(testPluginChild) != "" {
		runPluginMode(logging.NewDefaultLogger(cmdDockerPass))
		return
	}
	os.Exit(m.Run())
}

// Exercises the launch wiring an SDK upgrade can silently break: fd passing, PluginConfigFromEngine, yamux registration.
func TestStartPluginHandshake(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := daemon.NewServer(socketPath, engineName, version, commit, date)
	go srv.ListenAndServe()
	t.Cleanup(func() { srv.Close() })
	require.Eventually(t, func() bool {
		_, err := os.Stat(socketPath)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)

	t.Setenv(testPluginChild, "1")
	require.NoError(t, startPlugin(logging.NewDefaultLogger("test"), srv))

	require.Eventually(t, func() bool {
		for _, p := range srv.Registry.List() {
			if p.Name == cmdDockerPass && p.Client != nil {
				return true
			}
		}
		return false
	}, 15*time.Second, 50*time.Millisecond, "plugin did not register with the daemon")
}
