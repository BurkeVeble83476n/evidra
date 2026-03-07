package detectors

import "samebits.com/evidra-benchmark/internal/canon"

// TagProducer generates risk tags from an infrastructure operation.
type TagProducer interface {
	Name() string
	ProduceTags(action canon.CanonicalAction, raw []byte) []string
}
