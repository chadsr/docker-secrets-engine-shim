package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	"github.com/sirupsen/logrus"

	seclient "github.com/docker/secrets-engine/client"
	"github.com/docker/secrets-engine/x/secrets"
)

const (
	sePrefix    = "se://"
	nriConfFile = "/etc/docker/nri/conf.d/" + cmdNRIPlugin + ".conf"
)

type nriPlugin struct {
	stub   stub.Stub
	client seclient.Client
}

func (p *nriPlugin) Configure(_ context.Context, cfg, runtime, version string) (stub.EventMask, error) {
	logrus.Infof("connected to %s/%s", runtime, version)

	uid, err := resolveUID(cfg, nriConfFile)
	if err != nil {
		return 0, fmt.Errorf("resolving daemon UID: %w", err)
	}

	socketPath := daemonSocketPath(uid)
	p.client, err = seclient.New(seclient.WithSocketPath(socketPath))
	if err != nil {
		return 0, fmt.Errorf("creating secrets engine client: %w", err)
	}

	logrus.Infof("secret resolution via daemon socket %s", socketPath)
	return 0, nil
}

func (p *nriPlugin) CreateContainer(ctx context.Context, _ *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	adjustment := &api.ContainerAdjustment{}

	if p.client == nil {
		return adjustment, nil, nil
	}

	for _, env := range ctr.GetEnv() {
		key, val, ok := strings.Cut(env, "=")
		if !ok {
			continue
		}
		name, found := strings.CutPrefix(val, sePrefix)
		if !found {
			continue
		}

		resolved, err := p.resolve(ctx, name)
		if err != nil {
			return nil, nil, fmt.Errorf("container %s: %w", ctr.GetName(), err)
		}

		adjustment.RemoveEnv(key)
		adjustment.AddEnv(key, resolved)
	}

	if len(adjustment.Env) > 0 {
		logrus.Infof("container %s: resolved %d secret(s)", ctr.GetName(), len(adjustment.Env)/2)
	}
	return adjustment, nil, nil
}

func (p *nriPlugin) resolve(ctx context.Context, name string) (string, error) {
	pattern, err := seclient.ParsePattern(name)
	if err != nil {
		return "", fmt.Errorf("invalid se://%s: %w", name, err)
	}

	envelopes, err := p.client.GetSecrets(ctx, pattern)
	if err != nil {
		return "", fmt.Errorf("resolving se://%s: %w", name, err)
	}

	if len(envelopes) == 0 {
		return "", fmt.Errorf("se://%s: %w", name, secrets.ErrNotFound)
	}
	if len(envelopes) > 1 {
		return "", fmt.Errorf("se://%s: matched %d secrets", name, len(envelopes))
	}
	return string(envelopes[0].Value), nil
}

func (p *nriPlugin) onClose() {
	logrus.Info("connection to runtime lost, exiting")
	os.Exit(1)
}

func daemonSocketPath(uid int) string {
	return fmt.Sprintf("@docker-secrets-engine/%d/daemon.sock", uid)
}

func resolveUID(cfgStr, confFile string) (int, error) {
	if uid := uidFromConfigString(cfgStr); uid > 0 {
		return uid, nil
	}
	data, err := os.ReadFile(confFile)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", confFile, err)
	}
	uid := uidFromConfigString(string(data))
	if uid <= 0 {
		return 0, fmt.Errorf("no valid uid in %s", confFile)
	}
	return uid, nil
}

func uidFromConfigString(s string) int {
	for _, line := range strings.Split(s, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(k) != "uid" {
			continue
		}
		uid, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil && uid > 0 {
			return uid
		}
	}
	return 0
}

func runNRIPlugin() {
	logrus.Infof("%s NRI plugin %s (%s)", engineName, version, commit)

	var (
		name   string
		idx    string
		socket string
	)
	logrus.SetFormatter(&logrus.TextFormatter{PadLevelText: true})

	flag.StringVar(&name, "name", "", "NRI plugin name")
	flag.StringVar(&idx, "idx", "", "NRI plugin index")
	flag.StringVar(&socket, "socket", "", "NRI plugin socket path")
	flag.Parse()

	p := &nriPlugin{}
	opts := []stub.Option{
		stub.WithOnClose(p.onClose),
	}
	if name != "" {
		opts = append(opts, stub.WithPluginName(name))
	}
	if idx != "" {
		opts = append(opts, stub.WithPluginIdx(idx))
	}
	if socket != "" {
		opts = append(opts, stub.WithSocketPath(socket))
	}

	var err error
	if p.stub, err = stub.New(p, opts...); err != nil {
		logrus.Fatalf("failed to create NRI stub: %v", err)
	}

	if err = p.stub.Run(context.Background()); err != nil {
		logrus.Errorf("plugin exited: %v", err)
		os.Exit(1)
	}
}
