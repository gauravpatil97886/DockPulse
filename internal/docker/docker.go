package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type ContainerInfo struct {
	ID      string
	Name    string
	Status  string
	Image   string
	Created string
	Ports   string
	State   string
}

type ContainerStats struct {
	CPUPerc  string
	MemUsage string
	MemPerc  string
	NetIO    string
	BlockIO  string
	PIDs     string
}

// getClient creates a new Docker client
func getClient() (*client.Client, error) {
	return client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
}

// CheckDockerConnection verifies Docker daemon is accessible
func CheckDockerConnection() error {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return err
	}
	defer cli.Close()

	_, err = cli.Ping(context.Background())
	if err != nil {
		return fmt.Errorf("docker not running: %v", err)
	}
	return nil
}

// ListContainers returns all containers (running and stopped)
func ListContainers() ([]ContainerInfo, error) {
	cli, err := getClient()
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var result []ContainerInfo
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0][1:] // Remove leading slash
		}

		// Format ports
		ports := formatPorts(c.Ports)

		// Format created time
		created := time.Unix(c.Created, 0).Format("2006-01-02 15:04:05")

		info := ContainerInfo{
			ID:      c.ID,
			Name:    name,
			Status:  c.Status,
			Image:   c.Image,
			Created: created,
			Ports:   ports,
			State:   c.State,
		}
		result = append(result, info)
	}

	return result, nil
}

// StartContainer starts a stopped container
func StartContainer(containerID string) error {
	cli, err := getClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := context.Background()
	return cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
}

// StopContainer stops a running container
func StopContainer(containerID string) error {
	cli, err := getClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := context.Background()
	timeout := 10 // seconds
	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}
	return cli.ContainerStop(ctx, containerID, stopOptions)
}

// RestartContainer restarts a container
func RestartContainer(containerID string) error {
	cli, err := getClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := context.Background()
	timeout := 10 // seconds
	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}
	return cli.ContainerRestart(ctx, containerID, stopOptions)
}

// RemoveContainer removes a container (force removes if running)
func RemoveContainer(containerID string) error {
	cli, err := getClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := context.Background()
	return cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
}

// StreamLogs streams container logs
func StreamLogs(containerID string) (io.ReadCloser, error) {
	cli, err := getClient()
	if err != nil {
		return nil, err
	}

	return cli.ContainerLogs(context.Background(), containerID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Tail:       "500", // Last 500 lines
	})
}

// GetStats retrieves live container statistics
func GetStats(containerID string) (*ContainerStats, error) {
	cli, err := getClient()
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	ctx := context.Background()
	stats, err := cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, err
	}
	defer stats.Body.Close()

	var v types.StatsJSON
	if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
		return nil, err
	}

	// Calculate CPU percentage
	cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(v.CPUStats.SystemUsage - v.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	// Calculate memory usage
	memUsage := float64(v.MemoryStats.Usage)
	memLimit := float64(v.MemoryStats.Limit)
	memPercent := 0.0
	if memLimit > 0 {
		memPercent = (memUsage / memLimit) * 100.0
	}

	// Calculate network I/O
	var netRx, netTx uint64
	for _, network := range v.Networks {
		netRx += network.RxBytes
		netTx += network.TxBytes
	}

	// Calculate block I/O
	var blockRead, blockWrite uint64
	for _, bio := range v.BlkioStats.IoServiceBytesRecursive {
		if bio.Op == "Read" {
			blockRead += bio.Value
		} else if bio.Op == "Write" {
			blockWrite += bio.Value
		}
	}

	return &ContainerStats{
		CPUPerc:  fmt.Sprintf("%.2f%%", cpuPercent),
		MemUsage: fmt.Sprintf("%s / %s", formatBytes(uint64(memUsage)), formatBytes(uint64(memLimit))),
		MemPerc:  fmt.Sprintf("%.2f%%", memPercent),
		NetIO:    fmt.Sprintf("↓ %s / ↑ %s", formatBytes(netRx), formatBytes(netTx)),
		BlockIO:  fmt.Sprintf("↓ %s / ↑ %s", formatBytes(blockRead), formatBytes(blockWrite)),
		PIDs:     fmt.Sprintf("%d", v.PidsStats.Current),
	}, nil
}

