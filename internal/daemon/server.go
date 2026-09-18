package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	healthv1connect "github.com/docker/secrets-engine/x/api/health/v1/healthv1connect"
	pluginsv1connect "github.com/docker/secrets-engine/x/api/plugins/v1/pluginsv1connect"
	resolverv1connect "github.com/docker/secrets-engine/x/api/resolver/v1/resolverv1connect"
	"github.com/docker/secrets-engine/x/ipc"
	"github.com/docker/secrets-engine/x/logging"
)

type Server struct {
	socketPath string
	Registry   *Registry
	version    string
	commitHash string
	date       string
	server     *http.Server
	pendingMu  sync.Mutex
	pending    map[io.ReadWriteCloser]*http.Client
}

func NewServer(socketPath, engineName, version, commitHash, date string) *Server {
	s := &Server{
		socketPath: socketPath,
		Registry:   NewRegistry(),
		version:    version,
		commitHash: commitHash,
		date:       date,
		pending:    make(map[io.ReadWriteCloser]*http.Client),
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle(healthv1connect.NewVersionServiceHandler(
		&VersionService{Version: version, CommitHash: commitHash, Date: date},
	))

	mux.Handle(pluginsv1connect.NewRegisterServiceHandler(
		&RegistrationService{Registry: s.Registry, EngineName: engineName, Version: version, pendingClients: &s.pendingMu, pending: s.pending},
	))

	mux.Handle(resolverv1connect.NewResolverServiceHandler(
		&DaemonResolver{Registry: s.Registry},
	))

	mux.Handle(pluginsv1connect.NewPluginManagementServiceHandler(
		&ManagementService{Registry: s.Registry},
	))

	logger := logging.NewDefaultLogger("daemon")

	hijackPath, hijackHandler := ipc.NewHijackAcceptor(logger, func(ctx context.Context, conn io.ReadWriteCloser) {
		closer, client, err := ipc.NewServerIPC(logger, conn, mux, nil)
		if err != nil {
			log.Printf("hijack IPC setup error: %v", err)
			return
		}
		s.pendingMu.Lock()
		s.pending[conn] = client
		s.pendingMu.Unlock()

		<-ctx.Done()
		closer.Close()
		s.pendingMu.Lock()
		delete(s.pending, conn)
		s.pendingMu.Unlock()
	})
	mux.Handle(hijackPath, hijackHandler)

	s.server = &http.Server{Handler: mux}
	return s
}

func (s *Server) ListenAndServe() error {
	// Skip filesystem socket path creation for abstract sockets
	if !strings.HasPrefix(s.socketPath, "@") {
		if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o700); err != nil {
			return fmt.Errorf("creating socket directory: %w", err)
		}
		if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing stale socket: %w", err)
		}
	}

	l, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.socketPath, err)
	}

	return s.server.Serve(NewPeerCredListener(l))
}

func (s *Server) Close() error {
	return s.server.Close()
}

func (s *Server) Mux() http.Handler {
	return s.server.Handler
}

func (s *Server) RegisterPluginClient(conn io.ReadWriteCloser, client *http.Client) {
	s.pendingMu.Lock()
	s.pending[conn] = client
	s.pendingMu.Unlock()
}

// RemovePluginClient drops a pending client added by RegisterPluginClient.
func (s *Server) RemovePluginClient(conn io.ReadWriteCloser) {
	s.pendingMu.Lock()
	delete(s.pending, conn)
	s.pendingMu.Unlock()
}
