package engine

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

const ProjectIDLabelKey = "com.last-deploy.project_id"

type Docker struct {
	cli *client.Client
}

func NewDocker() (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Docker{cli: cli}, nil
}

func (d *Docker) Close() error {
	return d.cli.Close()
}

func (d *Docker) BuildProjectImage(ctx context.Context, projectID, contextDir, dockerfilePath string) error {
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	if strings.TrimSpace(contextDir) == "" {
		return fmt.Errorf("context dir is required")
	}
	if strings.TrimSpace(dockerfilePath) == "" {
		dockerfilePath = "Dockerfile"
	}
	if !filepath.IsAbs(dockerfilePath) {
		dockerfilePath = filepath.ToSlash(filepath.Clean(dockerfilePath))
	}

	r, err := tarDirectory(contextDir)
	if err != nil {
		return err
	}
	defer r.Close()

	tag := imageTag(projectID)
	resp, err := d.cli.ImageBuild(ctx, r, client.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: dockerfilePath,
		Remove:     true,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return consumeDockerJSONMessages(resp.Body)
}

func (d *Docker) RunProjectContainer(ctx context.Context, projectID string, hostPort, containerPort int) error {
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	if hostPort <= 0 || hostPort > 65535 {
		return fmt.Errorf("invalid host_port: %d", hostPort)
	}
	if containerPort <= 0 || containerPort > 65535 {
		return fmt.Errorf("invalid container_port: %d", containerPort)
	}

	exposed, err := network.ParsePort(fmt.Sprintf("%d/tcp", containerPort))
	if err != nil {
		return err
	}
	labels := map[string]string{
		ProjectIDLabelKey: projectID,
	}
	name := containerName(projectID)

	cfg := &container.Config{
		Image:        imageTag(projectID),
		Labels:       labels,
		ExposedPorts: network.PortSet{exposed: struct{}{}},
	}
	hostCfg := &container.HostConfig{
		PortBindings: network.PortMap{
			exposed: []network.PortBinding{
				{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: strconv.Itoa(hostPort)},
			},
		},
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
	}

	_, _ = d.cli.ContainerRemove(ctx, name, client.ContainerRemoveOptions{Force: true})

	created, err := d.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     cfg,
		HostConfig: hostCfg,
		Name:       name,
	})
	if err != nil {
		return err
	}
	_, err = d.cli.ContainerStart(ctx, created.ID, client.ContainerStartOptions{})
	return err
}

func (d *Docker) RemoveProjectContainers(ctx context.Context, projectID string) error {
	containers, err := d.listProjectContainers(ctx, projectID)
	if err != nil {
		return err
	}
	for _, c := range containers {
		_, _ = d.cli.ContainerStop(ctx, c.ID, client.ContainerStopOptions{Timeout: ptrSeconds(10 * time.Second)})
		if _, err := d.cli.ContainerRemove(ctx, c.ID, client.ContainerRemoveOptions{Force: true}); err != nil {
			return err
		}
	}
	return nil
}

func (d *Docker) StopProjectContainers(ctx context.Context, projectID string, timeout time.Duration) (int, error) {
	containers, err := d.listProjectContainers(ctx, projectID)
	if err != nil {
		return 0, err
	}
	for _, c := range containers {
		if _, err := d.cli.ContainerStop(ctx, c.ID, client.ContainerStopOptions{Timeout: ptrSeconds(timeout)}); err != nil {
			return 0, err
		}
	}
	return len(containers), nil
}

func (d *Docker) StartProjectContainers(ctx context.Context, projectID string) (int, error) {
	containers, err := d.listProjectContainers(ctx, projectID)
	if err != nil {
		return 0, err
	}
	for _, c := range containers {
		if _, err := d.cli.ContainerStart(ctx, c.ID, client.ContainerStartOptions{}); err != nil {
			return 0, err
		}
	}
	return len(containers), nil
}

func (d *Docker) PauseProjectContainers(ctx context.Context, projectID string) (int, error) {
	containers, err := d.listProjectContainers(ctx, projectID)
	if err != nil {
		return 0, err
	}
	for _, c := range containers {
		if _, err := d.cli.ContainerPause(ctx, c.ID, client.ContainerPauseOptions{}); err != nil {
			return 0, err
		}
	}
	return len(containers), nil
}

func (d *Docker) UnpauseProjectContainers(ctx context.Context, projectID string) (int, error) {
	containers, err := d.listProjectContainers(ctx, projectID)
	if err != nil {
		return 0, err
	}
	for _, c := range containers {
		if _, err := d.cli.ContainerUnpause(ctx, c.ID, client.ContainerUnpauseOptions{}); err != nil {
			return 0, err
		}
	}
	return len(containers), nil
}

func (d *Docker) RemoveProjectImage(ctx context.Context, projectID string) error {
	_, err := d.cli.ImageRemove(ctx, imageTag(projectID), client.ImageRemoveOptions{
		Force:         true,
		PruneChildren: true,
	})
	return err
}

func (d *Docker) RemoveProjectNetworks(ctx context.Context, projectID string) error {
	if projectID == "" {
		return fmt.Errorf("project id is required")
	}
	// Compose 创建的网络前缀: last-deploy-{projectID}
	prefix := "last-deploy-" + projectID
	networks, err := d.cli.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		return err
	}
	for _, n := range networks.Items {
		if strings.HasPrefix(n.Name, prefix) {
			_, _ = d.cli.NetworkRemove(ctx, n.ID, client.NetworkRemoveOptions{})
		}
	}
	return nil
}

func (d *Docker) listProjectContainers(ctx context.Context, projectID string) ([]container.Summary, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}
	f := make(client.Filters).Add("label", fmt.Sprintf("%s=%s", ProjectIDLabelKey, projectID))
	res, err := d.cli.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: f})
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

func imageTag(projectID string) string {
	return "last-deploy:" + projectID
}

func containerName(projectID string) string {
	return "last-deploy-" + projectID
}

func tarDirectory(dir string) (io.ReadCloser, error) {
	pr, pw := io.Pipe()
	go func() {
		tw := tar.NewWriter(pw)
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if rel == "." {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			hdr.Name = rel
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				_, copyErr := io.Copy(tw, f)
				_ = f.Close()
				if copyErr != nil {
					return copyErr
				}
			}
			return nil
		})
		_ = tw.Close()
		_ = pw.CloseWithError(err)
	}()
	return pr, nil
}

type dockerJSONMessage struct {
	Stream      string `json:"stream"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

func consumeDockerJSONMessages(r io.Reader) error {
	dec := json.NewDecoder(r)
	for {
		var m dockerJSONMessage
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if m.ErrorDetail.Message != "" {
			return fmt.Errorf("docker build: %s", m.ErrorDetail.Message)
		}
		if m.Error != "" {
			return fmt.Errorf("docker build: %s", m.Error)
		}
	}
}

func ptrSeconds(d time.Duration) *int {
	sec := int(d.Round(time.Second).Seconds())
	if sec < 0 {
		sec = 0
	}
	return &sec
}
