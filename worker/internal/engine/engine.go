// Package engine describes the Docker engine the agent runs jobs on, and finds the
// agent's own container on it.
package engine

import (
	"context"
	"io"
	"regexp"
	"slices"

	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"

	pb "github.com/lute/proto"
)

type Info struct {
	Version  string
	Name     string // the engine host's name: the container's own hostname is a random id
	OS       string
	Arch     string
	CPUs     int
	Rootless bool
	// The limits the engine can enforce; rootless Docker needs cgroup v2 delegation for them.
	MemoryLimit bool
	CPULimit    bool
	PidsLimit   bool
}

func Probe(ctx context.Context, cli client.APIClient) (Info, error) {
	info, err := cli.Info(ctx)
	if err != nil {
		return Info{}, err
	}
	return FromSystemInfo(info), nil
}

func FromSystemInfo(info system.Info) Info {
	return Info{
		Version:     info.ServerVersion,
		Name:        info.Name,
		OS:          info.OSType,
		Arch:        info.Architecture,
		CPUs:        info.NCPU,
		Rootless:    slices.Contains(info.SecurityOptions, "name=rootless"),
		MemoryLimit: info.MemoryLimit,
		CPULimit:    info.CPUCfsQuota,
		PidsLimit:   info.PidsLimit,
	}
}

// Missing names the resource limits the engine cannot enforce.
func (i Info) Missing() []string {
	var out []string
	if !i.MemoryLimit {
		out = append(out, "memory")
	}
	if !i.CPULimit {
		out = append(out, "cpu")
	}
	if !i.PidsLimit {
		out = append(out, "pids")
	}
	return out
}

func (i Info) Proto() *pb.EngineInfo {
	return &pb.EngineInfo{
		Kind:        "docker",
		Version:     i.Version,
		Rootless:    i.Rootless,
		MemoryLimit: i.MemoryLimit,
		CpuLimit:    i.CPULimit,
		PidsLimit:   i.PidsLimit,
		Os:          i.OS,
		Arch:        i.Arch,
		Cpus:        int32(i.CPUs),
		Hostname:    i.Name,
	}
}

// Docker bind-mounts <data-root>/containers/<id>/hostname into every container.
var hostnameMount = regexp.MustCompile(`/containers/([0-9a-f]{64})/hostname\s`)

// ContainerIDFromMountinfo returns the container id a /proc/self/mountinfo belongs to, or
// "" outside a container.
func ContainerIDFromMountinfo(r io.Reader) string {
	raw, err := io.ReadAll(r)
	if err != nil {
		return ""
	}
	m := hostnameMount.FindSubmatch(raw)
	if m == nil {
		return ""
	}
	return string(m[1])
}
