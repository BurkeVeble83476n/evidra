package k8s

import (
	"strings"

	"samebits.com/evidra/internal/canon"
	"samebits.com/evidra/internal/detectors"
)

func init() { detectors.Register(&DockerSocket{}) }

// DockerSocket detects hostPath mount of docker.sock.
type DockerSocket struct{}

func (d *DockerSocket) Tag() string          { return "k8s.docker_socket" }
func (d *DockerSocket) BaseSeverity() string { return "critical" }
func (d *DockerSocket) Metadata() detectors.TagMetadata {
	return detectors.TagMetadata{
		Tag:          d.Tag(),
		BaseSeverity: d.BaseSeverity(),
		Stability:    detectors.Stable,
		Level:        detectors.ResourceRisk,
		Domain:       "k8s",
		SourceKind:   "k8s_yaml",
		Summary:      "Pod mounts docker.sock via hostPath",
	}
}
func (d *DockerSocket) Detect(_ canon.CanonicalAction, raw []byte) bool {
	for _, obj := range ParseK8sYAML(raw) {
		spec := GetPodSpec(obj)
		if spec == nil {
			continue
		}
		volumes, ok := spec["volumes"].([]interface{})
		if !ok {
			continue
		}
		for _, v := range volumes {
			vol, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			hp, ok := vol["hostPath"].(map[string]interface{})
			if !ok {
				continue
			}
			p, _ := hp["path"].(string)
			if strings.Contains(strings.ToLower(p), "docker.sock") {
				return true
			}
		}
	}
	return false
}
