// dockerEnhancements.go - Enhanced Docker API Functions
// Add these to your internal/docker package

package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// NetworkInfo contains detailed network information
type NetworkInfo struct {
	NetworkMode string
	IPAddress   string
	IPPrefixLen int
	Gateway     string
	MacAddress  string
	Networks    map[string]*network.EndpointSettings
	Ports       []PortMapping
	DNS         []string
	DNSSearch   []string
	Hostname    string
}

// PortMapping represents a port mapping
type PortMapping struct {
	HostIP        string
	HostPort      string
	ContainerPort string
	Protocol      string
}

// VolumeDetail contains volume information
type VolumeDetail struct {
	Name       string
	Driver     string
	Mountpoint string
	Labels     map[string]string
	Scope      string
	Options    map[string]string
	UsageData  *VolumeUsage
	CreatedAt  time.Time
}

// VolumeUsage contains volume usage statistics
type VolumeUsage struct {
	Size     int64
	RefCount int64
}

// PerformanceMetrics contains detailed performance data
type PerformanceMetrics struct {
	CPUStats     CPUMetrics
	MemoryStats  MemoryMetrics
	NetworkStats NetworkMetrics
	BlockIOStats BlockIOMetrics
	ProcessStats ProcessMetrics
	Timestamp    time.Time
}

type CPUMetrics struct {
	TotalUsage     uint64
	PerCPUUsage    []uint64
	SystemCPUUsage uint64
	OnlineCPUs     uint32
	ThrottlingData ThrottlingData
}

type ThrottlingData struct {
	Periods          uint64
	ThrottledPeriods uint64
	ThrottledTime    uint64
}

type MemoryMetrics struct {
	Usage           uint64
	MaxUsage        uint64
	Limit           uint64
	Cache           uint64
	RSS             uint64
	Swap            uint64
	WorkingSet      uint64
	PageFaults      uint64
	MajorPageFaults uint64
}

type NetworkMetrics struct {
	RxBytes   uint64
	RxPackets uint64
	RxErrors  uint64
	RxDropped uint64
	TxBytes   uint64
	TxPackets uint64
	TxErrors  uint64
	TxDropped uint64
}

type BlockIOMetrics struct {
	ReadBytes  uint64
	WriteBytes uint64
	ReadOps    uint64
	WriteOps   uint64
}

type ProcessMetrics struct {
	ProcessCount        int
	ThreadCount         int
	FileDescriptorCount int
}

// GetNetworkInfo retrieves detailed network information

// GetVolumeDetails retrieves detailed volume information
func GetVolumeDetails(containerID string) ([]VolumeDetail, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	ctx := context.Background()
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	var volumes []VolumeDetail

	// Get mount information
	for _, mount := range inspect.Mounts {
		volume := VolumeDetail{
			Name:       mount.Name,
			Driver:     mount.Driver,
			Mountpoint: mount.Source,
		}

		// If it's a named volume, get additional details
		if mount.Type == "volume" && mount.Name != "" {
			volInfo, err := cli.VolumeInspect(ctx, mount.Name)
			if err == nil {
				volume.Labels = volInfo.Labels
				volume.Scope = volInfo.Scope
				volume.Options = volInfo.Options
				if volInfo.CreatedAt != "" {
					volume.CreatedAt, _ = time.Parse(time.RFC3339, volInfo.CreatedAt)
				}

				// Get usage data
				volume.UsageData = &VolumeUsage{
					Size:     volInfo.UsageData.Size,
					RefCount: volInfo.UsageData.RefCount,
				}
			}
		}

		volumes = append(volumes, volume)
	}

	return volumes, nil
}

