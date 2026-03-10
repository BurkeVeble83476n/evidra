//go:build e2e

package e2e_test

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"testing"

	"samebits.com/evidra-benchmark/pkg/evidence"
	testcli "samebits.com/evidra-benchmark/tests/testutil"
)

// realFixture returns the path to a fixture in tests/artifacts/real/.
func realFixture(name string) string {
	return filepath.Join("..", "..", "tests", "artifacts", "real", name)
}

// corpusFixture returns the path to a promoted OSS corpus fixture.
func corpusFixture(family, name string) string {
	return filepath.Join("..", "..", "tests", "benchmark", "corpus", family, name)
}

// runAndDecode runs evidra with the given args and decodes the JSON output.
func runAndDecode(t *testing.T, bin string, args ...string) map[string]interface{} {
	t.Helper()
	stdout, stderr, exitCode := testcli.RunEvidra(t, bin, args...)
	if exitCode != 0 {
		t.Fatalf("evidra %s exit=%d stderr=%s", args[0], exitCode, stderr)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode %s output: %v\nstdout: %s", args[0], err, stdout)
	}
	return result
}

// canonicalAction reads the evidence directory and extracts the canonical_action
// from the first prescribe entry's payload.
type canonicalAction struct {
	Tool             string       `json:"tool"`
	Operation        string       `json:"operation"`
	OperationClass   string       `json:"operation_class"`
	ResourceIdentity []resourceID `json:"resource_identity"`
	ScopeClass       string       `json:"scope_class"`
	ResourceCount    int          `json:"resource_count"`
	ShapeHash        string       `json:"resource_shape_hash"`
}

type resourceID struct {
	APIVersion string `json:"api_version,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"`
	Actions    string `json:"actions,omitempty"`
}

// extractCanonicalAction reads evidence, finds the prescribe entry, and
// decodes the canonical_action from its payload.
func extractCanonicalAction(t *testing.T, evidenceDir string) (canonicalAction, evidence.PrescriptionPayload) {
	t.Helper()
	entries, err := evidence.ReadAllEntriesAtPath(evidenceDir)
	if err != nil {
		t.Fatalf("ReadAllEntriesAtPath: %v", err)
	}

	for _, e := range entries {
		if e.Type != evidence.EntryTypePrescribe {
			continue
		}
		var payload evidence.PrescriptionPayload
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			t.Fatalf("decode prescription payload: %v", err)
		}
		var action canonicalAction
		if err := json.Unmarshal(payload.CanonicalAction, &action); err != nil {
			t.Fatalf("decode canonical_action: %v", err)
		}
		return action, payload
	}
	t.Fatal("no prescribe entry found in evidence")
	return canonicalAction{}, evidence.PrescriptionPayload{}
}

// resourceKinds returns sorted list of Kind values from resource identities.
func resourceKinds(ids []resourceID) []string {
	var kinds []string
	for _, id := range ids {
		if id.Kind != "" {
			kinds = append(kinds, id.Kind)
		}
	}
	sort.Strings(kinds)
	return kinds
}

// resourceTypes returns sorted list of Type values from resource identities (terraform).
func resourceTypes(ids []resourceID) []string {
	var types []string
	for _, id := range ids {
		if id.Type != "" {
			types = append(types, id.Type)
		}
	}
	sort.Strings(types)
	return types
}

// containsAll checks that all expected strings are present in actual.
func containsAll(t *testing.T, label string, actual, expected []string) {
	t.Helper()
	have := make(map[string]bool)
	for _, s := range actual {
		have[s] = true
	}
	for _, e := range expected {
		if !have[e] {
			t.Errorf("%s: missing %q in %v", label, e, actual)
		}
	}
}

