package daemon

import (
	"context"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	resolverv1 "github.com/docker/secrets-engine/x/api/resolver/v1"
	"github.com/docker/secrets-engine/x/secrets"
)

func TestDaemonResolver_GetSecrets_no_plugins(t *testing.T) {
	registry := NewRegistry()
	svc := &DaemonResolver{Registry: registry}

	req := &resolverv1.GetSecretsRequest{}
	req.SetPattern("mysecret")

	_, err := svc.GetSecrets(t.Context(), connect.NewRequest(req))
	assert.Error(t, err)
}

func TestDaemonResolver_GetSecrets_plugin_not_connected(t *testing.T) {
	registry := NewRegistry()
	pattern := secrets.MustParsePattern("**")
	registry.Register("test-plugin", "v1.0.0", pattern, nil)

	svc := &DaemonResolver{Registry: registry}

	req := &resolverv1.GetSecretsRequest{}
	req.SetPattern("mysecret")

	_, err := svc.GetSecrets(t.Context(), connect.NewRequest(req))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no connection")
}

func TestDaemonResolver_GetSecrets_invalid_pattern(t *testing.T) {
	registry := NewRegistry()
	svc := &DaemonResolver{Registry: registry}

	req := &resolverv1.GetSecretsRequest{}
	req.SetPattern("!!!invalid!!!")

	_, err := svc.GetSecrets(t.Context(), connect.NewRequest(req))
	require.Error(t, err)
}

func TestDaemonResolver_GetSecrets_enforces_request_timeout(t *testing.T) {
	registry := NewRegistry()
	hanging := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			<-r.Context().Done()
			return nil, r.Context().Err()
		}),
	}
	registry.Register("hang-plugin", "v1.0.0", secrets.MustParsePattern("**"), hanging)

	svc := &DaemonResolver{Registry: registry, RequestTimeout: 50 * time.Millisecond}

	req := &resolverv1.GetSecretsRequest{}
	req.SetPattern("mysecret")

	start := time.Now()
	_, err := svc.GetSecrets(t.Context(), connect.NewRequest(req))
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second, "plugin call must be bounded by the request timeout")
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
