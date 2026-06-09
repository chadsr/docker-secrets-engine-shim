package daemon

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	resolverv1 "github.com/docker/secrets-engine/x/api/resolver/v1"
	"github.com/docker/secrets-engine/x/secrets"
)

func TestRegistrationService_RegisterPlugin(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry}

	req := &resolverv1.RegisterPluginRequest{}
	req.SetName("test-plugin")
	req.SetVersion("v1.0.0")
	req.SetPattern("**")

	resp, err := svc.RegisterPlugin(t.Context(), connect.NewRequest(req))
	require.NoError(t, err)
	assert.Equal(t, "secrets-engine-shim", resp.Msg.GetEngineName())
	assert.Equal(t, "v0.1.0", resp.Msg.GetEngineVersion())

	plugins := registry.List()
	require.Len(t, plugins, 1)
	assert.Equal(t, "test-plugin", plugins[0].Name)
	assert.Equal(t, secrets.MustParsePattern("**"), plugins[0].Pattern)
}

func TestRegistrationService_RegisterPlugin_invalid_pattern(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry}

	req := &resolverv1.RegisterPluginRequest{}
	req.SetName("bad-plugin")
	req.SetPattern("!!!invalid!!!")

	_, err := svc.RegisterPlugin(t.Context(), connect.NewRequest(req))
	assert.Error(t, err)
}

func TestRegistrationService_RegisterPlugin_duplicate(t *testing.T) {
	registry := NewRegistry()
	svc := &RegistrationService{Registry: registry}

	req := &resolverv1.RegisterPluginRequest{}
	req.SetName("dup")
	req.SetPattern("**")

	_, err := svc.RegisterPlugin(t.Context(), connect.NewRequest(req))
	require.NoError(t, err)

	_, err = svc.RegisterPlugin(t.Context(), connect.NewRequest(req))
	assert.NoError(t, err)

	plugins := registry.List()
	assert.Len(t, plugins, 1)
}
