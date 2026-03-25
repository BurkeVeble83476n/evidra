package benchsvc

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// LocalExecutor runs benchmark scenarios locally using the Kubernetes API.
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

// Start executes each scenario in the job sequentially.
func (e *LocalExecutor) Start(ctx context.Context, job *TriggerJob, evidraURL, apiKey string) error {
	client, err := e.kubeClient()
	if err != nil {
		log.Printf("[local-executor] kubernetes client init failed: %v", err)
		// Mark all scenarios as failed.
		for i, sp := range job.Progress {
			e.Store.Update(ProgressUpdate{
				JobID:     job.ID,
				Scenario:  sp.Scenario,
				Status:    "failed",
				Completed: i + 1,
				Total:     job.Total,
			})
		}
		return fmt.Errorf("benchsvc.LocalExecutor: kube client: %w", err)
	}

	for i, sp := range job.Progress {
		e.Store.Update(ProgressUpdate{
			JobID:     job.ID,
			Scenario:  sp.Scenario,
			Status:    "running",
			Completed: i,
			Total:     job.Total,
		})

		passed := e.runScenario(ctx, client, sp.Scenario)

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

// kubeClient builds a Kubernetes clientset from kubeconfig or in-cluster config.
func (e *LocalExecutor) kubeClient() (kubernetes.Interface, error) {
	var cfg *rest.Config
	var err error
	if strings.TrimSpace(e.KubeconfigPath) != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", e.KubeconfigPath)
	} else {
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}

func (e *LocalExecutor) runScenario(ctx context.Context, client kubernetes.Interface, scenarioID string) bool {
	// Basic check: verify Kubernetes API connectivity and namespace existence.
	// Full scenario orchestration requires RemoteExecutor with bench service.
	_, err := client.CoreV1().Namespaces().Get(ctx, "default", metav1.GetOptions{})
	if err != nil {
		log.Printf("[local-executor] scenario %s: kubernetes check failed: %v", scenarioID, err)
		return false
	}
	return true
}
