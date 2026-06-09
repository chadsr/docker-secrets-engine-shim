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

	err := r.Register("test-plugin", pattern, &http.Client{})
	require.NoError(t, err)

	plugins := r.List()
	assert.Len(t, plugins, 1)
	assert.Equal(t, "test-plugin", plugins[0].Name)
}

func TestRegistry_Register_duplicate(t *testing.T) {
	r := NewRegistry()
	pattern := secrets.MustParsePattern("**")

	err := r.Register("test-plugin", pattern, &http.Client{})
	require.NoError(t, err)

	newClient := &http.Client{}
	err = r.Register("test-plugin", pattern, newClient)
	assert.NoError(t, err)

	entry, ok := r.Lookup("anything")
	require.True(t, ok)
	assert.Equal(t, newClient, entry.Client)
}

func TestRegistry_Lookup(t *testing.T) {
	r := NewRegistry()
	pattern := secrets.MustParsePattern("docker/**")

	err := r.Register("my-plugin", pattern, &http.Client{})
	require.NoError(t, err)

	entry, ok := r.Lookup("docker/auth/token")
	assert.True(t, ok)
	assert.Equal(t, "my-plugin", entry.Name)

	_, ok = r.Lookup("other/thing")
	assert.False(t, ok)
}

func TestRegistry_Lookup_wildcard(t *testing.T) {
	r := NewRegistry()
	pattern := secrets.MustParsePattern("**")

	err := r.Register("catch-all", pattern, &http.Client{})
	require.NoError(t, err)

	entry, ok := r.Lookup("anything/at/all")
	assert.True(t, ok)
	assert.Equal(t, "catch-all", entry.Name)
}