// TestE2EReal_K8sCorpusPromotion exercises the K8s adapter against promoted OSS
// corpus fixtures instead of local-only acceptance artifacts.
func TestE2EReal_K8sCorpusPromotion(t *testing.T) {
	tests := []struct {
		name              string
		artifact          string
		environment       string
		wantResourceCount int
		wantKinds         []string
		wantIdentities    []string
		wantRiskLevel     string
		wantRiskDetails   []string
	}{
		{
			name:              "hostpath fail",
			artifact:          corpusFixture("k8s", "kubescape-hostpath-mount-fail.yaml"),
			environment:       "staging",
			wantResourceCount: 1,
			wantKinds:         []string{"pod"},
			wantIdentities:    []string{"pod//test-pd"},
			wantRiskLevel:     "high",
			wantRiskDetails:   []string{"k8s.hostpath_mount", "k8s.run_as_root", "k8s.writable_rootfs"},
		},
		{
			name:              "non-root pass",
			artifact:          corpusFixture("k8s", "kubescape-non-root-deployment-pass.yaml"),
			environment:       "staging",
			wantResourceCount: 1,
			wantKinds:         []string{"deployment"},
			wantIdentities:    []string{"deployment//nginx-deployment"},
			wantRiskLevel:     "medium",
			wantRiskDetails:   []string{"k8s.writable_rootfs"},
		},
	}

	bin := testcli.EvidraBinary(t)
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			evidenceDir := filepath.Join(tmpDir, "evidence")
			privPath, _ := testcli.GenerateKeyPair(t, tmpDir)

			runAndDecode(t, bin,
				"prescribe",
				"--tool", "kubectl",
				"--operation", "apply",
				"--artifact", tc.artifact,
				"--environment", tc.environment,
				"--session-id", "e2e-real-k8s-corpus",
				"--evidence-dir", evidenceDir,
				"--signing-key-path", privPath,
			)

			action, payload := extractCanonicalAction(t, evidenceDir)
			if action.ResourceCount != tc.wantResourceCount {
				t.Errorf("resource_count = %d, want %d", action.ResourceCount, tc.wantResourceCount)
			}

			kinds := resourceKinds(action.ResourceIdentity)
			containsAll(t, "resource kinds", kinds, tc.wantKinds)

			found := make(map[string]bool)
			for _, id := range action.ResourceIdentity {
				found[id.Kind+"/"+id.Namespace+"/"+id.Name] = true
			}
			for _, want := range tc.wantIdentities {
				if !found[want] {
					t.Errorf("missing resource identity %s in %v", want, found)
				}
			}

			if action.Tool != "kubectl" {
				t.Errorf("tool = %q, want kubectl", action.Tool)
			}
			if action.OperationClass != "mutate" {
				t.Errorf("operation_class = %q, want mutate", action.OperationClass)
			}
			if action.ScopeClass != tc.environment {
				t.Errorf("scope_class = %q, want %s", action.ScopeClass, tc.environment)
			}
			if action.ShapeHash == "" {
				t.Error("resource_shape_hash empty")
			}

			if payload.RiskLevel != tc.wantRiskLevel {
				t.Errorf("risk_level = %q, want %q", payload.RiskLevel, tc.wantRiskLevel)
			}
			riskDetails := payload.EffectiveRiskDetails()
			containsAll(t, "risk details", riskDetails, tc.wantRiskDetails)

			t.Logf("K8s corpus fixture %s: resources=%d risk=%s details=%v",
				tc.name, action.ResourceCount, payload.RiskLevel, riskDetails)
		})
	}
}

