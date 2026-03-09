package all

import (
	_ "samebits.com/evidra-benchmark/internal/detectors/docker"
	_ "samebits.com/evidra-benchmark/internal/detectors/k8s"
	_ "samebits.com/evidra-benchmark/internal/detectors/ops"
	_ "samebits.com/evidra-benchmark/internal/detectors/terraform/aws"
)
