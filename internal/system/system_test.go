package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiskStatusFor(t *testing.T) {
	tests := []struct {
		pct  float64
		warn float64
		want string
	}{
		{45.0, 85.0, "OK"},
		{85.0, 85.0, "WARNING"},
		{90.0, 85.0, "CRITICAL"},
		{96.0, 92.0, "CRITICAL"},
	}
	for _, tc := range tests {
		if got := DiskStatusFor(tc.pct, tc.warn); got != tc.want {
			t.Errorf("DiskStatusFor(%.1f, %.1f) = %q, want %q", tc.pct, tc.warn, got, tc.want)
		}
	}
}

func TestGetSystemStatus(t *testing.T) {
	status, err := GetSystemStatus(context.Background())
	if err != nil {
		t.Fatalf("GetSystemStatus failed: %v", err)
	}
	if status.MemoryTotalBytes == 0 {
		t.Error("expected non-zero total memory bytes")
	}
	if status.MemoryPercent < 0 || status.MemoryPercent > 100 {
		t.Errorf("invalid memory percent: %.2f", status.MemoryPercent)
	}
	if status.CPUPercent < 0 || status.CPUPercent > 100 {
		t.Errorf("invalid cpu percent: %.2f", status.CPUPercent)
	}
	if status.DiskTotalGB <= 0 {
		t.Error("expected positive root disk capacity")
	}
}

func TestGetDiskPartitions(t *testing.T) {
	parts, _, err := GetDiskPartitions(context.Background(), false, 85.0)
	if err != nil {
		t.Fatalf("GetDiskPartitions failed: %v", err)
	}
	if len(parts) == 0 {
		t.Fatal("expected at least one mounted partition")
	}
	for _, p := range parts {
		if p.Mountpoint == "" || p.TotalBytes == 0 {
			t.Errorf("unexpected empty partition stats: %+v", p)
		}
	}
}

func TestProcessSampler(t *testing.T) {
	sampler := NewProcessSampler()
	procs, err := sampler.Sample(context.Background(), 5, false)
	if err != nil {
		t.Fatalf("Sample failed: %v", err)
	}
	if len(procs) == 0 {
		t.Fatal("expected at least one running process")
	}
	if len(procs) > 5 {
		t.Errorf("expected at most 5 processes, got %d", len(procs))
	}
}

func TestFindLargeDirs(t *testing.T) {
	root := t.TempDir()
	smallDir := filepath.Join(root, "small")
	largeDir := filepath.Join(root, "large")
	nestedDir := filepath.Join(largeDir, "nested")

	if err := os.MkdirAll(smallDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(smallDir, "a.bin"), make([]byte, 100), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nestedDir, "b.bin"), make([]byte, 4096), 0644); err != nil {
		t.Fatal(err)
	}

	dirs, _, err := FindLargeDirs(context.Background(), root, 2, 10)
	if err != nil {
		t.Fatalf("FindLargeDirs failed: %v", err)
	}
	if len(dirs) < 2 {
		t.Fatalf("expected at least 2 dirs, got %d", len(dirs))
	}
	if dirs[0].Path != largeDir || dirs[0].SizeBytes != 4096 {
		t.Errorf("expected top dir %s (4096 bytes), got %+v", largeDir, dirs[0])
	}
}
