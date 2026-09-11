package daemon

import (
	"net/http"
	"testing"

	"github.com/docker/secrets-engine/x/secrets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	pattern := secrets.MustParsePattern("**")

	r.Register("test-plugin", pattern, &http.Client{})

	plugins := r.List()
	require.Len(t, plugins, 1)
	assert.Equal(t, "test-plugin", plugins[0].Name)
}

func TestRegistry_Register_duplicate_replaces_client(t *testing.T) {
	r := NewRegistry()
	pattern := secrets.MustParsePattern("**")

	r.Register("test-plugin", pattern, &http.Client{})

	newClient := &http.Client{}
	r.Register("test-plugin", pattern, newClient)

	plugins := r.List()
	require.Len(t, plugins, 1)
	assert.Equal(t, newClient, plugins[0].Client)
}

func TestRegistry_FindForPattern(t *testing.T) {
	r := NewRegistry()

	r.Register("my-plugin", secrets.MustParsePattern("docker/**"), &http.Client{})

	entry, ok := r.FindForPattern(secrets.MustParsePattern("docker/auth/token"))
	assert.True(t, ok)
	assert.Equal(t, "my-plugin", entry.Name)

	_, ok = r.FindForPattern(secrets.MustParsePattern("other/thing"))
	assert.False(t, ok)
}