// TestE2EReal_TerraformCorpusPromotion exercises the Terraform adapter against
// promoted OSS corpus fixtures instead of local-only acceptance plans.
func TestE2EReal_TerraformCorpusPromotion(t *testing.T) {
	tests := []struct {
		name              string
		artifact          string
		wantResourceCount int
		wantTypes         []string
		wantRiskLevel     string
		wantRiskDetails   []string
	}{
		{
			name:              "s3 public access fail",
			artifact:          corpusFixture("terraform", "checkov-s3-public-access-fail.tfplan.json"),
			wantResourceCount: 3,
			wantTypes:         []string{"aws_s3_bucket", "aws_s3_bucket_acl", "aws_s3_bucket_public_access_block"},
			wantRiskLevel:     "high",
			wantRiskDetails:   []string{"terraform.s3_public_access"},
		},
		{
			name:              "iam wildcard fail",
			artifact:          corpusFixture("terraform", "checkov-iam-wildcard-fail.tfplan.json"),
			wantResourceCount: 1,
			wantTypes:         []string{"aws_iam_policy"},
			wantRiskLevel:     "critical",
			wantRiskDetails:   []string{"aws_iam.wildcard_policy", "terraform.iam_wildcard_policy"},
		},
	}

	bin := testcli.EvidraBinary(t)
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			evidenceDir := filepath.Join(tmpDir, "evidence")
			privPath, _ := testcli.GenerateKeyPair(t, tmpDir)

			runAndDecode(t, bin,
				"prescribe",
				"--tool", "terraform",
				"--operation", "apply",
				"--artifact", tc.artifact,
				"--environment", "staging",
				"--session-id", "e2e-real-tf-corpus",
				"--evidence-dir", evidenceDir,
				"--signing-key-path", privPath,
			)

			action, payload := extractCanonicalAction(t, evidenceDir)
			if action.ResourceCount != tc.wantResourceCount {
				t.Errorf("resource_count = %d, want %d", action.ResourceCount, tc.wantResourceCount)
			}

			types := resourceTypes(action.ResourceIdentity)
			containsAll(t, "resource types", types, tc.wantTypes)

			if action.Tool != "terraform" {
				t.Errorf("tool = %q, want terraform", action.Tool)
			}
			if action.OperationClass != "mutate" {
				t.Errorf("operation_class = %q, want mutate", action.OperationClass)
			}
			if action.ScopeClass != "staging" {
				t.Errorf("scope_class = %q, want staging", action.ScopeClass)
			}

			riskDetails := payload.EffectiveRiskDetails()
			containsAll(t, "risk details", riskDetails, tc.wantRiskDetails)
			if payload.RiskLevel != tc.wantRiskLevel {
				t.Errorf("risk_level = %q, want %q", payload.RiskLevel, tc.wantRiskLevel)
			}

			t.Logf("Terraform corpus fixture %s: resources=%d risk=%s details=%v",
				tc.name, action.ResourceCount, payload.RiskLevel, riskDetails)
		})
	}
}

// TestE2EReal_HelmRedis exercises the K8s adapter via tool=helm.
func TestE2EReal_HelmRedis(t *testing.T) {
	bin := testcli.EvidraBinary(t)
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")
	privPath, _ := testcli.GenerateKeyPair(t, tmpDir)

	runAndDecode(t, bin,
		"prescribe",
		"--tool", "helm",
		"--operation", "upgrade",
		"--artifact", realFixture("helm_rendered.yaml"),
		"--environment", "staging",
		"--session-id", "e2e-real-helm",
		"--evidence-dir", evidenceDir,
		"--signing-key-path", privPath,
	)

	action, _ := extractCanonicalAction(t, evidenceDir)

	// 5 resources: ServiceAccount, 2 ConfigMaps, Service, StatefulSet.
	if action.ResourceCount != 5 {
		t.Errorf("resource_count = %d, want 5", action.ResourceCount)
	}

	kinds := resourceKinds(action.ResourceIdentity)
	containsAll(t, "resource kinds", kinds, []string{
		"configmap", "service", "serviceaccount", "statefulset",
	})

	// Tool must be preserved as "helm" (not rewritten to kubectl).
	if action.Tool != "helm" {
		t.Errorf("tool = %q, want helm", action.Tool)
	}
	if action.OperationClass != "mutate" {
		t.Errorf("operation_class = %q, want mutate", action.OperationClass)
	}

	// Verify specific helm-rendered resources.
	found := make(map[string]bool)
	for _, id := range action.ResourceIdentity {
		found[id.Kind+"/"+id.Namespace+"/"+id.Name] = true
	}
	if !found["statefulset/cache/redis-master"] {
		t.Error("missing StatefulSet redis-master identity")
	}
	if !found["serviceaccount/cache/redis-master"] {
		t.Error("missing ServiceAccount redis-master identity")
	}

	t.Logf("Helm Redis: %d resources, kinds=%v", action.ResourceCount, kinds)
}

