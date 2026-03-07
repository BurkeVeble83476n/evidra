package detectors

import (
	"testing"

	"samebits.com/evidra-benchmark/internal/canon"
)

type staticProducer struct {
	name string
	tags []string
}

func (p *staticProducer) Name() string { return p.name }
func (p *staticProducer) ProduceTags(_ canon.CanonicalAction, _ []byte) []string {
	return append([]string(nil), p.tags...)
}

func TestProduceAll_DedupesAcrossProducers(t *testing.T) {
	t.Parallel()

	prodMu.Lock()
	orig := producers
	producers = []TagProducer{
		&staticProducer{name: "a", tags: []string{"k8s.privileged_container"}},
		&staticProducer{name: "b", tags: []string{"k8s.privileged_container", "custom.tag"}},
	}
	prodMu.Unlock()
	defer func() {
		prodMu.Lock()
		producers = orig
		prodMu.Unlock()
	}()

	tags := ProduceAll(canon.CanonicalAction{}, nil)
	if countTag(tags, "k8s.privileged_container") != 1 {
		t.Fatalf("expected deduped k8s.privileged_container, got %v", tags)
	}
	if countTag(tags, "custom.tag") != 1 {
		t.Fatalf("expected custom.tag, got %v", tags)
	}
}

func TestSARIFProducer(t *testing.T) {
	t.Parallel()

	p := &SARIFProducer{
		RuleMapping: map[string]string{
			"KSV012": "k8s.privileged_container",
			"KSV029": "k8s.run_as_root",
		},
	}

	raw := []byte(`{
  "runs": [{
    "results": [
      {"ruleId":"KSV012","level":"error"},
      {"ruleId":"KSV012","level":"warning"},
      {"ruleId":"KSV029","level":"warning"},
      {"ruleId":"UNKNOWN","level":"warning"}
    ]
  }]
}`)

	tags := p.ProduceTags(canon.CanonicalAction{}, raw)
	if countTag(tags, "k8s.privileged_container") != 1 {
		t.Fatalf("expected mapped privileged tag once, got %v", tags)
	}
	if countTag(tags, "k8s.run_as_root") != 1 {
		t.Fatalf("expected mapped run_as_root tag once, got %v", tags)
	}
}

func countTag(tags []string, want string) int {
	n := 0
	for _, t := range tags {
		if t == want {
			n++
		}
	}
	return n
}
