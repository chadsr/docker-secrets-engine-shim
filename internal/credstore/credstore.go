package credstore

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/cli/cli/config"
	"github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"

	pass "github.com/docker/secrets-engine/plugins/pass/store"
	"github.com/docker/secrets-engine/store"
	"github.com/docker/secrets-engine/x/secrets"
)

type credStore struct {
	mu          sync.Mutex
	programFunc client.ProgramFunc
}

var _ store.Store = &credStore{}

func New(programFunc client.ProgramFunc) store.Store {
	return &credStore{programFunc: programFunc}
}

func NewFromConfig() store.Store {
	return New(resolveProgramFunc())
}

func resolveProgramFunc() client.ProgramFunc {
	cfg, err := config.Load("")
	if err != nil {
		return nil
	}
	suffix := cfg.CredentialsStore
	if suffix == "" {
		return nil
	}
	return client.NewShellProgramFunc("docker-credential-" + suffix)
}

func idToServerURL(id store.ID) string {
	return id.String()
}

func (s *credStore) Save(_ context.Context, id store.ID, secret store.Secret) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, err := secret.Marshal()
	if err != nil {
		return fmt.Errorf("marshal secret: %w", err)
	}

	creds := &credentials.Credentials{
		ServerURL: idToServerURL(id),
		Username:  id.String(),
		Secret:    string(val),
	}
	return client.Store(s.programFunc, creds)
}

func (s *credStore) Upsert(ctx context.Context, id store.ID, secret store.Secret) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = client.Erase(s.programFunc, idToServerURL(id))

	val, err := secret.Marshal()
	if err != nil {
		return fmt.Errorf("marshal secret: %w", err)
	}

	creds := &credentials.Credentials{
		ServerURL: idToServerURL(id),
		Username:  id.String(),
		Secret:    string(val),
	}
	return client.Store(s.programFunc, creds)
}

func (s *credStore) Get(_ context.Context, id store.ID) (store.Secret, error) {
	cred, err := client.Get(s.programFunc, idToServerURL(id))
	if err != nil {
		return nil, fmt.Errorf("get secret: %w", err)
	}

	pv := pass.NewPassValue([]byte(cred.Secret))
	if md := secretMetadata(cred); md != nil {
		_ = pv.SetMetadata(md)
	}
	return pv, nil
}

func (s *credStore) Delete(_ context.Context, id store.ID) error {
	return client.Erase(s.programFunc, idToServerURL(id))
}

func (s *credStore) GetAllMetadata(_ context.Context) (map[store.ID]store.Secret, error) {
	list, err := client.List(s.programFunc)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}

	result := make(map[store.ID]store.Secret, len(list))
	for serverURL, username := range list {
		id, err := secrets.ParseID(serverURL)
		if err != nil {
			continue
		}
		pv := pass.NewPassValue(nil)
		_ = pv.SetMetadata(map[string]string{"ServerURL": serverURL, "Username": username})
		result[id] = pv
	}
	return result, nil
}

func (s *credStore) Filter(_ context.Context, pattern store.Pattern) (map[store.ID]store.Secret, error) {
	list, err := client.List(s.programFunc)
	if err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}

	result := make(map[store.ID]store.Secret)
	for _, serverURL := range list {
		id, err := secrets.ParseID(serverURL)
		if err != nil {
			continue
		}
		if !pattern.Match(id) {
			continue
		}
		cred, err := client.Get(s.programFunc, serverURL)
		if err != nil {
			continue
		}
		pv := pass.NewPassValue([]byte(cred.Secret))
		if md := secretMetadata(cred); md != nil {
			_ = pv.SetMetadata(md)
		}
		result[id] = pv
	}
	return result, nil
}

func secretMetadata(cred *credentials.Credentials) map[string]string {
	md := map[string]string{
		"ServerURL": cred.ServerURL,
		"Username":  cred.Username,
	}
	return md
}
