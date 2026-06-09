package daemon

import (
	"testing"

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
	registry.Register("test-plugin", pattern, nil)

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
