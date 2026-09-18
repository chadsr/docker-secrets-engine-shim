package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/docker/secrets-engine/x/api"
	pluginsv1 "github.com/docker/secrets-engine/x/api/plugins/v1"
	"github.com/docker/secrets-engine/x/secrets"
)

const defaultRequestTimeout = 30 * time.Second

type RegistrationService struct {
	Registry   *Registry
	EngineName string
	Version    string
}

func (s *RegistrationService) RegisterPlugin(ctx context.Context, req *connect.Request[pluginsv1.RegisterPluginRequest]) (*connect.Response[pluginsv1.RegisterPluginResponse], error) {
	name := req.Msg.GetName()
	if _, err := api.NewName(name); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid plugin name %q: %w", name, err))
	}

	// The official client drops plugins whose version does not parse, so reject bad versions at the door.
	version := req.Msg.GetVersion()
	if _, err := api.NewVersion(version); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid plugin version %q: %w", version, err))
	}

	sp := req.Msg.GetSecretsProvider()
	if sp == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("only secrets-provider plugins are supported"))
	}

	pattern, err := secrets.ParsePattern(sp.GetPattern())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid pattern %q: %w", sp.GetPattern(), err))
	}

	// Bind the registration to the plugin connection it arrived on.
	client, err := pluginClientFromContext(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}

	s.Registry.Register(name, version, pattern, client)

	resp := &pluginsv1.RegisterPluginResponse{}
	resp.SetEngineName(s.EngineName)
	resp.SetEngineVersion(s.Version)
	resp.SetRequestTimeout(durationpb.New(defaultRequestTimeout))
	return connect.NewResponse(resp), nil
}

// pluginClientKey keys the per-connection plugin client in request contexts.
type pluginClientKey struct{}

func pluginClientFromContext(ctx context.Context) (*http.Client, error) {
	ref, ok := ctx.Value(pluginClientKey{}).(*PluginClientRef)
	if !ok {
		return nil, errors.New("plugin registration must arrive over a plugin IPC connection")
	}
	if c := ref.Get(); c != nil {
		return c, nil
	}
	return nil, errors.New("plugin IPC client is not ready")
}
