package k8s

import (
	"testing"

	"samebits.com/evidra/internal/canon"
)

func TestDockerSocket(t *testing.T) {
	t.Parallel()
	d := &DockerSocket{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`
apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  volumes:
  - name: sock
    hostPath:
      path: /var/run/docker.sock
  containers:
  - name: app
    image: nginx
`)) {
		t.Fatalf("expected docker socket detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`
apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  containers:
  - name: app
    image: nginx
`)) {
		t.Fatalf("did not expect docker socket detection")
	}
}

func TestRunAsRoot(t *testing.T) {
	t.Parallel()
	d := &RunAsRoot{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`
apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  containers:
  - name: app
    image: nginx
    securityContext:
      runAsUser: 0
`)) {
		t.Fatalf("expected run_as_root detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`
apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  containers:
  - name: app
    image: nginx
    securityContext:
      runAsUser: 1000
      runAsNonRoot: true
`)) {
		t.Fatalf("did not expect run_as_root detection")
	}
}

func TestDangerousCapabilities(t *testing.T) {
	t.Parallel()
	d := &DangerousCapabilities{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`
apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  containers:
  - name: app
    image: nginx
    securityContext:
      capabilities:
        add: ["SYS_ADMIN"]
`)) {
		t.Fatalf("expected dangerous capabilities detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`
apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  containers:
  - name: app
    image: nginx
    securityContext:
      capabilities:
        add: ["CHOWN"]
`)) {
		t.Fatalf("did not expect dangerous capabilities detection")
	}
}

func TestClusterAdminBinding(t *testing.T) {
	t.Parallel()
	d := &ClusterAdminBinding{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata: {name: x}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: default
  namespace: default
`)) {
		t.Fatalf("expected cluster admin binding detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata: {name: x}
roleRef:
  kind: ClusterRole
  name: view
`)) {
		t.Fatalf("did not expect cluster admin binding detection")
	}
}

func TestWritableRootFS(t *testing.T) {
	t.Parallel()
	d := &WritableRootFS{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`
apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  containers:
  - name: app
    image: nginx
    securityContext:
      readOnlyRootFilesystem: false
`)) {
		t.Fatalf("expected writable rootfs detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`
apiVersion: v1
kind: Pod
metadata: {name: p}
spec:
  containers:
  - name: app
    image: nginx
    securityContext:
      readOnlyRootFilesystem: true
`)) {
		t.Fatalf("did not expect writable rootfs detection")
	}
}
