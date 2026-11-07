package daemon

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrSocketInUse: a live listener owns the socket; do not remove the file.
var ErrSocketInUse = errors.New("socket in use by another process")

const probeTimeout = 500 * time.Millisecond

// ListenUnix removes a stale socket file before binding; a live socket or non-socket file is an error.
func ListenUnix(path string) (net.Listener, error) {
	if !strings.HasPrefix(path, "@") {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("creating socket directory: %w", err)
		}
		if fi, err := os.Lstat(path); err == nil {
			if fi.Mode()&os.ModeSocket == 0 {
				return nil, fmt.Errorf("%s exists and is not a socket", path)
			}
			if err := probeSocket(path); err == nil {
				return nil, fmt.Errorf("another daemon is already listening on %s: %w", path, ErrSocketInUse)
			}
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("removing stale socket %s: %w", path, err)
			}
		}
	}

	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listening on %s: %w", path, err)
	}
	return l, nil
}

// probeSocket dials path; a nil error means a live listener answered.
func probeSocket(path string) error {
	conn, err := net.DialTimeout("unix", path, probeTimeout)
	if err != nil {
		return err
	}
	return conn.Close()
}