// InspectContainer returns detailed container information
func InspectContainer(containerID string) (string, error) {
	cli, err := getClient()
	if err != nil {
		return "", err
	}
	defer cli.Close()

	ctx := context.Background()
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}

	// Format the inspection data
	result := fmt.Sprintf(`Container Details:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[cyan]Basic Information[white]
  ID:           %s
  Name:         %s
  Image:        %s
  Created:      %s
  Status:       %s
  
[cyan]State[white]
  Running:      %v
  Paused:       %v
  Restarting:   %v
  PID:          %d
  Exit Code:    %d
  Started At:   %s
  Finished At:  %s

[cyan]Network Settings[white]
  IP Address:   %s
  Gateway:      %s
  MAC Address:  %s
  Ports:        %v

[cyan]Resource Limits[white]
  Memory:       %d MB
  CPU Shares:   %d
  
[cyan]Mounts[white]`,
		inspect.ID[:12],
		inspect.Name[1:],
		inspect.Config.Image,
		inspect.Created,
		inspect.State.Status,
		inspect.State.Running,
		inspect.State.Paused,
		inspect.State.Restarting,
		inspect.State.Pid,
		inspect.State.ExitCode,
		inspect.State.StartedAt,
		inspect.State.FinishedAt,
		inspect.NetworkSettings.IPAddress,
		inspect.NetworkSettings.Gateway,
		inspect.NetworkSettings.MacAddress,
		inspect.NetworkSettings.Ports,
		inspect.HostConfig.Memory/1024/1024,
		inspect.HostConfig.CPUShares,
	)

	// Add mounts
	for _, mount := range inspect.Mounts {
		result += fmt.Sprintf("\n  %s → %s (%s)", mount.Source, mount.Destination, mount.Type)
	}

	// Add environment variables
	result += "\n\n[cyan]Environment Variables[white]"
	for _, env := range inspect.Config.Env {
		result += fmt.Sprintf("\n  %s", env)
	}

	// Add labels
	if len(inspect.Config.Labels) > 0 {
		result += "\n\n[cyan]Labels[white]"
		for key, value := range inspect.Config.Labels {
			result += fmt.Sprintf("\n  %s: %s", key, value)
		}
	}

	return result, nil
}

// ExecCommand executes a command in a running container

// PauseContainer pauses a running container
func PauseContainer(containerID string) error {
	cli, err := getClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := context.Background()
	return cli.ContainerPause(ctx, containerID)
}

// UnpauseContainer unpauses a paused container
func UnpauseContainer(containerID string) error {
	cli, err := getClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := context.Background()
	return cli.ContainerUnpause(ctx, containerID)
}

// Helper function to format bytes
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Helper function to format ports
func formatPorts(ports []types.Port) string {
	if len(ports) == 0 {
		return "none"
	}
	result := ""
	for i, port := range ports {
		if i > 0 {
			result += ", "
		}
		if port.PublicPort != 0 {
			result += fmt.Sprintf("%d->%d/%s", port.PublicPort, port.PrivatePort, port.Type)
		} else {
			result += fmt.Sprintf("%d/%s", port.PrivatePort, port.Type)
		}
	}
	return result
}

// GetDockerInfo returns Docker system information
func GetDockerInfo() (string, error) {
	cli, err := getClient()
	if err != nil {
		return "", err
	}
	defer cli.Close()

	ctx := context.Background()
	info, err := cli.Info(ctx)
	if err != nil {
		return "", err
	}

	result := fmt.Sprintf(`Docker System Information:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Containers:    %d (Running: %d, Paused: %d, Stopped: %d)
Images:        %d
Server Version: %s
Storage Driver: %s
Kernel Version: %s
Operating System: %s
CPUs:          %d
Total Memory:   %.2f GB
Docker Root Dir: %s`,
		info.Containers,
		info.ContainersRunning,
		info.ContainersPaused,
		info.ContainersStopped,
		info.Images,
		info.ServerVersion,
		info.Driver,
		info.KernelVersion,
		info.OperatingSystem,
		info.NCPU,
		float64(info.MemTotal)/1024/1024/1024,
		info.DockerRootDir,
	)

	return result, nil
}
