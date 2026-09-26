package runner

import (
	"context"
	"log/slog"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// Reap removes every container, volume and network left by an earlier run of this worker,
// such as one killed mid-job. Call it before connecting, when nothing can be in flight.
func Reap(ctx context.Context, cli client.APIClient, workerID string) (removed int) {
	mine := filters.NewArgs(filters.Arg("label", LabelWorker+"="+workerID))

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: mine})
	if err != nil {
		slog.Warn("reaper: list containers", "err", err)
	}
	for _, c := range containers {
		if err := cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
			slog.Warn("reaper: remove container", "container", c.ID[:12], "err", err)
			continue
		}
		removed++
	}

	vols, err := cli.VolumeList(ctx, volume.ListOptions{Filters: mine})
	if err != nil {
		slog.Warn("reaper: list volumes", "err", err)
	}
	for _, v := range vols.Volumes {
		if err := cli.VolumeRemove(ctx, v.Name, true); err != nil {
			slog.Warn("reaper: remove volume", "volume", v.Name, "err", err)
			continue
		}
		removed++
	}

	nets, err := cli.NetworkList(ctx, network.ListOptions{Filters: mine})
	if err != nil {
		slog.Warn("reaper: list networks", "err", err)
	}
	for _, n := range nets {
		if err := cli.NetworkRemove(ctx, n.ID); err != nil {
			slog.Warn("reaper: remove network", "network", n.Name, "err", err)
			continue
		}
		removed++
	}
	if removed > 0 {
		slog.Info("reaper removed leftovers of an earlier run", "count", removed)
	}
	return removed
}
