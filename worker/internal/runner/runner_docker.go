package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

func runContainer(ctx context.Context, cli *client.Client, dir string, spec *Spec, jobLogger *slog.Logger) (exitCode int64, err error) {
	if err := pullImage(ctx, cli, spec.Runtime, jobLogger); err != nil {
		return 0, err
	}

	containerID, err := createAndStartContainer(ctx, cli, dir, spec, jobLogger)
	if err != nil {
		return 0, err
	}
	defer cli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})

	var logWG sync.WaitGroup
	logWG.Go(func() {
		if err := streamContainerLogs(ctx, cli, containerID, jobLogger); err != nil && ctx.Err() == nil {
			logSystem(jobLogger, slog.LevelInfo, "container log stream ended with error", slog.Any("err", err))
		}
	})

	exitCode, err = waitForExit(ctx, cli, containerID)
	logWG.Wait()
	if err != nil {
		return 0, err
	}

	return exitCode, nil
}

func pullImage(ctx context.Context, cli *client.Client, imageName string, jobLogger *slog.Logger) error {
	logSystem(jobLogger, slog.LevelInfo, "pulling image", slog.String("image", imageName))

	resp, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("image pull %q: %w", imageName, err)
	}
	defer resp.Close()
	_, _ = io.Copy(io.Discard, resp)

	logSystem(jobLogger, slog.LevelInfo, "image pull ok", slog.String("image", imageName))
	return nil
}

func createAndStartContainer(ctx context.Context, cli *client.Client, dir string, spec *Spec, jobLogger *slog.Logger) (string, error) {
	bind := dir + ":" + workspaceMount
	cfg := &container.Config{
		Image: spec.Runtime,
		Cmd:   []string{"bash", filepath.Join(workspaceMount, commandScriptName)},
		Env:   envFromParams(spec.RequestParams),
	}
	hostConfig := &container.HostConfig{Binds: []string{bind}}

	createResp, err := cli.ContainerCreate(ctx, cfg, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}
	containerID := createResp.ID

	logSystem(jobLogger, slog.LevelInfo, "container created", slog.String("container_id", containerID[:12]))

	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("container start: %w", err)
	}
	logSystem(jobLogger, slog.LevelInfo, "container started, waiting for exit")
	return containerID, nil
}

func waitForExit(ctx context.Context, cli *client.Client, containerID string) (int64, error) {
	statusCh, errCh := cli.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return 0, fmt.Errorf("container wait: %w", err)
		}
		return 0, nil
	case res := <-statusCh:
		return res.StatusCode, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// lineLogWriter buffers stdout chunks and emits one slog record per line (same as former post-hoc scanner).
type lineLogWriter struct {
	jobLogger *slog.Logger
	buf       []byte
}

func (w *lineLogWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		w.jobLogger.Info(line, slog.String("source", LogSourceContainer))
	}
	return len(p), nil
}

func (w *lineLogWriter) flush() {
	if len(w.buf) == 0 {
		return
	}
	w.jobLogger.Info(string(w.buf), slog.String("source", LogSourceContainer))
	w.buf = nil
}

func streamContainerLogs(ctx context.Context, cli *client.Client, containerID string, jobLogger *slog.Logger) error {
	logReader, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: false,
		Follow:     true,
	})
	if err != nil {
		return err
	}
	defer logReader.Close()

	lw := &lineLogWriter{jobLogger: jobLogger}
	_, err = stdcopy.StdCopy(lw, io.Discard, logReader)
	lw.flush()
	return err
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
