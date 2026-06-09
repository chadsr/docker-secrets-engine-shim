package daemon

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pluginsv1 "github.com/docker/secrets-engine/x/api/plugins/v1"
	"github.com/docker/secrets-engine/x/secrets"
)

func TestManagementService_ListPlugins(t *testing.T) {
	registry := NewRegistry()
	pattern := secrets.MustParsePattern("**")
	registry.Register("test-plugin", pattern, nil)

	svc := &ManagementService{Registry: registry}
	resp, err := svc.ListPlugins(t.Context(), connect.NewRequest(&pluginsv1.ListPluginsRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.GetPlugins(), 1)
	assert.Equal(t, "test-plugin", resp.Msg.GetPlugins()[0].GetName())
	assert.Equal(t, pluginsv1.RunStatus_RUN_STATUS_RUNNING, resp.Msg.GetPlugins()[0].GetRunStatus())
}

func TestManagementService_ListPlugins_empty(t *testing.T) {
	registry := NewRegistry()
	svc := &ManagementService{Registry: registry}

	resp, err := svc.ListPlugins(t.Context(), connect.NewRequest(&pluginsv1.ListPluginsRequest{}))
	require.NoError(t, err)
	assert.Empty(t, resp.Msg.GetPlugins())
}

func TestManagementService_EnablePlugin(t *testing.T) {
	registry := NewRegistry()
	svc := &ManagementService{Registry: registry}

	resp, err := svc.EnablePlugin(t.Context(), connect.NewRequest(&pluginsv1.EnablePluginRequest{}))
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestManagementService_DisablePlugin(t *testing.T) {
	registry := NewRegistry()
	svc := &ManagementService{Registry: registry}

	resp, err := svc.DisablePlugin(t.Context(), connect.NewRequest(&pluginsv1.DisablePluginRequest{}))
	require.NoError(t, err)
	assert.NotNil(t, resp)
}
