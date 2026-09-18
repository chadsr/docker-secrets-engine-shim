package daemon

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	resolverv1 "github.com/docker/secrets-engine/x/api/resolver/v1"
	resolverv1connect "github.com/docker/secrets-engine/x/api/resolver/v1/resolverv1connect"
	"github.com/docker/secrets-engine/x/secrets"
)

type DaemonResolver struct {
	Registry *Registry
	// RequestTimeout bounds each daemon-to-plugin call; zero means defaultRequestTimeout.
	RequestTimeout time.Duration
}

func (d *DaemonResolver) GetSecrets(ctx context.Context, req *connect.Request[resolverv1.GetSecretsRequest]) (*connect.Response[resolverv1.GetSecretsResponse], error) {
	patternStr := req.Msg.GetPattern()
	pattern, err := secrets.ParsePattern(patternStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid pattern %q: %w", patternStr, err))
	}

	entry, ok := d.Registry.FindForPattern(pattern)
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no plugin registered for pattern %q", patternStr))
	}

	if entry.Client == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("plugin %q has no connection", entry.Name))
	}

	// Bound the plugin call so a stuck plugin can't hang resolution
	ctx, cancel := context.WithTimeout(ctx, d.requestTimeout())
	defer cancel()

	client := resolverv1connect.NewResolverServiceClient(entry.Client, "http://unix")
	return client.GetSecrets(ctx, req)
}

func (d *DaemonResolver) requestTimeout() time.Duration {
	if d.RequestTimeout > 0 {
		return d.RequestTimeout
	}
	return defaultRequestTimeout
}
