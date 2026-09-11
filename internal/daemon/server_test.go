package daemon

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	healthv1 "github.com/docker/secrets-engine/x/api/health/v1"
	healthv1connect "github.com/docker/secrets-engine/x/api/health/v1/healthv1connect"
	pluginsv1 "github.com/docker/secrets-engine/x/api/plugins/v1"
	pluginsv1connect "github.com/docker/secrets-engine/x/api/plugins/v1/pluginsv1connect"
	resolverv1 "github.com/docker/secrets-engine/x/api/resolver/v1"
	resolverv1connect "github.com/docker/secrets-engine/x/api/resolver/v1/resolverv1connect"
	"github.com/docker/secrets-engine/x/ipc"
	"github.com/docker/secrets-engine/x/testhelper"
)

func TestServer_healthcheck(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := NewServer(socketPath, "test-engine", "v0.1.0", "abc123", "2026-06-03")
	go srv.ListenAndServe()
	t.Cleanup(func() { srv.Close() })
	waitForSocket(t, socketPath)

	client := unixHTTPClient(socketPath)
	resp, err := client.Get("http://unix/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_version(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := NewServer(socketPath, "test-engine", "v0.1.0", "abc123", "2026-06-03")
	go srv.ListenAndServe()
	t.Cleanup(func() { srv.Close() })
	waitForSocket(t, socketPath)

	client := unixHTTPClient(socketPath)
	versionClient := healthv1connect.NewVersionServiceClient(client, "http://unix")
	resp, err := versionClient.GetVersion(t.Context(), connect.NewRequest(&healthv1.GetVersionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "v0.1.0", resp.Msg.GetVersion())
}

func TestServer_plugin_hijack_and_register(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := NewServer(socketPath, "test-engine", "v0.1.0", "abc123", "2026-06-03")
	go srv.ListenAndServe()
	t.Cleanup(func() { srv.Close() })
	waitForSocket(t, socketPath)

	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	hijackedConn, err := ipc.Hijackify(conn, 2*time.Second)
	require.NoError(t, err)

	pluginMux := http.NewServeMux()
	pluginMux.Handle(resolverv1connect.NewResolverServiceHandler(&mockPluginResolver{}))
	pluginIPC, pluginClient, err := ipc.NewClientIPC(
		testhelper.TestLogger(t),
		hijackedConn,
		pluginMux,
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() { pluginIPC.Close() })

	regClient := pluginsv1connect.NewRegisterServiceClient(pluginClient, "http://unix")
	regReq := &pluginsv1.RegisterPluginRequest{}
	regReq.SetName("docker-pass")
	regReq.SetVersion("v0.1.0")
	sp := &pluginsv1.SecretsProvider{}
	sp.SetPattern("**")
	regReq.SetSecretsProvider(sp)
	regResp, err := regClient.RegisterPlugin(t.Context(), connect.NewRequest(regReq))
	require.NoError(t, err)
	assert.Equal(t, "test-engine", regResp.Msg.GetEngineName())

	plugins := srv.Registry.List()
	require.Len(t, plugins, 1)
	assert.Equal(t, "docker-pass", plugins[0].Name)
	assert.NotNil(t, plugins[0].Client)
}

func TestServer_resolve_secret_through_plugin(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "test.sock")
	srv := NewServer(socketPath, "test-engine", "v0.1.0", "abc123", "2026-06-03")
	go srv.ListenAndServe()
	t.Cleanup(func() { srv.Close() })
	waitForSocket(t, socketPath)

	conn, err := net.Dial("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	hijackedConn, err := ipc.Hijackify(conn, 2*time.Second)
	require.NoError(t, err)

	pluginMux := http.NewServeMux()
	pluginMux.Handle(resolverv1connect.NewResolverServiceHandler(&mockPluginResolver{}))
	pluginIPC, pluginClient, err := ipc.NewClientIPC(
		testhelper.TestLogger(t),
		hijackedConn,
		pluginMux,
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() { pluginIPC.Close() })

	regClient := pluginsv1connect.NewRegisterServiceClient(pluginClient, "http://unix")
	regReq := &pluginsv1.RegisterPluginRequest{}
	regReq.SetName("docker-pass")
	regReq.SetVersion("v0.1.0")
	sp := &pluginsv1.SecretsProvider{}
	sp.SetPattern("**")
	regReq.SetSecretsProvider(sp)
	_, err = regClient.RegisterPlugin(t.Context(), connect.NewRequest(regReq))
	require.NoError(t, err)

	daemonClient := unixHTTPClient(socketPath)
	resolverClient := resolverv1connect.NewResolverServiceClient(daemonClient, "http://unix")

	req := &resolverv1.GetSecretsRequest{}
	req.SetPattern("mysecret")
	resp, err := resolverClient.GetSecrets(t.Context(), connect.NewRequest(req))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetEnvelopes(), 1)
	assert.Equal(t, "mysecret", resp.Msg.GetEnvelopes()[0].GetId())
	assert.Equal(t, []byte("resolved-value"), resp.Msg.GetEnvelopes()[0].GetValue())
}

type mockPluginResolver struct{}

func (m *mockPluginResolver) GetSecrets(_ context.Context, req *connect.Request[resolverv1.GetSecretsRequest]) (*connect.Response[resolverv1.GetSecretsResponse], error) {
	envelope := &resolverv1.GetSecretsResponse_Envelope{}
	envelope.SetId(req.Msg.GetPattern())
	envelope.SetValue([]byte("resolved-value"))
	resp := &resolverv1.GetSecretsResponse{}
	resp.SetEnvelopes([]*resolverv1.GetSecretsResponse_Envelope{envelope})
	return connect.NewResponse(resp), nil
}

func unixHTTPClient(socketPath string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
}

func waitForSocket(t *testing.T, socketPath string) {
	t.Helper()
	require.Eventually(t, func() bool {
		_, err := os.Stat(socketPath)
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)
}
