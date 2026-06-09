package daemon

import (
	"context"

	"connectrpc.com/connect"

	healthv1 "github.com/docker/secrets-engine/x/api/health/v1"
)

type VersionService struct {
	Version    string
	CommitHash string
	Date       string
}

func (s *VersionService) GetVersion(_ context.Context, _ *connect.Request[healthv1.GetVersionRequest]) (*connect.Response[healthv1.GetVersionResponse], error) {
	resp := &healthv1.GetVersionResponse{}
	resp.SetVersion(s.Version)
	resp.SetCommitHash(s.CommitHash)
	resp.SetDate(s.Date)
	return connect.NewResponse(resp), nil
}
