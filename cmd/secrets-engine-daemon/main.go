package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/cli/cli-plugins/manager"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/spf13/cobra"

	"github.com/docker/secrets-engine/x/api"
	"github.com/docker/secrets-engine/x/ipc"
	"github.com/docker/secrets-engine/x/logging"
	"github.com/user/secrets-engine-shim/internal/daemon"
)

var (
	version = "v0.1.0-dev"
	commit  = "none"
	date    = "2026-06-03"
)

func main() {
	socketPath := api.DaemonSocketPath()
	logger := logging.NewDefaultLogger("daemon")

	srv := daemon.NewServer(socketPath, version, commit, date)

	engineSock := api.DefaultSocketPath()
	os.Remove(engineSock)
	if err := os.Symlink(socketPath, engineSock); err != nil {
		log.Printf("warning: could not symlink %s -> %s: %v", engineSock, socketPath, err)
	} else {
		fmt.Fprintf(os.Stderr, "Symlinked %s -> %s\n", engineSock, socketPath)
	}

	pluginPath, err := resolvePluginPath()
	if err != nil {
		log.Fatalf("could not find docker-pass plugin: %v", err)
	}

	if err := startPlugin(logger, srv, pluginPath); err != nil {
		log.Fatalf("could not start docker-pass plugin: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutting down...")
		os.Remove(engineSock)
		srv.Close()
	}()

	fmt.Fprintf(os.Stderr, "Listening on %s\n", socketPath)
	if err := srv.ListenAndServe(); err != nil {
		os.Remove(engineSock)
		log.Fatal(err)
	}
}

func startPlugin(logger logging.Logger, srv *daemon.Server, pluginPath string) error {
	cmd := exec.Command(pluginPath)

	localConn, fdWrapper, err := ipc.NewConnectionPair(cmd)
	if err != nil {
		return fmt.Errorf("creating connection pair: %w", err)
	}

	launchCfg := ipc.PluginConfigFromEngine{
		Name:                "docker-pass",
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
	}()

	fmt.Fprintf(os.Stderr, "Started docker-pass plugin (pid %d)\n", cmd.Process.Pid)
	return nil
}

func resolvePluginPath() (string, error) {
	p, err := manager.GetPlugin("pass", nopConfig{}, &cobra.Command{})
	if err != nil {
		return "", fmt.Errorf("docker-pass plugin not found: %w", err)
	}
	return p.Path, nil
}

type nopConfig struct{}

func (nopConfig) ConfigFile() *configfile.ConfigFile { return nil }
