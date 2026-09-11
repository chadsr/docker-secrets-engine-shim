package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/chadsr/docker-secrets-engine-shim/internal/daemon"
	"github.com/docker/secrets-engine/x/api"
	"github.com/docker/secrets-engine/x/ipc"
	"github.com/docker/secrets-engine/x/logging"
)

func runDaemon() {
	abstractSock := api.DaemonSocketPath()
	fsSock := api.DefaultSocketPath()
	logger := logging.NewDefaultLogger("daemon")

	srv := daemon.NewServer(abstractSock, engineName, version, commit, date)

	// The abstract socket is used by the NRI plugin and the SDK default,
	// the filesystem socket by mcp-gateway and `docker pass run`. A
	// symlink can't point to an abstract socket, so we serve both.
	os.MkdirAll(filepath.Dir(fsSock), 0o700)
	os.Remove(fsSock)
	fsListener, err := net.Listen("unix", fsSock)
	if err != nil {
		log.Fatalf("listen on %s: %v", fsSock, err)
	}
	if err := os.Chmod(fsSock, 0o600); err != nil {
		log.Printf("chmod %s: %v", fsSock, err)
	}
	go func() {
		if err := http.Serve(daemon.NewPeerCredListener(fsListener), srv.Mux()); err != nil {
			log.Printf("filesystem socket: %v", err)
		}
	}()

	if err := startPlugin(logger, srv); err != nil {
		log.Fatalf("could not start docker-pass plugin: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		fsListener.Close()
		os.Remove(fsSock)
		srv.Close()
	}()

	fmt.Fprintf(os.Stderr, "Listening on %s and %s\n", abstractSock, fsSock)
	if err := srv.ListenAndServe(); err != nil {
		os.Remove(fsSock)
		log.Fatal(err)
	}
}

// startPlugin spawns this binary as the docker-pass plugin subprocess.
func startPlugin(logger logging.Logger, srv *daemon.Server) error {
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
		closer, client, err := ipc.NewServerIPC(logger, localConn, srv.Mux(), nil)
		if err != nil {
			log.Printf("plugin IPC setup error: %v", err)
			return
		}
		srv.RegisterPluginClient(localConn, client)

		cmd.Process.Wait()
		closer.Close()
		srv.RemovePluginClient(localConn)
	}()

	fmt.Fprintf(os.Stderr, "Started docker-pass plugin (pid %d)\n", cmd.Process.Pid)
	return nil
}
