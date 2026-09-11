package daemon

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	pluginsv1 "github.com/docker/secrets-engine/x/api/plugins/v1"
	"github.com/docker/secrets-engine/x/secrets"
)

const defaultRequestTimeout = 30 * time.Second

type RegistrationService struct {
	Registry       *Registry
	EngineName     string
	Version        string
	pendingClients *sync.Mutex
	pending        map[io.ReadWriteCloser]*http.Client
}

func (s *RegistrationService) RegisterPlugin(_ context.Context, req *connect.Request[pluginsv1.RegisterPluginRequest]) (*connect.Response[pluginsv1.RegisterPluginResponse], error) {
	name := req.Msg.GetName()

	sp := req.Msg.GetSecretsProvider()
	if sp == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("only secrets-provider plugins are supported"))
	}

	pattern, err := secrets.ParsePattern(sp.GetPattern())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid pattern %q: %w", sp.GetPattern(), err))
	}

	var client *http.Client
	if s.pendingClients != nil && s.pending != nil {
		s.pendingClients.Lock()
		for conn, c := range s.pending {
			client = c
			delete(s.pending, conn)
			break
		}
		s.pendingClients.Unlock()
	}

	s.Registry.Register(name, pattern, client)

	resp := &pluginsv1.RegisterPluginResponse{}
	resp.SetEngineName(s.EngineName)
	resp.SetEngineVersion(s.Version)
	resp.SetRequestTimeout(durationpb.New(defaultRequestTimeout))
	return connect.NewResponse(resp), nil
}
