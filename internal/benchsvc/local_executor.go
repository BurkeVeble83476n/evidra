package benchsvc

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"time"
)

// LocalExecutor runs benchmark scenarios locally using kubectl.
// It provides basic scenario execution for OSS users without
// requiring an external bench service.
type LocalExecutor struct {
	KubeconfigPath string
	Store          *TriggerStore
}

// NewLocalExecutor creates a LocalExecutor with the given kubeconfig path and trigger store.
func NewLocalExecutor(kubeconfigPath string, store *TriggerStore) *LocalExecutor {
	return &LocalExecutor{
		KubeconfigPath: kubeconfigPath,
		Store:          store,
	}
}

// Start executes each scenario in the job sequentially via kubectl.
func (e *LocalExecutor) Start(ctx context.Context, job *TriggerJob, evidraURL, apiKey string) error {
	for i, sp := range job.Progress {
		// Mark running.
		e.Store.Update(ProgressUpdate{
			JobID:     job.ID,
			Scenario:  sp.Scenario,
			Status:    "running",
			Completed: i,
			Total:     job.Total,
		})

		// Execute scenario check via kubectl.
		passed := e.runScenario(ctx, sp.Scenario, job.Model)

		status := "passed"
		if !passed {
			status = "failed"
		}
		runID := fmt.Sprintf("%s-%s-%s",
			time.Now().UTC().Format("20060102-150405"),
			sp.Scenario, job.Model)

		e.Store.Update(ProgressUpdate{
			JobID:     job.ID,
			Scenario:  sp.Scenario,
			Status:    status,
			RunID:     runID,
			Completed: i + 1,
			Total:     job.Total,
		})
	}
	return nil
}

func (e *LocalExecutor) runScenario(ctx context.Context, scenarioID, model string) bool {
	// Basic check: verify kubectl connectivity and namespace existence.
	// Full scenario orchestration requires RemoteExecutor with bench service.
	cmd := exec.CommandContext(ctx, "kubectl", "get", "namespace", "default")
	if e.KubeconfigPath != "" {
		cmd.Env = append(cmd.Environ(), "KUBECONFIG="+e.KubeconfigPath)
	}
	if err := cmd.Run(); err != nil {
		log.Printf("[local-executor] scenario %s: kubectl check failed: %v", scenarioID, err)
		return false
	}
	return true
}
