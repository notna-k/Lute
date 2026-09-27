package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/pkg/stdcopy"
)

// job is one run's engine resources, removed by cleanup whatever happened.
type job struct {
	*Runner
	id  string
	log *slog.Logger

	network    string
	volume     string
	containers []string
}

func (j *job) labels() map[string]string {
	return map[string]string{LabelWorker: j.WorkerID, LabelJob: j.id}
}

// prepare creates the job's own network and its workspace volume.
func (j *job) prepare(ctx context.Context) error {
	name := "lute-job-" + j.id
	if _, err := j.Docker.NetworkCreate(ctx, name, network.CreateOptions{Driver: "bridge", Labels: j.labels()}); err != nil {
		return fmt.Errorf("network create: %w", err)
	}
	j.network = name

	vol, err := j.Docker.VolumeCreate(ctx, volume.CreateOptions{Name: "lute-ws-" + j.id, Labels: j.labels()})
	if err != nil {
		return fmt.Errorf("volume create: %w", err)
	}
	j.volume = vol.Name
	return nil
}

// cleanup removes containers first: a volume or network in use cannot be removed.
func (j *job) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()
	for _, id := range j.containers {
		if err := j.Docker.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
			slog.Warn("remove job container", "job_id", j.id, "container", id[:12], "err", err)
		}
	}
	if j.volume != "" {
		if err := j.Docker.VolumeRemove(ctx, j.volume, true); err != nil {
			slog.Warn("remove job volume", "job_id", j.id, "err", err)
		}
	}
	if j.network != "" {
		if err := j.Docker.NetworkRemove(ctx, j.network); err != nil {
			slog.Warn("remove job network", "job_id", j.id, "err", err)
		}
	}
}

func (j *job) hostConfig() *container.HostConfig {
	return &container.HostConfig{
		NetworkMode: container.NetworkMode(j.network),
		Mounts:      []mount.Mount{{Type: mount.TypeVolume, Source: j.volume, Target: workspaceMount}},
		SecurityOpt: []string{"no-new-privileges"},
	}
}

// create makes a container that cleanup will remove.
func (j *job) create(ctx context.Context, cfg *container.Config, hc *container.HostConfig) (string, error) {
	cfg.Labels = j.labels()
	resp, err := j.Docker.ContainerCreate(ctx, cfg, hc, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}
	j.containers = append(j.containers, resp.ID)
	return resp.ID, nil
}

// clone fills the workspace volume from repo in a throwaway git container and returns the commit.
func (j *job) clone(ctx context.Context, repo string) (string, error) {
	if _, err := j.pull(ctx, j.GitImage, false); err != nil {
		return "", err
	}
	logSystem(j.log, slog.LevelInfo, "cloning", slog.String("repository", repo))
	id, err := j.create(ctx, &container.Config{
		Image:      j.GitImage,
		Entrypoint: []string{"sh", "-c", cloneScript, "lute"},
		Cmd:        []string{repo},
		Env:        []string{"GIT_TERMINAL_PROMPT=0"},
	}, j.hostConfig())
	if err != nil {
		return "", err
	}
	if err := j.Docker.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("start clone: %w", err)
	}
	code, err := waitForExit(ctx, j.Docker, id)
	if err != nil {
		return "", err
	}

	var stdout, stderr bytes.Buffer
	if logs, err := j.Docker.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true}); err == nil {
		_, _ = stdcopy.StdCopy(&stdout, &stderr, logs)
		_ = logs.Close()
	}
	out := &lineLogWriter{jobLogger: j.log, source: sourceSystem}
	_, _ = out.Write(stderr.Bytes())
	out.flush()
	if code != 0 {
		return "", fmt.Errorf("git exited with code %d: %s", code, strings.TrimSpace(stderr.String()))
	}
	commit := lastLine(stdout.String())
	logSystem(j.log, slog.LevelInfo, "clone ok", slog.String("commit", commit))
	return commit, nil
}

