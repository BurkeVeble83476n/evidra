package detectors

import "samebits.com/evidra-benchmark/internal/canon"

// NativeProducer wraps registered in-process detectors as a TagProducer.
type NativeProducer struct{}

func (p *NativeProducer) Name() string { return "native" }

func (p *NativeProducer) ProduceTags(action canon.CanonicalAction, raw []byte) []string {
	return RunAll(action, raw)
}
