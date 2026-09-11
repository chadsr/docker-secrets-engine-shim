package credstore

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/docker-credential-helpers/client"

	pass "github.com/docker/secrets-engine/plugins/pass/store"
	"github.com/docker/secrets-engine/store"
	"github.com/docker/secrets-engine/x/secrets"
)

var fakeHelperPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "credstore-fakehelper")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	bin := filepath.Join(dir, "docker-credential-fake")
	out, err := exec.Command("go", "build", "-o", bin, "./testdata/fakehelper").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "building fake helper: %v\n%s", err, out)
		os.Exit(1)
	}
	fakeHelperPath = bin
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// newTestStore backs a store with the fake helper via absolute path; not parallel-safe (FAKE_HELPER_STATE is shared).
func newTestStore(t *testing.T) store.Store {
	t.Helper()
	stateFile := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(stateFile, []byte("{}"), 0o600))
	t.Setenv("FAKE_HELPER_STATE", stateFile)
	return New(client.NewShellProgramFunc(fakeHelperPath))
}

func mustID(t *testing.T, s string) store.ID {
	t.Helper()
	id, err := secrets.ParseID(s)
	require.NoError(t, err)
	return id
}

func mustValue(t *testing.T, s store.Secret) string {
	t.Helper()
	val, err := s.Marshal()
	require.NoError(t, err)
	return string(val)
}

func TestProgramFuncFromConfig(t *testing.T) {
	assert.Nil(t, programFuncFromConfig(nil))

	empty := &configfile.ConfigFile{}
	assert.Nil(t, programFuncFromConfig(empty))

	configured := &configfile.ConfigFile{}
	configured.CredentialsStore = "pass"
	assert.NotNil(t, programFuncFromConfig(configured))
}

func TestSaveAndGet(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id := mustID(t, "db/pass")
	require.NoError(t, s.Save(ctx, id, pass.NewPassValue([]byte("s3cr3t"))))

	got, err := s.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t", mustValue(t, got))
	assert.Equal(t, "db/pass", got.Metadata()["ServerURL"])
}

func TestUpsertOverwrites(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id := mustID(t, "db/pass")
	require.NoError(t, s.Save(ctx, id, pass.NewPassValue([]byte("old"))))
	require.NoError(t, s.Upsert(ctx, id, pass.NewPassValue([]byte("new"))))

	got, err := s.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "new", mustValue(t, got))
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	id := mustID(t, "db/pass")
	require.NoError(t, s.Save(ctx, id, pass.NewPassValue([]byte("s3cr3t"))))
	require.NoError(t, s.Delete(ctx, id))

	_, err := s.Get(ctx, id)
	assert.Error(t, err)
}

func TestFilter(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	for id, val := range map[string]string{
		"db/a":    "1",
		"db/b":    "2",
		"other/x": "3",
		"top":     "4",
	} {
		require.NoError(t, s.Save(ctx, mustID(t, id), pass.NewPassValue([]byte(val))))
	}

	result, err := s.Filter(ctx, secrets.MustParsePattern("db/*"))
	require.NoError(t, err)
	require.Len(t, result, 2)

	values := map[string]string{}
	for id, secret := range result {
		values[id.String()] = mustValue(t, secret)
	}
	assert.Equal(t, map[string]string{"db/a": "1", "db/b": "2"}, values)
}

func TestGetAllMetadata(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	for _, id := range []string{"db/a", "db/b", "other/x"} {
		require.NoError(t, s.Save(ctx, mustID(t, id), pass.NewPassValue([]byte("v-"+id))))
	}

	result, err := s.GetAllMetadata(ctx)
	require.NoError(t, err)
	require.Len(t, result, 3)

	for id, secret := range result {
		assert.Equal(t, id.String(), secret.Metadata()["ServerURL"])
	}
}

func TestSecretserviceKeyPrefix(t *testing.T) {
	ctx := context.Background()
	t.Setenv("FAKE_HELPER_PREFIX", "1")
	s := newTestStore(t)

	for _, id := range []string{"db/a", "db/b"} {
		require.NoError(t, s.Save(ctx, mustID(t, id), pass.NewPassValue([]byte("v-"+id))))
	}

	result, err := s.GetAllMetadata(ctx)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	filtered, err := s.Filter(ctx, secrets.MustParsePattern("db/*"))
	require.NoError(t, err)
	assert.Len(t, filtered, 2)
}

func TestUnconfiguredStore(t *testing.T) {
	ctx := context.Background()
	s := New(nil)

	id := mustID(t, "db/pass")
	assert.ErrorContains(t, s.Save(ctx, id, pass.NewPassValue([]byte("x"))), "no credential helper configured")
	_, err := s.Get(ctx, id)
	assert.ErrorContains(t, err, "no credential helper configured")
	err = s.Delete(ctx, id)
	assert.ErrorContains(t, err, "no credential helper configured")
	_, err = s.GetAllMetadata(ctx)
	assert.ErrorContains(t, err, "no credential helper configured")
	_, err = s.Filter(ctx, secrets.MustParsePattern("**"))
	assert.ErrorContains(t, err, "no credential helper configured")
}
