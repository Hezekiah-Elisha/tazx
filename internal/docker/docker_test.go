package docker

import (
	"errors"
	"math"
	"testing"

	"github.com/moby/moby/api/types/container"
)

func TestCalculateCPUPercent(t *testing.T) {
	stats := &container.StatsResponse{
		CPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 200_000_000,
			},
			SystemUsage: 1_000_000_000,
			OnlineCPUs:  2,
		},
		PreCPUStats: container.CPUStats{
			CPUUsage: container.CPUUsage{
				TotalUsage: 100_000_000,
			},
			SystemUsage: 500_000_000,
		},
	}

	got := CalculateCPUPercent(stats)
	// (100M / 500M) * 2 * 100 = 40.0%
	if math.Abs(got-40.0) > 0.001 {
		t.Errorf("CalculateCPUPercent = %.4f, want 40.0", got)
	}

	if got := CalculateCPUPercent(nil); got != 0 {
		t.Errorf("expected 0 for nil stats, got %.2f", got)
	}
}

func TestCalculateMemoryUsage(t *testing.T) {
	stats := &container.StatsResponse{
		MemoryStats: container.MemoryStats{
			Usage: 600 * 1024 * 1024,
			Limit: 1000 * 1024 * 1024,
			Stats: map[string]uint64{
				"inactive_file": 100 * 1024 * 1024,
			},
		},
	}

	used, limit, pct := CalculateMemoryUsage(stats)
	if used != 500*1024*1024 {
		t.Errorf("used = %d, want %d", used, 500*1024*1024)
	}
	if limit != 1000*1024*1024 {
		t.Errorf("limit = %d, want %d", limit, 1000*1024*1024)
	}
	if math.Abs(pct-50.0) > 0.001 {
		t.Errorf("pct = %.4f, want 50.0", pct)
	}
}

func TestCalculateNetworkAndBlockIO(t *testing.T) {
	stats := &container.StatsResponse{
		Networks: map[string]container.NetworkStats{
			"eth0": {RxBytes: 1000, TxBytes: 500},
			"eth1": {RxBytes: 250, TxBytes: 100},
		},
		BlkioStats: container.BlkioStats{
			IoServiceBytesRecursive: []container.BlkioStatEntry{
				{Op: "read", Value: 4096},
				{Op: "Write", Value: 8192},
				{Op: "Sync", Value: 1024},
			},
		},
	}

	rx, tx := CalculateNetworkIO(stats)
	if rx != 1250 || tx != 600 {
		t.Errorf("Net IO = (%d, %d), want (1250, 600)", rx, tx)
	}

	r, w := CalculateBlockIO(stats)
	if r != 4096 || w != 8192 {
		t.Errorf("Block IO = (%d, %d), want (4096, 8192)", r, w)
	}
}

func TestClassifyDockerError(t *testing.T) {
	kind, hint := ClassifyDockerError(errors.New("permission denied while trying to connect to /var/run/docker.sock"))
	if kind != "permission" || hint == "" {
		t.Errorf("unexpected permission classification: %s / %s", kind, hint)
	}

	kind, _ = ClassifyDockerError(errors.New("Cannot connect to the Docker daemon: no such file or directory"))
	if kind != "not_found" {
		t.Errorf("expected not_found, got %s", kind)
	}
}
