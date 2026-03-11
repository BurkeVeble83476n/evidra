package all

import (
	_ "samebits.com/evidra/internal/detectors/docker"
	_ "samebits.com/evidra/internal/detectors/k8s"
	_ "samebits.com/evidra/internal/detectors/ops"
	_ "samebits.com/evidra/internal/detectors/terraform/aws"
)
