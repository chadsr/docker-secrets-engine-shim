package daemon

import (
	"context"

	"connectrpc.com/connect"

	pluginsv1 "github.com/docker/secrets-engine/x/api/plugins/v1"
)

type ManagementService struct {
	Registry *Registry
}

func (m *ManagementService) ListPlugins(_ context.Context, _ *connect.Request[pluginsv1.ListPluginsRequest]) (*connect.Response[pluginsv1.ListPluginsResponse], error) {
	entries := m.Registry.List()
	plugins := make([]*pluginsv1.Plugin, 0, len(entries))
	for _, e := range entries {
		p := &pluginsv1.Plugin{}
		p.SetName(e.Name)
		p.SetVersion(e.Version)
		p.SetExternal(true)
		p.SetRunStatus(pluginsv1.RunStatus_RUN_STATUS_RUNNING)
		sp := &pluginsv1.SecretsProvider{}
		sp.SetPattern(e.Pattern.String())
		p.SetSecretsProvider(sp)
		plugins = append(plugins, p)
	}
	resp := &pluginsv1.ListPluginsResponse{}
	resp.SetPlugins(plugins)
	return connect.NewResponse(resp), nil
}

func (m *ManagementService) EnablePlugin(_ context.Context, _ *connect.Request[pluginsv1.EnablePluginRequest]) (*connect.Response[pluginsv1.EnablePluginResponse], error) {
	return connect.NewResponse(&pluginsv1.EnablePluginResponse{}), nil
}

func (m *ManagementService) DisablePlugin(_ context.Context, _ *connect.Request[pluginsv1.DisablePluginRequest]) (*connect.Response[pluginsv1.DisablePluginResponse], error) {
	return connect.NewResponse(&pluginsv1.DisablePluginResponse{}), nil
}
