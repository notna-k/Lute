package runner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// runContainer pulls the image, creates and runs the container, waits for exit,
// captures logs, and cleans up. Returns the container exit code and any error.
func runContainer(ctx context.Context, cli *client.Client, dir string, spec *Spec, jobLogger *slog.Logger) (exitCode int64, err error) {
	if err := pullImage(ctx, cli, spec.Runtime, jobLogger); err != nil {
		return 0, err
	}

	containerID, err := createAndStartContainer(ctx, cli, dir, spec, jobLogger)
	if err != nil {
		return 0, err
	}
	defer cli.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})

	exitCode, err = waitForExit(ctx, cli, containerID)
	if err != nil {
		return 0, err
	}

	captureLogs(context.Background(), cli, containerID, jobLogger)
	return exitCode, nil
}

func pullImage(ctx context.Context, cli *client.Client, imageName string, jobLogger *slog.Logger) error {
	jobLogSystem(jobLogger, "pulling image", slog.String("image", imageName))
	resp, err := cli.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("image pull %q: %w", imageName, err)
	}
	defer resp.Close()
	_, _ = io.Copy(io.Discard, resp)
	jobLogSystem(jobLogger, "image pull ok", slog.String("image", imageName))
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
	jobLogSystem(jobLogger, "container created", slog.String("container_id", containerID[:12]))

	if err := cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("container start: %w", err)
	}
	jobLogSystem(jobLogger, "container started, waiting for exit")
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

func captureLogs(ctx context.Context, cli *client.Client, containerID string, jobLogger *slog.Logger) {
	logReader, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: false})
	if err != nil {
		return
	}
	defer logReader.Close()

	var outBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, logReader)

	sc := bufio.NewScanner(&outBuf)
	for sc.Scan() {
		line := sc.Text()
		logContainerLine(jobLogger, line)
	}
}

func logContainerLine(jobLogger *slog.Logger, line string) {
	args := []any{slog.String("source", logSourceContainer)}
	slog.Default().Log(context.Background(), slog.LevelInfo, line, args...)
	if jobLogger != nil {
		jobLogger.Log(context.Background(), slog.LevelInfo, line, args...)
	}
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
