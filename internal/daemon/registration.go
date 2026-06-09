package daemon

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"connectrpc.com/connect"

	resolverv1 "github.com/docker/secrets-engine/x/api/resolver/v1"
	"github.com/docker/secrets-engine/x/secrets"
)

type RegistrationService struct {
	Registry       *Registry
	pendingClients *sync.Mutex
	pending        map[io.ReadWriteCloser]*http.Client
}

func (s *RegistrationService) RegisterPlugin(_ context.Context, req *connect.Request[resolverv1.RegisterPluginRequest]) (*connect.Response[resolverv1.RegisterPluginResponse], error) {
	name := req.Msg.GetName()
	patternStr := req.Msg.GetPattern()

	pattern, err := secrets.ParsePattern(patternStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid pattern %q: %w", patternStr, err))
	}

	var client *http.Client
	if s.pendingClients != nil && s.pending != nil {
		s.pendingClients.Lock()
		for _, c := range s.pending {
			client = c
			break
		}
		s.pendingClients.Unlock()
	}

	if err := s.Registry.Register(name, pattern, client); err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, err)
	}

	resp := &resolverv1.RegisterPluginResponse{}
	resp.SetEngineName("secrets-engine-shim")
	resp.SetEngineVersion("v0.1.0")
	return connect.NewResponse(resp), nil
}
