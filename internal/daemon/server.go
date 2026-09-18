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
}

func NewServer(socketPath, engineName, version, commitHash, date string) *Server {
	s := &Server{
		socketPath: socketPath,
		Registry:   NewRegistry(),
		version:    version,
		commitHash: commitHash,
		date:       date,
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
		&RegistrationService{Registry: s.Registry, EngineName: engineName, Version: version},
	))

	mux.Handle(resolverv1connect.NewResolverServiceHandler(
		&DaemonResolver{Registry: s.Registry},
	))

	mux.Handle(pluginsv1connect.NewPluginManagementServiceHandler(
		&ManagementService{Registry: s.Registry},
	))

	logger := logging.NewDefaultLogger("daemon")

	hijackPath, hijackHandler := ipc.NewHijackAcceptor(logger, func(ctx context.Context, conn io.ReadWriteCloser) {
		ref := &PluginClientRef{}
		closer, client, err := ipc.NewServerIPC(logger, conn, TagPluginClient(mux, ref), nil)
		if err != nil {
			log.Printf("hijack IPC setup error: %v", err)
			return
		}
		ref.Set(client)

		<-ctx.Done()
		closer.Close()
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

// PluginClientRef is the http.Client reaching a plugin over its IPC connection.
type PluginClientRef struct {
	mu sync.RWMutex
	c  *http.Client
}

func (r *PluginClientRef) Set(c *http.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.c = c
}

func (r *PluginClientRef) Get() *http.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.c
}

// TagPluginClient returns a handler that tags every request with ref.
func TagPluginClient(h http.Handler, ref *PluginClientRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), pluginClientKey{}, ref)))
	})
}
