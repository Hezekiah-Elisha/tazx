package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

// ContainerInfo holds read-only metadata for a Docker container.
type ContainerInfo struct {
	ID      string    `json:"id"`
	Names   string    `json:"names"`
	Image   string    `json:"image"`
	Status  string    `json:"status"`
	State   string    `json:"state"`
	Ports   string    `json:"ports"`
	Created time.Time `json:"created"`
}

// ContainerStats holds live resource usage statistics for a running container.
type ContainerStats struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Image           string  `json:"image"`
	CPUPercent      float64 `json:"cpu_percent"`
	MemUsedBytes    uint64  `json:"mem_used_bytes"`
	MemLimitBytes   uint64  `json:"mem_limit_bytes"`
	MemPercent      float64 `json:"mem_percent"`
	NetRxBytes      uint64  `json:"net_rx_bytes"`
	NetTxBytes      uint64  `json:"net_tx_bytes"`
	BlockReadBytes  uint64  `json:"block_read_bytes"`
	BlockWriteBytes uint64  `json:"block_write_bytes"`
	PIDs            uint64  `json:"pids"`
}

func withDefaultTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), d)
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

func formatContainerNames(rawNames []string) string {
	if len(rawNames) == 0 {
		return ""
	}
	cleaned := make([]string, 0, len(rawNames))
	for _, n := range rawNames {
		trimmed := strings.TrimPrefix(n, "/")
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, ", ")
}

// GetContainers retrieves read-only container listings from the Docker daemon.
func GetContainers(ctx context.Context, all bool) ([]ContainerInfo, error) {
	callCtx, cancel := withDefaultTimeout(ctx, 5*time.Second)
	defer cancel()

	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("Docker daemon not reachable: %w", err)
	}
	defer apiClient.Close()

	result, err := apiClient.ContainerList(callCtx, client.ContainerListOptions{
		All: all,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	list := make([]ContainerInfo, 0, len(result.Items))
	for _, ctr := range result.Items {
		shortID := ctr.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		portStrs := make([]string, 0, len(ctr.Ports))
		for _, p := range ctr.Ports {
			if p.PublicPort != 0 {
				portStrs = append(portStrs, fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			} else {
				portStrs = append(portStrs, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
			}
		}

		list = append(list, ContainerInfo{
			ID:      shortID,
			Names:   formatContainerNames(ctr.Names),
			Image:   ctr.Image,
			Status:  ctr.Status,
			State:   string(ctr.State),
			Ports:   strings.Join(portStrs, ", "),
			Created: time.Unix(ctr.Created, 0),
		})
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Names == list[j].Names {
			return list[i].ID < list[j].ID
		}
		return list[i].Names < list[j].Names
	})

	return list, nil
}

// GetContainerStats retrieves live CPU, memory, network, block I/O, and PID stats
// for running containers matching filter (ID prefix or name substring, case-insensitive).
func GetContainerStats(ctx context.Context, filter string) ([]ContainerStats, error) {
	callCtx, cancel := withDefaultTimeout(ctx, 15*time.Second)
	defer cancel()

	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("Docker daemon not reachable: %w", err)
	}
	defer apiClient.Close()

	listRes, err := apiClient.ContainerList(callCtx, client.ContainerListOptions{
		All: false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list running containers: %w", err)
	}

	filterLower := strings.ToLower(strings.TrimSpace(filter))
	type targetCtr struct {
		id    string
		short string
		name  string
		image string
	}
	var targets []targetCtr
	for _, ctr := range listRes.Items {
		shortID := ctr.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		name := formatContainerNames(ctr.Names)
		if filterLower != "" {
			matchID := strings.HasPrefix(strings.ToLower(ctr.ID), filterLower) || strings.HasPrefix(strings.ToLower(shortID), filterLower)
			matchName := strings.Contains(strings.ToLower(name), filterLower)
			if !matchID && !matchName {
				continue
			}
		}
		targets = append(targets, targetCtr{
			id:    ctr.ID,
			short: shortID,
			name:  name,
			image: ctr.Image,
		})
	}

	if len(targets) == 0 {
		return nil, nil
	}

	const maxConcurrency = 5
	sem := make(chan struct{}, maxConcurrency)
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		stats   []ContainerStats
		lastErr error
	)

	for _, t := range targets {
		wg.Add(1)
		go func(tc targetCtr) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-callCtx.Done():
				return
			}

			reqCtx, reqCancel := context.WithTimeout(callCtx, 8*time.Second)
			defer reqCancel()

			res, err := apiClient.ContainerStats(reqCtx, tc.id, client.ContainerStatsOptions{
				Stream:                false,
				IncludePreviousSample: true,
			})
			if err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
				return
			}
			defer res.Body.Close()

			var raw container.StatsResponse
			if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
				mu.Lock()
				lastErr = err
				mu.Unlock()
				return
			}

			memUsed, memLimit, memPct := CalculateMemoryUsage(&raw)
			netRx, netTx := CalculateNetworkIO(&raw)
			blkRead, blkWrite := CalculateBlockIO(&raw)

			mu.Lock()
			stats = append(stats, ContainerStats{
				ID:              tc.short,
				Name:            tc.name,
				Image:           tc.image,
				CPUPercent:      CalculateCPUPercent(&raw),
				MemUsedBytes:    memUsed,
				MemLimitBytes:   memLimit,
				MemPercent:      memPct,
				NetRxBytes:      netRx,
				NetTxBytes:      netTx,
				BlockReadBytes:  blkRead,
				BlockWriteBytes: blkWrite,
				PIDs:            raw.PidsStats.Current,
			})
			mu.Unlock()
		}(t)
	}
	wg.Wait()

	if len(stats) == 0 && lastErr != nil {
		return nil, fmt.Errorf("failed to collect container statistics: %w", lastErr)
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Name == stats[j].Name {
			return stats[i].ID < stats[j].ID
		}
		return stats[i].Name < stats[j].Name
	})

	return stats, nil
}