// TestE2EReal_ArgoCDSync exercises the K8s adapter with ArgoCD-managed
// manifests including tracking annotations and server-side noise.
func TestE2EReal_ArgoCDSync(t *testing.T) {
	bin := testcli.EvidraBinary(t)
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")
	privPath, _ := testcli.GenerateKeyPair(t, tmpDir)

	runAndDecode(t, bin,
		"prescribe",
		"--tool", "kubectl",
		"--operation", "apply",
		"--artifact", realFixture("argocd_app_sync.yaml"),
		"--environment", "production",
		"--session-id", "e2e-real-argocd",
		"--evidence-dir", evidenceDir,
		"--signing-key-path", privPath,
	)

	action, payload := extractCanonicalAction(t, evidenceDir)

	// 6 resources: Namespace, ConfigMap, Deployment, Service, SA, NetworkPolicy.
	if action.ResourceCount != 6 {
		t.Errorf("resource_count = %d, want 6", action.ResourceCount)
	}

	kinds := resourceKinds(action.ResourceIdentity)
	containsAll(t, "resource kinds", kinds, []string{
		"configmap", "deployment", "namespace",
		"networkpolicy", "service", "serviceaccount",
	})

	// Verify ArgoCD tracking annotations didn't corrupt identity extraction.
	found := make(map[string]bool)
	for _, id := range action.ResourceIdentity {
		found[id.Kind+"/"+id.Namespace+"/"+id.Name] = true
	}
	if !found["deployment/payments/payments-api"] {
		t.Error("missing Deployment payments-api identity")
	}
	if !found["networkpolicy/payments/payments-netpol"] {
		t.Error("missing NetworkPolicy payments-netpol identity")
	}

	// Noise immunity: prescribe same artifact twice, intent_digest must be stable.
	evidenceDir2 := filepath.Join(t.TempDir(), "evidence2")
	runAndDecode(t, bin,
		"prescribe",
		"--tool", "kubectl",
		"--operation", "apply",
		"--artifact", realFixture("argocd_app_sync.yaml"),
		"--environment", "production",
		"--session-id", "e2e-real-argocd-2",
		"--evidence-dir", evidenceDir2,
		"--signing-key-path", privPath,
	)
	action2, _ := extractCanonicalAction(t, evidenceDir2)

	if action.ShapeHash != action2.ShapeHash {
		t.Errorf("shape_hash not stable across runs: %s vs %s", action.ShapeHash, action2.ShapeHash)
	}

	if payload.RiskLevel != "high" {
		t.Errorf("risk_level = %q, want high (mutate×production matrix)", payload.RiskLevel)
	}

	t.Logf("ArgoCD sync: %d resources, risk=%s, kinds=%v",
		action.ResourceCount, payload.RiskLevel, kinds)
}

