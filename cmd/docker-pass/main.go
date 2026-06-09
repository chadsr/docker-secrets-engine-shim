package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/docker/secrets-engine/plugin"
	passplugin "github.com/docker/secrets-engine/plugins/pass"
	"github.com/docker/secrets-engine/plugins/pass/commands"
	"github.com/docker/secrets-engine/x/api"
	"github.com/docker/secrets-engine/x/logging"
	"github.com/docker/secrets-engine/x/secrets"
	"github.com/user/secrets-engine-shim/internal/credstore"
)

var (
	version = "v0.0.0-dev"
	commit  = "n/a"
)

type pluginMetadata struct {
	SchemaVersion   string `json:"SchemaVersion"`
	Vendor          string `json:"Vendor"`
	Version         string `json:"Version"`
	ShortDescription string `json:"ShortDescription"`
}

func main() {
	args := os.Args[1:]
	if len(args) >= 1 && args[0] == "docker-cli-plugin-metadata" {
		json.NewEncoder(os.Stdout).Encode(pluginMetadata{
			SchemaVersion:    "0.1.0",
			Vendor:           "Docker Inc.",
			Version:          version,
			ShortDescription: "Manage your local secrets",
		})
		return
	}

	if len(args) >= 1 && (args[0] == "pass" || args[0] == "docker-pass") {
		args = args[1:]
	}

	logger := logging.NewDefaultLogger("docker-pass")

	if os.Getenv(api.PluginLaunchedByEngineVar) != "" {
		runPluginMode(logger)
		return
	}

	s := credstore.NewFromConfig()
	ctx := context.Background()
	root := passplugin.Root(ctx, s, commands.VersionInfo{Version: version, Commit: commit})
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runPluginMode(logger logging.Logger) {
	s := credstore.NewFromConfig()
	pp, err := passplugin.NewPassPlugin(logger, s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create pass plugin: %v\n", err)
		os.Exit(1)
	}

	stub, err := plugin.New(
		pp,
		plugin.Config{
			Version: api.MustNewVersion(version),
			Pattern: secrets.MustParsePattern("**"),
			Logger:  logger,
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create plugin: %v\n", err)
		os.Exit(1)
	}

	if err := stub.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "plugin exited: %v\n", err)
		os.Exit(1)
	}
}


