package daemon

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/docker/secrets-engine/x/api/plugins/v1"
	"github.com/docker/secrets-engine/x/secrets"
)

func TestRegistrationService_RegisterPlugin(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry, EngineName: "test-engine", Version: "test-v0.1.0"}

	req := registerReq("test-plugin", "v1.0.0", "**")

	resp, err := svc.RegisterPlugin(t.Context(), connect.NewRequest(req))
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

	_, err := svc.RegisterPlugin(t.Context(), connect.NewRequest(req))
	assert.Error(t, err)
}

func TestRegistrationService_RegisterPlugin_duplicate(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry}

	req := registerReq("dup", "v1.0.0", "**")

	_, err := svc.RegisterPlugin(t.Context(), connect.NewRequest(req))
	require.NoError(t, err)

	_, err = svc.RegisterPlugin(t.Context(), connect.NewRequest(req))
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
