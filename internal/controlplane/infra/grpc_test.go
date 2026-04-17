package infra

import (
	"context"
	"testing"

	scmv1 "github.com/alexisjcarr/scm/pkg/api/scm/v1"
)

func TestGRPCServerRejectsUnauthorizedAgentRequests(t *testing.T) {
	t.Parallel()

	server := NewGRPCServer(nil, map[string]string{"agent-1": "expected-token"})

	if _, err := server.RegisterAgent(context.Background(), &scmv1.RegisterAgentRequest{
		AgentID:   "agent-1",
		HostID:    "host-1",
		AuthToken: "wrong-token",
	}); err == nil {
		t.Fatal("RegisterAgent returned nil error for invalid token")
	}

	if _, err := server.FetchWork(context.Background(), &scmv1.FetchWorkRequest{
		AgentID:   "agent-1",
		AuthToken: "wrong-token",
	}); err == nil {
		t.Fatal("FetchWork returned nil error for invalid token")
	}
}
