// Command docker-secrets-engine-shim is a multicall binary.
// The same ELF acts as the daemon, the docker-pass CLI plugin, and the dockerd NRI plugin.
package main

import (
	"os"
	"path/filepath"
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
	switch filepath.Base(os.Args[0]) {
	case cmdDockerPass, cmdPass:
		runDockerPass()
	case cmdNRIPlugin:
		runNRIPlugin()
	default:
		runDaemon()
	}
}
