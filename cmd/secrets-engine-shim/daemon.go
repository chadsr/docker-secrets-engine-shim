package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/chadsr/docker-secrets-engine-shim/internal/daemon"
	"github.com/docker/secrets-engine/x/api"
	"github.com/docker/secrets-engine/x/ipc"
)

func runDaemon() {
	abstractSock := api.StandaloneSocketPath()
	fsSock := api.DesktopSocketPath()

	srv := daemon.NewServer(abstractSock, engineName, version, commit, date)
	logger := srv.Logger

	// Abstract socket: NRI plugin, SDK default. Filesystem socket: mcp-gateway, `docker pass run`
	if err := os.MkdirAll(filepath.Dir(fsSock), 0o700); err != nil {
		logger.Fatalf("creating socket directory %s: %v", filepath.Dir(fsSock), err)
	}
	os.Remove(fsSock)
	fsListener, err := net.Listen("unix", fsSock)
	if err != nil {
		logger.Fatalf("listen on %s: %v", fsSock, err)
	}
	if err := os.Chmod(fsSock, 0o600); err != nil {
		logger.Warnf("chmod %s: %v", fsSock, err)
	}
	go func() {
		err := http.Serve(daemon.NewPeerCredListener(fsListener), srv.Mux())
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			logger.WithError(err).Error("filesystem socket serve loop died")
		}
	}()

	if err := startPlugin(logger, srv); err != nil {
		logger.Fatalf("could not start docker-pass plugin: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("shutting down")
		fsListener.Close()
		os.Remove(fsSock)
		srv.Close()
	}()

	logger.Infof("listening on %s and %s", abstractSock, fsSock)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		os.Remove(fsSock)
		logger.Fatal(err)
	}
}

// startPlugin spawns this binary as the docker-pass plugin subprocess.
func startPlugin(logger *logrus.Entry, srv *daemon.Server) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	cmd := exec.Command(self)
	// argv[0] selects docker-pass mode in the multicall dispatch.
	cmd.Args[0] = cmdDockerPass

	localConn, fdWrapper, err := ipc.NewConnectionPair(cmd)
	if err != nil {
		return fmt.Errorf("creating connection pair: %w", err)
	}

	launchCfg := ipc.PluginConfigFromEngine{
		Name:                cmdDockerPass,
		RegistrationTimeout: 10 * time.Second,
		Custom:              fdWrapper.ToCustomCfg(),
	}
	cfgStr, err := launchCfg.ToString()
	if err != nil {
		return fmt.Errorf("marshaling plugin config: %w", err)
	}
	cmd.Env = append(os.Environ(), api.PluginLaunchedByEngineVar+"="+cfgStr)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting plugin subprocess: %w", err)
	}

	go func() {
		ref := &daemon.PluginClientRef{}
		closer, client, err := ipc.NewServerIPC(logger, localConn, daemon.TagPluginClient(srv.Mux(), ref), nil)
		if err != nil {
			logger.WithError(err).Error("plugin IPC setup error")
			return
		}
		ref.Set(client)

		cmd.Process.Wait()
		closer.Close()
		srv.Registry.Unregister(cmdDockerPass)
	}()

	logger.Infof("started docker-pass plugin (pid %d)", cmd.Process.Pid)
	return nil
}
