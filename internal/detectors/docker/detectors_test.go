package docker

import (
	"testing"

	"samebits.com/evidra/internal/canon"
)

func TestPrivileged(t *testing.T) {
	t.Parallel()
	d := &Privileged{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`
services:
  app:
    image: nginx
    privileged: true
`)) {
		t.Fatalf("expected privileged detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`
services:
  app:
    image: nginx
`)) {
		t.Fatalf("did not expect privileged detection")
	}
}

func TestSocketMount(t *testing.T) {
	t.Parallel()
	d := &SocketMount{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`
services:
  app:
    image: nginx
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
`)) {
		t.Fatalf("expected socket mount detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`
services:
  app:
    image: nginx
    volumes:
      - data:/data
`)) {
		t.Fatalf("did not expect socket mount detection")
	}
}

func TestSocketMount_DoesNotTreatVolumeTypeAsSocketPath(t *testing.T) {
	t.Parallel()

	d := &SocketMount{}
	if d.Detect(canon.CanonicalAction{}, []byte(`
services:
  app:
    image: nginx
    volumes:
      - type: /var/run/docker.sock
        source: data
        target: /data
`)) {
		t.Fatalf("did not expect socket mount detection from volume type")
	}
}

func TestHostNetwork(t *testing.T) {
	t.Parallel()
	d := &HostNetwork{}
	if !d.Detect(canon.CanonicalAction{}, []byte(`
services:
  app:
    image: nginx
    network_mode: host
`)) {
		t.Fatalf("expected host network detection")
	}
	if d.Detect(canon.CanonicalAction{}, []byte(`
services:
  app:
    image: nginx
    network_mode: bridge
`)) {
		t.Fatalf("did not expect host network detection")
	}
}
