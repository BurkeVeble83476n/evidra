package argocd

const (
	CorrelationModeExplicit   = "explicit"
	CorrelationModeBestEffort = "best_effort"

	SourceSystem   = "argocd_controller"
	SourceStart    = "argocd_controller_start"
	SourceComplete = "argocd_controller_complete"

	EventKindSyncStarted   = "sync_started"
	EventKindSyncCompleted = "sync_completed"
)

type Correlation struct {
	Mode           string
	TenantID       string
	AgentID        string
	RunID          string
	SessionID      string
	TraceID        string
	PrescriptionID string
}

type LifecycleEvent struct {
	Key                  string
	Source               string
	Kind                 string
	Phase                string
	Health               string
	Application          string
	ApplicationNamespace string
	ApplicationUID       string
	Namespace            string
	Cluster              string
	Project              string
	Environment          string
	Revision             string
	OperationID          string
	ArtifactDigest       string
	Correlation          Correlation
	ScopeDimensions      map[string]string
}
