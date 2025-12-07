package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// ExecCommand executes a single command in a container and returns the output
func ExecCommand(containerID, command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	// Parse command into parts
	cmdParts := strings.Fields(command)
	if len(cmdParts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	// Create exec configuration
	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"/bin/sh", "-c", command},
	}

	// Create exec instance
	execIDResp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec instance
	resp, err := cli.ContainerExecAttach(ctx, execIDResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Read output
	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Reader)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read output: %w", err)
	}

	// Check exit code
	inspectResp, err := cli.ContainerExecInspect(ctx, execIDResp.ID)
	if err != nil {
		return buf.String(), fmt.Errorf("command executed but failed to inspect: %w", err)
	}

	if inspectResp.ExitCode != 0 {
		return buf.String(), fmt.Errorf("command exited with code %d", inspectResp.ExitCode)
	}

	return buf.String(), nil
}

// ExecCommandWithTimeout executes a command with a custom timeout
func ExecCommandWithTimeout(containerID, command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer cli.Close()

	execConfig := types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"/bin/sh", "-c", command},
	}

	execIDResp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execIDResp.ID, types.ExecStartCheck{})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, resp.Reader)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read output: %w", err)
	}

	return buf.String(), nil
}

// ExecInteractive creates an interactive exec session
// Note: For full interactive shell, you'd need to handle TTY and raw terminal mode
func ExecInteractive(containerID string, command string) (io.Reader, io.Writer, io.Closer, error) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	execConfig := types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          []string{"/bin/sh", "-c", command},
	}

	execIDResp, err := cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		cli.Close()
		return nil, nil, nil, fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execIDResp.ID, types.ExecStartCheck{
		Tty: true,
	})
	if err != nil {
		cli.Close()
		return nil, nil, nil, fmt.Errorf("failed to attach to exec: %w", err)
	}

	// Return reader, writer, and the connection as closer
	return resp.Reader, resp.Conn, resp.Conn, nil
}

// ListProcesses lists running processes in a container
func ListProcesses(containerID string) (string, error) {
	return ExecCommand(containerID, "ps aux")
}

// GetEnvironmentVariables gets all environment variables from a container
func GetEnvironmentVariables(containerID string) (string, error) {
	return ExecCommand(containerID, "env | sort")
}

// GetFileSystem gets filesystem information
func GetFileSystem(containerID string) (string, error) {
	return ExecCommand(containerID, "df -h")
}

func GetNetworkInfo(containerID string) (*NetworkInfo, error) {
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

	networkInfo := &NetworkInfo{
		NetworkMode: string(inspect.HostConfig.NetworkMode),
		MacAddress:  inspect.NetworkSettings.MacAddress,
		Networks:    inspect.NetworkSettings.Networks,
		DNS:         inspect.HostConfig.DNS,
		DNSSearch:   inspect.HostConfig.DNSSearch,
		Hostname:    inspect.Config.Hostname,
	}

	// Get IP address from default network
	if len(inspect.NetworkSettings.Networks) > 0 {
		for _, network := range inspect.NetworkSettings.Networks {
			networkInfo.IPAddress = network.IPAddress
			networkInfo.Gateway = network.Gateway
			networkInfo.IPPrefixLen = network.IPPrefixLen
			break
		}
	}

	// Parse port mappings
	for containerPort, bindings := range inspect.NetworkSettings.Ports {
		for _, binding := range bindings {
			networkInfo.Ports = append(networkInfo.Ports, PortMapping{
				HostIP:        binding.HostIP,
				HostPort:      binding.HostPort,
				ContainerPort: containerPort.Port(),
				Protocol:      containerPort.Proto(),
			})
		}
	}

	return networkInfo, nil
}

// CheckHealth performs basic health checks
func CheckHealth(containerID string) (map[string]string, error) {
	checks := make(map[string]string)

	// Check if container is responsive
	_, err := ExecCommandWithTimeout(containerID, "echo 'alive'", 5*time.Second)
	if err != nil {
		checks["responsive"] = "❌ No"
	} else {
		checks["responsive"] = "✅ Yes"
	}

	// Check disk space
	diskOutput, err := ExecCommand(containerID, "df -h / | tail -1 | awk '{print $5}'")
	if err == nil {
		checks["disk_usage"] = strings.TrimSpace(diskOutput)
	} else {
		checks["disk_usage"] = "Unknown"
	}

	// Check memory
	memOutput, err := ExecCommand(containerID, "free -h | grep Mem | awk '{print $3\"/\"$2}'")
	if err == nil {
		checks["memory_usage"] = strings.TrimSpace(memOutput)
	} else {
		checks["memory_usage"] = "Unknown"
	}

	return checks, nil
}