// pull fetches image (always, or only when it is missing) and returns its digest.
func (j *job) pull(ctx context.Context, ref string, always bool) (string, error) {
	if !always {
		if inspect, err := j.Docker.ImageInspect(ctx, ref); err == nil {
			return digestOf(inspect), nil
		}
	}
	logSystem(j.log, slog.LevelInfo, "pulling image", slog.String("image", ref))
	resp, err := j.Docker.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("image pull %q: %w", ref, err)
	}
	_, _ = io.Copy(io.Discard, resp)
	_ = resp.Close()
	inspect, err := j.Docker.ImageInspect(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("inspect %q: %w", ref, err)
	}
	digest := digestOf(inspect)
	logSystem(j.log, slog.LevelInfo, "image pull ok", slog.String("image", ref), slog.String("digest", digest))
	return digest, nil
}

func digestOf(inspect image.InspectResponse) string {
	for _, d := range inspect.RepoDigests {
		if _, digest, ok := strings.Cut(d, "@"); ok {
			return digest
		}
	}
	return inspect.ID
}

// runCommand runs the job's script in its runtime image and returns the exit code.
func (j *job) runCommand(ctx context.Context, spec *Spec) (int64, error) {
	hc := j.hostConfig()
	hc.Resources = container.Resources{Memory: j.Limits.Memory, NanoCPUs: j.Limits.NanoCPUs}
	if j.Limits.Pids > 0 {
		hc.PidsLimit = &j.Limits.Pids
	}
	id, err := j.create(ctx, &container.Config{
		Image:      spec.Runtime,
		Cmd:        shellCommand(scriptPath),
		Env:        envFromParams(spec.RequestParams),
		WorkingDir: workspaceMount,
	}, hc)
	if err != nil {
		return 0, err
	}
	script, err := scriptArchive(spec.Command)
	if err != nil {
		return 0, err
	}
	if err := j.Docker.CopyToContainer(ctx, id, "/", script, container.CopyToContainerOptions{}); err != nil {
		return 0, fmt.Errorf("copy script: %w", err)
	}
	logSystem(j.log, slog.LevelInfo, "container created", slog.String("container_id", id[:12]))

	if err := j.Docker.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return 0, fmt.Errorf("container start: %w", err)
	}
	logSystem(j.log, slog.LevelInfo, "container started, waiting for exit")

	var logWG sync.WaitGroup
	logWG.Go(func() {
		if err := j.streamLogs(ctx, id); err != nil && ctx.Err() == nil {
			logSystem(j.log, slog.LevelInfo, "container log stream ended with error", slog.Any("err", err))
		}
	})
	code, err := waitForExit(ctx, j.Docker, id)
	logWG.Wait()
	return code, err
}

func (j *job) streamLogs(ctx context.Context, id string) error {
	logs, err := j.Docker.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true, Follow: true})
	if err != nil {
		return err
	}
	defer func() { _ = logs.Close() }()
	w := &lineLogWriter{jobLogger: j.log, source: sourceContainer}
	_, err = stdcopy.StdCopy(w, io.Discard, logs)
	w.flush()
	return err
}

type containerWaiter interface {
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
}

func waitForExit(ctx context.Context, cli containerWaiter, id string) (int64, error) {
	statusCh, errCh := cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("container wait: %w", err)
	case res := <-statusCh:
		if res.Error != nil && res.Error.Message != "" {
			return 0, errors.New(res.Error.Message)
		}
		return res.StatusCode, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// scriptArchive is the tar CopyToContainer extracts at /: the command as /lute/cmd.sh.
func scriptArchive(command string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	dir := strings.TrimPrefix(scriptPath[:strings.LastIndex(scriptPath, "/")], "/")
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: dir + "/", Mode: 0o755}); err != nil {
		return nil, err
	}
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: strings.TrimPrefix(scriptPath, "/"), Mode: 0o755, Size: int64(len(command))}); err != nil {
		return nil, err
	}
	if _, err := tw.Write([]byte(command)); err != nil {
		return nil, err
	}
	return &buf, tw.Close()
}

// shellPicker prefers bash but falls back to sh: alpine-based images (node:*-alpine and
// friends, used throughout the docs) ship only busybox sh.
const shellPicker = `if command -v bash >/dev/null 2>&1; then exec bash "$1"; else exec sh "$1"; fi`

// shellCommand's extra "lute" is $0 for the -c program; the script path is $1.
func shellCommand(scriptPath string) []string {
	return []string{"sh", "-c", shellPicker, "lute", scriptPath}
}

func envFromParams(params map[string]string) []string {
	if len(params) == 0 {
		return nil
	}
	out := make([]string, 0, len(params))
	for k, v := range params {
		out = append(out, k+"="+v)
	}
	return out
}
