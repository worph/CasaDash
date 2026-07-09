// Package dockerx wraps the Docker Engine API for the operations CasaDash needs:
// discovering compose projects, per-project lifecycle, and an event stream for
// live status. The heavier compose install engine is layered on top separately.
package dockerx

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// Compose label keys set by `docker compose`.
const (
	labelProject    = "com.docker.compose.project"
	labelService    = "com.docker.compose.service"
	labelWorkingDir = "com.docker.compose.project.working_dir"
	labelConfigFile = "com.docker.compose.project.config_files"
)

// Client is a thin wrapper over the Docker API client.
type Client struct {
	cli *client.Client
}

// New connects to the Docker engine (honouring DOCKER_HOST, default socket).
func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

// Ping checks connectivity to the daemon.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	return err
}

// Port is a published container port mapping.
type Port struct {
	Private uint16
	Public  uint16
}

// Container is a compose-managed container, flattened for our needs.
type Container struct {
	ID          string
	Project     string
	Service     string
	WorkingDir  string
	ConfigFiles string
	State       string // running, exited, created, ...
	Ports       []Port
}

// ListProjectContainers returns all containers that belong to a compose project.
func (c *Client) ListProjectContainers(ctx context.Context) ([]Container, error) {
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	out := make([]Container, 0, len(list))
	for _, ct := range list {
		project := ct.Labels[labelProject]
		if project == "" {
			continue
		}
		var ports []Port
		for _, p := range ct.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, Port{Private: p.PrivatePort, Public: p.PublicPort})
			}
		}
		out = append(out, Container{
			ID:          ct.ID,
			Project:     project,
			Service:     ct.Labels[labelService],
			WorkingDir:  ct.Labels[labelWorkingDir],
			ConfigFiles: ct.Labels[labelConfigFile],
			State:       ct.State,
			Ports:       ports,
		})
	}
	return out, nil
}

func (c *Client) projectContainerIDs(ctx context.Context, project string) ([]string, error) {
	f := filters.NewArgs(filters.Arg("label", labelProject+"="+project))
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list))
	for _, ct := range list {
		ids = append(ids, ct.ID)
	}
	if len(ids) == 0 {
		return nil, errors.New("no containers for project " + project)
	}
	return ids, nil
}

// StartProject starts every container in a project.
func (c *Client) StartProject(ctx context.Context, project string) error {
	ids, err := c.projectContainerIDs(ctx, project)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// StopProject stops every container in a project.
func (c *Client) StopProject(ctx context.Context, project string) error {
	ids, err := c.projectContainerIDs(ctx, project)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := c.cli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// RestartProject restarts every container in a project.
func (c *Client) RestartProject(ctx context.Context, project string) error {
	ids, err := c.projectContainerIDs(ctx, project)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := c.cli.ContainerRestart(ctx, id, container.StopOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// RemoveProject stops and removes every container in a project (best-effort),
// deleting anonymous volumes. Named app data under DATA_ROOT is left intact.
func (c *Client) RemoveProject(ctx context.Context, project string) error {
	ids, err := c.projectContainerIDs(ctx, project)
	if err != nil {
		return err
	}
	for _, id := range ids {
		_ = c.cli.ContainerStop(ctx, id, container.StopOptions{})
		if err := c.cli.ContainerRemove(ctx, id, container.RemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		}); err != nil {
			return err
		}
	}
	return nil
}

// PullImage pulls an image, invoking onProgress with an aggregate download
// percentage (0-100 across all layers) and the latest status string.
func (c *Client) PullImage(ctx context.Context, ref string, onProgress func(pct float64, status string)) error {
	rc, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()

	type detail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	}
	type msg struct {
		Status         string `json:"status"`
		ID             string `json:"id"`
		ProgressDetail detail `json:"progressDetail"`
		Error          string `json:"error"`
	}
	layers := map[string]detail{}
	dec := json.NewDecoder(rc)
	for {
		var m msg
		if err := dec.Decode(&m); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if m.Error != "" {
			return errors.New(m.Error)
		}
		if m.ID != "" && m.ProgressDetail.Total > 0 {
			layers[m.ID] = m.ProgressDetail
		}
		if onProgress != nil {
			var cur, tot int64
			for _, d := range layers {
				cur += d.Current
				tot += d.Total
			}
			pct := 0.0
			if tot > 0 {
				pct = float64(cur) / float64(tot) * 100
			}
			onProgress(pct, m.Status)
		}
	}
}

// FirstContainerID returns one container id for a project (the log/stats target).
func (c *Client) FirstContainerID(ctx context.Context, project string) (string, error) {
	ids, err := c.projectContainerIDs(ctx, project)
	if err != nil {
		return "", err
	}
	return ids[0], nil
}

// ContainerLogs returns the (multiplexed) log stream for a container.
func (c *Client) ContainerLogs(ctx context.Context, id, tail string, follow bool) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       tail,
	})
}

// ContainerStatsStream returns a streaming stats reader for a container.
func (c *Client) ContainerStatsStream(ctx context.Context, id string) (io.ReadCloser, error) {
	resp, err := c.cli.ContainerStats(ctx, id, true)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// WatchContainers calls onEvent (debounced by the caller) whenever a container
// lifecycle event occurs, until ctx is cancelled.
func (c *Client) WatchContainers(ctx context.Context, onEvent func()) {
	f := filters.NewArgs(filters.Arg("type", string(events.ContainerEventType)))
	msgs, errs := c.cli.Events(ctx, events.ListOptions{Filters: f})
	for {
		select {
		case <-ctx.Done():
			return
		case <-msgs:
			onEvent()
		case err := <-errs:
			if err != nil {
				// Stream broke (daemon restart etc). Give up; caller may restart.
				return
			}
		}
	}
}