// GetPerformanceMetrics retrieves comprehensive performance metrics
func GetPerformanceMetrics(containerID string) (*PerformanceMetrics, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
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

	var containerStats types.StatsJSON
	if err := json.NewDecoder(stats.Body).Decode(&containerStats); err != nil {
		return nil, err
	}

	metrics := &PerformanceMetrics{
		Timestamp: time.Now(),
	}

	// CPU Metrics
	metrics.CPUStats = CPUMetrics{
		TotalUsage:     containerStats.CPUStats.CPUUsage.TotalUsage,
		PerCPUUsage:    containerStats.CPUStats.CPUUsage.PercpuUsage,
		SystemCPUUsage: containerStats.CPUStats.SystemUsage,
		OnlineCPUs:     containerStats.CPUStats.OnlineCPUs,
		ThrottlingData: ThrottlingData{
			Periods:          containerStats.CPUStats.ThrottlingData.Periods,
			ThrottledPeriods: containerStats.CPUStats.ThrottlingData.ThrottledPeriods,
			ThrottledTime:    containerStats.CPUStats.ThrottlingData.ThrottledTime,
		},
	}

	// Memory Metrics
	metrics.MemoryStats = MemoryMetrics{
		Usage:           containerStats.MemoryStats.Usage,
		MaxUsage:        containerStats.MemoryStats.MaxUsage,
		Limit:           containerStats.MemoryStats.Limit,
		Cache:           containerStats.MemoryStats.Stats["cache"],
		RSS:             containerStats.MemoryStats.Stats["rss"],
		Swap:            containerStats.MemoryStats.Stats["swap"],
		WorkingSet:      containerStats.MemoryStats.Stats["working_set"],
		PageFaults:      containerStats.MemoryStats.Stats["pgfault"],
		MajorPageFaults: containerStats.MemoryStats.Stats["pgmajfault"],
	}

	// Network Metrics
	for _, netStats := range containerStats.Networks {
		metrics.NetworkStats.RxBytes += netStats.RxBytes
		metrics.NetworkStats.RxPackets += netStats.RxPackets
		metrics.NetworkStats.RxErrors += netStats.RxErrors
		metrics.NetworkStats.RxDropped += netStats.RxDropped
		metrics.NetworkStats.TxBytes += netStats.TxBytes
		metrics.NetworkStats.TxPackets += netStats.TxPackets
		metrics.NetworkStats.TxErrors += netStats.TxErrors
		metrics.NetworkStats.TxDropped += netStats.TxDropped
	}

	// Block I/O Metrics
	for _, ioStat := range containerStats.BlkioStats.IoServiceBytesRecursive {
		if ioStat.Op == "Read" {
			metrics.BlockIOStats.ReadBytes += ioStat.Value
		} else if ioStat.Op == "Write" {
			metrics.BlockIOStats.WriteBytes += ioStat.Value
		}
	}

	for _, ioStat := range containerStats.BlkioStats.IoServicedRecursive {
		if ioStat.Op == "Read" {
			metrics.BlockIOStats.ReadOps += ioStat.Value
		} else if ioStat.Op == "Write" {
			metrics.BlockIOStats.WriteOps += ioStat.Value
		}
	}

	// Process Metrics
	metrics.ProcessStats.ProcessCount = int(containerStats.PidsStats.Current)

	return metrics, nil
}

type HijackedStream struct {
	reader io.Reader
	closer func() error
}

func (h *HijackedStream) Read(p []byte) (int, error) {
	return h.reader.Read(p)
}

func (h *HijackedStream) Close() error {
	return h.closer()
}

// ExecCommandStream executes a command and returns output stream
func ExecCommandStream(containerID string, cmd []string) (io.ReadCloser, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}

	execID, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return nil, err
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, types.ExecStartCheck{})
	if err != nil {
		return nil, err
	}

	// ✅ Wrap resp.Close() so it matches func() error signature
	stream := &HijackedStream{
		reader: resp.Reader,
		closer: func() error {
			resp.Close()
			return nil
		},
	}

	return stream, nil
}

// GetProcessList returns list of processes in container
func GetProcessList(containerID string) ([]ProcessInfo, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	ctx := context.Background()
	processes, err := cli.ContainerTop(ctx, containerID, []string{})
	if err != nil {
		return nil, err
	}

	var processList []ProcessInfo
	for _, proc := range processes.Processes {
		if len(proc) >= 8 {
			processList = append(processList, ProcessInfo{
				PID:     proc[1],
				User:    proc[0],
				CPU:     proc[2],
				Memory:  proc[3],
				VSZ:     proc[4],
				RSS:     proc[5],
				TTY:     proc[6],
				Stat:    proc[7],
				Command: strings.Join(proc[8:], " "),
			})
		}
	}

	return processList, nil
}

type ProcessInfo struct {
	PID     string
	User    string
	CPU     string
	Memory  string
	VSZ     string
	RSS     string
	TTY     string
	Stat    string
	Command string
}

