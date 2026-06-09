package daemon

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	resolverv1 "github.com/docker/secrets-engine/x/api/resolver/v1"
	resolverv1connect "github.com/docker/secrets-engine/x/api/resolver/v1/resolverv1connect"
	"github.com/docker/secrets-engine/x/secrets"
)

type DaemonResolver struct {
	Registry *Registry
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

	client := resolverv1connect.NewResolverServiceClient(entry.Client, "http://unix")
	return client.GetSecrets(ctx, req)
}
