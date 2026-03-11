package detectors

import (
	"sync"

	"samebits.com/evidra/internal/canon"
)

var (
	prodMu    sync.RWMutex
	producers []TagProducer
)

func init() {
	RegisterProducer(&NativeProducer{})
}

// RegisterProducer adds a tag producer to the global chain.
func RegisterProducer(p TagProducer) {
	if p == nil {
		return
	}
	prodMu.Lock()
	defer prodMu.Unlock()
	producers = append(producers, p)
}

// ProduceAll executes all registered producers and returns deduplicated tags.
func ProduceAll(action canon.CanonicalAction, raw []byte) []string {
	prodMu.RLock()
	defer prodMu.RUnlock()

	seen := make(map[string]bool)
	var tags []string
	for _, p := range producers {
		for _, tag := range p.ProduceTags(action, raw) {
			if tag == "" || seen[tag] {
				continue
			}
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	return tags
}
