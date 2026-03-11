package k8s

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ParseK8sYAML decodes one or more YAML documents.
func ParseK8sYAML(raw []byte) ([]map[string]interface{}, error) {
	var objects []map[string]interface{}
	decoder := yaml.NewDecoder(bufio.NewReader(bytes.NewReader(raw)))
	for {
		var obj map[string]interface{}
		if err := decoder.Decode(&obj); err == io.EOF {
			break
		} else if err != nil {
			return objects, fmt.Errorf("parse k8s yaml: %w", err)
		}
		if obj == nil {
			continue
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

// GetPodSpec returns pod spec from Pod/Deployment-like objects.
func GetPodSpec(obj map[string]interface{}) map[string]interface{} {
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	if template, ok := spec["template"].(map[string]interface{}); ok {
		if tspec, ok := template["spec"].(map[string]interface{}); ok {
			return tspec
		}
	}
	return spec
}

// GetAllContainers returns containers + initContainers if present.
func GetAllContainers(obj map[string]interface{}) []map[string]interface{} {
	spec := GetPodSpec(obj)
	if spec == nil {
		return nil
	}
	var containers []map[string]interface{}
	for _, key := range []string{"containers", "initContainers"} {
		list, ok := spec[key].([]interface{})
		if !ok {
			continue
		}
		for _, c := range list {
			if cm, ok := c.(map[string]interface{}); ok {
				containers = append(containers, cm)
			}
		}
	}
	return containers
}

// GetBool reads a bool key from map.
func GetBool(m map[string]interface{}, key string) bool {
	v, ok := m[key].(bool)
	return ok && v
}

func getString(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func hasDangerousCapability(vals []interface{}) bool {
	for _, v := range vals {
		s, _ := v.(string)
		switch strings.ToUpper(s) {
		case "SYS_ADMIN", "NET_ADMIN", "NET_RAW", "ALL":
			return true
		}
	}
	return false
}
