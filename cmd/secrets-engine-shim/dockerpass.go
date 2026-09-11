package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/docker/secrets-engine/client"
	"github.com/docker/secrets-engine/plugin"
	passplugin "github.com/docker/secrets-engine/plugins/pass"
	"github.com/docker/secrets-engine/plugins/pass/commands"
	"github.com/docker/secrets-engine/x/api"
	"github.com/docker/secrets-engine/x/logging"
	"github.com/docker/secrets-engine/x/secrets"
	"github.com/spf13/cobra"
	"github.com/user/secrets-engine-shim/internal/credstore"
)

type pluginMetadata struct {
	SchemaVersion    string `json:"SchemaVersion"`
	Vendor           string `json:"Vendor"`
	Version          string `json:"Version"`
	ShortDescription string `json:"ShortDescription"`
}

func runDockerPass() {
	args := os.Args[1:]
	if len(args) >= 1 && args[0] == "docker-cli-plugin-metadata" {
		_ = json.NewEncoder(os.Stdout).Encode(pluginMetadata{
			SchemaVersion:    "0.1.0",
			Vendor:           engineName,
			Version:          version,
			ShortDescription: "Manage your local secrets",
		})
		return
	}

	if len(args) >= 1 && (args[0] == cmdPass || args[0] == cmdDockerPass) {
		args = args[1:]
	}

	logger := logging.NewDefaultLogger(cmdDockerPass)

	if os.Getenv(api.PluginLaunchedByEngineVar) != "" {
		runPluginMode(logger)
		return
	}

	store := credstore.NewFromConfig()
	ctx := commands.WithStore(context.Background(), store)

	root := &cobra.Command{
		Use:               cmdDockerPass,
		Short:             "Manage your local secrets",
		SilenceUsage:      true,
		SilenceErrors:     true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	root.SetHelpCommand(&cobra.Command{Hidden: true})
	root.AddCommand(
		commands.SetCommand(),
		commands.GetCommand(),
		commands.ListCommand(),
		commands.RmCommand(),
		commands.RunCommand(),
		commands.VersionCommand(commands.VersionInfo{Version: version, Commit: commit}),
		pluginsCommand(),
	)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func runPluginMode(logger logging.Logger) {
	store := credstore.NewFromConfig()
	pp, err := passplugin.NewPassPlugin(logger, store)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create pass plugin: %v\n", err)
		os.Exit(1)
	}

	stub, err := plugin.NewSecretsProvider(pp, plugin.Config{
		Version: api.MustNewVersion(version),
		SecretsProviderConfig: &plugin.SecretsProviderConfig{
			Pattern: secrets.MustParsePattern("**"),
		},
		Logger: logger,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create plugin: %v\n", err)
		os.Exit(1)
	}

	if err := stub.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "plugin exited: %v\n", err)
		os.Exit(1)
	}
}

func pluginsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Manage secrets engine plugins.",
	}
	cmd.AddCommand(
		pluginsListCommand(),
		pluginsEnableCommand(),
		pluginsDisableCommand(),
	)
	return cmd
}

func pluginsListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List registered secrets engine plugins.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			pm, err := client.PluginManagementFromClient(c)
			if err != nil {
				return err
			}
			plugins, err := pm.ListPlugins(cmd.Context())
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, p := range plugins {
				pattern := ""
				if p.SecretsProvider != nil {
					pattern = p.SecretsProvider.Pattern.String()
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Version, p.RunStatus, pattern)
			}
			return w.Flush()
		},
	}
}

func pluginsEnableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable a secrets engine plugin.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			pm, err := client.PluginManagementFromClient(c)
			if err != nil {
				return err
			}
			return pm.EnablePlugin(cmd.Context(), args[0])
		},
	}
}

func pluginsDisableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable a secrets engine plugin.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New()
			if err != nil {
				return err
			}
			pm, err := client.PluginManagementFromClient(c)
			if err != nil {
				return err
			}
			return pm.DisablePlugin(cmd.Context(), args[0])
		},
	}
}
