package proxy

import "testing"

func TestIsMutation(t *testing.T) {
	t.Parallel()

	mutations := []string{
		"kubectl apply -f manifest.yaml",
		"kubectl patch deployment web -n bench",
		"kubectl delete pod nginx",
		"kubectl create configmap foo",
		"kubectl scale deployment web --replicas=3",
		"kubectl rollout restart deployment/web",
		"kubectl set image deployment/web nginx=nginx:1.27",
		"kubectl drain node-1",
		"helm install myrelease ./chart",
		"helm upgrade myrelease ./chart",
		"helm uninstall myrelease",
		"helm rollback myrelease 1",
		"terraform apply -auto-approve",
		"terraform destroy",
		"terraform import aws_instance.foo i-123",
	}

	for _, cmd := range mutations {
		if !IsMutation(cmd) {
			t.Errorf("expected %q to be a mutation", cmd)
		}
	}

	readOnly := []string{
		"kubectl get pods -n bench",
		"kubectl describe deployment web",
		"kubectl logs nginx",
		"kubectl top pods",
		"kubectl explain deployment",
		"helm list",
		"helm status myrelease",
		"helm template myrelease ./chart",
		"terraform plan",
		"terraform show",
		"terraform output",
		"cat /etc/hosts",
		"echo hello",
		"",
		"kubectl",
	}

	for _, cmd := range readOnly {
		if IsMutation(cmd) {
			t.Errorf("expected %q to be read-only", cmd)
		}
	}
}

func TestParseCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		wantTool string
		wantOp   string
	}{
		{"kubectl apply -f x.yaml", "kubectl", "apply"},
		{"helm install release chart", "helm", "install"},
		{"terraform plan", "terraform", "plan"},
		{"", "", ""},
		{"kubectl", "kubectl", ""},
	}

	for _, tt := range tests {
		tool, op := ParseCommand(tt.input)
		if tool != tt.wantTool || op != tt.wantOp {
			t.Errorf("ParseCommand(%q) = (%q, %q), want (%q, %q)", tt.input, tool, op, tt.wantTool, tt.wantOp)
		}
	}
}
