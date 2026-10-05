// Command docker-secrets-engine-shim is a multicall binary: the daemon, the docker-pass CLI plugin, and the dockerd NRI plugin.
package main

import (
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var (
	version = "v0.0.0"
	commit  = "unknown"
	date    = "unknown"
)

const (
	engineName    = "secrets-engine-shim"
	cmdDockerPass = "docker-pass"
	cmdPass       = "pass"
	cmdNRIPlugin  = "10-secrets-engine"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{PadLevelText: true})
	if lvl, err := logrus.ParseLevel(os.Getenv("SECRETS_ENGINE_SHIM_LOG_LEVEL")); err == nil {
		logrus.SetLevel(lvl)
	}

	switch filepath.Base(os.Args[0]) {
	case cmdDockerPass, cmdPass:
		runDockerPass()
	case cmdNRIPlugin:
		runNRIPlugin()
	default:
		runDaemon()
	}
}
