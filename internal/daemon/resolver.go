package daemon

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/sirupsen/logrus"

	resolverv1 "github.com/docker/secrets-engine/x/api/resolver/v1"
	resolverv1connect "github.com/docker/secrets-engine/x/api/resolver/v1/resolverv1connect"
	"github.com/docker/secrets-engine/x/secrets"
)

type DaemonResolver struct {
	Registry *Registry
	Logger   *logrus.Entry
	// RequestTimeout bounds each daemon-to-plugin call; zero means defaultRequestTimeout.
	RequestTimeout time.Duration
}

func (d *DaemonResolver) GetSecrets(ctx context.Context, req *connect.Request[resolverv1.GetSecretsRequest]) (*connect.Response[resolverv1.GetSecretsResponse], error) {
	patternStr := req.Msg.GetPattern()
	resp, err := d.getSecrets(ctx, req, patternStr)
	d.logResolve(patternStr, err)
	return resp, err
}

func (d *DaemonResolver) getSecrets(ctx context.Context, req *connect.Request[resolverv1.GetSecretsRequest], patternStr string) (*connect.Response[resolverv1.GetSecretsResponse], error) {
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

func (d *DaemonResolver) logResolve(pattern string, err error) {
	entry := d.Logger.WithField("pattern", pattern)
	if err != nil {
		entry = entry.WithField("code", connect.CodeOf(err).String()).WithError(err)
		switch connect.CodeOf(err) {
		case connect.CodeNotFound, connect.CodeInvalidArgument, connect.CodePermissionDenied:
			entry.Info("resolve failed")
		default:
			entry.Warn("resolve failed")
		}
		return
	}
	entry.Info("resolve")
}

func (d *DaemonResolver) requestTimeout() time.Duration {
	if d.RequestTimeout > 0 {
		return d.RequestTimeout
	}
	return defaultRequestTimeout
}

// allowAllAuthorizer allows everything: the shim has no identity provider to consult.
type allowAllAuthorizer struct {
	logger *logrus.Entry
}

func (a allowAllAuthorizer) Authorize(_ context.Context, patterns ...secrets.Pattern) (secrets.AuthorizeResponse, error) {
	names := make([]string, 0, len(patterns))
	for _, p := range patterns {
		names = append(names, p.String())
	}
	a.logger.WithField("patterns", names).Info("authorize: allow")
	// Zero Expiry means the decision never expires.
	return secrets.AuthorizeResponse{Allow: true}, nil
}
