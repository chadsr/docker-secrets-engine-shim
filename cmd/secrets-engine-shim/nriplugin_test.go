package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/containerd/nri/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	seclient "github.com/docker/secrets-engine/client"
	"github.com/docker/secrets-engine/x/secrets"
)

type fakeClient struct {
	seclient.Client
	getSecrets func(ctx context.Context, pattern secrets.Pattern) ([]secrets.Envelope, error)
}

func (f *fakeClient) GetSecrets(ctx context.Context, pattern secrets.Pattern) ([]secrets.Envelope, error) {
	return f.getSecrets(ctx, pattern)
}

func staticClient(envelopes []secrets.Envelope, err error) *fakeClient {
	return &fakeClient{getSecrets: func(context.Context, secrets.Pattern) ([]secrets.Envelope, error) {
		return envelopes, err
	}}
}

func resolvingClient(values map[string]string) *fakeClient {
	return &fakeClient{getSecrets: func(_ context.Context, pattern secrets.Pattern) ([]secrets.Envelope, error) {
		v, ok := values[pattern.String()]
		if !ok {
			return nil, errors.New("not found")
		}
		return []secrets.Envelope{{Value: []byte(v)}}, nil
	}}
}

func TestDaemonSocketPath(t *testing.T) {
	assert.Equal(t, "@docker-secrets-engine/1000/daemon.sock", daemonSocketPath(1000))
}

func TestUIDFromConfigString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"valid", "uid: 1000\n", 1000},
		{"no space", "uid:1000", 1000},
		{"extra whitespace", "  uid  :  1000  \n", 1000},
		{"among other lines", "# comment\nuid: 4242\nother: x\n", 4242},
		{"first match wins", "uid: 1\nuid: 2\n", 1},
		{"missing key", "user: 1000\n", 0},
		{"invalid number", "uid: abc\n", 0},
		{"zero", "uid: 0\n", 0},
		{"negative", "uid: -1\n", 0},
		{"no colon", "uid 1000\n", 0},
		{"empty", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, uidFromConfigString(tt.in))
		})
	}
}

func TestResolveUID(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "10-secrets-engine.conf")
	require.NoError(t, os.WriteFile(conf, []byte("uid: 4242\n"), 0o644))

	t.Run("cfg takes precedence over file", func(t *testing.T) {
		uid, err := resolveUID("uid: 1000", conf)
		require.NoError(t, err)
		assert.Equal(t, 1000, uid)
	})

	t.Run("falls back to file", func(t *testing.T) {
		uid, err := resolveUID("", conf)
		require.NoError(t, err)
		assert.Equal(t, 4242, uid)
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := resolveUID("", filepath.Join(t.TempDir(), "absent.conf"))
		assert.Error(t, err)
	})

	t.Run("no valid uid in file", func(t *testing.T) {
		bad := filepath.Join(t.TempDir(), "bad.conf")
		require.NoError(t, os.WriteFile(bad, []byte("uid: abc\n"), 0o644))
		_, err := resolveUID("", bad)
		assert.ErrorContains(t, err, "no valid uid")
	})
}

func TestResolve(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		p := &nriPlugin{client: staticClient([]secrets.Envelope{{Value: []byte("s3cr3t")}}, nil)}
		v, err := p.resolve(t.Context(), "db/pass")
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", v)
	})

	t.Run("no match", func(t *testing.T) {
		p := &nriPlugin{client: staticClient(nil, nil)}
		_, err := p.resolve(t.Context(), "db/pass")
		assert.ErrorContains(t, err, "not found")
	})

	t.Run("ambiguous match", func(t *testing.T) {
		p := &nriPlugin{client: staticClient([]secrets.Envelope{{Value: []byte("a")}, {Value: []byte("b")}}, nil)}
		_, err := p.resolve(t.Context(), "db/*")
		assert.ErrorContains(t, err, "matched 2 secrets")
	})

	t.Run("client error", func(t *testing.T) {
		p := &nriPlugin{client: staticClient(nil, errors.New("daemon unreachable"))}
		_, err := p.resolve(t.Context(), "db/pass")
		assert.ErrorContains(t, err, "daemon unreachable")
	})

	t.Run("invalid pattern", func(t *testing.T) {
		p := &nriPlugin{client: staticClient(nil, nil)}
		_, err := p.resolve(t.Context(), "a=b")
		assert.ErrorContains(t, err, "invalid se://a=b")
	})
}

func TestCreateContainer(t *testing.T) {
	create := func(t *testing.T, p *nriPlugin, env []string) *api.ContainerAdjustment {
		t.Helper()
		adj, updates, err := p.CreateContainer(t.Context(), nil, &api.Container{Name: "test", Env: env})
		require.NoError(t, err)
		assert.Empty(t, updates)
		return adj
	}

	adjustments := func(t *testing.T, adj *api.ContainerAdjustment) (added map[string]string, removed map[string]bool) {
		t.Helper()
		added, removed = map[string]string{}, map[string]bool{}
		for _, kv := range adj.Env {
			if key, marked := api.IsMarkedForRemoval(kv.Key); marked {
				removed[key] = true
			} else {
				added[kv.Key] = kv.Value
			}
		}
		return added, removed
	}

	t.Run("resolves se:// env", func(t *testing.T) {
		p := &nriPlugin{client: resolvingClient(map[string]string{"db/pass": "s3cr3t"})}
		adj := create(t, p, []string{"TOKEN=se://db/pass"})
		added, removed := adjustments(t, adj)
		assert.Equal(t, map[string]string{"TOKEN": "s3cr3t"}, added)
		assert.Contains(t, removed, "TOKEN")
	})

	t.Run("resolves multiple and leaves plain env alone", func(t *testing.T) {
		p := &nriPlugin{client: resolvingClient(map[string]string{"a": "1", "b": "2"})}
		adj := create(t, p, []string{"A=se://a", "B=se://b", "C=plain"})
		added, _ := adjustments(t, adj)
		assert.Equal(t, map[string]string{"A": "1", "B": "2"}, added)
	})

	t.Run("non-prefixed se:// ignored", func(t *testing.T) {
		p := &nriPlugin{client: resolvingClient(nil)}
		adj := create(t, p, []string{"X=pre se://a"})
		assert.Empty(t, adj.Env)
	})

	t.Run("entry without equals ignored", func(t *testing.T) {
		p := &nriPlugin{client: resolvingClient(nil)}
		adj := create(t, p, []string{"INVALID"})
		assert.Empty(t, adj.Env)
	})

	t.Run("value with extra equals not a secret", func(t *testing.T) {
		p := &nriPlugin{client: resolvingClient(nil)}
		adj := create(t, p, []string{"K=a=b"})
		assert.Empty(t, adj.Env)
	})

	t.Run("nil client skips resolution", func(t *testing.T) {
		p := &nriPlugin{}
		adj := create(t, p, []string{"TOKEN=se://db/pass"})
		assert.Empty(t, adj.Env)
	})

	t.Run("resolution error keeps literal env", func(t *testing.T) {
		p := &nriPlugin{client: resolvingClient(nil)}
		adj := create(t, p, []string{"TOKEN=se://missing"})
		assert.Empty(t, adj.Env)
	})
}
