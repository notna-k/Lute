package engine

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/system"
	"github.com/google/go-cmp/cmp"
)

func load(t *testing.T, name string) system.Info {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var info system.Info
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatal(err)
	}
	return info
}

// The fixtures are /info from a real rootful and a real rootless dockerd; the
// no-delegation one is the rootless answer with the limits a daemon without cgroup v2
// delegation reports as unsupported.
func TestFromSystemInfo(t *testing.T) {
	tests := []struct {
		fixture string
		want    Info
		missing []string
	}{
		{
			fixture: "info-rootful.json",
			want: Info{Version: "29.5.3", Name: "build-host", OS: "linux", Arch: "x86_64", CPUs: 8,
				Rootless: false, MemoryLimit: true, CPULimit: true, PidsLimit: true},
		},
		{
			fixture: "info-rootless.json",
			want: Info{Version: "29.5.3", Name: "build-host", OS: "linux", Arch: "x86_64", CPUs: 8,
				Rootless: true, MemoryLimit: true, CPULimit: true, PidsLimit: true},
		},
		{
			fixture: "info-rootless-no-delegation.json",
			want: Info{Version: "29.5.3", Name: "build-host", OS: "linux", Arch: "x86_64", CPUs: 8,
				Rootless: true},
			missing: []string{"memory", "cpu", "pids"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			got := FromSystemInfo(load(t, tt.fixture))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.missing, got.Missing()); diff != "" {
				t.Errorf("missing limits (-want +got):\n%s", diff)
			}
		})
	}
}

func TestContainerIDFromMountinfo(t *testing.T) {
	const id = "7f3c2d9e8b1a4c6d5e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d"
	inContainer := `1265 1180 0:112 / / rw,relatime - overlay overlay rw
1271 1265 259:2 /home/ci/.local/share/lute-worker /var/lib/lute-worker rw - ext4 /dev/nvme0n1p2 rw
1272 1265 259:2 /home/ci/.local/share/docker/containers/` + id + `/resolv.conf /etc/resolv.conf rw - ext4 /dev/nvme0n1p2 rw
1273 1265 259:2 /home/ci/.local/share/docker/containers/` + id + `/hostname /etc/hostname rw - ext4 /dev/nvme0n1p2 rw
`
	if got := ContainerIDFromMountinfo(strings.NewReader(inContainer)); got != id {
		t.Errorf("in a container: got %q", got)
	}
	onHost := "22 1 259:2 / / rw,relatime shared:1 - ext4 /dev/nvme0n1p2 rw\n"
	if got := ContainerIDFromMountinfo(strings.NewReader(onHost)); got != "" {
		t.Errorf("on the host: got %q, want none", got)
	}
}
