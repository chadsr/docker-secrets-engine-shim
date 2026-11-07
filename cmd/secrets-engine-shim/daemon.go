package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/chadsr/docker-secrets-engine-shim/internal/daemon"
	"github.com/docker/secrets-engine/x/api"
	"github.com/docker/secrets-engine/x/ipc"
)

func runDaemon() {
	root := newDaemonCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// newDaemonCommand handles --help/--version and rejects bad flags before any socket is touched.
func newDaemonCommand() *cobra.Command {
	root := &cobra.Command{
		Use:               "docker-secrets-engine-shim",
		Short:             "Run the secrets engine shim daemon.",
		SilenceUsage:      true,
		SilenceErrors:     true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		Args:              cobra.NoArgs,
		Version:           version,
	}
	root.SetVersionTemplate(fmt.Sprintf("docker-secrets-engine-shim %s (commit %s, built %s)\n", version, commit, date))
	root.Run = func(_ *cobra.Command, _ []string) { daemonMain() }
	return root
}

func daemonMain() {
	abstractSock := api.StandaloneSocketPath()
	fsSock := api.DesktopSocketPath()

	srv := daemon.NewServer(abstractSock, engineName, version, commit, date)
	logger := srv.Logger

	// The abstract socket is exclusive: bind it first so a second instance fails before touching anything.
	abstractListener, err := daemon.ListenUnix(abstractSock)
	if err != nil {
		logger.WithError(err).Fatal("could not bind abstract socket; is another instance already running?")
	}

	// Abstract socket: NRI plugin, SDK default. Filesystem socket: mcp-gateway, `docker pass run`.
	fsListener, err := daemon.ListenUnix(fsSock)
	if err != nil {
		abstractListener.Close()
		logger.Fatalf("listening on %s: %v", fsSock, err)
	}
	if err := os.Chmod(fsSock, 0o600); err != nil {
		logger.Warnf("chmod %s: %v", fsSock, err)
	}
	go func() {
		if err := srv.Serve(fsListener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
			logger.WithError(err).Error("filesystem socket serve loop died")
		}
	}()

	if err := startPlugin(logger, srv); err != nil {
		abstractListener.Close()
		fsListener.Close()
		os.Remove(fsSock)
		logger.Fatalf("could not start docker-pass plugin: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Info("shutting down")
		abstractListener.Close()
		fsListener.Close()
		os.Remove(fsSock)
		srv.Close()
	}()

	logger.Infof("listening on %s and %s", abstractSock, fsSock)
	if err := srv.Serve(abstractListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
