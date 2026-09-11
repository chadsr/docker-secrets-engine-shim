// Command docker-secrets-engine-shim is a multicall binary: the same ELF acts
// as the daemon, the docker-pass CLI plugin, and the dockerd NRI plugin,
// dispatching on argv[0] (basename of the invoked path). Install symlinks:
//
//	/usr/bin/docker-secrets-engine-shim                              (the ELF)
//	/usr/bin/docker-pass                → docker-secrets-engine-shim
//	/usr/lib/docker/cli-plugins/docker-pass → ../../bin/docker-secrets-engine-shim
//	/usr/libexec/docker/nri-plugins/10-secrets-engine → ../../../bin/docker-secrets-engine-shim
//
// The docker-pass and 10-secrets-engine symlink names match upstream so
// Docker's plugin discovery and dockerd's NRI config matching work unchanged.
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
