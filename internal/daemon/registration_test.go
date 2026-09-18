package daemon

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/docker/secrets-engine/x/api/plugins/v1"
	"github.com/docker/secrets-engine/x/secrets"
)

func ctxWithPluginClient(ctx context.Context, c *http.Client) context.Context {
	ref := &PluginClientRef{}
	ref.Set(c)
	return context.WithValue(ctx, pluginClientKey{}, ref)
}

func TestRegistrationService_RegisterPlugin_binds_client_from_request_context(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry}

	client := &http.Client{}
	_, err := svc.RegisterPlugin(ctxWithPluginClient(t.Context(), client), connect.NewRequest(registerReq("test-plugin", "v1.0.0", "**")))
	require.NoError(t, err)

	plugins := registry.List()
	require.Len(t, plugins, 1)
	assert.Equal(t, client, plugins[0].Client, "registration must bind to the client of the connection it arrived on")
	assert.Equal(t, "v1.0.0", plugins[0].Version)
}

func TestRegistrationService_RegisterPlugin(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry, EngineName: "test-engine", Version: "test-v0.1.0"}

	req := registerReq("test-plugin", "v1.0.0", "**")

	resp, err := svc.RegisterPlugin(ctxWithPluginClient(t.Context(), &http.Client{}), connect.NewRequest(req))
	require.NoError(t, err)
	assert.Equal(t, "test-engine", resp.Msg.GetEngineName())
	assert.Equal(t, "test-v0.1.0", resp.Msg.GetEngineVersion())

	plugins := registry.List()
	require.Len(t, plugins, 1)
	assert.Equal(t, "test-plugin", plugins[0].Name)
	assert.Equal(t, secrets.MustParsePattern("**"), plugins[0].Pattern)
}

func TestRegistrationService_RegisterPlugin_invalid_pattern(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry}

	req := registerReq("bad-plugin", "v1.0.0", "!!!invalid!!!")

	_, err := svc.RegisterPlugin(ctxWithPluginClient(t.Context(), &http.Client{}), connect.NewRequest(req))
	assert.Error(t, err)
}

func TestRegistrationService_RegisterPlugin_invalid_name(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry}

	_, err := svc.RegisterPlugin(ctxWithPluginClient(t.Context(), &http.Client{}), connect.NewRequest(registerReq("", "v1.0.0", "**")))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	assert.Empty(t, registry.List())
}

func TestRegistrationService_RegisterPlugin_invalid_version(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry}

	for _, version := range []string{"", "1.0.0", "vlatest"} {
		_, err := svc.RegisterPlugin(ctxWithPluginClient(t.Context(), &http.Client{}), connect.NewRequest(registerReq("test-plugin", version, "**")))
		require.Error(t, err, "version %q must be rejected", version)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	}
	assert.Empty(t, registry.List())
}

func TestRegistrationService_RegisterPlugin_requires_plugin_connection(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry}

	_, err := svc.RegisterPlugin(t.Context(), connect.NewRequest(registerReq("test-plugin", "v1.0.0", "**")))
	require.Error(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Empty(t, registry.List())
}

func TestRegistrationService_RegisterPlugin_duplicate(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry}

	ctx := ctxWithPluginClient(t.Context(), &http.Client{})
	req := registerReq("dup", "v1.0.0", "**")

	_, err := svc.RegisterPlugin(ctx, connect.NewRequest(req))
	require.NoError(t, err)

	_, err = svc.RegisterPlugin(ctx, connect.NewRequest(req))
	assert.NoError(t, err)

	plugins := registry.List()
	assert.Len(t, plugins, 1)
}

func registerReq(name, ver, pattern string) *pluginsv1.RegisterPluginRequest {
	sp := &pluginsv1.SecretsProvider{}
	sp.SetPattern(pattern)
	req := &pluginsv1.RegisterPluginRequest{}
	req.SetName(name)
	req.SetVersion(ver)
	req.SetSecretsProvider(sp)
	return req
}