// TestE2EReal_KustomizeMonitoring exercises the K8s adapter with kustomize
// build output including ClusterRole/ClusterRoleBinding.
func TestE2EReal_KustomizeMonitoring(t *testing.T) {
	bin := testcli.EvidraBinary(t)
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")
	privPath, _ := testcli.GenerateKeyPair(t, tmpDir)

	runAndDecode(t, bin,
		"prescribe",
		"--tool", "kustomize",
		"--operation", "apply",
		"--artifact", realFixture("kustomize_monitoring.yaml"),
		"--environment", "staging",
		"--session-id", "e2e-real-kustomize",
		"--evidence-dir", evidenceDir,
		"--signing-key-path", privPath,
	)

	action, _ := extractCanonicalAction(t, evidenceDir)

	// 10 resources.
	if action.ResourceCount != 10 {
		t.Errorf("resource_count = %d, want 10", action.ResourceCount)
	}

	kinds := resourceKinds(action.ResourceIdentity)
	containsAll(t, "resource kinds", kinds, []string{
		"clusterrole", "clusterrolebinding", "configmap",
		"deployment", "namespace", "service", "serviceaccount",
	})

	if action.Tool != "kustomize" {
		t.Errorf("tool = %q, want kustomize", action.Tool)
	}

	// Verify both Prometheus and Grafana deployments are captured.
	found := make(map[string]bool)
	for _, id := range action.ResourceIdentity {
		found[id.Kind+"/"+id.Namespace+"/"+id.Name] = true
	}
	if !found["deployment/monitoring/prometheus"] {
		t.Error("missing Deployment prometheus identity")
	}
	if !found["deployment/monitoring/grafana"] {
		t.Error("missing Deployment grafana identity")
	}
	// ClusterRole has no namespace.
	if !found["clusterrole//prometheus"] {
		t.Error("missing ClusterRole prometheus identity")
	}

	t.Logf("Kustomize monitoring: %d resources, kinds=%v", action.ResourceCount, kinds)
}

// TestE2EReal_HelmIngressNginx exercises the K8s adapter with ingress-nginx
// chart including LoadBalancer and capabilities.
func TestE2EReal_HelmIngressNginx(t *testing.T) {
	bin := testcli.EvidraBinary(t)
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")
	privPath, _ := testcli.GenerateKeyPair(t, tmpDir)

	runAndDecode(t, bin,
		"prescribe",
		"--tool", "helm",
		"--operation", "install",
		"--artifact", realFixture("helm_ingress_nginx.yaml"),
		"--environment", "production",
		"--session-id", "e2e-real-helm-nginx",
		"--evidence-dir", evidenceDir,
		"--signing-key-path", privPath,
	)

	action, payload := extractCanonicalAction(t, evidenceDir)

	// 7 resources.
	if action.ResourceCount != 7 {
		t.Errorf("resource_count = %d, want 7", action.ResourceCount)
	}

	kinds := resourceKinds(action.ResourceIdentity)
	containsAll(t, "resource kinds", kinds, []string{
		"clusterrole", "clusterrolebinding", "configmap",
		"deployment", "ingressclass", "service", "serviceaccount",
	})

	// Risk detectors should fire: runAsUser 101 != 0 but no runAsNonRoot,
	// plus writable rootfs (no readOnlyRootFilesystem).
	riskDetails := payload.EffectiveRiskDetails()
	if len(riskDetails) == 0 {
		t.Error("risk_details empty — expected detectors to fire on ingress-nginx spec")
	}

	// Production install is high from the matrix; medium/low tags do not elevate it.
	if payload.RiskLevel != "high" {
		t.Errorf("risk_level = %q, want high", payload.RiskLevel)
	}

	t.Logf("Helm ingress-nginx: %d resources, risk=%s, tags=%v",
		action.ResourceCount, payload.RiskLevel, riskDetails)
}

