package detectors_test

import (
	"testing"

	"samebits.com/evidra-benchmark/internal/canon"
	"samebits.com/evidra-benchmark/internal/detectors"
	_ "samebits.com/evidra-benchmark/internal/detectors/all"
)

func TestRunAll_K8sTags(t *testing.T) {
	t.Parallel()

	yaml := []byte(`apiVersion: v1
kind: Pod
metadata:
  name: test
spec:
  hostPID: true
  volumes:
  - name: data
    hostPath:
      path: /var/data
  containers:
  - name: app
    image: nginx
    securityContext:
      privileged: true
`)

	tags := detectors.RunAll(canon.CanonicalAction{}, yaml)
	assertContains(t, tags, "k8s.privileged_container")
	assertContains(t, tags, "k8s.host_namespace_escape")
	assertContains(t, tags, "k8s.hostpath_mount")
}

func assertContains(t *testing.T, tags []string, want string) {
	t.Helper()
	for _, got := range tags {
		if got == want {
			return
		}
	}
	t.Fatalf("tags %v does not contain %q", tags, want)
}
