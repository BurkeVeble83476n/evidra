package experiments

import (
	"context"
	"fmt"
	"strings"
)

type ArtifactAgent interface {
	Name() string
	RunArtifact(context.Context, ArtifactAgentRequest) (ArtifactAgentResult, error)
}

func newArtifactAgent(name string) (ArtifactAgent, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "dry-run", "dry_run", "dryrun":
		return &dryRunAgent{}, nil
	case "claude":
		return &claudeAgent{}, nil
	case "bifrost":
		return &bifrostAgent{}, nil
	default:
		return nil, fmt.Errorf("%w: artifact agent %q", ErrUnsupportedAgent, name)
	}
}