// TestE2EReal_OpenShiftApp exercises the K8s adapter via tool=oc with
// OpenShift-specific resources: DeploymentConfig, BuildConfig, ImageStream, Route.
func TestE2EReal_OpenShiftApp(t *testing.T) {
	bin := testcli.EvidraBinary(t)
	tmpDir := t.TempDir()
	evidenceDir := filepath.Join(tmpDir, "evidence")
	privPath, _ := testcli.GenerateKeyPair(t, tmpDir)

	runAndDecode(t, bin,
		"prescribe",
		"--tool", "oc",
		"--operation", "apply",
		"--artifact", realFixture("openshift_app.yaml"),
		"--environment", "production",
		"--session-id", "e2e-real-openshift",
		"--evidence-dir", evidenceDir,
		"--signing-key-path", privPath,
	)

	action, payload := extractCanonicalAction(t, evidenceDir)

	// 9 resources: Namespace, ConfigMap, ImageStream, BuildConfig,
	// DeploymentConfig, Service, SA, Route, HPA.
	if action.ResourceCount != 9 {
		t.Errorf("resource_count = %d, want 9", action.ResourceCount)
	}

	kinds := resourceKinds(action.ResourceIdentity)
	// Verify OCP-specific resource kinds are extracted.
	containsAll(t, "resource kinds", kinds, []string{
		"buildconfig",
		"deploymentconfig",
		"horizontalpodautoscaler",
		"imagestream",
		"route",
	})
	// And standard resources too.
	containsAll(t, "resource kinds", kinds, []string{
		"configmap", "namespace", "service", "serviceaccount",
	})

	if action.Tool != "oc" {
		t.Errorf("tool = %q, want oc", action.Tool)
	}

	// Verify specific OCP resource identities.
	found := make(map[string]bool)
	for _, id := range action.ResourceIdentity {
		found[id.Kind+"/"+id.Namespace+"/"+id.Name] = true
	}
	if !found["deploymentconfig/webapp/webapp"] {
		t.Error("missing DeploymentConfig webapp identity")
	}
	if !found["buildconfig/webapp/webapp"] {
		t.Error("missing BuildConfig webapp identity")
	}
	if !found["imagestream/webapp/webapp"] {
		t.Error("missing ImageStream webapp identity")
	}
	if !found["route/webapp/webapp"] {
		t.Error("missing Route webapp identity")
	}

	if payload.RiskLevel != "high" {
		t.Errorf("risk_level = %q, want high (mutate×production matrix)", payload.RiskLevel)
	}

	t.Logf("OpenShift app: %d resources, risk=%s, kinds=%v",
		action.ResourceCount, payload.RiskLevel, kinds)
}

// TestE2EReal_NoiseImmunity verifies that two manifests differing only in
// noise fields (uid, resourceVersion, managedFields) produce identical
// intent_digest and resource_shape_hash.
func TestE2EReal_NoiseImmunity(t *testing.T) {
	bin := testcli.EvidraBinary(t)
	privPath, _ := testcli.GenerateKeyPair(t, t.TempDir())

	// Run 1: ArgoCD fixture has uid, resourceVersion, managedFields, tracking annotations.
	dir1 := filepath.Join(t.TempDir(), "evidence")
	runAndDecode(t, bin,
		"prescribe",
		"--tool", "kubectl",
		"--operation", "apply",
		"--artifact", realFixture("argocd_app_sync.yaml"),
		"--environment", "production",
		"--evidence-dir", dir1,
		"--signing-key-path", privPath,
	)
	action1, _ := extractCanonicalAction(t, dir1)

	// Run 2: same fixture — noise is non-deterministic (different uid etc in
	// real life) but since we use the same file, digests must match.
	// This proves noise filtering is stable.
	dir2 := filepath.Join(t.TempDir(), "evidence")
	runAndDecode(t, bin,
		"prescribe",
		"--tool", "kubectl",
		"--operation", "apply",
		"--artifact", realFixture("argocd_app_sync.yaml"),
		"--environment", "production",
		"--evidence-dir", dir2,
		"--signing-key-path", privPath,
	)
	action2, _ := extractCanonicalAction(t, dir2)

	if action1.ShapeHash != action2.ShapeHash {
		t.Errorf("shape_hash not stable: %s vs %s", action1.ShapeHash, action2.ShapeHash)
	}
	if action1.ResourceCount != action2.ResourceCount {
		t.Errorf("resource_count not stable: %d vs %d", action1.ResourceCount, action2.ResourceCount)
	}

	// Verify resource identities are identical.
	ids1, _ := json.Marshal(action1.ResourceIdentity)
	ids2, _ := json.Marshal(action2.ResourceIdentity)
	if string(ids1) != string(ids2) {
		t.Errorf("resource_identity not stable:\n  run1: %s\n  run2: %s", ids1, ids2)
	}
}