// CalculateCPUPercent computes container CPU utilization percentage following Docker CLI semantics.
func CalculateCPUPercent(s *container.StatsResponse) float64 {
	if s == nil {
		return 0.0
	}
	if s.CPUStats.CPUUsage.TotalUsage < s.PreCPUStats.CPUUsage.TotalUsage ||
		s.CPUStats.SystemUsage < s.PreCPUStats.SystemUsage {
		return 0.0
	}
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage - s.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(s.CPUStats.SystemUsage - s.PreCPUStats.SystemUsage)
	if systemDelta <= 0.0 || cpuDelta <= 0.0 {
		return 0.0
	}

	onlineCPUs := float64(s.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(s.CPUStats.CPUUsage.PercpuUsage))
	}
	if onlineCPUs == 0 {
		onlineCPUs = 1.0
	}

	return (cpuDelta / systemDelta) * onlineCPUs * 100.0
}

// CalculateMemoryUsage calculates active container memory usage (excluding reclaimable page cache)
// and usage percentage against memory limit.
func CalculateMemoryUsage(s *container.StatsResponse) (used uint64, limit uint64, percent float64) {
	if s == nil {
		return 0, 0, 0
	}
	limit = s.MemoryStats.Limit
	rawUsage := s.MemoryStats.Usage

	// Adjust for page cache following Docker CLI logic (cgroup v2 inactive_file, cgroup v1 total_inactive_file/cache).
	var cache uint64
	if s.MemoryStats.Stats != nil {
		if v, ok := s.MemoryStats.Stats["inactive_file"]; ok {
			cache = v
		} else if v, ok := s.MemoryStats.Stats["total_inactive_file"]; ok {
			cache = v
		} else if v, ok := s.MemoryStats.Stats["cache"]; ok {
			cache = v
		}
	}

	if rawUsage > cache {
		used = rawUsage - cache
	} else {
		used = 0
	}

	if limit > 0 {
		percent = (float64(used) / float64(limit)) * 100.0
	}
	return used, limit, percent
}

// CalculateNetworkIO sums received and transmitted bytes across all container interfaces.
func CalculateNetworkIO(s *container.StatsResponse) (rx uint64, tx uint64) {
	if s == nil {
		return 0, 0
	}
	for _, netStats := range s.Networks {
		rx += netStats.RxBytes
		tx += netStats.TxBytes
	}
	return rx, tx
}

// CalculateBlockIO sums block read and write bytes from BlkioStats.
func CalculateBlockIO(s *container.StatsResponse) (readBytes uint64, writeBytes uint64) {
	if s == nil {
		return 0, 0
	}
	for _, entry := range s.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(entry.Op) {
		case "read":
			readBytes += entry.Value
		case "write":
			writeBytes += entry.Value
		}
	}
	return readBytes, writeBytes
}

// ClassifyDockerError categorizes common Docker daemon connection errors and returns safe troubleshooting guidance.
func ClassifyDockerError(err error) (kind string, hint string) {
	if err == nil {
		return "", ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "permission denied"):
		return "permission", "Your user lacks access to the Docker socket (/var/run/docker.sock). Grant access only to trusted users via the 'docker' group (note: Docker socket access is equivalent to root privileges)."
	case strings.Contains(msg, "no such file") || strings.Contains(msg, "cannot connect") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "not reachable"):
		return "not_found", "Ensure the Docker engine is installed and running (for example: 'systemctl status docker')."
	case strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timeout"):
		return "timeout", "Docker daemon timed out responding; check daemon health or host load."
	default:
		return "other", "Verify Docker daemon status and DOCKER_HOST environment configuration."
	}
}
