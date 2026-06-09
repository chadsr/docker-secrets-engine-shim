package daemon

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	healthv1 "github.com/docker/secrets-engine/x/api/health/v1"
)

func TestVersionService_GetVersion(t *testing.T) {
	svc := &VersionService{
		Version:    "v0.1.0",
		CommitHash: "abc123",
		Date:       "2026-06-03",
	}

	resp, err := svc.GetVersion(t.Context(), connect.NewRequest(&healthv1.GetVersionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "v0.1.0", resp.Msg.GetVersion())
	assert.Equal(t, "abc123", resp.Msg.GetCommitHash())
	assert.Equal(t, "2026-06-03", resp.Msg.GetDate())
}
