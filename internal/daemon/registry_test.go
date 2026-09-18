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

	r.Register("test-plugin", "v1.0.0", pattern, &http.Client{})

	plugins := r.List()
	require.Len(t, plugins, 1)
	assert.Equal(t, "test-plugin", plugins[0].Name)
	assert.Equal(t, "v1.0.0", plugins[0].Version)
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	r.Register("test-plugin", "v1.0.0", secrets.MustParsePattern("**"), &http.Client{})

	r.Unregister("test-plugin")
	assert.Empty(t, r.List())

	r.Unregister("absent")
	assert.Empty(t, r.List())
}

func TestRegistry_Register_duplicate_replaces_client(t *testing.T) {
	r := NewRegistry()
	pattern := secrets.MustParsePattern("**")

	r.Register("test-plugin", "v1.0.0", pattern, &http.Client{})

	newClient := &http.Client{}
	r.Register("test-plugin", "v1.1.0", pattern, newClient)

	plugins := r.List()
	require.Len(t, plugins, 1)
	assert.Equal(t, newClient, plugins[0].Client)
	assert.Equal(t, "v1.1.0", plugins[0].Version)
}

func TestRegistry_FindForPattern(t *testing.T) {
	r := NewRegistry()

	r.Register("my-plugin", "v1.0.0", secrets.MustParsePattern("docker/**"), &http.Client{})

	entry, ok := r.FindForPattern(secrets.MustParsePattern("docker/auth/token"))
	assert.True(t, ok)
	assert.Equal(t, "my-plugin", entry.Name)

	_, ok = r.FindForPattern(secrets.MustParsePattern("other/thing"))
	assert.False(t, ok)
}