// GetContainerLogs retrieves logs with options
func GetContainerLogs(containerID string, since time.Time, tail string) (io.ReadCloser, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      since.Format(time.RFC3339),
		Tail:       tail,
		Timestamps: true,
		Follow:     false,
	}

	logs, err := cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return nil, err
	}

	return logs, nil
}

// GetNetworkConnections retrieves active network connections
func GetNetworkConnections(containerID string) ([]NetworkConnection, error) {
	// Execute netstat inside container
	output, err := ExecCommand(containerID, "netstat -tunp")
	if err != nil {
		// Try ss if netstat is not available
		output, err = ExecCommand(containerID, "ss -tunp")
		if err != nil {
			return nil, err
		}
	}

	var connections []NetworkConnection
	lines := strings.Split(output, "\n")

	for _, line := range lines[2:] { // Skip header lines
		fields := strings.Fields(line)
		if len(fields) >= 5 {
			connections = append(connections, NetworkConnection{
				Proto:      fields[0],
				LocalAddr:  fields[3],
				RemoteAddr: fields[4],
				State:      getConnectionState(fields),
			})
		}
	}

	return connections, nil
}

type NetworkConnection struct {
	Proto      string
	LocalAddr  string
	RemoteAddr string
	State      string
	PID        string
}

func getConnectionState(fields []string) string {
	if len(fields) >= 6 {
		return fields[5]
	}
	return "UNKNOWN"
}

// CheckContainerHealth performs comprehensive health check
func CheckContainerHealth(containerID string) (map[string]string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	ctx := context.Background()
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	health := make(map[string]string)

	// Container state
	if inspect.State.Running {
		health["status"] = "running"
	} else {
		health["status"] = "stopped"
	}

	// Health check status
	if inspect.State.Health != nil {
		health["health_status"] = inspect.State.Health.Status
		if len(inspect.State.Health.Log) > 0 {
			health["health_output"] = inspect.State.Health.Log[len(inspect.State.Health.Log)-1].Output
		}
	} else {
		health["health_status"] = "no_healthcheck"
	}

	// Get resource usage
	stats, err := GetPerformanceMetrics(containerID)
	if err == nil {
		cpuPercent := calculateCPUPercentage(stats)
		memPercent := float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit) * 100

		health["cpu_usage"] = fmt.Sprintf("%.2f%%", cpuPercent)
		health["memory_usage"] = fmt.Sprintf("%.2f%%", memPercent)

		// Health assessment
		if cpuPercent > 90 {
			health["cpu_health"] = "critical"
		} else if cpuPercent > 70 {
			health["cpu_health"] = "warning"
		} else {
			health["cpu_health"] = "healthy"
		}

		if memPercent > 90 {
			health["memory_health"] = "critical"
		} else if memPercent > 80 {
			health["memory_health"] = "warning"
		} else {
			health["memory_health"] = "healthy"
		}
	}

	// Restart count
	health["restart_count"] = fmt.Sprintf("%d", inspect.RestartCount)

	// Exit code
	health["exit_code"] = fmt.Sprintf("%d", inspect.State.ExitCode)

	// OOM Killed
	health["oom_killed"] = fmt.Sprintf("%t", inspect.State.OOMKilled)

	return health, nil
}

func calculateCPUPercentage(metrics *PerformanceMetrics) float64 {
	cpuDelta := float64(metrics.CPUStats.TotalUsage)
	systemDelta := float64(metrics.CPUStats.SystemCPUUsage)

	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent := (cpuDelta / systemDelta) * float64(metrics.CPUStats.OnlineCPUs) * 100.0
		return cpuPercent
	}
	return 0.0
}

// CreateSnapshot creates a container snapshot
func CreateSnapshot(containerID string, imageName string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := context.Background()

	// Commit container to image
	commitOptions := types.ContainerCommitOptions{
		Reference: imageName,
		Comment:   "Snapshot created by DevOps Dashboard",
		Author:    "DevOps Dashboard",
	}

	_, err = cli.ContainerCommit(ctx, containerID, commitOptions)
	return err
}

// PruneContainers removes stopped containers
func PruneContainers() error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := context.Background()
	_, err = cli.ContainersPrune(ctx, filters.Args{})
	return err
}

// PruneVolumes removes unused volumes
func PruneVolumes() error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := context.Background()
	_, err = cli.VolumesPrune(ctx, filters.Args{})
	return err
}
